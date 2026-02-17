# Tasks: Transport Layer

**Input**: Design documents from `/specs/002-transport-layer/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Included. The spec requires testable acceptance scenarios and the plan includes test files for each source file.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create package structure and package documentation

- [ ] T001 (antwort-dcb.1) Create `pkg/transport/` and `pkg/transport/http/` directory structure per plan.md
- [ ] T002 (antwort-dcb.2) Create `pkg/transport/doc.go` with package documentation describing the transport layer, handler interfaces, middleware chain, zero-dependency constraint, and relationship to Spec 001

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core interfaces and utilities used by ALL user stories. Must complete before any story work.

- [ ] T003 (antwort-pdm.1) [P] Implement handler interfaces in `pkg/transport/handler.go`: `ResponseCreator` interface with `CreateResponse(ctx context.Context, req *api.CreateResponseRequest, w ResponseWriter) error`. `ResponseStore` interface with `GetResponse(ctx context.Context, id string) (*api.Response, error)` and `DeleteResponse(ctx context.Context, id string) error`. `ResponseWriter` interface with `WriteEvent(ctx context.Context, event api.StreamEvent) error`, `WriteResponse(ctx context.Context, resp *api.Response) error`, and `Flush() error`. Include `ResponseCreatorFunc` adapter type for convenient function-to-interface conversion. (FR-017, FR-018, FR-020)
- [ ] T004 (antwort-pdm.2) [P] Implement in-flight registry in `pkg/transport/inflight.go`: `InFlightRegistry` struct with `sync.Mutex`-protected `map[string]context.CancelFunc`. Methods: `Register(id string, cancel context.CancelFunc)`, `Cancel(id string) bool`, `Remove(id string)`. Must be safe for concurrent access. (FR-023)
- [ ] T005 (antwort-pdm.3) [P] Implement error mapping helper in `pkg/transport/errors.go`: `HTTPStatusFromError(err *api.APIError) int` function mapping `invalid_request` to 400, `not_found` to 404, `too_many_requests` to 429, `server_error` and `model_error` to 500. `WriteErrorResponse(w http.ResponseWriter, err *api.APIError, statusCode int)` helper that serializes `api.ErrorResponse` to JSON. (FR-013, FR-014)
- [ ] T006 (antwort-pdm.4) [P] Implement handler interface tests in `pkg/transport/handler_test.go`: verify `ResponseCreatorFunc` adapter works correctly, verify interface satisfaction with mock implementations.
- [ ] T007 (antwort-pdm.5) [P] Implement in-flight registry tests in `pkg/transport/inflight_test.go`: table-driven tests for Register/Cancel/Remove, concurrent access test with goroutines, verify Cancel returns true for registered IDs and false for unknown IDs.
- [ ] T008 (antwort-pdm.6) [P] Implement error mapping tests in `pkg/transport/errors_test.go`: table-driven tests for all five error types mapping to correct HTTP status codes, verify WriteErrorResponse produces correct JSON output matching ErrorResponse format.

**Checkpoint**: Handler interfaces, in-flight registry, and error mapping ready. All user stories can now proceed.

---

## Phase 3: User Story 1 - Non-Streaming Request/Response (Priority: P1) 🎯 MVP

**Goal**: A developer can send a POST request with `stream: false` and receive a JSON response. Error mapping and request validation (JSON parsing, body size, content type) work correctly.

**Independent Test**: Send HTTP requests to a test server with a mock ResponseCreator, verify correct HTTP status codes, content types, and response bodies.

### Implementation for User Story 1

- [ ] T009 (antwort-fwe.1) [US1] Implement SSE ResponseWriter in `pkg/transport/http/sse.go`: struct wrapping `http.ResponseWriter` and `http.NewResponseController`. Tracks state (idle/streaming/completed). `WriteEvent` serializes `api.StreamEvent` as `event: {type}\ndata: {json}\n\n`, detects terminal events and sends `data: [DONE]\n\n`. `WriteResponse` writes complete JSON response. `Flush` delegates to ResponseController. Enforces mutual exclusion between WriteEvent and WriteResponse. Returns error after terminal event. (FR-007, FR-008, FR-009, FR-010, FR-011, FR-012, FR-020)
- [ ] T010 (antwort-fwe.2) [US1] Implement HTTP adapter in `pkg/transport/http/adapter.go`: `Adapter` struct accepting `ResponseCreator` (required) and optional `ResponseStore`. `ServerConfig` with `Addr`, `MaxBodySize`, `ShutdownTimeout` fields and defaults (`:8080`, 10MB, 30s). Register routes using Go 1.22 `http.ServeMux`: `POST /v1/responses`, `GET /v1/responses/{id}`, `DELETE /v1/responses/{id}`. POST handler: enforce Content-Type `application/json` (FR-006), wrap body with `http.MaxBytesReader` (FR-005), decode JSON with `json.NewDecoder` (FR-004), check `req.Stream` to create SSE or JSON ResponseWriter, dispatch to ResponseCreator. Handle `*http.MaxBytesError` as 413 and JSON parse errors as 400. Map handler `*api.APIError` to HTTP status codes using error mapping helper. GET/DELETE handlers: return error if no ResponseStore configured (FR-019), extract ID via `r.PathValue("id")`, dispatch to store. 405 handling is automatic via ServeMux. (FR-001 through FR-006a, FR-007, FR-013, FR-014, FR-015, FR-019)
- [ ] T011 (antwort-fwe.3) [US1] Implement SSE ResponseWriter tests in `pkg/transport/http/sse_test.go`: test WriteResponse produces correct JSON with `Content-Type: application/json`. Test WriteEvent produces correct SSE format (`event: {type}\ndata: {json}\n\n`). Test terminal event followed by `data: [DONE]\n\n`. Test WriteEvent after terminal returns error. Test WriteResponse after WriteEvent returns error (mutual exclusion). Test Flush delegates correctly. Test SSE headers (Cache-Control, Connection). (SC-001, SC-003)
- [ ] T012 (antwort-fwe.4) [US1] Implement HTTP adapter tests in `pkg/transport/http/adapter_test.go`: using `httptest.NewServer` with mock ResponseCreator. Test valid non-streaming POST returns 200 with JSON. Test invalid JSON body returns 400. Test oversized body returns 413. Test wrong Content-Type returns 415. Test unknown path returns 404. Test handler error mapping (all 5 error types to correct HTTP status). Test GET/DELETE without store returns error. Test method not allowed (PUT on /v1/responses). (SC-001, SC-002)

**Checkpoint**: Non-streaming requests work end-to-end. Error handling covers all transport-level and handler-level error types.

---

## Phase 4: User Story 2 - SSE Streaming (Priority: P1)

**Goal**: A developer can send a POST request with `stream: true` and receive Server-Sent Events. Events are flushed immediately. Client disconnect is detected and cancels the handler context.

**Independent Test**: Send streaming request to test server with mock handler emitting events, verify SSE wire format, [DONE] sentinel, and client disconnect detection.

### Implementation for User Story 2

- [ ] T013 (antwort-ofx.1) [US2] Implement streaming request handling in `pkg/transport/http/adapter.go`: extend the POST handler to detect `req.Stream == true`, create SSE ResponseWriter with proper headers, set up `context.WithCancel` derived from the request context, register in the in-flight registry (extracting response ID from the first `response.created` event), defer registry cleanup. Handle the case where the handler returns an error before any events are written (FR-015) vs after streaming has begun (FR-016). (FR-008, FR-010, FR-011, FR-012, FR-015, FR-016, FR-021)
- [ ] T014 (antwort-ofx.2) [US2] Implement streaming tests in `pkg/transport/http/adapter_test.go` (append to existing): test streaming POST returns `Content-Type: text/event-stream`. Test mock handler emitting multiple delta events produces correct SSE output. Test terminal event followed by `data: [DONE]\n\n`. Test handler error before any events returns JSON error (not SSE). Test handler error after streaming begins sends `response.failed` event. Test client disconnect cancels handler context (using a channel-coordinated mock). (SC-001, SC-003, SC-004)

**Checkpoint**: Both streaming and non-streaming paths work. SSE wire format matches the contract in `contracts/sse-wire-format.md`.

---

## Phase 5: User Story 3 - Retrieve and Delete Stored Responses (Priority: P2)

**Goal**: GET and DELETE endpoints work with a mock response store. DELETE checks the in-flight registry first, then falls through to the store.

**Independent Test**: Send GET and DELETE requests with a mock ResponseStore, verify correct responses and status codes.

### Implementation for User Story 3

- [ ] T015 (antwort-06j.1) [US3] Implement GET and DELETE handlers in `pkg/transport/http/adapter.go`: GET handler extracts ID from path, validates response ID format using `api.ValidateResponseID` (returns 400 for malformed IDs), delegates to `ResponseStore.GetResponse`, returns JSON response or error. DELETE handler validates ID format, then checks `InFlightRegistry.Cancel(id)` (if found, return success); otherwise delegates to `ResponseStore.DeleteResponse`. Returns 204 on successful deletion. Returns 404 for not found. (FR-002, FR-003)
- [ ] T016 (antwort-06j.2) [US3] Implement GET/DELETE tests in `pkg/transport/http/adapter_test.go` (append): test GET returns stored response. Test GET with unknown ID returns 404. Test GET with malformed ID returns 400. Test DELETE returns 204 on success. Test DELETE with unknown ID returns 404. Test DELETE with malformed ID returns 400. Test DELETE checks in-flight registry before store. (SC-002)

**Checkpoint**: Full CRUD surface for the OpenResponses API works with mock implementations.

---

## Phase 6: User Story 4 - Middleware Chain (Priority: P2)

**Goal**: Recovery, request ID, and logging middleware work in the correct order. Custom middleware can be added.

**Independent Test**: Configure middleware, send requests, verify panic recovery returns 500, request ID appears in headers and context, and log entries contain expected fields.

### Implementation for User Story 4

- [ ] T017 (antwort-9uc.1) [US4] Implement middleware chain in `pkg/transport/middleware.go`: `Middleware` type as `func(ResponseCreator) ResponseCreator`. `Chain(middlewares ...Middleware) Middleware` function that composes middleware in order. `DefaultMiddleware()` function returning the standard chain (recovery, requestID, logging). Context key types for request ID. (FR-024, FR-028)
- [ ] T018 (antwort-9uc.2) [P] [US4] Implement recovery middleware in `pkg/transport/recovery.go`: catches panics via `defer recover()`, returns HTTP 500 with `api.NewServerError("internal server error")`. Must work for both streaming and non-streaming paths. For streaming, if panic occurs after headers sent, close the connection. (FR-025)
- [ ] T019 (antwort-9uc.3) [P] [US4] Implement request ID middleware in `pkg/transport/requestid.go`: check incoming `X-Request-ID` header; if present, use it; otherwise generate a new unique ID. Store in context via the context key from middleware.go. Add to response headers. (FR-026)
- [ ] T020 (antwort-9uc.4) [P] [US4] Implement logging middleware in `pkg/transport/logging.go`: use `log/slog` to emit structured log entry after request completes. Fields: method, path, status code, duration, request ID. Use the request ID from context (set by request ID middleware). (FR-027)
- [ ] T021 (antwort-9uc.5) [US4] Wire middleware into the HTTP adapter in `pkg/transport/http/adapter.go`: apply middleware chain to the ResponseCreator before routing. Ensure middleware wraps the create handler but GET/DELETE handlers also benefit from HTTP-level request ID and logging. (FR-028)
- [ ] T022 (antwort-9uc.6) [US4] Implement middleware tests in `pkg/transport/middleware_test.go`: test recovery catches panic and returns error. Test request ID generates unique IDs. Test request ID propagates existing header. Test logging emits expected fields (use a custom slog handler to capture output). Test chain applies middleware in correct order using a recording middleware. (SC-005, SC-007)

**Checkpoint**: All built-in middleware works. Requests have traceable IDs in headers and logs.

---

## Phase 7: User Story 5 - Cancel In-Flight Streaming (Priority: P3)

**Goal**: DELETE request cancels an in-flight streaming response. The stream terminates with a `response.cancelled` event.

**Independent Test**: Start a slow streaming response, send DELETE to cancel it, verify context cancellation and stream termination.

### Implementation for User Story 5

- [ ] T023 (antwort-hqo.1) [US5] Implement cancellation integration in `pkg/transport/http/adapter.go`: ensure the streaming POST handler registers the response ID in the InFlightRegistry after the first `response.created` event is received. Ensure the DELETE handler checks the registry before the store. When cancelled, the handler's context is cancelled, causing it to stop and (optionally) emit a `response.cancelled` event. The adapter detects this via the context and sends `data: [DONE]\n\n`. (FR-023)
- [ ] T024 (antwort-hqo.2) [US5] Implement cancellation tests in `pkg/transport/http/adapter_test.go` (append): test DELETE during streaming cancels handler context. Test cancelled stream terminates with `response.cancelled` event and `[DONE]`. Test DELETE for non-in-flight ID falls through to store. Test concurrent cancel and stream completion (race condition handling). (SC-004)

**Checkpoint**: Explicit cancellation works end-to-end.

---

## Phase 8: Server Lifecycle

**Purpose**: Server startup and graceful shutdown

- [ ] T025 (antwort-gnk.1) Implement server lifecycle in `pkg/transport/server.go`: `Server` struct wrapping `http.Server` with `ServerConfig`. `NewServer(opts ...Option)` constructor with functional options (`WithAddr`, `WithCreator`, `WithStore`, `WithMaxBodySize`, `WithShutdownTimeout`, `WithMiddleware`). `ListenAndServe()` method that runs server in goroutine, listens for SIGINT/SIGTERM via `signal.NotifyContext`, then calls `http.Server.Shutdown` with timeout context. (FR-022)
- [ ] T026 (antwort-gnk.2) Implement server lifecycle tests in `pkg/transport/server_test.go`: test server starts and accepts requests. Test graceful shutdown completes in-flight requests. Test shutdown timeout is respected. Test functional options configure correctly. (SC-006)

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Package-level verification and documentation

- [ ] T027 (antwort-os0.1) Verify all tests pass with `go test -v -count=1 ./pkg/transport/...` and fix any failures
- [ ] T028 (antwort-os0.2) Run `go vet ./pkg/transport/...` and fix any issues
- [ ] T029 (antwort-os0.3) Validate SSE wire format against `specs/002-transport-layer/contracts/sse-wire-format.md` (manual review: event format, headers, [DONE] sentinel, error handling)
- [ ] T030 (antwort-os0.4) Run quickstart.md code examples mentally against the implemented API to verify accuracy, update quickstart.md if any function signatures changed during implementation

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Setup completion, BLOCKS all user stories
- **US1 (Phase 3)**: Depends on Foundational. BLOCKS US2 (streaming extends the POST handler and SSE writer)
- **US2 (Phase 4)**: Depends on US1 (extends adapter.go and sse.go with streaming logic)
- **US3 (Phase 5)**: Depends on US1 (appends handlers to adapter.go created in Phase 3)
- **US4 (Phase 6)**: Depends on Foundational. Can run in PARALLEL with US1/US2/US3 (middleware is a separate concern). Wiring into adapter (T021) depends on adapter existing.
- **US5 (Phase 7)**: Depends on US2 (needs streaming) and US3 (needs DELETE handler)
- **Server Lifecycle (Phase 8)**: Depends on US1 (needs adapter). Can run in parallel with US3, US4.
- **Polish (Phase 9)**: Depends on all phases complete

### User Story Dependencies

```
Phase 1 (Setup)
    |
    v
Phase 2 (Foundational: interfaces, registry, errors)
    |
    |
    v
Phase 3 (US1: non-streaming)
    |
    ├──────────────────┐
    v                  v
Phase 4 (US2: SSE)  Phase 5 (US3: GET/DELETE)
    |                  |
    ├────────────────────────────────────┘
    v
Phase 7 (US5: cancellation)

Phase 6 (US4: middleware) ← PARALLEL with US1-US3, wire after adapter exists
Phase 8 (Server lifecycle) ← PARALLEL with US3, US4, after US1

Phase 9 (Polish) ← after all
```

### Parallel Opportunities

Within Phase 2:
- T003 (interfaces), T004 (registry), T005 (errors) are independent files, run in parallel
- T006 (interface tests), T007 (registry tests), T008 (error tests) are independent, run in parallel

Within Phase 6 (US4):
- T017 (chain), T018 (recovery), T019 (request ID), T020 (logging) are independent middleware pieces

Across Phases:
- US3 (GET/DELETE) and US4 (middleware) can run in parallel with US1/US2
- Server lifecycle (Phase 8) can run in parallel with US3, US4

---

## Implementation Strategy

### MVP First (User Stories 1 + 2)

1. Complete Phase 1: Setup (T001-T002)
2. Complete Phase 2: Foundational (T003-T008)
3. Complete Phase 3: User Story 1 - non-streaming (T009-T012)
4. Complete Phase 4: User Story 2 - streaming (T013-T014)
5. **STOP and VALIDATE**: Test both streaming and non-streaming with mock handlers
6. This delivers a functional HTTP/SSE server that Spec 03 (Provider) can depend on

### Incremental Delivery

1. Setup + Foundational -> Core interfaces and utilities ready
2. Add US1 -> Non-streaming requests work (JSON round-trip validated)
3. Add US2 -> Streaming works (SSE wire format validated, enough for Spec 03)
4. Add US3 -> Full CRUD surface (GET/DELETE for stored responses)
5. Add US4 -> Production middleware (recovery, tracing, logging)
6. Add US5 -> Cancellation support
7. Add Server Lifecycle -> Production-ready startup/shutdown
8. Polish -> Final verification and documentation sync

### Single Developer Strategy

Execute phases sequentially: 1 -> 2 -> 3 -> 4 -> 5 -> 6 -> 7 -> 8 -> 9.
Total: 30 tasks across 9 phases.

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- All types (`CreateResponseRequest`, `Response`, `StreamEvent`, `APIError`, `ErrorResponse`) are reused from `pkg/api` (Spec 001), NOT redefined
- This package has ZERO external dependencies (Go stdlib only)
- Every acceptance scenario from spec.md maps to at least one test case
- Commit after each phase checkpoint

<!-- SDD-TRAIT:beads -->
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
