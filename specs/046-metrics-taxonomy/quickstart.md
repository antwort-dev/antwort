# Quickstart: Integrating the Metrics Taxonomy

**Date**: 2026-03-29  
**Audience**: Developers implementing the metrics taxonomy expansion

## Overview

This guide walks through the process of adding the 23 new metrics to Antwort's observability infrastructure. The implementation follows the existing spec 013 patterns and requires no external dependencies.

---

## Prerequisites

- Existing Prometheus setup from spec 013
- Access to `/metrics` endpoint (verify: `curl http://localhost:8080/metrics`)
- Go 1.23+ development environment

---

## Phase 1: Define Metrics in `pkg/observability/metrics.go`

### Step 1.1: Add Responses Layer Metrics

```go
// Responses Layer metrics
var (
    ResponsesTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "antwort_responses_total",
            Help: "Total responses by model, status, and mode",
        },
        []string{"model", "status", "mode"},
    )
    
    ResponsesDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "antwort_responses_duration_seconds",
            Help:    "Response duration by model and mode",
            Buckets: LLMBuckets,
        },
        []string{"model", "mode"},
    )
    
    ResponsesActive = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "antwort_responses_active",
            Help: "Active in-flight responses by mode",
        },
        []string{"mode"},
    )
    
    ResponsesChainedTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "antwort_responses_chained_total",
            Help: "Responses using previous_response_id (conversation chaining)",
        },
        []string{"model"},
    )
    
    ResponsesTokensTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "antwort_responses_tokens_total",
            Help: "Token usage by model and direction",
        },
        []string{"model", "type"},
    )
)
```

### Step 1.2: Add Engine Layer Metrics

```go
var (
    EngineIterationsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "antwort_engine_iterations_total",
            Help: "Total agentic loop iterations by model",
        },
        []string{"model"},
    )
    
    EngineIterationDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "antwort_engine_iteration_duration_seconds",
            Help:    "Iteration duration by model",
            Buckets: LLMBuckets,
        },
        []string{"model"},
    )
    
    EngineMaxIterationsHit = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "antwort_engine_max_iterations_hit_total",
            Help: "Responses hitting max iterations limit",
        },
        []string{"model"},
    )
    
    EngineToolDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "antwort_engine_tool_duration_seconds",
            Help:    "Tool execution duration by tool name",
            Buckets: LLMBuckets,
        },
        []string{"tool_name"},
    )
    
    EngineConversationDepth = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "antwort_engine_conversation_depth",
            Help:    "Conversation item count by model",
            Buckets: []float64{1, 2, 5, 10, 20, 50}, // Conversation length buckets
        },
        []string{"model"},
    )
)
```

### Step 1.3: Add Storage Layer Metrics

```go
var (
    StorageOperationsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "antwort_storage_operations_total",
            Help: "Storage operations by backend, operation type, and result",
        },
        []string{"backend", "operation", "result"},
    )
    
    StorageOperationDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "antwort_storage_operation_duration_seconds",
            Help:    "Storage operation duration by backend and operation",
            Buckets: LLMBuckets,
        },
        []string{"backend", "operation"},
    )
    
    StorageResponsesStored = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "antwort_storage_responses_stored",
            Help: "Current response count in storage by backend",
        },
        []string{"backend"},
    )
    
    StorageConnectionsActive = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "antwort_storage_connections_active",
            Help: "Active PostgreSQL connection pool connections",
        },
    )
)
```

### Step 1.4: Add Files/Vector Store Metrics

```go
var (
    FilesUploadedTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "antwort_files_uploaded_total",
            Help: "Files uploaded by content type",
        },
        []string{"content_type"},
    )
    
    FilesIngestionDuration = prometheus.NewHistogram(
        prometheus.HistogramOpts{
            Name:    "antwort_files_ingestion_duration_seconds",
            Help:    "File ingestion pipeline duration",
            Buckets: LLMBuckets,
        },
    )
    
    VectorstoreSearchesTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "antwort_vectorstore_searches_total",
            Help: "Vector store searches by store and result",
        },
        []string{"store_id", "result"},
    )
    
    VectorstoreSearchDuration = prometheus.NewHistogram(
        prometheus.HistogramOpts{
            Name:    "antwort_vectorstore_search_duration_seconds",
            Help:    "Vector store search latency",
            Buckets: LLMBuckets,
        },
    )
    
    VectorstoreItemsStored = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "antwort_vectorstore_items_stored",
            Help: "Item count per vector store",
        },
        []string{"store_id"},
    )
)
```

### Step 1.5: Add Background Worker Metrics

```go
var (
    BackgroundQueued = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "antwort_background_queued",
            Help: "Background response queue depth",
        },
    )
    
    BackgroundClaimedTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "antwort_background_claimed_total",
            Help: "Responses claimed by worker",
        },
        []string{"worker_id"},
    )
    
    BackgroundStaleTotal = prometheus.NewCounter(
        prometheus.CounterOpts{
            Name: "antwort_background_stale_total",
            Help: "Stale responses detected and reclaimed",
        },
    )
    
    BackgroundWorkerHeartbeatAge = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "antwort_background_worker_heartbeat_age_seconds",
            Help: "Time since last worker heartbeat",
        },
        []string{"worker_id"},
    )
)
```

### Step 1.6: Update `init()` to Register All Metrics

```go
func init() {
    prometheus.MustRegister(
        // Existing spec 013 metrics
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
        
        // New Responses Layer
        ResponsesTotal,
        ResponsesDuration,
        ResponsesActive,
        ResponsesChainedTotal,
        ResponsesTokensTotal,
        
        // New Engine Layer
        EngineIterationsTotal,
        EngineIterationDuration,
        EngineMaxIterationsHit,
        EngineToolDuration,
        EngineConversationDepth,
        
        // New Storage Layer
        StorageOperationsTotal,
        StorageOperationDuration,
        StorageResponsesStored,
        StorageConnectionsActive,
        
        // New Files/Vector Store Layer
        FilesUploadedTotal,
        FilesIngestionDuration,
        VectorstoreSearchesTotal,
        VectorstoreSearchDuration,
        VectorstoreItemsStored,
        
        // New Background Layer
        BackgroundQueued,
        BackgroundClaimedTotal,
        BackgroundStaleTotal,
        BackgroundWorkerHeartbeatAge,
    )
}
```

---

## Phase 2: Instrument Recording Points

### Step 2.1: Response Metrics (in `pkg/transport/http/adapter.go`)

In the `handleCreateResponse()` handler, at response completion:

```go
func (a *Adapter) handleCreateResponse(w http.ResponseWriter, r *http.Request) {
    // ... extract mode from request (sync/streaming/background)
    startTime := time.Now()
    
    // Increment active gauge
    observability.ResponsesActive.WithLabelValues(mode).Inc()
    defer observability.ResponsesActive.WithLabelValues(mode).Dec()
    
    // ... process response ...
    
    duration := time.Since(startTime).Seconds()
    observability.ResponsesDuration.WithLabelValues(model, mode).Observe(duration)
    observability.ResponsesTotal.WithLabelValues(model, status, mode).Inc()
    observability.ResponsesTokensTotal.WithLabelValues(model, "input").Add(float64(usage.InputTokens))
    observability.ResponsesTokensTotal.WithLabelValues(model, "output").Add(float64(usage.OutputTokens))
    
    // If chained response
    if req.PreviousResponseID != "" {
        observability.ResponsesChainedTotal.WithLabelValues(model).Inc()
    }
}
```

### Step 2.2: Engine Metrics (in `pkg/engine/loop.go`)

In the agentic loop main loop:

```go
for turn := 0; turn < maxTurns; turn++ {
    turnStart := time.Now()
    observability.EngineIterationsTotal.WithLabelValues(req.Model).Inc()
    
    // ... provider call ...
    
    // Record iteration duration
    observability.EngineIterationDuration.WithLabelValues(req.Model).Observe(time.Since(turnStart).Seconds())
    
    // ... tool execution ...
}

// Check if max iterations hit
if turn == maxTurns {
    observability.EngineMaxIterationsHit.WithLabelValues(req.Model).Inc()
}

// Record conversation depth before final response
depth := len(rehydratedConversation)
observability.EngineConversationDepth.WithLabelValues(req.Model).Observe(float64(depth))
```

In tool execution methods:

```go
func (e *Engine) executeToolsConcurrently(ctx context.Context, calls []tools.ToolCall) []tools.ToolResult {
    // ... for each tool ...
    toolStart := time.Now()
    result, err := exec.Execute(ctx, tc)
    observability.EngineToolDuration.WithLabelValues(tc.Name).Observe(time.Since(toolStart).Seconds())
    // ...
}
```

### Step 2.3: Storage Metrics (in `pkg/storage/memory/store.go` and `pkg/storage/postgres/store.go`)

```go
func (s *MemoryStore) Get(ctx context.Context, id string) (*api.Response, error) {
    start := time.Now()
    s.mu.RLock()
    resp, ok := s.responses[id]
    s.mu.RUnlock()
    
    duration := time.Since(start).Seconds()
    if ok {
        observability.StorageOperationsTotal.WithLabelValues("memory", "get", "success").Inc()
    } else {
        observability.StorageOperationsTotal.WithLabelValues("memory", "get", "error").Inc()
    }
    observability.StorageOperationDuration.WithLabelValues("memory", "get").Observe(duration)
    
    if !ok {
        return nil, storage.ErrNotFound
    }
    return resp, nil
}

func (s *MemoryStore) Save(ctx context.Context, resp *api.Response) error {
    start := time.Now()
    s.mu.Lock()
    s.responses[resp.ID] = resp
    count := len(s.responses)
    s.mu.Unlock()
    
    duration := time.Since(start).Seconds()
    observability.StorageOperationsTotal.WithLabelValues("memory", "save", "success").Inc()
    observability.StorageOperationDuration.WithLabelValues("memory", "save").Observe(duration)
    observability.StorageResponsesStored.WithLabelValues("memory").Set(float64(count))
    
    return nil
}

func (s *MemoryStore) Delete(ctx context.Context, id string) error {
    start := time.Now()
    s.mu.Lock()
    delete(s.responses, id)
    count := len(s.responses)
    s.mu.Unlock()
    
    duration := time.Since(start).Seconds()
    observability.StorageOperationsTotal.WithLabelValues("memory", "delete", "success").Inc()
    observability.StorageOperationDuration.WithLabelValues("memory", "delete").Observe(duration)
    observability.StorageResponsesStored.WithLabelValues("memory").Set(float64(count))
    
    return nil
}
```

Similar patterns apply to PostgreSQL store.

### Step 2.4: Files Metrics (in `pkg/files/api.go` and `pkg/files/pipeline.go`)

```go
func (p *FilesProvider) Upload(ctx context.Context, req *api.UploadRequest) (*api.UploadResponse, error) {
    observability.FilesUploadedTotal.WithLabelValues(req.ContentType).Inc()
    
    pipelineStart := time.Now()
    err := p.pipeline.Process(ctx, file, vectorStore)
    observability.FilesIngestionDuration.Observe(time.Since(pipelineStart).Seconds())
    
    // ...
}
```

### Step 2.5: Vector Store Metrics (in `pkg/tools/builtins/filesearch/api.go`)

```go
func (p *FileSearchProvider) Execute(ctx context.Context, call tools.ToolCall) (*tools.ToolResult, error) {
    searchStart := time.Now()
    results, err := p.backend.Search(ctx, query, vectorStoreID)
    duration := time.Since(searchStart).Seconds()
    
    if err != nil {
        observability.VectorstoreSearchesTotal.WithLabelValues(vectorStoreID, "error").Inc()
        return nil, err
    }
    
    observability.VectorstoreSearchesTotal.WithLabelValues(vectorStoreID, "success").Inc()
    observability.VectorstoreSearchDuration.Observe(duration)
    
    // ...
}
```

### Step 2.6: Background Metrics (in `pkg/engine/background.go`)

```go
func (w *Worker) pollOnce(ctx context.Context) {
    // Query queue depth
    queued, err := w.engine.store.QueryQueueDepth(ctx)
    if err == nil {
        observability.BackgroundQueued.Set(float64(queued))
    }
    
    // Claim responses
    claimed, err := w.engine.store.ClaimResponses(ctx, w.workerID, 10)
    for _, resp := range claimed {
        observability.BackgroundClaimedTotal.WithLabelValues(w.workerID).Inc()
        // ... process ...
    }
    
    // Detect stale
    stale, err := w.engine.store.DetectStaleResponses(ctx)
    if err == nil {
        if len(stale) > 0 {
            observability.BackgroundStaleTotal.Add(float64(len(stale)))
        }
    }
    
    // Update heartbeat age
    age := time.Since(w.lastHeartbeat).Seconds()
    observability.BackgroundWorkerHeartbeatAge.WithLabelValues(w.workerID).Set(age)
}
```

---

## Phase 3: Testing

### Unit Test Example (in `pkg/observability/metrics_test.go`)

```go
func TestMetricsRegistration(t *testing.T) {
    // Verify all 35 metrics are registered (12 from spec 013 + 23 new)
    require.NotNil(t, observability.ResponsesTotal)
    require.NotNil(t, observability.EngineIterationsTotal)
    require.NotNil(t, observability.StorageOperationsTotal)
    require.NotNil(t, observability.FilesUploadedTotal)
    require.NotNil(t, observability.BackgroundQueued)
    // ... more assertions
}

func TestResponseMetricsRecording(t *testing.T) {
    observability.ResponsesTotal.WithLabelValues("gpt-4", "completed", "sync").Inc()
    observability.ResponsesDuration.WithLabelValues("gpt-4", "sync").Observe(1.5)
    
    // Verify metrics are recorded (via scrape or registry inspection)
}
```

### Integration Test Example

```go
func TestMetricsEndpoint(t *testing.T) {
    // Start server with metrics enabled
    // Send test request
    // Scrape /metrics endpoint
    // Verify metrics are present
    resp, err := http.Get("http://localhost:8080/metrics")
    require.NoError(t, err)
    defer resp.Body.Close()
    
    body, _ := io.ReadAll(resp.Body)
    require.Contains(t, string(body), "antwort_responses_total")
    require.Contains(t, string(body), "antwort_engine_iterations_total")
    require.Contains(t, string(body), "antwort_storage_operations_total")
}
```

---

## Verification Checklist

- [ ] All 23 new metrics defined in `metrics.go`
- [ ] All metrics registered in `init()`
- [ ] Response metrics recorded in HTTP adapter
- [ ] Engine metrics recorded in loop and tool execution
- [ ] Storage metrics recorded in memory and PostgreSQL stores
- [ ] Files and vector store metrics recorded
- [ ] Background worker metrics recorded
- [ ] Unit tests pass (metric registration)
- [ ] Integration tests pass (metric recording)
- [ ] `/metrics` endpoint exposes all new metrics
- [ ] No breaking changes to spec 013 metrics
- [ ] Cardinality documentation added to config reference

---

## Troubleshooting

### Metrics not appearing in `/metrics`

- Verify metrics are registered in `init()`
- Check `Observability.Metrics.Enabled` in config
- Verify metrics endpoint is accessible: `curl http://localhost:8080/metrics`

### Label cardinality issues

- Review labels with high cardinality (`model`, `tool_name`)
- Use Prometheus relabeling to group/drop unwanted labels
- Document cardinality risks in operator runbook

### Missing observations

- Verify instrumentation code is in correct execution path
- Check for error conditions that skip metric recording
- Add debug logging to verify recording is called

---

## Next Steps

1. Run `/speckit.tasks` to generate implementation task breakdown
2. Assign tasks to team members
3. Implement metrics registration (Phase 1)
4. Implement instrumentation points (Phase 2)
5. Write and run tests (Phase 3)
6. Update configuration reference documentation
7. Create example Grafana dashboards (operator reference)
8. Deploy and verify metrics in production

---

**Ready to implement?** Start with Step 1.1 in `pkg/observability/metrics.go`.
