# Tasks: OpenResponses Conformance Testing

**Input**: Design documents from `/specs/006-conformance/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, quickstart.md

**Tests**: Conformance validation is the test. Go unit tests for server and mock components.

**Organization**: Tasks grouped by user story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: User Story 1 - Server Wiring Layer (Priority: P1) MVP

**Goal**: Run antwort as a standalone server that accepts OpenResponses API requests.

**Independent Test**: Start server, curl a request, get a valid response.

### Implementation for User Story 1

- [ ] T001 (antwort-5ia.1) [US1] Implement server binary in `cmd/server/main.go`: read env vars (ANTWORT_BACKEND_URL, ANTWORT_MODEL, ANTWORT_PORT, ANTWORT_STORAGE_DSN), create vLLM provider, optionally create in-memory or PostgreSQL store, create engine, create HTTP adapter, start server with graceful shutdown on SIGINT/SIGTERM. Include health endpoint at `/healthz` (FR-001, FR-002, FR-003, FR-004, FR-005).
- [ ] T002 (antwort-5ia.2) [US1] Write a simple Go integration test in `cmd/server/main_test.go` that starts the server with a mock backend (httptest.Server), sends a POST /v1/responses request, and verifies a valid response is returned.

**Checkpoint**: Antwort runs as a standalone server.

---

## Phase 2: User Story 2 - Mock Backend (Priority: P1)

**Goal**: Deterministic Chat Completions responses for all 6 conformance test scenarios.

**Independent Test**: Start mock, send Chat Completions requests matching each scenario, verify responses.

### Implementation for User Story 2

- [ ] T003 (antwort-zxs.1) [US2] Implement mock backend in `cmd/mock-backend/main.go`: HTTP server on configurable port (default 9090) handling POST /v1/chat/completions. Route to response generators based on request content analysis (FR-006, FR-012).
- [ ] T004 (antwort-zxs.2) [US2] [P] Implement non-streaming response generators in `cmd/mock-backend/main.go`: basic text ("Hello, nice day!" for "Say hello"), system prompt (pirate-style response), tool call (get_weather function_call), image acknowledgment, multi-turn continuation. Return valid chatCompletionResponse with usage stats (FR-007, FR-009, FR-010, FR-011).
- [ ] T005 (antwort-zxs.3) [US2] [P] Implement streaming response generator in `cmd/mock-backend/main.go`: for streaming requests, return SSE chunks with role delta, content deltas, finish_reason, usage, and [DONE] sentinel. Handle "Count from 1 to 5" specifically (FR-008).
- [ ] T006 (antwort-zxs.4) [US2] Write mock backend tests in `cmd/mock-backend/main_test.go`: verify each scenario returns valid Chat Completions responses. Test streaming output. Test determinism (same request = same response).

**Checkpoint**: Mock backend handles all 6 conformance scenarios deterministically.

---

## Phase 3: User Story 3 - Conformance Suite Integration (Priority: P1)

**Goal**: Run the official OpenResponses compliance suite against antwort.

**Independent Test**: Start mock + antwort, run suite, verify pass.

### Implementation for User Story 3

- [ ] T007 (antwort-4wz.1) [US3] Create `conformance/Containerfile` that builds a container from the openresponses/openresponses repo with bun installed. The container runs the compliance test suite with configurable BASE_URL and MODEL env vars. Output results to stdout (FR-013, FR-014).
- [ ] T008 (antwort-4wz.2) [US3] Create `conformance/run.sh` pipeline script: start mock-backend in background, start antwort server in background, wait for readiness (poll /healthz), run compliance container via podman, capture results, kill background processes, exit with suite status. Handle cleanup on script exit (trap) (FR-024, FR-025).
- [ ] T009 (antwort-4wz.3) [US3] Create `conformance/profiles.json` defining test profiles: "core" includes ["basic-response", "streaming-response", "system-prompt", "tool-calling", "multi-turn"], "extended" includes all 6 tests. The runner script reads this and filters results post-hoc (FR-017, FR-018, FR-019, FR-020, FR-021).
- [ ] T010 (antwort-4wz.4) [US3] Implement result parsing in `conformance/run.sh`: parse compliance suite output, apply profile filter, compute score (passed/total for profile), output structured JSON result (FR-015, FR-016, FR-026).

**Checkpoint**: `conformance/run.sh` runs end-to-end and reports conformance score.

---

## Phase 4: User Story 4 - CI Integration (Priority: P2)

**Goal**: Single make target for conformance testing.

**Independent Test**: `make conformance PROFILE=core` runs and reports score.

### Implementation for User Story 4

- [ ] T011 (antwort-5ty.1) [US4] Create top-level `Makefile` with targets: `build` (builds server + mock), `conformance` (runs conformance/run.sh with PROFILE), `clean`. Include `PROFILE` variable defaulting to "core" (FR-023).
- [ ] T012 (antwort-5ty.2) [US4] Add `conformance/README.md` documenting how to run conformance tests, what each profile covers, how to interpret results, and prerequisites (podman, Go).

**Checkpoint**: `make conformance` works end-to-end.

---

## Phase 5: Polish & Validation

**Purpose**: Verify conformance score, edge cases, documentation.

- [ ] T013 (antwort-0pm.1) [P] Run the full pipeline and verify core profile achieves 5/5 score. Debug any test failures by comparing antwort output against expected schemas.
- [ ] T014 (antwort-0pm.2) [P] Run the full pipeline with extended profile and verify image input test passes (or identify what mock adjustment is needed).
- [ ] T015 (antwort-0pm.3) Verify determinism: run the suite twice, compare results, confirm identical scores.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Server)**: No dependencies. Start immediately.
- **Phase 2 (Mock)**: No dependencies. Can start in parallel with Phase 1.
- **Phase 3 (Suite)**: Depends on Phase 1 and Phase 2 (needs both running).
- **Phase 4 (CI)**: Depends on Phase 3 (needs the pipeline working).
- **Phase 5 (Polish)**: Depends on Phase 3.

### Parallel Opportunities

- Phase 1 and Phase 2 can start in parallel (independent binaries)
- Within Phase 2: T004 and T005 can run in parallel (different response types)
- Within Phase 5: T013 and T014 can run in parallel

---

## Implementation Strategy

### MVP First

1. Complete Phase 1: Server wiring (runnable antwort)
2. Complete Phase 2: Mock backend (deterministic responses)
3. Complete Phase 3: Conformance suite (first score!)
4. **STOP and CELEBRATE**: First conformance score achieved

### Target Scores

- Core profile: 5/5 (basic, streaming, system prompt, tools, multi-turn)
- Extended profile: 6/6 (all tests including image input)

---

## Notes

- The official compliance suite is run as-is, never forked or modified
- Profile filtering is post-hoc: all tests run, results are filtered for scoring
- Podman is used exclusively for container operations (no Docker)
- The mock backend is intentionally simple (pattern matching, not an LLM)
- Server wiring is minimal (env vars only, full config is Spec 09)
