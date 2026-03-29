# Research: Metrics Taxonomy Infrastructure

**Date**: 2026-03-29  
**Focus**: Existing observability infrastructure, metric patterns, and instrumentation points

## Executive Summary

The codebase has a well-established observability foundation from spec 013. All metrics use Prometheus' `client_golang` library with centralized registration in `pkg/observability/`. The existing patterns are:

- **Centralized registration**: All metrics defined in `pkg/observability/metrics.go`, registered in `init()`
- **Distributed recording**: Metrics recorded at observation points throughout engine, transport, provider layers
- **LLM-optimized buckets**: Existing `LLMBuckets` (0.1s to 120s) used for all duration histograms
- **No external dependencies**: Reuses existing Prometheus integration; no new libraries needed
- **HTTP middleware**: All HTTP-level metrics captured by `MetricsMiddleware` in `pkg/observability/middleware.go`

This research confirms the proposed 23-metric expansion follows existing patterns and requires no architectural changes.

---

## Finding 1: Existing Observability Package Structure

**File**: `/Users/rhuss/Development/ai/antwort-046-metrics-taxonomy/pkg/observability/metrics.go`

The observability package contains:

### Metric Definitions (12 existing metrics)

1. **HTTP Metrics**:
   - `antwort_requests_total` (CounterVec): method, status, model
   - `antwort_request_duration_seconds` (HistogramVec): method, model
   - `antwort_streaming_connections_active` (Gauge): [no labels]

2. **Provider Metrics**:
   - `antwort_provider_requests_total` (CounterVec): provider, model, status
   - `antwort_provider_latency_seconds` (HistogramVec): provider, model
   - `antwort_provider_tokens_total` (CounterVec): provider, model, direction

3. **Tool Metrics**:
   - `antwort_tool_executions_total` (CounterVec): tool_name, status

4. **Rate Limiting Metrics**:
   - `antwort_ratelimit_rejected_total` (CounterVec): tier

5. **OTel GenAI Semantic Conventions** (4 metrics):
   - `gen_ai_client_token_usage` (HistogramVec): operation_name, provider, token_type, request_model, response_model
   - `gen_ai_client_operation_duration_seconds` (HistogramVec): operation_name, provider, request_model, response_model, error_type
   - `gen_ai_server_time_to_first_token_seconds` (HistogramVec): operation_name, provider, request_model
   - `gen_ai_server_time_per_output_token_seconds` (HistogramVec): operation_name, provider, request_model

### Bucket Configurations

```go
LLMBuckets = []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120}  // Duration in seconds
TokenBuckets = []float64{1, 4, 16, 64, 256, 1024, 4096, 16384}  // Powers of 4 for token counts
```

### Registration Pattern

All metrics registered in `init()` using `prometheus.MustRegister()`:

```go
func init() {
    prometheus.MustRegister(
        RequestsTotal,
        RequestDuration,
        StreamingConnections,
        ProviderRequestsTotal,
        ProviderLatency,
        ProviderTokensTotal,
        ToolExecutionsTotal,
        RateLimitRejectedTotal,
        GenAIClientTokenUsage,
        GenAIClientOperationDuration,
        GenAIServerTimeToFirstToken,
        GenAIServerTimePerOutputToken,
    )
}
```

### Helper Functions

- `RecordGenAIMetrics()`: Bulk recording of OTel gen_ai.* metrics for a single provider interaction

**Decision**: The new 23 metrics should follow this exact pattern - define all in `metrics.go`, register in `init()`, use existing bucket configurations.

---

## Finding 2: HTTP Middleware and Request Metrics

**File**: `/Users/rhuss/Development/ai/antwort-046-metrics-taxonomy/pkg/observability/middleware.go`

The `MetricsMiddleware` wraps HTTP handlers to record:
- `antwort_requests_total`: Per-request counter with method, status class (2xx/4xx/5xx), model
- `antwort_request_duration_seconds`: Histogram of request duration
- `antwort_streaming_connections_active`: Gauge tracking active SSE streaming connections

**Key Implementation Details**:
- Wraps `http.Handler` returning `http.HandlerFunc`
- Uses `statusWriter` wrapper to capture HTTP status code
- Detects SSE streaming from `Accept: text/event-stream` header
- Increments/decrements gauge for streaming connections
- Records both request duration and status class

**Decision**: New response-layer metrics (responses_total, responses_duration_seconds, responses_active, responses_chained_total, responses_tokens_total) should be recorded at the HTTP adapter level (in `pkg/transport/http/adapter.go`) following similar patterns.

---

## Finding 3: Engine and Provider Instrumentation

**Files**: `/Users/rhuss/Development/ai/antwort-046-metrics-taxonomy/pkg/engine/loop.go`

The agentic loop records provider and tool metrics:

### Provider Metrics (in `runAgenticLoop`, lines 45-63)

```go
startTime := time.Now()
provResp, err := e.provider.Complete(ctx, provReq)
duration := time.Since(startTime)
provName := e.provider.Name()

if err != nil {
    observability.ProviderRequestsTotal.WithLabelValues(provName, req.Model, "error").Inc()
    observability.ProviderLatency.WithLabelValues(provName, req.Model).Observe(duration.Seconds())
    // ... error handling
} else {
    observability.ProviderRequestsTotal.WithLabelValues(provName, req.Model, "success").Inc()
    observability.ProviderLatency.WithLabelValues(provName, req.Model).Observe(duration.Seconds())
    observability.ProviderTokensTotal.WithLabelValues(provName, req.Model, "input").Add(float64(provResp.Usage.InputTokens))
    observability.ProviderTokensTotal.WithLabelValues(provName, req.Model, "output").Add(float64(provResp.Usage.OutputTokens))
    observability.RecordGenAIMetrics(provName, req.Model, duration, ...)
}
```

### Tool Execution Metrics (in `executeToolsConcurrently/executeToolsSequentially`, lines 554, 571, 580, etc.)

```go
observability.ToolExecutionsTotal.WithLabelValues(tc.Name, "error").Inc()  // When executor not found
observability.ToolExecutionsTotal.WithLabelValues(tc.Name, status).Inc()    // After execution (status: success/error)
```

**Key Observations**:
- Metrics recorded at natural observation points (after provider call, after tool execution)
- Uses `time.Now()` and `time.Since()` for duration measurement
- Status values: "error" (explicit failures), "success", or derived from result
- No overhead-minimizing tricks; straightforward synchronous recording

**Decision**: New engine metrics (iterations_total, iteration_duration_seconds, tool_duration_seconds, conversation_depth, max_iterations_hit_total) should follow this pattern. Iteration counting happens in the `for turn := 0; turn < maxTurns; turn++` loop. Tool duration is already being measured for tools; we just need to record it separately. Conversation depth requires counting items in rehydrated conversations before the final response.

---

## Finding 4: Storage Layer Architecture

**Files**: 
- `/Users/rhuss/Development/ai/antwort-046-metrics-taxonomy/pkg/storage/doc.go`
- `/Users/rhuss/Development/ai/antwort-046-metrics-taxonomy/pkg/storage/memory/` (multiple files)
- `/Users/rhuss/Development/ai/antwort-046-metrics-taxonomy/pkg/storage/postgres/` (multiple files)

### Interface

Storage implementations implement `transport.ResponseStore` interface with methods:
- `Get(ctx context.Context, id string) (*api.Response, error)`
- `Save(ctx context.Context, resp *api.Response) error`
- `Delete(ctx context.Context, id string) error`
- `List(ctx context.Context, ...) ([]api.Response, error)`

### Implementations

1. **Memory Store** (`pkg/storage/memory/store.go`):
   - In-memory map-based storage
   - Simple, fast for testing
   - No persistence

2. **PostgreSQL Store** (`pkg/storage/postgres/store.go`):
   - Connection pooling via `pgx`
   - Schema migrations
   - Supports multiple tenants via schema partitioning

**Decision**: Storage metrics require wrapping the store implementations. Two approaches:
1. **Decorator pattern**: Create `MetricsStore` wrapper that delegates to underlying store and records metrics
2. **Direct instrumentation**: Add metrics recording directly in store methods

Option 1 is preferable (non-invasive, testable), but requires new wrapper type. Given the existing codebase uses direct instrumentation, we'll use **Option 2**: add metrics recording to existing store implementations.

**Instrumentation Points**:
- `Get`: Record `storage_operations_total` (backend, get, success/error) and `storage_operation_duration_seconds`
- `Save`: Record `storage_operations_total` (backend, save, success/error), duration, and `storage_responses_stored` gauge increment
- `Delete`: Record `storage_operations_total` (backend, delete, success/error), duration, and `storage_responses_stored` gauge decrement
- `List`: Record `storage_operations_total` (backend, list, success/error) and duration

### PostgreSQL Specific

- **Connection Pool Gauge**: Record active connections from `pgx.Pool.Stat()` - can be done in a background goroutine or on-demand in Get/Save/Delete methods

---

## Finding 5: Background Worker Architecture

**File**: `/Users/rhuss/Development/ai/antwort-046-metrics-taxonomy/pkg/engine/background.go`

### Worker Structure

```go
type Worker struct {
    engine   *Engine
    workerID string
    cfg      config.BackgroundConfig
    cancel   context.CancelFunc
    wg       sync.WaitGroup
    mu       sync.RWMutex
    cancelRegistry map[string]context.CancelFunc  // In-flight request tracking
}
```

### Key Methods

1. **`Start(ctx context.Context)`** (lines 44-64):
   - Runs a poll loop with ticker interval
   - Calls `pollOnce(ctx)` repeatedly
   - Logs startup and shutdown messages

2. **`pollOnce(ctx context.Context)`** (likely around line 100+):
   - Queries queue for unclaimed responses
   - Attempts to claim responses with worker heartbeat
   - Processes claimed responses through engine
   - Detects and reclaims stale responses

3. **`Stop()`** (lines 68-87):
   - Graceful shutdown with drain timeout
   - Waits for in-flight requests to complete

**Instrumentation Points**:
- **`antwort_background_queued`** (gauge): Current queue depth (read from storage query result count)
- **`antwort_background_claimed_total`** (counter): Increment when response claimed successfully
- **`antwort_background_stale_total`** (counter): Increment when stale response detected and reclaimed
- **`antwort_background_worker_heartbeat_age_seconds`** (gauge): Heartbeat freshness (timestamp diff)

---

## Finding 6: Files API and Ingestion Pipeline

**Files**:
- `/Users/rhuss/Development/ai/antwort-046-metrics-taxonomy/pkg/files/api.go`
- `/Users/rhuss/Development/ai/antwort-046-metrics-taxonomy/pkg/files/pipeline.go`

### Files API Handler

Located in `api.go`:
- Handles file upload endpoint
- Stores file in file store
- Triggers ingestion pipeline
- Returns file metadata

### Ingestion Pipeline

Located in `pipeline.go`:
- Orchestrates: extract → chunk → embed → store
- Calls extractors (content extraction)
- Calls chunker (document segmentation)
- Calls embedding model (vector generation)
- Stores vectors in vector store backend

**Instrumentation Points**:
- **`antwort_files_uploaded_total`** (counter): Increment on successful upload with content_type label
- **`antwort_files_ingestion_duration_seconds`** (histogram): Record total ingestion pipeline duration (extract + chunk + embed + store)

---

## Finding 7: Vector Store and File Search

**Files**:
- `/Users/rhuss/Development/ai/antwort-046-metrics-taxonomy/pkg/tools/builtins/filesearch/api.go`
- `/Users/rhuss/Development/ai/antwort-046-metrics-taxonomy/pkg/tools/builtins/filesearch/provider.go`

### File Search API

Located in `api.go`:
- Handles `file_search` tool calls
- Performs semantic search in vector store
- Returns search results

### Vector Store Backend

Located in `provider.go`:
- Implements embedding and search operations
- Manages vector store connections
- Supports multiple vector store backends (e.g., Qdrant)

**Instrumentation Points**:
- **`antwort_vectorstore_searches_total`** (counter): Increment on search completion with store_id and result (success/error) labels
- **`antwort_vectorstore_search_duration_seconds`** (histogram): Record search latency
- **`antwort_vectorstore_items_stored`** (gauge): Track current item count per vector store (store_id label)

---

## Finding 8: Server Wiring and Metrics Endpoint

**File**: `/Users/rhuss/Development/ai/antwort-046-metrics-taxonomy/cmd/server/main.go`

### Metrics Endpoint Setup (lines 184-188)

```go
if cfg.Observability.Metrics.Enabled {
    metricsPath := cfg.Observability.Metrics.Path
    mux.Handle("GET "+metricsPath, promhttp.Handler())
    slog.Info("metrics endpoint enabled", "path", metricsPath)
}
```

### Middleware Chain (lines 191-213)

```go
var handler http.Handler = corsMiddleware(mux)

if cfg.Observability.Metrics.Enabled {
    handler = observability.MetricsMiddleware(handler)
}

// ... other middleware (scope, auth)
handler = scopeMiddleware(handler)
handler = authMiddleware(handler)
```

**Key Decisions**:
- Metrics middleware applied BEFORE auth (all requests counted)
- CORS middleware outermost
- Metrics endpoint registered separately (bypasses all auth middleware)
- Configurable path and enabled flag

**Decision**: No changes needed to server wiring. All new metrics will be automatically exposed at the `/metrics` endpoint (or configured path) via `promhttp.Handler()`.

---

## Summary of Instrumentation Patterns

The codebase follows these consistent patterns:

1. **Metric Definition**: Defined as package-level variables in `pkg/observability/metrics.go`
2. **Registration**: All metrics registered in `init()` function with `prometheus.MustRegister()`
3. **Recording**: Direct synchronous recording at observation points using `.WithLabelValues(...).Inc/Observe()`
4. **Duration Measurement**: `time.Now()` at start, `time.Since(start).Seconds()` for observation
5. **Error Handling**: Status labels distinguish success from error (not exception throwing)
6. **Buckets**: Reuse existing `LLMBuckets` for durations; `TokenBuckets` for counts
7. **Middleware**: HTTP-level metrics captured by `MetricsMiddleware` wrapper

**Decision**: All new metrics should follow these exact patterns. No architectural innovation needed.

---

## Open Questions Resolved

**Q1: Should metrics always be enabled or gated by feature flags?**
- **Answer**: Always registered at startup. No feature flags in code. Operators disable via configuration (`observability.metrics.enabled: false`). Unused metrics emit zero values (standard Prometheus behavior).

**Q2: How to handle cardinality for `model` and `tool_name` labels?**
- **Answer**: Use values as-is in code. Operators control cardinality via Prometheus relabeling (e.g., `metric_relabel_configs`). Document the risk in config reference.

**Q3: Should there be a wrapper/decorator pattern for storage metrics?**
- **Answer**: Direct instrumentation is simpler and matches existing patterns. Add metrics recording to memory and PostgreSQL store methods.

**Q4: How to measure conversation depth?**
- **Answer**: Count items in rehydrated conversation before building final response in `runAgenticLoop` / `runAgenticLoopStreaming`.

**Q5: How to track background queue depth?**
- **Answer**: Query result from `pollOnce()` shows queued response count. Record as gauge snapshot per poll cycle.

---

## Conclusion

All technical unknowns are resolved. The infrastructure exists to support 23 new metrics following established patterns. Ready for Phase 1 design and Phase 2 implementation.
