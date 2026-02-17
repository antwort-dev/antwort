# Plan Review Summary: Transport Layer (Spec 002)

**Reviewer**: Claude Code (automated plan review)
**Date**: 2026-02-17
**Status**: Ready for implementation with minor issues noted

---

## 1. Coverage Matrix

### Functional Requirements to Task Mapping

| Requirement | Description | Implementing Task(s) | Test Task(s) | Gaps |
|-------------|-------------|----------------------|--------------|------|
| FR-001 | Accept POST /v1/responses, deserialize to CreateResponseRequest | T010 | T012 | None |
| FR-002 | Accept GET /v1/responses/{id}, return stored response | T015 | T016 | None |
| FR-003 | Accept DELETE /v1/responses/{id}, check in-flight then store | T015, T023 | T016, T024 | None |
| FR-004 | Validate well-formed JSON before dispatch | T010 | T012 | None |
| FR-005 | Enforce configurable max request body size (HTTP 413) | T010 | T012 | None |
| FR-006 | Reject non-application/json Content-Type for POST (HTTP 415) | T010 | T012 | None |
| FR-006a | Return HTTP 405 for valid paths with unsupported methods | T010 (automatic via ServeMux) | T012 | None |
| FR-007 | Non-streaming returns JSON with application/json | T009 | T011, T012 | None |
| FR-008 | Streaming returns SSE with text/event-stream | T009, T013 | T011, T014 | None |
| FR-009 | SSE format: `event: {type}\ndata: {json}\n\n` | T009 | T011 | None |
| FR-010 | Send `data: [DONE]\n\n` after terminal event | T009 | T011, T014 | None |
| FR-011 | Flush each event immediately (no buffering) | T009, T013 | T011, T014 | None |
| FR-012 | SSE headers: Cache-Control, Connection | T009 | T011 | None |
| FR-013 | Error type to HTTP status code mapping | T005 | T008, T012 | None |
| FR-014 | All errors use ErrorResponse wrapper format | T005 | T008 | None |
| FR-015 | Error before streaming begins returns JSON (not SSE) | T013 | T014 | None |
| FR-016 | Error after streaming begins sends response.failed event | T013 | T014 | None |
| FR-017 | Define ResponseCreator interface | T003 | T006 | None |
| FR-018 | Define ResponseStore interface | T003 | T006 | None |
| FR-019 | Adapter accepts required ResponseCreator + optional ResponseStore | T010 | T012 | None |
| FR-020 | Define ResponseWriter with mutual exclusion semantics | T003, T009 | T006, T011 | None |
| FR-021 | Cancel handler context within 1s on client disconnect | T013 | T014 | See note [1] |
| FR-022 | Graceful shutdown with configurable deadline | T025 | T026 | None |
| FR-023 | In-flight registry for cancel functions | T004, T023 | T007, T024 | None |
| FR-024 | Define middleware mechanism wrapping ResponseCreator | T017 | T022 | None |
| FR-025 | Recovery middleware (panic to HTTP 500) | T018 | T022 | None |
| FR-026 | Request ID middleware (generate or propagate X-Request-ID) | T019 | T022 | None |
| FR-027 | Logging middleware (method, path, status, duration, request ID) | T020 | T022 | None |
| FR-028 | Middleware execution order: recovery, request ID, logging, custom | T017, T021 | T022 | None |

**Notes**:
- [1] FR-021 specifies "within 1 second" but no task explicitly describes testing this timing constraint. T014 tests that client disconnect cancels the handler context, but does not describe a timing assertion. This is a minor gap; the "within 1s" constraint is inherently satisfied by Go's `http.Request.Context()` mechanism (which cancels nearly instantly), so explicit timing tests are likely unnecessary in practice, but worth noting.

### Success Criteria Coverage

| Criterion | Description | Covered By |
|-----------|-------------|------------|
| SC-001 | OpenAI-compatible client sends/receives without modification | T011, T012, T014 |
| SC-002 | Error types map to correct HTTP status codes, ErrorResponse format | T008, T012, T016 |
| SC-003 | Events flushed immediately, no application-layer buffering | T011, T014 |
| SC-004 | Client disconnect detected, handler context cancelled within 1s | T014, T024 |
| SC-005 | Panic results in HTTP 500 (not dropped connection) | T022 |
| SC-006 | Graceful shutdown completes within deadline | T026 |
| SC-007 | Unique request ID in response headers and structured logs | T022 |

### Edge Cases Coverage

| Edge Case | Covered By |
|-----------|------------|
| Request body exceeds maximum size (413) | T010, T012 |
| Unsupported Content-Type (415) | T010, T012 |
| Graceful shutdown with in-flight streams | T025, T026 |
| WriteEvent fails mid-stream (client disconnect) | T009, T013 |
| Malformed response ID in GET/DELETE (400) | T015, T016 |
| Handler error before any events sent (JSON error, not SSE) | T013, T014 |

---

## 2. Task Quality Assessment

### Specificity for LLM Execution

**Overall**: Good. Tasks include enough context for an LLM to implement without ambiguity in most cases.

**Strengths**:
- Every task references exact file paths
- Interface signatures are spelled out in T003 (method names, parameter types, return types)
- FR references tie tasks back to requirements
- SSE wire format contract provides concrete examples of expected output

**Issues**:

1. **T013 (streaming request handling)**: References "extracting response ID from the first `response.created` event" but the streaming POST handler creates a `context.WithCancel` and registers in the in-flight registry. The problem is that the response ID is not known at the start of the POST handler; it is only known after the handler creates it and emits `response.created`. The task description says to register "after the first `response.created` event is received," but the adapter does not receive events (the handler writes them to the ResponseWriter). This creates an architectural ambiguity about how the adapter learns the response ID. The SSE ResponseWriter would need to intercept the first event and extract the ID, or the handler must communicate the ID back to the adapter through some other mechanism. **This is the single most important design gap in the plan.**

2. **T018 (recovery middleware)**: States "For streaming, if panic occurs after headers sent, close the connection" but does not detail how to detect whether headers have been sent. The ResponseWriter state machine (idle/streaming/completed) could be used, but the recovery middleware wraps `ResponseCreator`, not the HTTP handler, so it does not have direct access to the `http.ResponseWriter`. The interaction between recovery middleware and the SSE ResponseWriter needs clarification.

3. **T021 (wire middleware into adapter)**: States "GET/DELETE handlers also benefit from HTTP-level request ID and logging" but the middleware is defined as wrapping `ResponseCreator`, not `http.Handler`. The task does not explain how GET/DELETE handlers (which call `ResponseStore`, not `ResponseCreator`) get request ID and logging. This needs clarification. The plan.md mentions "HTTP-level middleware uses the standard `func(http.Handler) http.Handler` pattern" but the tasks do not clearly specify which middleware operates at which layer.

### File Path Coverage

All tasks include explicit file paths. Every source file and test file is accounted for.

### Dependency Ordering

Dependencies are correctly ordered with one exception:

- **T021 depends on T017-T020 AND on T010** (the adapter must exist before middleware is wired in). The dependency graph in the plan correctly notes this ("wire after adapter exists"), and T021 is sequenced after T017-T020 within Phase 6. However, T021 also modifies `adapter.go`, which was created in Phase 3 (T010). If Phase 6 runs in parallel with Phases 4/5 (which also modify `adapter.go`), there is a conflict. See Red Flag Scan below.

### Task Size

Most tasks are well-scoped. Two tasks are on the larger side:

- **T009**: Covers the entire SSE ResponseWriter (state machine, WriteEvent, WriteResponse, Flush, terminal detection, [DONE] sentinel, mutual exclusion, SSE headers). This maps to 7 FRs (FR-007 through FR-012, FR-020). Acceptable because these are all tightly coupled within a single struct, but it is a substantial implementation.
- **T010**: Covers the entire HTTP adapter (route registration, POST handler, GET handler, DELETE handler, content-type validation, body size limiting, JSON decoding, error mapping). This maps to 8 FRs. This is the largest single task. Consider whether the initial GET/DELETE stubs in T010 conflict with the "real" GET/DELETE implementation in T015.

---

## 3. Red Flag Scan

### Circular Dependencies

**None found.** The dependency graph is a clean DAG:
- `pkg/api` has no dependency on `pkg/transport`
- `pkg/transport` depends on `pkg/api` (types only)
- `pkg/transport/http` depends on `pkg/transport` (interfaces) and `pkg/api` (serialization)

### Same-File Conflicts

**Critical concern**: `pkg/transport/http/adapter.go` is written or modified by five tasks across four phases:

| Task | Phase | Action |
|------|-------|--------|
| T010 | Phase 3 (US1) | Create adapter.go with routes, POST handler, GET/DELETE stubs |
| T013 | Phase 4 (US2) | Extend POST handler with streaming logic |
| T015 | Phase 5 (US3) | Implement full GET/DELETE handlers |
| T021 | Phase 6 (US4) | Wire middleware into adapter |
| T023 | Phase 7 (US5) | Add cancellation integration |

The plan states that Phase 5 (US3) and Phase 6 (US4) can run in parallel. Both modify `adapter.go`. **This is a same-file conflict.** In practice, with a single developer executing sequentially, this is not a problem. But the [P] markings and dependency graph suggest parallelism is possible, which would cause merge conflicts or race conditions.

**Similarly**, `pkg/transport/http/adapter_test.go` is appended to by T012, T014, T016, and T024 across four phases. Same concern applies.

**Recommendation**: Remove the parallelism claim between Phase 5 and Phase 6 for any task touching `adapter.go`. Alternatively, clearly document that parallelism is only safe for the middleware implementation tasks (T017-T020) but that T021 must be sequenced after T015.

### Missing Test Coverage

1. **Graceful shutdown with active SSE streams**: T026 tests graceful shutdown with "in-flight requests" but does not specifically mention long-lived SSE connections. Graceful shutdown of SSE streams is a distinct concern (the server must signal handlers to stop, not just wait for short-lived requests to finish). This is mentioned in the edge cases but not explicitly covered by a test task.

2. **Concurrent cancel and stream completion race**: T024 mentions "Test concurrent cancel and stream completion (race condition handling)" which is good. However, there is no explicit test for what happens when a DELETE arrives after the response has completed but before the registry is cleaned up.

3. **HTTP/2 client disconnect detection**: The research.md notes an HTTP/2 race condition where `ctx.Err()` may briefly be nil after disconnect. No test covers this scenario.

### Parallelism Markings

- Within Phase 2, T003/T004/T005 are correctly marked [P] (independent files)
- Within Phase 2, T006/T007/T008 are correctly marked [P] (independent test files)
- Within Phase 6, T018/T019/T020 are correctly marked [P] (independent middleware files)
- T017 is NOT marked [P] but defines the context keys that T019 needs. Since T017 defines `Middleware` type and context keys in `middleware.go`, and T019 imports those keys, T017 should block T019. The current task ordering (T017 before T019) handles this implicitly.

---

## 4. NFR Validation

### Performance

- **Event flushing**: FR-011 and SC-003 require immediate flushing. T009 and T011 cover this. The implementation uses `http.NewResponseController.Flush()` which provides per-event flushing.
- **Body size limiting**: FR-005 uses `http.MaxBytesReader` (streaming, memory-efficient). Covered by T010.
- **No explicit performance benchmarks**: No tasks include benchmark tests (`go test -bench`). For a transport layer, benchmarks for event serialization throughput and concurrent connection handling would be valuable but are not strictly required at this stage.

### Reliability

- **Panic recovery**: FR-025, tested in T022. Covers both "returns 500" and "server continues accepting requests."
- **Graceful shutdown**: FR-022, implemented in T025, tested in T026.
- **Client disconnect detection**: FR-021, implemented in T013, tested in T014.
- **Write failure detection**: Edge case in spec, covered implicitly by SSE ResponseWriter's Flush error handling in T009.

### Observability

- **Request ID**: FR-026, implemented in T019, tested in T022. Propagated in headers and context.
- **Structured logging**: FR-027, implemented in T020, tested in T022. Uses `log/slog`.
- **No metrics**: Explicitly out of scope (Spec 07). Acceptable.

### Graceful Shutdown

- Implemented in T025 using `http.Server.Shutdown()` with configurable timeout.
- Tested in T026 (server starts, accepts requests, shuts down, respects timeout).
- **Gap**: No explicit test for graceful shutdown with active SSE streams. The `Shutdown()` method waits for active requests, but SSE connections are long-lived. Without explicit cancellation of SSE handler contexts during shutdown, the server would wait until the shutdown timeout expires. This interaction should be tested.

### Client Disconnect Detection

- Implemented via request context cancellation in T013.
- Tested in T014 with "channel-coordinated mock."
- The research notes the HTTP/2 caveat, but it is not testable in a unit test context.

---

## 5. Risk Assessment

### Risk 1: Response ID Registration Timing (High)

**Description**: The in-flight registry requires a response ID to register the cancel function, but the response ID is generated by the handler (not the adapter). The adapter does not know the response ID until the handler emits the first `response.created` event through the ResponseWriter. T023 states to register "after the first `response.created` event is received," but the architectural mechanism for the adapter to intercept or observe events written through the ResponseWriter is not designed.

**Impact**: If this is not resolved cleanly, the cancellation feature (US5) will require either (a) a callback from ResponseWriter to the adapter on first event, (b) the handler explicitly communicating the ID back, or (c) the adapter pre-generating the ID and passing it in. Each approach has different trade-offs.

**Mitigation**: Add a design decision or clarification in the plan. The cleanest approach is likely to have the SSE ResponseWriter accept a callback (`onResponseCreated func(id string)`) that the adapter provides when constructing the writer. The writer calls this callback when it sees the first `response.created` event.

### Risk 2: Middleware Layer Ambiguity (Medium)

**Description**: The plan defines two parallel middleware patterns: `func(ResponseCreator) ResponseCreator` for business-level middleware and `func(http.Handler) http.Handler` for HTTP-level middleware. The tasks do not clearly specify which middleware operates at which layer. Recovery, request ID, and logging middleware are described as wrapping `ResponseCreator`, but request ID needs to set HTTP response headers and logging needs the HTTP status code, neither of which is accessible through the `ResponseCreator` interface alone.

**Impact**: During implementation, the developer will need to make design decisions about middleware layering that are not fully specified in the tasks. This could lead to inconsistencies or rework.

**Mitigation**: Clarify in the data model or plan whether the built-in middleware (recovery, request ID, logging) operates at the HTTP handler level or the ResponseCreator level. The most natural approach is to have these three operate as `func(http.Handler) http.Handler` middleware (since they need HTTP-level access), with the `func(ResponseCreator) ResponseCreator` pattern reserved for business-level middleware added by later specs.

### Risk 3: Same-File Contention Across Phases (Low-Medium)

**Description**: `adapter.go` and `adapter_test.go` are modified by 5 and 4 tasks respectively across 4 phases. While sequential execution avoids conflicts, the plan suggests some phases can run in parallel, which would cause merge conflicts.

**Impact**: For a single developer, this is manageable (execute sequentially). For parallel execution or handoff between sessions, it creates friction.

**Mitigation**: Clearly mark all tasks modifying `adapter.go` as sequential with explicit dependency chains. Alternatively, restructure the adapter to separate the POST handler, GET handler, DELETE handler, and route registration into separate files.

---

## 6. Reviewer Guidance

When reviewing the implementation of this spec, focus on these areas:

### 1. SSE Wire Format Correctness
Verify the exact byte-level format of SSE output against `contracts/sse-wire-format.md`. Check: event type line format, data line format, blank line separators, `[DONE]` sentinel (no `event:` line), and terminal event detection. Test with a real SSE client library (not just string comparison) to catch subtle formatting issues.

### 2. ResponseWriter State Machine
The ResponseWriter has three states (idle, streaming, completed) with mutual exclusion between `WriteEvent` and `WriteResponse`. Verify that all invalid transitions return errors, that no data is written after a terminal event, and that the state transitions are thread-safe if the handler could call WriteEvent from multiple goroutines (though this is unlikely in practice).

### 3. Error Handling at Stream Boundaries
Pay special attention to the boundary between "error before streaming" (returns JSON 4xx/5xx) and "error after streaming has begun" (sends `response.failed` event via SSE). The adapter must track whether any SSE headers/events have been sent to choose the correct error format. Verify this works when the handler returns an error immediately vs. after several events.

### 4. In-Flight Registry and Cancellation Integration
Review the timing and lifecycle of registry entries. Verify that entries are registered at the right moment (after the response ID is known), cleaned up on normal completion, cleaned up on cancel, and that the race between cancel and completion is handled correctly with proper locking.

### 5. Middleware Layering
Verify that recovery middleware catches panics from all code paths (not just the handler), that request ID is available to all downstream middleware and the handler, and that logging captures the final HTTP status code accurately. Pay attention to whether middleware operates at the HTTP handler level or the ResponseCreator level, and whether the two layers interact correctly.

---

## 7. Overall Verdict

**Score: 88%**

**Verdict: Ready for implementation with two clarifications needed.**

The plan is thorough, well-structured, and covers all 28 functional requirements with clear task-to-requirement traceability. The phased approach with checkpoints is sound. The dependency graph is correct. Test coverage is comprehensive with explicit acceptance scenario mapping.

**What works well**:
- Every FR is mapped to at least one implementation task and one test task
- Clean separation of concerns (interfaces, adapter, SSE, middleware, server lifecycle)
- Phase dependency ordering is logical and supports incremental delivery
- Research decisions are well-justified with alternatives considered
- Wire format contract provides unambiguous SSE format specification
- Edge cases from the spec are all addressed in tasks

**Issues to address before implementation**:

1. **(Must fix)** Clarify the mechanism for the adapter to learn the response ID from the handler for in-flight registry registration (Risk 1). Add a design note or modify T009/T023 to specify the callback or interception pattern.

2. **(Should fix)** Clarify whether built-in middleware (recovery, request ID, logging) operates as `func(http.Handler) http.Handler` or `func(ResponseCreator) ResponseCreator`. T017-T020 describe them as ResponseCreator middleware, but they need HTTP-level access for headers and status codes. Add a design decision to the plan or data model (Risk 2).

3. **(Minor)** Remove or qualify the parallelism claim between Phase 5 and Phase 6, since both modify `adapter.go`. The sequential execution strategy already handles this, but the dependency diagram could confuse a multi-agent setup.

4. **(Minor)** Add a test case to T026 for graceful shutdown with active SSE streams, verifying that long-lived connections are properly terminated within the shutdown deadline.

**Bottom line**: The plan is well above the quality bar for implementation. The two "must/should fix" items are design clarifications that can be resolved at the start of implementation without restructuring the plan. No tasks need to be added or removed; the existing 30-task breakdown covers the spec completely.
