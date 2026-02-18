# Tasks: Agentic Loop & Tool Orchestration

**Input**: Design documents from `/specs/004-agentic-loop/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, quickstart.md

**Tests**: Tests are included as part of implementation tasks (Go convention: test file alongside source file).

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Prerequisite Amendment)

**Purpose**: Amend Spec 001 types and create package structure

- [ ] T001 (antwort-iq0.1) Add `ResponseStatusRequiresAction ResponseStatus = "requires_action"` to `pkg/api/types.go`. Add `requires_action` as terminal event to SSE writer's `terminalEvents` map in `pkg/transport/http/sse.go`.
- [ ] T002 (antwort-iq0.2) Update state machine in `pkg/api/state.go`: add `requires_action` as valid transition from `in_progress`, add `requires_action` as terminal status (empty allowed transitions). Update tests in `pkg/api/state_test.go` for the new transitions.
- [ ] T003 (antwort-iq0.3) Create package directory `pkg/tools/` and `pkg/tools/doc.go` with package documentation describing the tool executor interface and types.

---

## Phase 2: Foundational (Tool Types & Executor Interface)

**Purpose**: Define the ToolExecutor interface, ToolCall, ToolResult types, and filtering logic. MUST be complete before any user story work.

**CRITICAL**: No user story work can begin until this phase is complete.

- [ ] T004 (antwort-48b.1) Define ToolExecutor interface (Kind, CanExecute, Execute), ToolKind enum (Function, MCP, Sandbox), ToolCall, and ToolResult types in `pkg/tools/executor.go` (FR-001, FR-002, FR-003). Write interface contract tests in `pkg/tools/executor_test.go` verifying a mock executor satisfies the interface.
- [ ] T005 (antwort-48b.2) [P] Implement allowed_tools filtering in `pkg/tools/filter.go`: FilterAllowedTools function that checks tool call names against the allowed list, returns allowed/rejected split. Write table-driven tests in `pkg/tools/filter_test.go` covering: all allowed, some rejected, empty allowed list (all pass), unknown tool name (FR-008, FR-009, FR-010).
- [ ] T006 (antwort-48b.3) [P] Add `MaxAgenticTurns` (int, default 10) and `Executors` ([]ToolExecutor, nil-safe) fields to Config in `pkg/engine/config.go` (FR-004, FR-015).
- [ ] T007 (antwort-48b.4) Update Engine constructor `New()` in `pkg/engine/engine.go` to accept executors from Config. Store executors in Engine struct. Add helper `hasExecutors()` and `findExecutor(toolName)` methods. Ensure nil/empty executors preserves existing single-shot behavior (FR-004).

**Checkpoint**: Tool types and executor interface ready. User story implementation can now begin.

---

## Phase 3: User Story 1 - Single-Turn Function Tool Call (Priority: P1) MVP

**Goal**: Preserve existing single-shot tool call behavior when no executors are registered. Verify backward compatibility.

**Independent Test**: Send a request with tools to an engine with no executors, verify response contains function_call items with status `completed`.

### Implementation for User Story 1

- [ ] T008 (antwort-99y.1) [US1] Write engine backward compatibility tests in `pkg/engine/engine_test.go`: verify that engine with no executors returns function_call items with `completed` status (same as Spec 003 behavior). Test `tool_choice: "none"` sends tools but does not enter loop. Test `tool_choice: "required"` with no tool calls returns response as-is.
- [ ] T009 (antwort-99y.2) [US1] Implement `tool_choice: "none"` enforcement in `pkg/engine/engine.go`: when tool_choice is "none", skip agentic loop entry regardless of tool calls in response. Tool calls remain in output with `completed` status (FR-005, FR-006).

**Checkpoint**: Existing single-shot behavior preserved. No executors = same behavior as Spec 003.

---

## Phase 4: User Story 2 - Multi-Turn Agentic Loop (Priority: P1)

**Goal**: Implement the core agentic loop: inference -> tool calls -> execute -> feed back -> inference, until final answer or termination.

**Independent Test**: Mock executor returns fixed results. Mock provider returns tool call on turn 1, text answer on turn 2. Verify complete response contains tool call, result, and final answer.

### Implementation for User Story 2

- [ ] T010 (antwort-a79.1) [US2] Implement agentic loop for non-streaming in `pkg/engine/loop.go`: `runAgenticLoop` function that accepts provider, executors, translated request, and config. Loop calls provider.Complete, checks for tool calls, dispatches to executors, collects results, appends to conversation, and repeats. Return final Response with all accumulated output items and cumulative usage. Respects MaxAgenticTurns (FR-011, FR-012, FR-013, FR-015, FR-017, FR-027, FR-028).
- [ ] T011 (antwort-a79.2) [US2] Implement concurrent tool execution in `pkg/engine/loop.go`: when multiple tool calls in a turn, use sync.WaitGroup to fan out execution, collect results via channel, feed all back before next inference call (FR-016).
- [ ] T012 (antwort-a79.3) [US2] Implement executor dispatch in `pkg/engine/loop.go`: for each tool call, find matching executor via CanExecute, call Execute, convert ToolResult to api.Item (function_call_output). If executor returns error, create function_call_output with is_error=true (FR-027, FR-029).
- [ ] T013 (antwort-a79.4) [US2] Integrate agentic loop into `pkg/engine/engine.go`: in CreateResponse, after provider response, check if tool calls exist AND executors are registered AND tool_choice != "none". If so, call runAgenticLoop. For non-streaming, wrap single-turn Complete in loop. For streaming, extend handleStreaming to support multi-turn (FR-011, FR-022, FR-023, FR-024, FR-025).
- [ ] T014 (antwort-a79.5) [US2] Implement streaming agentic loop in `pkg/engine/loop.go`: `runAgenticLoopStreaming` that manages event emission across turns. Emit response.created/in_progress once at start. Between turns (tool execution), emit output_item.added/done for tool results. Emit response.completed once at end. Use existing streamState for sequence numbers (FR-022, FR-023, FR-024, FR-025).
- [ ] T015 (antwort-a79.6) [US2] Write agentic loop non-streaming tests in `pkg/engine/loop_test.go`: 2-turn loop (tool call -> result -> final answer), 3-turn loop, multiple concurrent tool calls in one turn, tool error fed back to model, max turns limit produces incomplete, context cancellation. Use mock provider with turn-aware responses and mock executor.
- [ ] T016 (antwort-a79.7) [US2] Write agentic loop streaming tests in `pkg/engine/loop_test.go`: verify single continuous event stream across 2 turns, response.created once, response.completed once, intermediate tool call items emitted correctly. Test context cancellation mid-turn.

**Checkpoint**: Multi-turn agentic loop works for both streaming and non-streaming. Tools executed concurrently. Errors fed back gracefully.

---

## Phase 5: User Story 3 - Client-Executed Function Tools (Priority: P1)

**Goal**: Implement requires_action status for tool calls that no executor can handle.

**Independent Test**: Engine with executors that can't handle function tools returns requires_action. Follow-up with results continues conversation.

### Implementation for User Story 3

- [ ] T017 (antwort-9yj.1) [US3] Implement function-tool detection in `pkg/engine/loop.go`: when checking tool calls against executors, if any tool call has no matching executor (all CanExecute return false), classify the turn as requiring client action. Return response with `requires_action` status and function_call items in output (FR-018, FR-019).
- [ ] T018 (antwort-9yj.2) [US3] Handle mixed tool kinds in `pkg/engine/loop.go`: if a turn contains both server-executable and client-executable tool calls, pause the entire turn with `requires_action` (do not execute server-side tools partially). All tool calls returned to client.
- [ ] T019 (antwort-9yj.3) [US3] Implement streaming requires_action terminal event in `pkg/engine/engine.go`: when the loop returns requires_action status during streaming, emit response with `requires_action` status in the terminal event (FR-024, FR-026).
- [ ] T020 (antwort-9yj.4) [US3] Write requires_action tests in `pkg/engine/engine_test.go`: non-streaming requires_action with function tools, streaming requires_action, follow-up request with previous_response_id and function_call_output continues conversation (FR-021). Test mixed tool kinds pauses entire turn.

**Checkpoint**: Client-executed function tools return requires_action. Follow-up requests continue correctly.

---

## Phase 6: User Story 4 - Allowed Tools Filtering (Priority: P2)

**Goal**: Restrict which tools the model may invoke via allowed_tools.

**Independent Test**: Request with 3 tools and allowed_tools=["A"]. Model calls non-allowed tool B. Verify error fed back.

### Implementation for User Story 4

- [ ] T021 (antwort-cr5.1) [US4] Integrate allowed_tools filtering into agentic loop in `pkg/engine/loop.go`: before dispatching tool calls, check each against allowed_tools list using filter.go. For rejected calls, create function_call_output with is_error=true and descriptive message. Feed rejected results back to model alongside successful results (FR-008, FR-009, FR-010).
- [ ] T022 (antwort-cr5.2) [US4] Write allowed_tools tests in `pkg/engine/loop_test.go`: all tools allowed (no filter), some tools rejected (error fed back), empty allowed_tools (all pass), model calls unknown tool (error fed back).

**Checkpoint**: Allowed tools filtering prevents execution of restricted tools with clear error feedback.

---

## Phase 7: User Story 5 - Safety Limits and Loop Termination (Priority: P2)

**Goal**: Ensure the loop terminates safely under all conditions.

**Independent Test**: Max turns of 2 with model always producing tool calls. Verify incomplete status.

### Implementation for User Story 5

- [ ] T023 (antwort-3li.1) [US5] Verify max turns enforcement in `pkg/engine/loop_test.go`: configure MaxAgenticTurns=2, mock provider always returns tool calls, verify loop terminates with `incomplete` status and accumulated output items.
- [ ] T024 (antwort-3li.2) [US5] Verify context cancellation in `pkg/engine/loop_test.go`: cancel context mid-turn during tool execution, verify loop terminates with `cancelled` status.
- [ ] T025 (antwort-3li.3) [US5] Verify provider error mid-loop in `pkg/engine/loop_test.go`: provider returns error on turn 2, verify loop terminates with `failed` status and error details.

**Checkpoint**: All termination conditions verified. Safety limits prevent runaway loops.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Edge cases, documentation, and final validation

- [ ] T026 (antwort-0of.1) [P] Handle edge case: executor registered but CanExecute returns false for all tools in `pkg/engine/loop.go`. Verify behavior matches function-tool path (requires_action).
- [ ] T027 (antwort-0of.2) [P] Handle edge case: model returns tool call referencing unknown tool (not in request's tool list). Feed error back to model.
- [ ] T028 (antwort-0of.3) [P] Handle edge case: model returns both text content and tool calls. Verify both included in output, tool calls trigger loop.
- [ ] T029 (antwort-0of.4) Run `go vet ./...` and `go test ./...` across all packages to verify compilation and test passing.
- [ ] T030 (antwort-0of.5) Validate quickstart.md code examples compile and match actual API signatures.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies. Start immediately.
- **Phase 2 (Foundational)**: Depends on Phase 1 (requires_action status, package structure).
- **Phase 3 (US1)**: Depends on Phase 2 (executor interface and engine wiring).
- **Phase 4 (US2)**: Depends on Phase 2 (executor interface). Independent of Phase 3.
- **Phase 5 (US3)**: Depends on Phase 4 (agentic loop must exist to detect unhandled tools).
- **Phase 6 (US4)**: Depends on Phase 4 (agentic loop). Independent of Phase 5.
- **Phase 7 (US5)**: Depends on Phase 4 (agentic loop). Independent of Phase 5/6.
- **Phase 8 (Polish)**: Depends on Phases 4-5 (core loop and requires_action).

### User Story Dependencies

- **US1 (Single-Turn)**: Foundation only. Backward compatibility.
- **US2 (Agentic Loop)**: Foundation only. Core capability.
- **US3 (requires_action)**: Extends US2 loop with client-execution detection.
- **US4 (Allowed Tools)**: Independent. Adds filtering to US2 loop.
- **US5 (Safety Limits)**: Independent. Tests termination conditions of US2 loop.

### Parallel Opportunities

Within Phase 2:
- T005, T006 can run in parallel (different files)

After Phase 2:
- US1 (Phase 3) and US2 (Phase 4) can start in parallel (independent)

After Phase 4:
- US4 (Phase 6) and US5 (Phase 7) can start in parallel (independent)

Within Phase 8:
- T026, T027, T028 can all run in parallel (different edge cases)

---

## Implementation Strategy

### MVP First (User Stories 1 + 2)

1. Complete Phase 1: Setup (requires_action status, package structure)
2. Complete Phase 2: Foundational (executor interface, config, engine wiring)
3. Complete Phase 3: US1 (backward compatibility verification)
4. Complete Phase 4: US2 (agentic loop, the core)
5. **STOP and VALIDATE**: Multi-turn loop works with mock executor

### Incremental Delivery

1. Setup + Foundational -> Types and interface ready
2. US1 -> Backward compatibility confirmed
3. US2 -> Multi-turn agentic loop works (MVP!)
4. US3 -> Client-executed function tools with requires_action
5. US4 -> Allowed tools filtering
6. US5 -> Safety limits verified
7. Polish -> Edge cases and validation

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- US2 is the core: the agentic loop. Most other stories extend it.
- US1 is really a backward compatibility verification
- US4 and US5 add safety features to the loop and can be done in any order
- Go convention: test files sit alongside source files (`*_test.go`)
- All tests use mock executors and mock providers, no external dependencies
