# Tasks: Terminal Streaming Events

## Phase 1: Event Type Definitions

**Purpose**: Add new SSE event type constants and serialization.

- [ ] T001 Add `EventResponseIncomplete`, `EventError`, `EventRefusalDelta`, `EventRefusalDone` constants to `pkg/api/events.go`. Add MarshalJSON cases for each (FR-001 through FR-007)
- [ ] T002 [P] Add `response.incomplete`, `error`, `response.refusal.delta`, `response.refusal.done` to StreamEventType enum in `api/openapi.yaml` (FR-008, FR-009)

**Checkpoint**: New event types compile and serialize correctly.

---

## Phase 2: US1 - Incomplete Response Detection (P1)

**Goal**: Streaming responses that hit max tokens emit `response.incomplete` instead of `response.completed`.

**Independent Test**: Send request with low max tokens, verify terminal event is `response.incomplete`.

- [ ] T003 [US1] Update `emitStreamComplete()` in `pkg/engine/engine.go` to emit `response.incomplete` (instead of `response.completed`) when `finalStatus` is `ResponseStatusIncomplete`. Populate `incomplete_details.reason` with "max_output_tokens" (FR-001, FR-002)
- [ ] T004 [US1] Verify non-streaming path: ensure `handleNonStreaming()` sets status "incomplete" with `incomplete_details` when provider returns `ResponseStatusIncomplete` (FR-003)
- [ ] T005 [US1] Update mock backend in `test/integration/helpers_test.go` to return `finish_reason: "length"` when the prompt contains a trigger word (e.g., "truncate")
- [ ] T006 [US1] Add integration test `TestStreamingIncompleteEvent` in `test/integration/streaming_test.go`: verify `response.incomplete` terminal event when model hits token limit (FR-001, FR-010)
- [ ] T007 [US1] Add integration test `TestNonStreamingIncompleteStatus` in `test/integration/responses_test.go`: verify response status is "incomplete" with `incomplete_details` (FR-003)

**Checkpoint**: Incomplete detection works in both streaming and non-streaming paths.

---

## Phase 3: US2 - Error Stream Event (P1)

**Goal**: Pre-response errors emit a standalone `error` event.

- [ ] T008 [US2] Update `handleStreaming()` in `pkg/engine/engine.go` to emit an `error` event when the provider fails before any response events are emitted. Existing `response.failed` behavior remains for errors within a response context (FR-004, FR-005, FR-011)
- [ ] T009 [US2] Add integration test `TestStreamErrorEvent` in `test/integration/streaming_test.go`: trigger a provider error, verify `error` event is received (FR-004)

**Checkpoint**: Error events work for pre-response failures.

---

## Phase 4: US3 - Refusal Events (P3)

**Goal**: Refusal content from the provider is streamed as refusal delta/done events.

- [ ] T010 [US3] Add refusal event mapping in `pkg/engine/events.go`: detect refusal content from provider and emit `response.refusal.delta`/`response.refusal.done` (FR-006, FR-007)
- [ ] T011 [US3] Add `Refusal` field to `ChatMessage` in `pkg/provider/openaicompat/types.go` and pass it through as a provider event if non-empty

**Checkpoint**: Refusal events work (testable against commercial backends that populate refusal).

---

## Phase 5: Polish & Validation

- [ ] T012 Run `make api-test` and verify all existing tests pass with zero regressions (FR-010)
- [ ] T013 Run full test suite (`go test ./pkg/... ./test/integration/`) and verify zero regressions

**Checkpoint**: All tests green. SSE event count increased from 17 to 21.

---

## Dependencies

- Phase 1: No dependencies
- Phase 2 (US1): Depends on Phase 1
- Phase 3 (US2): Depends on Phase 1, independent of US1
- Phase 4 (US3): Depends on Phase 1, independent of US1/US2
- Phase 5: Depends on Phases 2, 3, and 4
