# Tasks: OpenResponses API Compliance

## Phase 1: Foundational Type Changes (P1)

**Purpose**: Add all missing fields to the API types and provider request types. These are prerequisites for all user stories.

- [x] T001 [P] Add `metadata`, `user`, `frequency_penalty`, `presence_penalty`, `top_logprobs`, `reasoning`, `text`, `parallel_tool_calls`, `max_tool_calls` fields to `CreateResponseRequest` in `pkg/api/types.go` (FR-001 through FR-009)
- [x] T002 [P] Add `user` field to `Response` struct in `pkg/api/types.go`. Add `User` to required response fields (FR-002)
- [x] T003 [P] Add `FrequencyPenalty`, `PresencePenalty`, `TopLogprobs`, `Reasoning`, `User` fields to `ProviderRequest` in `pkg/provider/types.go` (FR-003 through FR-007)
- [x] T004 [P] Add `frequency_penalty`, `presence_penalty`, `top_logprobs`, `user` fields to `ChatCompletionRequest` in `pkg/provider/openaicompat/types.go` and map them in `TranslateToChat()` in `pkg/provider/openaicompat/translate.go` (FR-003 through FR-005)

**Checkpoint**: All types updated. Compilation must succeed.

---

## Phase 2: US1 - Request Field Passthrough (P1)

**Goal**: All 9 passthrough fields are accepted, forwarded to the provider, and echoed in the response.

**Independent Test**: Send a request with all fields populated, verify each appears in the response.

- [x] T005 [US1] Map new request fields in `translateRequest()` in `pkg/engine/translate.go`: forward `frequency_penalty`, `presence_penalty`, `top_logprobs`, `reasoning` to `ProviderRequest` fields (FR-003 through FR-007)
- [x] T006 [US1] Echo `metadata` and `user` from request to response in `handleNonStreaming()` and `handleStreaming()` in `pkg/engine/engine.go` (FR-001, FR-002)
- [x] T007 [US1] Echo `parallel_tool_calls` from request to response (default true when omitted) in `pkg/engine/engine.go` (FR-008)
- [x] T008 [US1] Implement `max_tool_calls` enforcement: validate in engine before loop starts, terminate agentic loop when limit reached, in `pkg/engine/loop.go` (FR-009)
- [x] T009 [US1] Implement `parallel_tool_calls: false` sequential dispatch: when false, execute tool calls one at a time instead of concurrently in `pkg/engine/loop.go` (FR-008)
- [x] T010 [US1] Add integration test for passthrough fields round-trip in `test/integration/responses_test.go`: send request with all P1 fields, verify each appears in response (FR-001 through FR-009)

**Checkpoint**: All P1 fields accepted and echoed. `make api-test` passes.

---

## Phase 3: US2 - Response Verbosity Control (P2)

**Goal**: The `include` field filters which optional response sections are returned.

**Independent Test**: Send request with `include` filter, verify excluded sections are absent.

- [x] T011 [US2] Add `include` field (array of strings) to `CreateResponseRequest` in `pkg/api/types.go` (FR-010)
- [ ] T012 [US2] Implement response filtering based on `include` values in `pkg/engine/engine.go`: when `include` is set, omit sections not listed (usage, reasoning, etc.) from the response before returning (FR-010)
- [ ] T013 [US2] Add integration test for `include` filtering in `test/integration/responses_test.go`: verify fields are omitted when not in `include`, and all fields present when `include` is absent (FR-010)

**Checkpoint**: `include` filtering works. Backward compatible when omitted.

---

## Phase 4: US3 - Stream Configuration (P2)

**Goal**: `stream_options` controls streaming behavior, starting with `include_usage`.

**Independent Test**: Send streaming request with `stream_options.include_usage: true`, verify usage in completion event.

- [x] T014 [US3] Add `stream_options` field to `CreateResponseRequest` in `pkg/api/types.go` (FR-011)
- [ ] T015 [US3] Pass `stream_options.include_usage` through engine to streaming handler. When true, include usage data in the `response.completed` event in `pkg/engine/loop.go` (FR-011)
- [ ] T016 [US3] Add integration test for `stream_options` in `test/integration/streaming_test.go`: verify usage included when requested, behavior unchanged when omitted (FR-011)

**Checkpoint**: Streaming usage control works.

---

## Phase 5: Spec Alignment & Polish

**Purpose**: Update OpenAPI spec, run oasdiff, verify compliance improvement.

- [x] T017 [P] Update `api/openapi.yaml` to add all new request and response fields (metadata, user, frequency_penalty, presence_penalty, top_logprobs, reasoning, text, parallel_tool_calls, max_tool_calls, include, stream_options) (FR-012)
- [ ] T018 [P] Update `api/DIVERGENCES.md` to reflect reduced divergence list (FR-013)
- [x] T019 Run `make api-test` and verify oasdiff shows fewer "request-property-removed" warnings (FR-013)
- [x] T020 Run full test suite (`go test ./pkg/... ./test/integration/`) and verify zero regressions (SC-003)

**Checkpoint**: `make api-test` passes. oasdiff warnings reduced. All existing tests green.

---

## Dependencies

- Phase 1: No dependencies (type changes only)
- Phase 2 (US1): Depends on Phase 1 (needs the type fields)
- Phase 3 (US2): Depends on Phase 1; independent of US1
- Phase 4 (US3): Depends on Phase 1; independent of US1 and US2
- Phase 5: Depends on Phases 2, 3, and 4
