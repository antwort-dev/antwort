# Brainstorm 42: Metrics Taxonomy

**Date**: 2026-03-21
**Participants**: Roland Huss
**Inspiration**: SMG's 6-layer, 40+ metric system
**Goal**: Design a structured Prometheus metrics taxonomy for Antwort, learning from SMG's layered approach while reflecting Antwort's architectural differences.

## Motivation

Antwort has spec 013 (observability) on the roadmap but hasn't implemented Prometheus metrics yet. The existing observability surface is:
- Structured logging via `slog` (spec 026)
- Debug categories (`ANTWORT_DEBUG=providers,engine`)
- Audit logging (spec 042)

What's missing: quantitative metrics that operators can alert on, dashboard, and trend. SMG's metric taxonomy is a good reference because it covers the full request lifecycle in layers that map well to Antwort's architecture.

## SMG's Metric Layers (Reference)

| Layer | Scope | Example Metrics |
|-------|-------|-----------------|
| 1. HTTP | Gateway edge | requests_total, duration, active connections, rate limit decisions |
| 2. Router | Request routing | per-model/endpoint counts, pipeline stage durations, TTFT, TPOT, token counts |
| 3. Worker | Backend health | active connections/requests, health status, circuit breaker state, retries |
| 4. Discovery | Backend registration | registrations, sync duration, workers discovered |
| 5. MCP Tools | Tool execution | tool calls, duration, active servers, loop iterations |
| 6. Database | Storage ops | operation counts, duration, active connections, items stored |

Total: 40+ metrics with well-chosen labels.

## Proposed Antwort Metric Layers

Antwort's architecture maps to different layers than SMG. The key difference: Antwort is an **agentic orchestration gateway**, not a routing/load-balancing gateway. The metric taxonomy should reflect this.

### Layer 1: HTTP (Transport)

Gateway edge metrics. Standard for any HTTP service.

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `antwort_http_requests_total` | Counter | `method`, `path`, `status_code` | Total HTTP requests |
| `antwort_http_request_duration_seconds` | Histogram | `method`, `path` | Request duration (receipt to response complete) |
| `antwort_http_connections_active` | Gauge | | Currently active connections |
| `antwort_http_request_size_bytes` | Histogram | `method`, `path` | Request body size |
| `antwort_http_response_size_bytes` | Histogram | `method`, `path` | Response body size |

Note: No rate limiting metrics (Antwort delegates this to ingress). Auth rejection metrics could go here or in a separate auth layer.

### Layer 2: Responses (API)

Antwort-specific: the Responses API is the core abstraction.

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `antwort_responses_total` | Counter | `model`, `status`, `mode` | Responses created (mode: sync/streaming/background) |
| `antwort_responses_duration_seconds` | Histogram | `model`, `mode` | Total response processing time |
| `antwort_responses_active` | Gauge | `mode` | Currently processing responses |
| `antwort_responses_chained_total` | Counter | `model` | Responses using `previous_response_id` |
| `antwort_responses_tokens_total` | Counter | `model`, `type` | Token counts (input/output, from usage) |
| `antwort_responses_ttft_seconds` | Histogram | `model` | Time to first token (streaming) |

The `mode` label distinguishes sync, streaming, and background, which is unique to Antwort.

### Layer 3: Engine (Agentic Loop)

The agentic loop is Antwort's differentiator. No equivalent in SMG.

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `antwort_engine_iterations_total` | Counter | `model` | Agentic loop iterations |
| `antwort_engine_iteration_duration_seconds` | Histogram | `model` | Duration per iteration |
| `antwort_engine_max_iterations_hit_total` | Counter | `model` | Responses that hit the iteration limit |
| `antwort_engine_tool_calls_total` | Counter | `model`, `tool_name`, `result` | Tool invocations (result: success/error) |
| `antwort_engine_tool_duration_seconds` | Histogram | `tool_name` | Tool execution duration |
| `antwort_engine_conversation_depth` | Histogram | `model` | Number of items in rehydrated conversation |

### Layer 4: Provider (Backend)

Backend communication metrics. Parallels SMG's Worker layer but simpler (single backend, not pool).

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `antwort_provider_requests_total` | Counter | `provider`, `model`, `result` | Requests to backend (result: success/error/timeout) |
| `antwort_provider_request_duration_seconds` | Histogram | `provider`, `model` | Backend request duration |
| `antwort_provider_active_requests` | Gauge | `provider` | Currently in-flight backend requests |
| `antwort_provider_circuit_breaker_state` | Gauge | `provider` | Circuit breaker state (if brainstorm 41 implemented) |
| `antwort_provider_retries_total` | Counter | `provider` | Retry attempts |

### Layer 5: Storage

Database operation metrics. Similar concept to SMG's DB layer.

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `antwort_storage_operations_total` | Counter | `backend`, `operation`, `result` | Storage operations (backend: memory/postgres) |
| `antwort_storage_operation_duration_seconds` | Histogram | `backend`, `operation` | Operation duration |
| `antwort_storage_responses_stored` | Gauge | `backend` | Total responses in storage |
| `antwort_storage_connections_active` | Gauge | | Active PostgreSQL connections |

### Layer 6: Files & Vector Store

RAG-specific metrics. No equivalent in SMG (RAG is not SMG's focus).

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `antwort_files_uploaded_total` | Counter | `content_type` | Files uploaded |
| `antwort_files_ingestion_duration_seconds` | Histogram | | File ingestion pipeline duration |
| `antwort_vectorstore_searches_total` | Counter | `store_id`, `result` | Vector similarity searches |
| `antwort_vectorstore_search_duration_seconds` | Histogram | | Search duration |
| `antwort_vectorstore_items_stored` | Gauge | `store_id` | Items in vector store |

### Layer 7: Background Workers (if applicable)

For distributed mode (spec 044).

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `antwort_background_queued` | Gauge | | Responses waiting in queue |
| `antwort_background_claimed_total` | Counter | `worker_id` | Responses claimed by workers |
| `antwort_background_stale_total` | Counter | | Stale responses detected and reclaimed |
| `antwort_background_worker_heartbeat_age_seconds` | Gauge | `worker_id` | Time since last heartbeat |

## Total: ~35 metrics across 7 layers

This is comparable to SMG's 40+ but reflects Antwort's different focus (agentic orchestration vs routing).

## Implementation Approach

### Option A: `prometheus/client_golang` (recommended)
- De facto standard for Go Prometheus metrics
- Only external dependency would be this library (or expose via OpenTelemetry)
- Constitution says "Go standard library only" for core, but observability is an adapter concern (like `pgx` for PostgreSQL)

### Option B: OpenTelemetry SDK
- More flexible (supports Prometheus export + OTLP push)
- Heavier dependency
- Better alignment with OpenShift/RHOAI observability stack

### Option C: Expvar + custom /metrics
- Pure stdlib (expvar is in Go stdlib)
- Non-standard format, would need a Prometheus translation layer
- Not recommended

### Recommendation
Option A for initial implementation. Option B as a future migration if OpenTelemetry becomes a requirement for RHOAI integration.

## Metric Naming Conventions

Following Prometheus best practices:
- Prefix: `antwort_`
- Units in name: `_seconds`, `_bytes`, `_total`
- Counter suffix: `_total`
- Use labels for dimensions, not metric name proliferation
- Keep cardinality bounded: `model` and `tool_name` labels could explode, consider a label allowlist

## What NOT to Copy from SMG

1. **Discovery metrics**: Antwort doesn't discover backends dynamically
2. **Cache routing metrics**: Not applicable (no KV-cache awareness)
3. **Per-worker granularity**: Antwort talks to one backend URL, not a pool
4. **Token-level pipeline stages** (`tokenize`, `chat_template`, `detokenize`): These happen inside vLLM, not in Antwort

## Relation to Existing Spec 013

Spec 013 (observability) was planned early in the project. It should be updated to incorporate this metrics taxonomy. The spec should also cover:
- Health endpoint (`/healthz`, `/readyz`)
- OpenTelemetry trace context propagation
- Structured log correlation with trace IDs

## Open Questions

1. Should metrics be behind a feature flag or always enabled? (SMG always exposes them.)
2. Should we use a `/metrics` endpoint or push to an OTLP collector?
3. How do we handle metric cardinality for `model` labels when users configure many models?
4. Should audit events (spec 042) also emit corresponding metrics, or are logs sufficient?
