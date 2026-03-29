# Tasks: Production Metrics Taxonomy

**Input**: Design documents from `/specs/046-metrics-taxonomy/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, quickstart.md

**Tests**: Integration tests are included as spec 013 established a testing pattern for metrics. E2E tests follow constitution v1.8.0 requirements.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Metric Definitions)

**Purpose**: Define all 23 new metrics in the centralized observability package

- [x] T001 Define Responses layer metrics (ResponsesTotal, ResponsesDuration, ResponsesActive, ResponsesChainedTotal, ResponsesTokensTotal) in `pkg/observability/metrics.go`
- [x] T002 [P] Define Engine layer metrics (EngineIterationsTotal, EngineIterationDuration, EngineMaxIterationsHit, EngineToolDuration, EngineConversationDepth) with ConversationDepthBuckets in `pkg/observability/metrics.go`
- [x] T003 [P] Define Storage layer metrics (StorageOperationsTotal, StorageOperationDuration, StorageResponsesStored, StorageConnectionsActive) in `pkg/observability/metrics.go`
- [x] T004 [P] Define Files/Vector Store layer metrics (FilesUploadedTotal, FilesIngestionDuration, VectorstoreSearchesTotal, VectorstoreSearchDuration, VectorstoreItemsStored) in `pkg/observability/metrics.go`
- [x] T005 [P] Define Background Worker layer metrics (BackgroundQueued, BackgroundClaimedTotal, BackgroundStaleTotal, BackgroundWorkerHeartbeatAge) in `pkg/observability/metrics.go`
- [x] T006 Register all 23 new metrics in `init()` alongside existing 12 metrics in `pkg/observability/metrics.go`
- [x] T007 Add unit test verifying all 35 metrics are registered and can record observations in `pkg/observability/metrics_test.go`

**Checkpoint**: All 23 metrics defined and registered. `/metrics` endpoint exposes them (with zero values).

---

## Phase 2: User Story 1 - Monitor Responses API by Mode (Priority: P1)

**Goal**: Operators can monitor response processing by mode (sync/streaming/background), see active responses, completion rates, token throughput, and conversation chaining.

**Independent Test**: Deploy Antwort, send sync, streaming, and background requests, scrape `/metrics`, verify per-mode counters and gauges.

### Implementation for User Story 1

- [x] T008 [US1] Add response mode helper function `responseMode(req)` returning "sync"/"streaming"/"background" in `pkg/engine/engine.go`
- [x] T009 [US1] Instrument `CreateResponse()` in `pkg/engine/engine.go`: increment `ResponsesActive` gauge at entry (defer Dec), record `ResponsesChainedTotal` when `PreviousResponseID` is set
- [x] T010 [US1] Instrument `handleNonStreaming()` in `pkg/engine/engine.go`: record `ResponsesTotal` (model, status, mode), `ResponsesDuration` (model, mode), and `ResponsesTokensTotal` (model, input/output) after response completion
- [x] T011 [US1] Instrument `handleStreaming()` in `pkg/engine/engine.go`: record `ResponsesTotal`, `ResponsesDuration`, and `ResponsesTokensTotal` after stream completion
- [x] T012 [US1] Instrument `handleBackground()` in `pkg/engine/engine.go`: record `ResponsesTotal` with status "queued" and mode "background"
- [x] T013 [US1] Instrument `runAgenticLoop()` in `pkg/engine/loop.go`: record `ResponsesTotal`, `ResponsesDuration`, and `ResponsesTokensTotal` at loop completion (both normal and max-turns-reached paths)
- [x] T014 [US1] Instrument `runAgenticLoopStreaming()` in `pkg/engine/loop.go`: record `ResponsesTotal`, `ResponsesDuration`, and `ResponsesTokensTotal` at stream loop completion
- [ ] T015 [US1] Add integration test: send sync request, scrape `/metrics`, verify `antwort_responses_total{mode="sync"}` incremented in `test/integration/metrics_test.go`

**Checkpoint**: Response metrics recorded for all three modes. Operators can see per-mode throughput and active counts.

---

## Phase 3: User Story 2 - Monitor Agentic Loop Behavior (Priority: P1)

**Goal**: Operators can monitor agentic loop iteration depth, tool execution duration, conversation complexity, and max-iterations-hit events.

**Independent Test**: Send agentic requests with tool calls, scrape `/metrics`, verify iteration counters and tool duration histograms.

### Implementation for User Story 2

- [x] T016 [US2] Instrument iteration counting in `runAgenticLoop()` in `pkg/engine/loop.go`: record `EngineIterationsTotal` per turn, `EngineIterationDuration` per turn (provider call + tool execution)
- [x] T017 [US2] Instrument max-iterations-hit in `runAgenticLoop()` in `pkg/engine/loop.go`: record `EngineMaxIterationsHit` when loop exits at `maxTurns`
- [x] T018 [P] [US2] Instrument tool execution duration in `executeToolsConcurrently()` and `executeToolsSequentially()` in `pkg/engine/loop.go`: wrap `exec.Execute()` with timing, record `EngineToolDuration` with `tool_name` label
- [x] T019 [US2] Instrument conversation depth in `runAgenticLoop()` in `pkg/engine/loop.go`: count items in `provReq.Messages` (rehydrated history) before loop starts, record `EngineConversationDepth`
- [x] T020 [P] [US2] Instrument streaming agentic loop: apply same iteration, max-iterations-hit, and conversation depth metrics to `runAgenticLoopStreaming()` in `pkg/engine/loop.go`
- [ ] T021 [US2] Add integration test: send agentic request with tools, scrape `/metrics`, verify `antwort_engine_iterations_total` and `antwort_engine_tool_duration_seconds` in `test/integration/metrics_test.go`

**Checkpoint**: Engine metrics show iteration depth, tool performance, and conversation complexity.

---

## Phase 4: User Story 3 - Monitor Storage Operations (Priority: P2)

**Goal**: Operators can monitor storage operation counts, latency by backend and operation type, response count, and PostgreSQL connection pool usage.

**Independent Test**: Run requests that exercise storage (create + retrieve responses), scrape `/metrics`, verify operation counters and duration histograms.

### Implementation for User Story 3

- [x] T022 [US3] Instrument `GetResponse()`, `SaveResponse()`, `DeleteResponse()`, `ListResponses()` in `pkg/storage/memory/memory.go`: record `StorageOperationsTotal` (backend="memory", operation, result) and `StorageOperationDuration`
- [x] T023 [US3] Track stored response count in memory store: update `StorageResponsesStored` gauge (backend="memory") on save/delete in `pkg/storage/memory/memory.go`
- [x] T024 [P] [US3] Instrument `GetResponse()`, `SaveResponse()`, `DeleteResponse()`, `ListResponses()` in `pkg/storage/postgres/postgres.go`: record `StorageOperationsTotal` (backend="postgres", operation, result) and `StorageOperationDuration`
- [x] T025 [P] [US3] Track stored response count in PostgreSQL store: update `StorageResponsesStored` gauge (backend="postgres") on save/delete in `pkg/storage/postgres/postgres.go`
- [x] T026 [US3] Expose PostgreSQL connection pool usage: read `pool.Stat().AcquiredConns()` and set `StorageConnectionsActive` gauge after each operation in `pkg/storage/postgres/postgres.go`
- [ ] T027 [US3] Add integration test: store and retrieve responses, scrape `/metrics`, verify `antwort_storage_operations_total` and `antwort_storage_operation_duration_seconds` in `test/integration/metrics_test.go`

**Checkpoint**: Storage metrics show operation patterns, latency distribution, and connection pool health.

---

## Phase 5: User Story 4 - Monitor Files and Vector Store (Priority: P2)

**Goal**: Operators can monitor file upload rates, ingestion pipeline duration, and vector store search performance.

**Independent Test**: Upload files via Files API, run file_search queries, scrape `/metrics`, verify upload counters and search duration histograms.

### Implementation for User Story 4

- [x] T028 [US4] Instrument file upload in `handleUpload()` in `pkg/files/api.go`: record `FilesUploadedTotal` with `content_type` label after successful file storage
- [x] T029 [US4] Instrument ingestion pipeline in `pkg/files/pipeline.go`: time the full extract-chunk-embed-store cycle, record `FilesIngestionDuration`
- [x] T030 [P] [US4] Instrument vector store search in `pkg/tools/builtins/filesearch/provider.go`: record `VectorstoreSearchesTotal` (store_id, result) and `VectorstoreSearchDuration` around search calls
- [x] T031 [P] [US4] Track vector store item count: update `VectorstoreItemsStored` gauge (store_id) after successful upsert in ingestion pipeline in `pkg/files/pipeline.go`
- [ ] T032 [US4] Add integration test: upload file, trigger search, scrape `/metrics`, verify `antwort_files_uploaded_total` and `antwort_vectorstore_searches_total` in `test/integration/metrics_test.go`

**Checkpoint**: Files and vector store metrics show upload rates, ingestion duration, and search performance.

---

## Phase 6: User Story 5 - Monitor Background Workers (Priority: P3)

**Goal**: Operators can monitor background response queue depth, claim rates per worker, stale response detection, and worker heartbeat freshness.

**Independent Test**: Submit background responses, start workers, scrape `/metrics`, verify queue depth gauge and claim counters.

### Implementation for User Story 5

- [x] T033 [US5] Instrument queue depth in `pollOnce()` in `pkg/engine/background.go`: set `BackgroundQueued` gauge based on store query result
- [x] T034 [US5] Instrument response claiming in `pollOnce()` in `pkg/engine/background.go`: increment `BackgroundClaimedTotal` with `worker_id` label after successful `ClaimQueuedResponse()`
- [x] T035 [P] [US5] Instrument stale detection in `detectStale()` in `pkg/engine/background.go`: increment `BackgroundStaleTotal` by count of stale responses found
- [x] T036 [P] [US5] Instrument heartbeat age in heartbeat goroutine in `pkg/engine/background.go`: set `BackgroundWorkerHeartbeatAge` gauge with `worker_id` label to `time.Since(lastHeartbeat).Seconds()`
- [ ] T037 [US5] Add integration test: submit background request, run worker poll cycle, scrape `/metrics`, verify `antwort_background_queued` and `antwort_background_claimed_total` in `test/integration/metrics_test.go`

**Checkpoint**: Background worker metrics show queue health, claim activity, and worker status.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Documentation and comprehensive test coverage

- [ ] T038 [P] Create metrics reference documentation listing all 35 metrics with names, types, labels, descriptions, and cardinality notes in `docs/modules/reference/pages/metrics.adoc`
- [ ] T039 [P] Add E2E test: start server with mock backend, send requests across all modes, scrape `/metrics`, verify all 23 new metrics present with expected values in `test/e2e/metrics_test.go`
- [x] T040 Run full test suite (`go test ./...`) and verify no regressions to existing spec 013 metrics

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies. Defines and registers all metrics.
- **US1 (Phase 2)**: Depends on Setup (T006 registration). Can start immediately after.
- **US2 (Phase 3)**: Depends on Setup (T006 registration). Independent of US1.
- **US3 (Phase 4)**: Depends on Setup (T006 registration). Independent of US1/US2.
- **US4 (Phase 5)**: Depends on Setup (T006 registration). Independent of US1/US2/US3.
- **US5 (Phase 6)**: Depends on Setup (T006 registration). Independent of US1/US2/US3/US4.
- **Polish (Phase 7)**: Depends on all user stories being complete.

### User Story Dependencies

- **US1 (P1)**: Independent. Can start after Phase 1 Setup.
- **US2 (P1)**: Independent. Can start after Phase 1 Setup. Can run in parallel with US1.
- **US3 (P2)**: Independent. Can start after Phase 1 Setup. Can run in parallel with US1/US2.
- **US4 (P2)**: Independent. Can start after Phase 1 Setup. Can run in parallel with all others.
- **US5 (P3)**: Independent. Can start after Phase 1 Setup. Can run in parallel with all others.

### Within Each User Story

- Metric definitions must be registered (Phase 1) before instrumentation
- Non-streaming instrumentation before streaming variants (where applicable)
- Core instrumentation before integration tests

### Parallel Opportunities

- T001-T005 can all run in parallel (different metric layers, same file but independent sections)
- All 5 user stories are independent and can run in parallel after Phase 1
- Within each story, tasks marked [P] can run in parallel
- T038 and T039 can run in parallel (docs vs tests)

---

## Parallel Example: User Story 2

```bash
# After Phase 1 completes, these can run in parallel:
Task T018: "Instrument tool execution duration in executeToolsConcurrently/executeToolsSequentially"
Task T020: "Instrument streaming agentic loop with same metrics"

# Then sequentially:
Task T016: "Instrument iteration counting in runAgenticLoop"
Task T017: "Instrument max-iterations-hit"
Task T019: "Instrument conversation depth"
Task T021: "Integration test for engine metrics"
```

---

## Implementation Strategy

### MVP First (User Stories 1 + 2 Only)

1. Complete Phase 1: Setup (metric definitions and registration)
2. Complete Phase 2: US1 (response metrics by mode)
3. Complete Phase 3: US2 (engine/agentic loop metrics)
4. **STOP and VALIDATE**: Test US1 and US2 independently
5. Operators can now monitor response throughput and agentic loop behavior

### Incremental Delivery

1. Setup (Phase 1) -> All metrics registered with zero values
2. Add US1 (Phase 2) -> Response mode monitoring (MVP!)
3. Add US2 (Phase 3) -> Agentic loop monitoring
4. Add US3 (Phase 4) -> Storage monitoring
5. Add US4 (Phase 5) -> Files/vector store monitoring
6. Add US5 (Phase 6) -> Background worker monitoring
7. Polish (Phase 7) -> Documentation, E2E tests, regression check
8. Each story adds new visibility without affecting previous stories

---

## Notes

- [P] tasks = different files or independent code sections, no dependencies
- [Story] label maps task to specific user story for traceability
- All instrumentation follows existing spec 013 patterns: `observability.MetricName.WithLabelValues(...).Inc/Observe()`
- Commit after each phase completion for clean git history
- Total tasks: 40
- Tasks per story: US1=8, US2=6, US3=6, US4=5, US5=5
- Setup tasks: 7, Polish tasks: 3
