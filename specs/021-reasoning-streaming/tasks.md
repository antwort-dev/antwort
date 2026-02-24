# Tasks: Reasoning Streaming Events

## Phase 1: Event Type Definitions

**Purpose**: Add the new SSE event type constants and serialization.

- [x] T001 Add `EventReasoningDelta` and `EventReasoningDone` constants to `pkg/api/events.go`. Add MarshalJSON cases for both event types (reasoning.delta includes delta + item_id + output_index + content_index; reasoning.done includes item_id + output_index + content_index) (FR-001, FR-002, FR-003)
- [x] T002 [P] Add `response.reasoning.delta` and `response.reasoning.done` to the `StreamEventType` enum in `api/openapi.yaml` (FR-009, FR-010)

**Checkpoint**: New event types compile and serialize correctly.

---

## Phase 2: US1 - Streaming Reasoning Events (P1)

**Goal**: Reasoning tokens stream to clients as SSE events during streaming responses.

**Independent Test**: Send streaming request to reasoning mock, verify reasoning delta events received.

- [x] T003 [US1] Map `ProviderEventReasoningDelta` to `EventReasoningDelta` SSE events in `pkg/engine/events.go`. Track reasoning state (item ID, accumulated text) in `streamState` (FR-001, FR-003, FR-004)
- [x] T004 [US1] Map `ProviderEventReasoningDone` to `EventReasoningDone` SSE events in `pkg/engine/events.go` (FR-002)
- [x] T005 [US1] Update `handleStreaming()` in `pkg/engine/engine.go` to emit reasoning output_item.added lifecycle events when reasoning starts, and include the reasoning item in the final output (FR-005, FR-006)
- [x] T006 [US1] Update mock backend in `test/integration/helpers_test.go` to produce `reasoning_content` in streaming chunks when the prompt contains a trigger word (e.g., "reason")
- [x] T007 [US1] Add integration test `TestStreamingReasoningEvents` in `test/integration/streaming_test.go`: verify reasoning.delta events are received, reasoning.done is received, then text deltas follow (FR-001, FR-002, FR-007)

**Checkpoint**: Streaming reasoning events work end-to-end with mock backend.

---

## Phase 3: US2 - Non-Streaming Reasoning Items (P1)

**Goal**: Non-streaming responses include reasoning items when the model produces reasoning content.

**Independent Test**: Send non-streaming request to reasoning mock, verify reasoning item in output.

- [x] T008 [US2] Verify reasoning item ordering in `handleNonStreaming()` in `pkg/engine/engine.go`: ensure reasoning items appear before text items in the output array (FR-006)
- [x] T009 [US2] Update mock backend in `test/integration/helpers_test.go` to produce `reasoning_content` in non-streaming responses when triggered
- [x] T010 [US2] Add integration test `TestNonStreamingReasoningItem` in `test/integration/responses_test.go`: verify reasoning item appears in output with correct type and content (FR-005, FR-007)

**Checkpoint**: Non-streaming reasoning items work.

---

## Phase 4: US3 - Agentic Loop Reasoning (P2)

**Goal**: Reasoning events are emitted during each turn of the agentic loop.

- [x] T011 [US3] Update `runAgenticLoopStreaming()` in `pkg/engine/loop.go` to handle reasoning events across turns, resetting reasoning state between turns while maintaining sequence numbers (FR-004)

**Checkpoint**: Reasoning works in multi-turn agentic streaming.

---

## Phase 5: Polish & Validation

- [x] T012 Run `make api-test` and verify all existing tests pass with zero regressions (FR-007, FR-008)
- [x] T013 Run full test suite (`go test ./pkg/... ./test/integration/`) and verify zero regressions

**Checkpoint**: All tests green. SSE event count increased from 15 to 17.

---

## Dependencies

- Phase 1: No dependencies
- Phase 2 (US1): Depends on Phase 1 (needs event type constants)
- Phase 3 (US2): Independent of US1
- Phase 4 (US3): Depends on Phase 2 (builds on streaming reasoning)
- Phase 5: Depends on Phases 2, 3, and 4
