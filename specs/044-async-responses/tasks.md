# Tasks: Async Responses (Background Mode)

**Input**: Design documents from `/specs/044-async-responses/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: API type changes and configuration additions needed before any feature work

- [ ] T001 Add `Background bool` field to `CreateResponseRequest` in `pkg/api/types.go`
- [ ] T002 Add `queued` -> `cancelled` transition to `ValidateResponseTransition` in `pkg/api/state.go`
- [ ] T003 Add `BackgroundConfig` struct and `Mode` field to config in `pkg/config/config.go`
- [ ] T004 [P] Add `Status` and `Background` filter fields to `ListOptions` in `pkg/transport/handler.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Storage interface extensions and implementations that ALL user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T005 Add `UpdateResponse`, `ClaimQueuedResponse`, `CleanupExpired` methods to `ResponseStore` interface in `pkg/transport/handler.go`
- [ ] T006 Add `ResponseUpdate` type (Status, Output, Error, Usage, CompletedAt, WorkerHeartbeat fields) in `pkg/transport/handler.go`
- [ ] T007 [P] Implement `UpdateResponse` in memory store in `pkg/storage/memory/memory.go`
- [ ] T008 [P] Implement `ClaimQueuedResponse` in memory store (mutex-guarded scan, atomic status transition) in `pkg/storage/memory/background.go`
- [ ] T009 [P] Implement `CleanupExpired` in memory store in `pkg/storage/memory/background.go`
- [ ] T010 [P] Implement `ListResponses` status/background filtering in memory store in `pkg/storage/memory/memory.go`
- [ ] T011 [P] Create PostgreSQL migration `003_add_background.sql` adding `background_request`, `worker_id`, `worker_heartbeat` columns and indexes in `pkg/storage/postgres/migrations/003_add_background.sql`
- [ ] T012 [P] Implement `UpdateResponse` in PostgreSQL store in `pkg/storage/postgres/postgres.go`
- [ ] T013 [P] Implement `ClaimQueuedResponse` in PostgreSQL store using `FOR UPDATE SKIP LOCKED` in `pkg/storage/postgres/background.go`
- [ ] T014 [P] Implement `CleanupExpired` in PostgreSQL store in `pkg/storage/postgres/background.go`
- [ ] T015 [P] Implement `ListResponses` status/background filtering in PostgreSQL store in `pkg/storage/postgres/postgres.go`

**Checkpoint**: Storage layer ready for background processing. All new interface methods implemented in both stores.

---

## Phase 3: User Story 1 - Fire-and-Forget Inference Request (Priority: P1) MVP

**Goal**: Client submits `background: true` request, gets immediate `queued` response, polls for result

**Independent Test**: Submit request with `background: true`, verify immediate return with `status: "queued"`. Poll GET /v1/responses/{id} and verify status progression to `completed`.

### Implementation for User Story 1

- [ ] T016 [US1] Add background validation to engine: reject `background: true` + `store: false`, reject `background: true` + `stream: true` in `pkg/engine/engine.go`
- [ ] T017 [US1] Implement background request routing in engine: when `background: true`, save response with status `queued` and serialized request, return immediately in `pkg/engine/engine.go`
- [ ] T018 [US1] Create worker loop: poll for queued requests, claim atomically, process through engine pipeline, update status on completion/failure in `pkg/engine/background.go`
- [ ] T019 [US1] Add heartbeat update during worker processing (periodic goroutine updating `worker_heartbeat` while request is in-flight) in `pkg/engine/background.go`
- [ ] T020 [US1] Add stale detection to worker poll cycle: check for `in_progress` responses with expired heartbeat, mark as `failed` in `pkg/engine/background.go`
- [ ] T021 [US1] Add `--mode` flag to server binary, wire gateway/worker/integrated startup in `cmd/server/main.go`
- [ ] T022 [US1] Integration test: submit background request in integrated mode, poll until completed, verify output matches synchronous equivalent in `test/integration/background_test.go`

**Checkpoint**: Core background mode works end-to-end in integrated mode. Client can submit, poll, and retrieve completed background responses.

---

## Phase 4: User Story 2 - Distributed Worker Processing (Priority: P1)

**Goal**: Gateway and worker run as separate processes sharing PostgreSQL. Background requests flow from gateway to worker.

**Independent Test**: Start gateway and worker as separate processes with shared PostgreSQL. Submit background request to gateway, verify worker picks it up and processes to completion.

### Implementation for User Story 2

- [ ] T023 [US2] Implement gateway-only mode: serve HTTP, queue background requests, skip worker startup in `cmd/server/main.go`
- [ ] T024 [US2] Implement worker-only mode: start worker loop, skip HTTP server, connect to shared storage in `cmd/server/main.go`
- [ ] T025 [US2] Add worker ID generation (unique per process, used for claim tracking) in `pkg/engine/background.go`
- [ ] T026 [US2] Integration test: start gateway and worker as separate subprocesses with PostgreSQL, submit background request to gateway, poll until worker completes it in `test/integration/background_distributed_test.go`

**Checkpoint**: Distributed architecture works. Gateway and worker are independently scalable processes.

---

## Phase 5: User Story 3 - Background Request Cancellation (Priority: P2)

**Goal**: Client cancels queued or in-progress background requests via DELETE

**Independent Test**: Submit background request, send DELETE, verify status transitions to `cancelled`. For in-progress requests, verify processing stops.

### Implementation for User Story 3

- [ ] T027 [US3] Enhance DELETE handler: detect background response, update status to `cancelled` instead of soft-delete for `queued`/`in_progress` responses in `pkg/transport/http/responses.go`
- [ ] T028 [US3] Add cancellation detection to worker: check response status during processing loop, abort if `cancelled` in `pkg/engine/background.go`
- [ ] T029 [US3] Add in-process cancellation registry for integrated mode: store cancel functions keyed by response ID, invoke on DELETE in `pkg/engine/background.go`
- [ ] T030 [US3] Integration test: submit background request, cancel while queued and while in-progress, verify `cancelled` status in `test/integration/background_test.go`

**Checkpoint**: Cancellation works for both queued and in-progress background requests.

---

## Phase 6: User Story 4 - Background Request Listing and Filtering (Priority: P2)

**Goal**: List endpoint supports filtering by status and background flag

**Independent Test**: Submit multiple background and synchronous requests, verify list endpoint returns correct filtered results.

### Implementation for User Story 4

- [ ] T031 [P] [US4] Parse `status` and `background` query parameters in list handler in `pkg/transport/http/adapter.go`
- [ ] T032 [US4] Integration test: create mixed responses, verify `?status=queued` and `?background=true` filters return correct results in `test/integration/background_test.go`

**Checkpoint**: Operators can monitor background request state via the list endpoint.

---

## Phase 7: User Story 5 - Graceful Shutdown with Background Drain (Priority: P3)

**Goal**: Workers drain in-flight requests on SIGTERM, mark undrained as failed

**Independent Test**: Start worker, submit long-running background request, send SIGTERM, verify request completes or is marked failed with shutdown reason.

### Implementation for User Story 5

- [ ] T033 [US5] Implement graceful drain in worker: on shutdown signal, stop polling, wait for in-flight requests up to drain timeout, mark remaining as `failed` with reason in `pkg/engine/background.go`
- [ ] T034 [US5] Wire drain timeout from config (`engine.background.drain_timeout`) in `cmd/server/main.go`
- [ ] T035 [US5] Integration test: start worker, submit slow background request, send SIGTERM, verify drain behavior in `test/integration/background_test.go`

**Checkpoint**: Workers shut down gracefully without orphaning background requests.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: TTL cleanup, audit logging, documentation, and quickstart

- [ ] T036 [P] Add TTL cleanup to worker poll cycle: delete terminal background responses older than configured TTL in `pkg/engine/background.go`
- [ ] T037 [P] Add audit logging for background lifecycle events (queued, started, completed, failed, cancelled) in `pkg/engine/background.go`
- [ ] T038 [P] Create background mode API reference documentation in `docs/modules/reference/pages/background-responses.adoc`
- [ ] T039 [P] Create background mode tutorial in `docs/modules/tutorial/pages/background-mode.adoc`
- [ ] T040 [P] Create gateway+worker quickstart with kustomize manifests in `quickstarts/09-background/`
- [ ] T041 [P] Update config-reference.adoc and environment-variables.adoc with new background settings in `docs/modules/reference/pages/`
- [ ] T042 Update README.md with spec 044 in the spec table

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (types and config must exist first)
- **US1 (Phase 3)**: Depends on Phase 2 (storage interface must be implemented)
- **US2 (Phase 4)**: Depends on Phase 3 (core background processing must work before distributing)
- **US3 (Phase 5)**: Depends on Phase 3 (need working background requests to cancel)
- **US4 (Phase 6)**: Depends on Phase 2 (list filtering is a storage-level feature)
- **US5 (Phase 7)**: Depends on Phase 3 (need worker loop to add drain behavior)
- **Polish (Phase 8)**: Depends on Phase 3 minimum, ideally all user stories complete

### User Story Dependencies

- **US1 (P1)**: Foundational -> US1 (core MVP)
- **US2 (P1)**: US1 -> US2 (distributed requires core working first)
- **US3 (P2)**: US1 -> US3 (cancel requires background requests to exist)
- **US4 (P2)**: Foundational -> US4 (list filtering is independent of processing)
- **US5 (P3)**: US1 -> US5 (drain requires worker loop)

### Parallel Opportunities

After Phase 2 completes:
- US4 (list filtering) can run in parallel with US1 (different files)
- After US1 completes: US2, US3, US5 can proceed in parallel
- All Polish tasks (T036-T042) are parallelizable

---

## Parallel Example: Phase 2 (Foundational)

```bash
# After T005-T006 (interface changes), these can all run in parallel:
Task T007: "Implement UpdateResponse in memory store"
Task T008: "Implement ClaimQueuedResponse in memory store"
Task T009: "Implement CleanupExpired in memory store"
Task T010: "Implement ListResponses filtering in memory store"
Task T011: "Create PostgreSQL migration 003_add_background.sql"
Task T012: "Implement UpdateResponse in PostgreSQL store"
Task T013: "Implement ClaimQueuedResponse in PostgreSQL store"
Task T014: "Implement CleanupExpired in PostgreSQL store"
Task T015: "Implement ListResponses filtering in PostgreSQL store"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T004)
2. Complete Phase 2: Foundational (T005-T015)
3. Complete Phase 3: User Story 1 (T016-T022)
4. **STOP and VALIDATE**: Test background mode in integrated mode
5. Deploy/demo if ready

### Incremental Delivery

1. Setup + Foundational -> Foundation ready
2. US1 (fire-and-forget) -> Test independently -> Deploy (MVP!)
3. US2 (distributed workers) -> Test with separate processes -> Deploy
4. US3 (cancellation) + US4 (filtering) -> Test independently -> Deploy
5. US5 (graceful shutdown) -> Test drain behavior -> Deploy
6. Polish (docs, audit, TTL cleanup) -> Final release

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently


## Beads Task Management

This project uses beads (`bd`) for persistent task tracking across sessions:
- Run `/sdd:beads-task-sync` to create bd issues from this file
- `bd ready --json` returns unblocked tasks (dependencies resolved)
- `bd close <id>` marks a task complete (use `-r "reason"` for close reason, NOT `--comment`)
- `bd comments add <id> "text"` adds a detailed comment to an issue
- `bd sync` persists state to git
- `bd create "DISCOVERED: [short title]" --labels discovered` tracks new work
  - Keep titles crisp (under 80 chars); add details via `bd comments add <id> "details"`
- Run `/sdd:beads-task-sync --reverse` to update checkboxes from bd state
- **Always use `jq` to parse bd JSON output, NEVER inline Python one-liners**
