# Tasks: E2E Testing with LLM Recording/Replay

**Input**: Design documents from `/specs/043-e2e-testing/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md

**Tests**: The feature IS the test infrastructure. Tests are inherent to every task.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create project structure, add dependencies, set up recording format

- [x] T001 Add `github.com/openai/openai-go` dependency to `go.mod` via `go get github.com/openai/openai-go`. This is a test-only dependency used by E2E tests.
- [x] T002 Create `test/e2e/` directory structure with `e2e_test.go` containing `TestMain`, environment variable parsing (ANTWORT_BASE_URL, ANTWORT_API_KEY, ANTWORT_ALICE_KEY, ANTWORT_BOB_KEY, ANTWORT_MODEL, ANTWORT_AUDIT_FILE), and openai-go client factory helpers. Include build tag `//go:build e2e` so these tests don't run with `go test ./...` (only via explicit `go test -tags e2e ./test/e2e/`).
- [x] T003 Create `test/e2e/recordings/README.md` documenting the recording JSON format (request, response, streaming, chunks, metadata fields) and how to create new recordings.

**Checkpoint**: E2E test skeleton exists, openai-go SDK available, recording format documented.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Implement replay mode in mock-backend, the foundation all E2E tests depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [x] T004 Add request normalization and hashing to `cmd/mock-backend/main.go`: create `normalizeRequest(method, path string, body []byte) string` that sorts JSON keys recursively, strips `stream_options`, and returns SHA256 hex hash of `method + "\n" + path + "\n" + normalized_body`.
- [x] T005 Add recording file loader to `cmd/mock-backend/main.go`: create `loadRecordings(dir string) (map[string]*Recording, error)` that reads all `*.json` files from the directory, parses them into Recording structs, and indexes by request hash. Recording struct has Request, Response, Streaming, Chunks, and Metadata fields.
- [x] T006 Add replay request handler to `cmd/mock-backend/main.go`: when `--recordings-dir` flag is set, intercept all requests to `/v1/chat/completions` and `/v1/responses`. Compute request hash, look up recording, return stored response. For streaming recordings, write SSE chunks with 1ms delay between each. Return 500 with diagnostic JSON (hash, available hashes) on miss.
- [x] T007 Add `--recordings-dir`, `--mode`, and `--record-target` CLI flags to `cmd/mock-backend/main.go`. When `--recordings-dir` is empty, use existing deterministic mock behavior (backward compatible). Parse flags using `flag` package.
- [x] T008 Add record mode to `cmd/mock-backend/main.go`: when `--mode record` or `--mode record-if-missing`, forward requests to `--record-target` URL, capture the response, save as JSON recording file in `--recordings-dir`. For streaming responses, buffer all SSE chunks before saving.
- [x] T009 Write tests for replay logic in `cmd/mock-backend/main_test.go`: test request normalization (key sorting, stream_options removal), hash computation (deterministic for same input), recording loading (valid JSON, corrupt file, empty dir), replay matching (hit, miss, streaming), and backward compatibility (no recordings-dir = deterministic mock).
- [x] T010 Handcraft initial recording files in `test/e2e/recordings/` based on the existing mock-backend response format (see `cmd/mock-backend/main.go` response structures). Create at minimum: `chat-basic.json` (non-streaming Chat Completion with model, choices, usage), `chat-streaming.json` (streaming with SSE chunks including role, content deltas, finish, and [DONE]), `chat-tool-call.json` (tool call turn with get_weather function), `chat-tool-result.json` (final text response after tool result). Use the recording format from `test/e2e/recordings/README.md`. Llama-stack conversion (T027) is a separate, later task.

**Checkpoint**: Mock-backend supports replay mode. Recordings exist. Can start the replay backend and get deterministic LLM responses.

---

## Phase 3: User Story 5 - Replay Backend Verification (Priority: P1) MVP

**Goal**: Verify the replay backend works correctly with both protocols and both modes (streaming/non-streaming).

**Independent Test**: Start replay backend with recordings, send requests, verify correct responses returned.

### Implementation for User Story 5

- [x] T011 [US5] Write `test/e2e/replay_test.go` with tests that start the replay backend in-process (using `httptest.NewServer`) and verify: non-streaming replay returns correct response body, streaming replay returns correct SSE chunks, missing recording returns 500 with diagnostic info, no recordings-dir falls back to deterministic mock.
- [x] T012 [US5] Add a Responses API recording (`test/e2e/recordings/responses-api-basic.json`) in the Responses API format and verify the replay backend serves it correctly for requests to `/v1/responses`.
- [x] T013 [US5] Verify recording-if-missing mode works: start with partial recordings, send both known and unknown requests (pointing at a test HTTP server as record-target), verify known requests are replayed and unknown requests are recorded to new files.

**Checkpoint**: Replay backend is fully functional and tested for both protocols.

---

## Phase 4: User Story 1 - Core API E2E Tests (Priority: P1)

**Goal**: Verify core response lifecycle (create, stream, get, list, delete) through the deployed stack using openai-go SDK.

**Independent Test**: Deploy antwort + replay backend, run SDK-based tests.

### Implementation for User Story 1

- [x] T014 [US1] Write `test/e2e/responses_test.go` with `TestE2ECreateResponse`: use openai-go SDK to POST a non-streaming response, verify the response has a valid ID, model, output text, and usage matching the recording.
- [x] T015 [US1] Add `TestE2EStreamingResponse` to `test/e2e/responses_test.go`: use openai-go SDK streaming API, collect all events, verify complete SSE lifecycle (response.created, text deltas, response.completed) and final text content.
- [x] T016 [US1] Add `TestE2EGetResponse` to `test/e2e/responses_test.go`: create a response, then GET it by ID, verify the stored response matches.
- [x] T017 [US1] Add `TestE2EListResponses` to `test/e2e/responses_test.go`: create multiple responses, list them, verify count and ordering.
- [x] T018 [US1] Add `TestE2EDeleteResponse` to `test/e2e/responses_test.go`: create a response, delete it, verify GET returns not found.

**Checkpoint**: Core API lifecycle works end-to-end through SDK.

---

## Phase 5: User Story 2 - Authentication and Ownership E2E Tests (Priority: P1)

**Goal**: Verify API key auth and per-user resource isolation in a deployed environment.

**Independent Test**: Deploy antwort with auth configured, run tests as different users.

### Implementation for User Story 2

- [x] T019 [US2] Write `test/e2e/auth_test.go` with `TestE2EAuthAccepted`: create openai-go client with valid API key, make a request, verify 200 success.
- [x] T020 [US2] Add `TestE2EAuthRejected` to `test/e2e/auth_test.go`: create client with invalid API key, verify request is rejected.
- [x] T021 [US2] Add `TestE2EOwnershipIsolation` to `test/e2e/auth_test.go`: alice creates a response, bob tries to GET it (should get not found), alice GETs it (should succeed).

**Checkpoint**: Multi-user auth and isolation work end-to-end.

---

## Phase 6: User Story 3 - Agentic Loop E2E Tests (Priority: P1)

**Goal**: Verify multi-turn tool calling with replayed LLM responses.

**Independent Test**: Deploy antwort with tool executor and replay backend, run tool call scenario.

### Implementation for User Story 3

- [x] T022 [US3] Write `test/e2e/agentic_test.go` with `TestE2EToolCallNonStreaming`: send a request with tools configured that triggers a tool call recording, verify the final response includes both the tool call output and the LLM's final text response.
- [x] T023 [US3] Add `TestE2EToolCallStreaming` to `test/e2e/agentic_test.go`: same scenario with streaming enabled, verify tool lifecycle SSE events are received.

**Checkpoint**: Agentic loop with tool calling works end-to-end.

---

## Phase 7: User Story 4 - Audit Verification E2E Tests (Priority: P2)

**Goal**: Verify audit events are emitted in a deployed environment.

**Independent Test**: Deploy antwort with audit logging to file, perform operations, read audit file.

### Implementation for User Story 4

- [x] T024 [US4] Write `test/e2e/audit_test.go` with `TestE2EAuditEvents`: make authenticated requests (create, delete), then retrieve the audit log via `kubectl exec <pod> -- cat /tmp/audit.log` (use ANTWORT_POD_NAME env var or discover via `kubectl get pods -l app.kubernetes.io/name=antwort`). Parse JSON lines, verify auth.success and resource.created/resource.deleted events with correct fields. For local dev mode (no cluster), read the file directly from ANTWORT_AUDIT_FILE path.
- [x] T025 [US4] Add `TestE2EAuditAuthFailure` to `test/e2e/audit_test.go`: send request with invalid key, retrieve audit log via same mechanism as T024, verify auth.failure event.

**Checkpoint**: Audit logging works in deployed environment.

---

## Phase 8: User Story 6 - Recording New Interactions (Priority: P3)

**Goal**: Developers can record new LLM interactions for test scenarios.

**Independent Test**: Start mock-backend in record mode, make requests, verify recording files created.

### Implementation for User Story 6

- [x] T026 [US6] Write recording integration test in `cmd/mock-backend/main_test.go`: start mock-backend in record mode pointing at a test HTTP server, send requests, verify JSON recording files are created in the recordings directory with correct format and content.
- [x] T027 [US6] Create `scripts/convert-llamastack-recordings.go`: reads llama-stack recording JSON files, strips `__type__`/`__data__` wrappers, reconstructs SSE chunks for streaming responses, outputs antwort recording format. Filters for Chat Completions format only.

**Checkpoint**: Recording and conversion tools work.

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: CI pipeline integration, kustomize overlay, documentation

- [x] T028 [P] Create `quickstarts/01-minimal/e2e/kustomization.yaml` extending the `ci` overlay: add auth configuration (API keys for alice/bob), audit logging configuration (enabled, JSON format, file output), and replay-backend recordings mount.
- [x] T029 [P] Update `.github/workflows/ci.yml` to extend the `kubernetes` job with E2E test steps: deploy using e2e overlay, wait for readiness, run `go test -tags e2e ./test/e2e/ -v` with port-forward and appropriate env vars.
- [x] T030 [P] Add `make e2e` target to `Makefile` for local E2E test execution: starts replay-backend with recordings, starts antwort with auth+audit config, runs E2E tests, cleans up.
- [x] T031 Update `Containerfile.mock` to support the new `--recordings-dir` flag: ensure the distroless image can access a mounted recordings directory. No code changes needed if recordings are mounted at runtime.
- [x] T032 Run full CI pipeline locally (or validate existing tests still pass): `go test ./pkg/... ./test/integration/... -timeout 120s` to verify no regressions from the openai-go dependency addition.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (T001 for openai-go dep)
- **US5 Replay Backend (Phase 3)**: Depends on Phase 2 (T004-T010)
- **US1 Core API (Phase 4)**: Depends on Phase 2 (replay backend working) + Phase 1 (SDK)
- **US2 Auth (Phase 5)**: Depends on Phase 2 + US1 (test helpers)
- **US3 Agentic (Phase 6)**: Depends on Phase 2 + US1 (test helpers) + tool call recordings (T010)
- **US4 Audit (Phase 7)**: Depends on Phase 2 + US1 (test helpers) + audit config
- **US6 Recording (Phase 8)**: Depends on Phase 2 (record mode in T008)
- **Polish (Phase 9)**: Depends on all user stories being complete

### User Story Dependencies

- **US5 (P1)**: After Foundational. Tests replay backend in isolation.
- **US1 (P1)**: After Foundational. Core API tests.
- **US2 (P1)**: After US1 (reuses test helpers). Auth tests.
- **US3 (P1)**: After US1 (reuses test helpers). Agentic tests.
- **US4 (P2)**: After US1 (reuses test helpers). Audit tests.
- **US6 (P3)**: After Foundational. Recording/conversion tools.

### Parallel Opportunities

- T001, T002, T003 all parallel (different files)
- T004, T005 parallel (different functions in same file, but can be developed together)
- T014-T018 all parallel (different test functions)
- T019-T021 all parallel (different test functions)
- T028, T029, T030 all parallel (different files)
- US1, US2, US3 can proceed in parallel after Foundational (different test files)

---

## Implementation Strategy

### MVP First (US5 + US1 Only)

1. Complete Phase 1: Setup (T001-T003)
2. Complete Phase 2: Foundational (T004-T010)
3. Complete Phase 3: US5 - Replay backend verification (T011-T013)
4. Complete Phase 4: US1 - Core API tests (T014-T018)
5. **STOP and VALIDATE**: Replay backend works, core API tests pass
6. This is a usable E2E test infrastructure

### Incremental Delivery

1. Setup + Foundational -> Replay backend ready
2. US5 -> Replay verified
3. US1 -> Core API E2E tests passing
4. US2 -> Auth E2E tests
5. US3 -> Agentic loop E2E tests
6. US4 -> Audit E2E tests
7. US6 -> Recording tools
8. Polish -> CI integration, local dev workflow

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- E2E tests use build tag `//go:build e2e` to avoid running with `go test ./...`
- All E2E tests depend on the replay backend being reachable via ANTWORT_BASE_URL
- Recordings are handcrafted or converted from llama-stack for Phase 1
- 32 total tasks across 9 phases

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
