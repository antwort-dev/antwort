# Feature Specification: Production Metrics Taxonomy

**Feature Branch**: `046-metrics-taxonomy`
**Created**: 2026-03-29
**Status**: Draft
**Input**: Expand Prometheus metrics to a comprehensive 7-layer taxonomy building on the 12 core metrics from spec 013.

## Overview

Spec 013 established Antwort's Prometheus metrics foundation: 12 metrics covering HTTP requests, provider latency/tokens, tool executions, rate limiting, and OTel GenAI semantic conventions. This is sufficient for basic monitoring but leaves significant operational blind spots.

Operators cannot currently answer questions like:
- How deep are agentic loop conversations running?
- How many background responses are queued vs claimed?
- What's the storage operation latency?
- How fast is file ingestion?
- Are vector store searches performing well?

This spec adds ~23 new metrics across 5 new layers (Responses API, Engine, Storage, Files/Vector Store, Background Workers), giving operators complete visibility into every subsystem. The existing `pkg/observability/` package and `prometheus/client_golang` dependency are reused.

## Clarifications

### Session 2026-03-29

- Q: Should provider resilience metrics (circuit breaker state, retries) be included? -> A: No, deferred to the backend resilience spec (brainstorm 41). This spec covers observability of current functionality only.
- Q: Should metrics always be enabled or gated by feature flags? -> A: Always registered. Individual layers auto-activate when their subsystem is in use (e.g., background metrics only emit values when background mode is active). No feature flags needed.
- Q: How to handle cardinality for `model` and `tool_name` labels? -> A: Use values as-is. Cardinality control is an operator concern (Prometheus relabeling). Document the cardinality risk.

## User Scenarios & Testing

### User Story 1 - Monitor Responses API by Mode (Priority: P1)

An operator monitors Antwort's response processing, distinguishing between sync, streaming, and background modes. They can see active responses, completion rates, token throughput by mode, and TTFT for streaming. This enables capacity planning per mode and alerting on queue buildup.

**Why this priority**: Response-level metrics are the most actionable for capacity planning. Mode distinction is unique to Antwort and invisible without dedicated metrics.

**Independent Test**: Deploy Antwort, send sync, streaming, and background requests, scrape `/metrics`, verify per-mode counters and gauges.

**Acceptance Scenarios**:

1. **Given** a running Antwort instance processing responses, **When** `/metrics` is scraped, **Then** `antwort_responses_total` shows counts by model, status, and mode (sync/streaming/background)
2. **Given** active streaming responses, **When** `/metrics` is scraped, **Then** `antwort_responses_active` shows current processing count by mode
3. **Given** responses with `previous_response_id`, **When** `/metrics` is scraped, **Then** `antwort_responses_chained_total` increments for conversation-chained responses

---

### User Story 2 - Monitor Agentic Loop Behavior (Priority: P1)

An operator monitors the agentic loop to understand tool usage patterns, loop depth, and conversation complexity. They can see how many iterations loops take, which tools are called most, and how often responses hit the max-iteration limit. This helps tune `max_turns` configuration and identify misbehaving tool/model combinations.

**Why this priority**: The agentic loop is Antwort's differentiator and the most opaque subsystem. Without iteration and tool-duration metrics, operators cannot diagnose slow or runaway loops.

**Independent Test**: Send agentic requests with tool calls, scrape `/metrics`, verify iteration counters and tool duration histograms.

**Acceptance Scenarios**:

1. **Given** an agentic request completing in 3 iterations, **When** `/metrics` is scraped, **Then** `antwort_engine_iterations_total` increments by 3 for that model
2. **Given** tool executions during the loop, **When** `/metrics` is scraped, **Then** `antwort_engine_tool_duration_seconds` records per-tool latency distributions
3. **Given** a response hitting the max iteration limit, **When** `/metrics` is scraped, **Then** `antwort_engine_max_iterations_hit_total` increments

---

### User Story 3 - Monitor Storage Operations (Priority: P2)

An operator monitors storage layer health. They can see operation counts and latency by backend (memory/postgres) and operation type (get/save/delete/list). For PostgreSQL, they can monitor active connection count. This helps diagnose storage bottlenecks and plan connection pool sizing.

**Why this priority**: Storage latency directly impacts response time. Connection exhaustion is a common production failure mode.

**Independent Test**: Run requests that exercise storage (create + retrieve responses), scrape `/metrics`, verify operation counters and duration histograms.

**Acceptance Scenarios**:

1. **Given** responses being stored and retrieved, **When** `/metrics` is scraped, **Then** `antwort_storage_operations_total` shows counts by backend, operation, and result
2. **Given** PostgreSQL storage configured, **When** `/metrics` is scraped, **Then** `antwort_storage_connections_active` shows current connection pool usage

---

### User Story 4 - Monitor Files and Vector Store (Priority: P2)

An operator monitors the RAG pipeline: file uploads, ingestion processing time, and vector search performance. They can see upload rates, ingestion duration distribution, and search latency, which helps tune chunking parameters and identify slow content extraction.

**Why this priority**: RAG pipeline performance is invisible today. Slow ingestion or search directly impacts response quality and latency.

**Independent Test**: Upload files via Files API, run file_search queries, scrape `/metrics`, verify upload counters and search duration histograms.

**Acceptance Scenarios**:

1. **Given** files being uploaded, **When** `/metrics` is scraped, **Then** `antwort_files_uploaded_total` increments with content type labels
2. **Given** vector store searches, **When** `/metrics` is scraped, **Then** `antwort_vectorstore_search_duration_seconds` records search latency

---

### User Story 5 - Monitor Background Workers (Priority: P3)

An operator monitors the background response queue in distributed mode. They can see queue depth, claim rates per worker, stale response detection, and worker heartbeat freshness. This enables alerting on queue buildup and worker health.

**Why this priority**: Background mode is an advanced deployment pattern. Monitoring is critical for distributed operations but fewer users run this mode.

**Independent Test**: Submit background responses, start workers, scrape `/metrics`, verify queue depth gauge and claim counters.

**Acceptance Scenarios**:

1. **Given** background responses queued, **When** `/metrics` is scraped, **Then** `antwort_background_queued` shows current queue depth
2. **Given** workers claiming responses, **When** `/metrics` is scraped, **Then** `antwort_background_claimed_total` shows claims per worker
3. **Given** a stale worker detected, **When** `/metrics` is scraped, **Then** `antwort_background_stale_total` increments

---

### Edge Cases

- What happens when a subsystem is not configured (e.g., no PostgreSQL, no vector store)? Metrics are registered but emit zero values. No errors from unused metrics.
- What happens with very high cardinality `model` or `tool_name` labels? Values are used as-is. Operators should use Prometheus relabeling to control cardinality. Document this in the configuration reference.
- What happens when metrics collection impacts performance? Prometheus counter/histogram operations are lock-free atomic operations. Overhead is negligible (nanoseconds per recording).

## Requirements

### Functional Requirements

**Responses Layer**

- **FR-001**: The system MUST record `antwort_responses_total` counter with labels: `model`, `status` (completed/failed/cancelled/incomplete), `mode` (sync/streaming/background)
- **FR-002**: The system MUST record `antwort_responses_duration_seconds` histogram with labels: `model`, `mode`
- **FR-003**: The system MUST record `antwort_responses_active` gauge with label: `mode`
- **FR-004**: The system MUST record `antwort_responses_chained_total` counter with label: `model` for responses using `previous_response_id`
- **FR-005**: The system MUST record `antwort_responses_tokens_total` counter with labels: `model`, `type` (input/output)

**Engine Layer**

- **FR-006**: The system MUST record `antwort_engine_iterations_total` counter with label: `model`
- **FR-007**: The system MUST record `antwort_engine_iteration_duration_seconds` histogram with label: `model`
- **FR-008**: The system MUST record `antwort_engine_max_iterations_hit_total` counter with label: `model`
- **FR-009**: The system MUST record `antwort_engine_tool_duration_seconds` histogram with label: `tool_name`
- **FR-010**: The system MUST record `antwort_engine_conversation_depth` histogram with label: `model` counting items in rehydrated conversations

**Storage Layer**

- **FR-011**: The system MUST record `antwort_storage_operations_total` counter with labels: `backend` (memory/postgres), `operation` (get/save/delete/list), `result` (success/error)
- **FR-012**: The system MUST record `antwort_storage_operation_duration_seconds` histogram with labels: `backend`, `operation`
- **FR-013**: The system MUST record `antwort_storage_responses_stored` gauge with label: `backend`
- **FR-014**: The system MUST record `antwort_storage_connections_active` gauge for PostgreSQL connection pool usage

**Files and Vector Store Layer**

- **FR-015**: The system MUST record `antwort_files_uploaded_total` counter with label: `content_type`
- **FR-016**: The system MUST record `antwort_files_ingestion_duration_seconds` histogram for file processing pipeline duration
- **FR-017**: The system MUST record `antwort_vectorstore_searches_total` counter with labels: `store_id`, `result` (success/error)
- **FR-018**: The system MUST record `antwort_vectorstore_search_duration_seconds` histogram
- **FR-019**: The system MUST record `antwort_vectorstore_items_stored` gauge with label: `store_id`

**Background Workers Layer**

- **FR-020**: The system MUST record `antwort_background_queued` gauge for responses waiting in queue
- **FR-021**: The system MUST record `antwort_background_claimed_total` counter with label: `worker_id`
- **FR-022**: The system MUST record `antwort_background_stale_total` counter for stale responses detected and reclaimed
- **FR-023**: The system MUST record `antwort_background_worker_heartbeat_age_seconds` gauge with label: `worker_id`

**Metric Standards**

- **FR-024**: All duration histograms MUST use LLM-tuned bucket boundaries consistent with spec 013: 0.1, 0.5, 1, 2, 5, 10, 30, 60, 120 seconds
- **FR-025**: All metric names MUST follow Prometheus naming conventions: `antwort_` prefix, `_total` suffix for counters, unit in name (`_seconds`, `_bytes`)
- **FR-026**: All new metrics MUST be registered in the existing `pkg/observability/` package alongside the spec 013 metrics

### Key Entities

- **Metric Layer**: A logical grouping of related metrics (Responses, Engine, Storage, Files, Background). Each layer corresponds to a subsystem in Antwort's architecture.
- **Metric Recording Point**: A specific location in the code where a metric observation is recorded. Each functional requirement maps to one or more recording points.

## Success Criteria

### Measurable Outcomes

- **SC-001**: The `/metrics` endpoint exposes all 23 new metrics alongside the existing 12, for a total of 35 Prometheus metrics
- **SC-002**: Operators can build a Grafana dashboard showing response throughput by mode, agentic loop depth, storage latency, and background queue depth using only Antwort's metrics
- **SC-003**: Each metric layer can be validated independently: sending a response, running a tool, storing data, uploading a file, or submitting a background request produces the expected metric observations
- **SC-004**: Metric recording adds less than 1ms overhead per request (atomic counter/histogram operations only)
- **SC-005**: All metric names and labels follow Prometheus naming conventions and are consistent with the existing spec 013 metrics

## Assumptions

- The existing `prometheus/client_golang` dependency and `/metrics` endpoint from spec 013 are reused. No new external dependencies.
- Metrics are always registered at startup. Unused metrics (e.g., background metrics when not in background mode) emit zero values, which is standard Prometheus behavior.
- Provider resilience metrics (circuit breaker state, retry counts) are out of scope and will be added by the backend resilience spec.
- Storage metrics require instrumentation at the storage interface level, wrapping existing implementations.

## Dependencies

- **Spec 013 (Observability)**: Foundation package, `/metrics` endpoint, existing 12 metrics
- **Spec 044 (Async Responses)**: Background worker infrastructure for FR-020 through FR-023
- **Spec 034 (Files API)**: File upload and ingestion pipeline for FR-015 and FR-016
- **Spec 039 (Vector Store)**: Vector store search operations for FR-017 through FR-019

## Scope Boundaries

### In Scope

- 23 new Prometheus metrics across 5 layers (Responses, Engine, Storage, Files/Vector Store, Background)
- Instrumentation points in engine, storage, files, vector store, and background packages
- Documentation of all metrics in the configuration reference
- Unit tests for metric registration and recording

### Out of Scope

- Provider resilience metrics (circuit breaker, retries) - deferred to backend resilience spec
- Grafana dashboard JSON (operators build their own)
- Alerting rules (deployment-specific)
- OpenTelemetry tracing (separate concern)
- HTTP layer additions (request/response size) - minor, can be added later
