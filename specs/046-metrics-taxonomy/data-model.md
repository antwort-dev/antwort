# Data Model: Metrics Taxonomy

**Date**: 2026-03-29  
**Scope**: 23 new Prometheus metrics across 5 subsystem layers

## Overview

This document defines the entities, attributes, and relationships for the metrics taxonomy expansion. All metrics are Prometheus scalar types (Counter, Histogram, Gauge) with optional labels for dimensionality.

---

## Layer 1: Responses API

### Entity: Response

Tracks all responses created through the `/v1/responses` endpoint.

#### Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `antwort_responses_total` | Counter | model, status, mode | Total responses by model, completion status (completed/failed/cancelled/incomplete), and mode (sync/streaming/background) |
| `antwort_responses_duration_seconds` | Histogram | model, mode | Response duration distribution (sync: request-to-response time; streaming: first-byte-to-close time; background: queue-to-completion time) |
| `antwort_responses_active` | Gauge | mode | Current number of in-flight responses by mode |
| `antwort_responses_chained_total` | Counter | model | Responses that reference a previous_response_id (conversation chaining) |
| `antwort_responses_tokens_total` | Counter | model, type | Token usage by direction (input/output) |

#### Attributes

- **model**: LLM model used for response (e.g., "gpt-4", "llama-2-70b")
- **status**: Final response status (completed, failed, cancelled, incomplete)
- **mode**: Response delivery mode (sync, streaming, background)
- **previous_response_id**: Optional reference to parent response in conversation chain
- **tokens.input**: Input token count
- **tokens.output**: Output token count

#### Constraints

- All status values are terminal states at recording time
- Modes are mutually exclusive per response
- Token counts are non-negative integers

---

## Layer 2: Engine / Agentic Loop

### Entity: AgenticIteration

Tracks each turn of the multi-turn agentic loop within a response.

#### Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `antwort_engine_iterations_total` | Counter | model | Total iterations (turns) executed by model |
| `antwort_engine_iteration_duration_seconds` | Histogram | model | Duration per iteration (provider call + tool execution) |
| `antwort_engine_max_iterations_hit_total` | Counter | model | Responses that hit the max_iterations limit |
| `antwort_engine_tool_duration_seconds` | Histogram | tool_name | Duration of tool execution by tool name |
| `antwort_engine_conversation_depth` | Histogram | model | Number of items in rehydrated conversation (conversation history length) |

#### Attributes

- **model**: LLM model used
- **iteration**: 1-indexed turn number (1 to max_turns)
- **tool_name**: Name of tool executed (e.g., "file_search", "web_search", "code_interpreter")
- **conversation_items**: Array of items in rehydrated conversation (messages + tool results)

#### Constraints

- Iterations are numbered sequentially starting at 1
- Conversation depth is a non-negative integer
- Tool names match tool registry definitions
- Max iterations hit is a boolean condition (counter incremented once per response that hits limit)

---

## Layer 3: Storage

### Entity: StorageOperation

Tracks all storage backend operations (Get, Save, Delete, List).

#### Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `antwort_storage_operations_total` | Counter | backend, operation, result | Total operations by backend, operation type (get/save/delete/list), and result (success/error) |
| `antwort_storage_operation_duration_seconds` | Histogram | backend, operation | Operation latency distribution |
| `antwort_storage_responses_stored` | Gauge | backend | Current number of responses in storage |
| `antwort_storage_connections_active` | Gauge | [none] | Active connections in PostgreSQL connection pool |

#### Attributes

- **backend**: Storage backend type (memory, postgres)
- **operation**: Operation type (get, save, delete, list)
- **result**: Operation outcome (success, error)
- **response_id**: Identifier of response being stored
- **error_type**: Category of error if result=error (not in metric label)

#### Constraints

- Backend values: "memory", "postgres"
- Operation values: "get", "save", "delete", "list"
- Result values: "success", "error"
- Connection count gauge only populated for PostgreSQL backend
- Memory backend gauges remain static (no real-time connection tracking)

---

## Layer 4: Files and Vector Store

### Entity: FileUpload

Tracks file uploads and ingestion pipeline processing.

#### Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `antwort_files_uploaded_total` | Counter | content_type | Total files uploaded by content type (application/pdf, text/plain, etc.) |
| `antwort_files_ingestion_duration_seconds` | Histogram | [none] | Ingestion pipeline duration (extract → chunk → embed → store) |

#### Attributes

- **content_type**: MIME type of uploaded file
- **file_id**: Unique file identifier
- **vector_store_id**: Target vector store for ingestion

---

### Entity: VectorStoreSearch

Tracks vector store search operations.

#### Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `antwort_vectorstore_searches_total` | Counter | store_id, result | Total searches by vector store and result (success/error) |
| `antwort_vectorstore_search_duration_seconds` | Histogram | [none] | Search latency distribution |
| `antwort_vectorstore_items_stored` | Gauge | store_id | Current item count per vector store |

#### Attributes

- **store_id**: Vector store identifier (e.g., "pinecone-prod", "qdrant-local")
- **result**: Search outcome (success, error)
- **query_tokens**: Embedding dimension / token count of query
- **result_count**: Number of results returned

#### Constraints

- Store IDs are configured and immutable
- Result values: "success", "error"
- Item count gauge per store for capacity monitoring

---

## Layer 5: Background Workers

### Entity: BackgroundResponse

Tracks background response queue management and worker processing.

#### Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `antwort_background_queued` | Gauge | [none] | Current depth of background response queue |
| `antwort_background_claimed_total` | Counter | worker_id | Total responses claimed by worker |
| `antwort_background_stale_total` | Counter | [none] | Responses detected as stale and reclaimed |
| `antwort_background_worker_heartbeat_age_seconds` | Gauge | worker_id | Time since last worker heartbeat |

#### Attributes

- **worker_id**: Unique identifier for background worker (e.g., "worker-1", "worker-abc123")
- **response_id**: Background response in queue
- **claimed_at**: Timestamp of claim
- **worker_heartbeat_ttl**: Configured heartbeat timeout
- **heartbeat_updated_at**: Last heartbeat timestamp

#### Constraints

- Queue depth is a non-negative integer
- Worker IDs are assigned at worker startup
- Stale detection triggers claim reclamation (total counter increments)
- Heartbeat age measured in seconds since last update

---

## Cross-Layer Relationships

### Response Hierarchy

```
Response (Responses Layer)
├── Iterations (Engine Layer)
│   ├── ToolExecution
│   │   └── Tool Duration (Engine Layer)
│   └── Conversation (Engine Layer)
├── StorageOperation (Storage Layer)
│   └── Operation Duration
└── Background Queue (Background Layer) [if mode=background]
    ├── Claimed By Worker
    └── Stale Detection
```

### Data Flow

1. **Request → Response Creation**: `responses_total` increments, `responses_active` increments (by mode)
2. **Agentic Loop**: Per iteration, `iterations_total` increments, `iteration_duration_seconds` records
3. **Tool Execution**: Per tool call, `tool_duration_seconds` records
4. **Storage**: Operations recorded at Get/Save/Delete/List with `storage_operations_total` and `storage_operation_duration_seconds`
5. **File/Vector**: Uploads increment `files_uploaded_total`, searches increment `vectorstore_searches_total`
6. **Background**: Queue state tracked, claims recorded, stale detected
7. **Response Completion**: `responses_total` final status recorded, `responses_active` decrements (by mode), `responses_duration_seconds` records

---

## Label Cardinality Analysis

### High Cardinality (Operator Risk)

| Label | Cardinality | Mitigation |
|-------|-----------|-----------|
| `model` | 5-100+ (unlimited) | Use Prometheus relabeling to group/drop high-cardinality models |
| `tool_name` | 3-20 typically | Most are fixed built-ins; custom tools add cardinality |

### Medium Cardinality

| Label | Cardinality | Notes |
|-------|-----------|-------|
| `worker_id` | 1-N workers | Bounded by cluster size |
| `store_id` | 1-10 typically | Bounded by configured vector stores |

### Low Cardinality

| Label | Cardinality | Notes |
|-------|-----------|-------|
| `status` | 4 values (completed/failed/cancelled/incomplete) | Fixed enum |
| `mode` | 3 values (sync/streaming/background) | Fixed enum |
| `operation` | 4 values (get/save/delete/list) | Fixed enum |
| `result` | 2 values (success/error) | Fixed enum |
| `backend` | 2 values (memory/postgres) | Fixed enum |
| `direction` | 2 values (input/output) | Fixed enum |

---

## Validation Rules

### Metric Recording Rules

1. **Counters**: Only increment (`.Inc()`) or add positive values (`.Add()`)
2. **Histograms**: Record positive float values in seconds or count units
3. **Gauges**: Can be set, incremented, or decremented
4. **Labels**: Must be non-empty strings; no nulls or empty label values

### Label Value Rules

- `model`: Must match request model or provider-returned model name
- `tool_name`: Must match registered tool name
- `worker_id`: Must be unique per worker instance
- `store_id`: Must match configured vector store identifier
- Enum labels (status, mode, etc.): Must match defined set exactly

---

## Metrics Registration Contract

All 23 metrics are registered at application startup in `pkg/observability/init()`:

```go
prometheus.MustRegister(
    // Responses Layer
    ResponsesTotal,
    ResponsesDuration,
    ResponsesActive,
    ResponsesChainedTotal,
    ResponsesTokensTotal,
    
    // Engine Layer
    EngineIterationsTotal,
    EngineIterationDuration,
    EngineMaxIterationsHit,
    EngineToolDuration,
    EngineConversationDepth,
    
    // Storage Layer
    StorageOperationsTotal,
    StorageOperationDuration,
    StorageResponsesStored,
    StorageConnectionsActive,
    
    // Files/Vector Store Layer
    FilesUploadedTotal,
    FilesIngestionDuration,
    VectorstoreSearchesTotal,
    VectorstoreSearchDuration,
    VectorstoreItemsStored,
    
    // Background Layer
    BackgroundQueued,
    BackgroundClaimedTotal,
    BackgroundStaleTotal,
    BackgroundWorkerHeartbeatAge,
)
```

---

## Gotchas and Edge Cases

### Empty Label Values

If a label value is not known at recording time (e.g., `worker_id` not set), the code must handle gracefully:
- Never record with empty label values
- Use placeholder like "unknown" if necessary (adds cardinality)
- Better: ensure label is always populated

### Gauges Without Prior Reset

Background queue depth gauge is set per poll cycle. If a worker dies, stale responses remain in queue until reclaimed. Gauge will show them until next poll.

### Conversation Depth Histogram Bucket Alignment

Conversation depth is not a duration (seconds), so `LLMBuckets` is inappropriate. Use default buckets or custom buckets matching expected conversation lengths (e.g., [1, 2, 5, 10, 20, 50]).

### Background Worker Heartbeat Age

Heartbeat age is the time since last heartbeat update. This requires capturing timestamp at heartbeat update and computing age on each poll. Not a duration histogram (no natural max), so use Gauge instead.

---

## Summary

The 23-metric taxonomy provides complete visibility into:
- **User-facing**: Response throughput, latency, and mode distribution
- **Engine**: Iteration complexity and tool performance
- **Infrastructure**: Storage and vector store performance
- **Operational**: Background queue health and worker status

All metrics follow Prometheus conventions and integrate with the existing spec 013 infrastructure.
