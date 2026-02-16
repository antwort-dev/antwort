# Tasks: Core Protocol & Data Model

**Input**: Design documents from `/specs/001-core-protocol/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Included. The plan explicitly defines test files for each source file. Tests validate SC-001 through SC-006.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Initialize Go module and package structure

- [x] T001 (antwort-7ih.1) Initialize Go module (`go mod init github.com/rhuss/antwort`) and create `pkg/api/` directory structure per plan.md
- [x] T002 (antwort-7ih.2) Create `pkg/api/doc.go` with package documentation describing the core protocol types, zero-dependency constraint, and relationship to OpenResponses spec

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core utilities used by ALL user stories. Must complete before any story work.

- [x] T003 (antwort-pur.1) [P] Implement ID generation in `pkg/api/id.go`: `NewResponseID()` returning `resp_` + 24-char alphanumeric, `NewItemID()` returning `item_` + 24-char alphanumeric, using `crypto/rand`. Include ID format validation function `ValidateResponseID(string) bool` and `ValidateItemID(string) bool`.
- [x] T004 (antwort-pur.2) [P] Implement error types in `pkg/api/errors.go`: `APIError` struct with `Type`, `Code`, `Param`, `Message` fields. `ErrorType` constants for `server_error`, `invalid_request`, `not_found`, `model_error`, `too_many_requests`. Constructor functions: `NewInvalidRequestError(param, message)`, `NewNotFoundError(message)`, `NewServerError(message)`, `NewModelError(message)`, `NewTooManyRequestsError(message)`. `APIError` must implement `error` interface. Include `ErrorResponse` wrapper struct. (FR-030, FR-031, FR-032)
- [x] T005 (antwort-pur.3) [P] Implement ID generation tests in `pkg/api/id_test.go`: table-driven tests for format validation (`resp_` prefix, `item_` prefix, correct length, alphanumeric charset), uniqueness (generate 1000 IDs, verify no duplicates), and invalid ID rejection.
- [x] T006 (antwort-pur.4) [P] Implement error type tests in `pkg/api/errors_test.go`: JSON round-trip for each error type, verify constructor functions set correct Type and Param fields, verify `error` interface implementation.

**Checkpoint**: ID generation and error types ready. All user stories can now proceed.

---

## Phase 3: User Story 1 - Submit a Prompt and Receive a Response (Priority: P1) 🎯 MVP

**Goal**: Implement all core data types (Item, Content, Request, Response) with correct JSON serialization and request validation. A developer can construct a valid request, validate it, and parse a valid response.

**Independent Test**: Construct a `CreateResponseRequest` with mixed input Items, validate it, construct a `Response` with output Items, serialize both to JSON, deserialize back, verify zero data loss.

### Implementation for User Story 1

- [x] T007 (antwort-dd9.1) [P] [US1] Implement content types in `pkg/api/types.go`: `ContentPart` struct (type, text, url, data, media_type) for user input (`input_text`, `input_image`, `input_audio`, `input_video`). `OutputContentPart` struct (type, text, annotations, logprobs) for model output (`output_text`, `summary_text`). `Annotation` struct (type, text, start_index, end_index). `TokenLogprob` struct (token, logprob, top_logprobs). (FR-006, FR-007, FR-007a, FR-008)
- [x] T008 (antwort-dd9.2) [P] [US1] Implement item type-specific data structs in `pkg/api/types.go`: `MessageData` (role, content, output), `FunctionCallData` (name, call_id, arguments), `FunctionCallOutputData` (call_id, output), `ReasoningData` (content, encrypted_content, summary). Include `MessageRole` type with constants `RoleUser`, `RoleAssistant`, `RoleSystem`. Include `ItemType` type with constants `ItemTypeMessage`, `ItemTypeFunctionCall`, `ItemTypeFunctionCallOutput`, `ItemTypeReasoning`. Include `ItemStatus` type with constants `ItemStatusInProgress`, `ItemStatusIncomplete`, `ItemStatusCompleted`, `ItemStatusFailed`. (FR-001, FR-009 through FR-014)
- [x] T009 (antwort-dd9.3) [US1] Implement `Item` struct in `pkg/api/types.go`: polymorphic Item with `ID`, `Type`, `Status`, and optional pointer fields `*MessageData`, `*FunctionCallData`, `*FunctionCallOutputData`, `*ReasoningData`, `Extension json.RawMessage`. All fields use `json:"...,omitempty"` tags. Include `IsExtensionType(ItemType) bool` function that checks for `provider:type` pattern. (FR-001, FR-002, FR-003, FR-004)
- [x] T010 (antwort-dd9.4) [US1] Implement `ToolChoice` union type in `pkg/api/types.go`: custom type with `MarshalJSON`/`UnmarshalJSON` supporting string values (`auto`, `required`, `none`) and structured object (`{type: "function", name: "..."}`). Include `ToolChoiceAuto`, `ToolChoiceRequired`, `ToolChoiceNone` constants. Include `ToolDefinition` struct (type, name, description, parameters as `json.RawMessage`). (FR-020, research RT-4)
- [x] T011 (antwort-dd9.5) [US1] Implement request and response types in `pkg/api/types.go`: `CreateResponseRequest` with all fields from data-model.md (model, input, instructions, tools, tool_choice, allowed_tools, store as `*bool`, stream, previous_response_id, truncation, service_tier, max_output_tokens as `*int`, temperature as `*float64`, top_p as `*float64`, extensions as `map[string]json.RawMessage`). `Response` with all fields (id, object, status, output, model, usage, error, previous_response_id, created_at, extensions). `ResponseStatus` type with constants including `ResponseStatusQueued`. `Usage` struct. (FR-015 through FR-025)
- [x] T012 (antwort-dd9.6) [US1] Implement `ValidationConfig` and request validation in `pkg/api/validation.go`: `ValidationConfig` struct with `MaxInputItems`, `MaxContentSize`, `MaxTools` fields and `DefaultValidationConfig()` function. `ValidateRequest(req *CreateResponseRequest, cfg ValidationConfig) *APIError` that checks: model required (FR-015), input non-empty (FR-016), max_output_tokens positive if set, temperature 0.0-2.0 if set, top_p 0.0-1.0 if set, truncation is `auto` or `disabled` if set, tool_choice references existing tool when forced (FR-038), input size limits (FR-040). `ValidateItem(item *Item) *APIError` that checks: ID format, valid type (standard or provider:type pattern per FR-037), exactly one type-specific field populated. (FR-036 through FR-040)
- [x] T013 (antwort-dd9.7) [US1] Implement JSON round-trip tests in `pkg/api/types_test.go`: table-driven tests for each type: `Item` with each type variant (message/user, message/assistant, function_call, function_call_output, reasoning), `ContentPart` for each input type, `OutputContentPart` with annotations and logprobs, `ToolChoice` string and object forms, `CreateResponseRequest` with all optional fields, `Response` with usage and error. Each test marshals to JSON, unmarshals back, and verifies deep equality. Verify `omitempty` behavior (nil fields absent from JSON). (SC-001, SC-002)
- [x] T014 (antwort-dd9.8) [US1] Implement validation tests in `pkg/api/validation_test.go`: table-driven tests covering all acceptance scenarios from User Story 1: valid request accepted, missing model rejected with `invalid_request` and `param: "model"`, empty input rejected, mixed item types parsed correctly, max_output_tokens=0 rejected, temperature out of range rejected, forced tool_choice referencing missing tool rejected, configurable size limits exceeded rejected. (SC-004)

**Checkpoint**: Core types compile, serialize correctly to/from JSON, and validation catches all invalid requests.

---

## Phase 4: User Story 2 - Streaming Events with State Transitions (Priority: P1)

**Goal**: Implement all streaming event types and state machine validation. The transport layer (Spec 02) can use these types directly.

**Independent Test**: Construct each streaming event type, verify it serializes correctly. Validate all allowed and disallowed state transitions for both Response and Item.

### Implementation for User Story 2

- [x] T015 (antwort-9z5.1) [P] [US2] Implement streaming event types in `pkg/api/events.go`: `StreamEventType` string type with constants for all delta events (`EventOutputItemAdded`, `EventContentPartAdded`, `EventOutputTextDelta`, `EventOutputTextDone`, `EventFunctionCallArgsDelta`, `EventFunctionCallArgsDone`, `EventContentPartDone`, `EventOutputItemDone`) and state machine events (`EventResponseCreated`, `EventResponseQueued`, `EventResponseInProgress`, `EventResponseCompleted`, `EventResponseFailed`, `EventResponseCancelled`). `StreamEvent` struct with all fields from data-model.md (type, sequence_number, response, item, part, delta, item_id, output_index, content_index). `IsExtensionEvent(StreamEventType) bool` for `provider:event_type` pattern. (FR-026, FR-027, FR-028, FR-028a, FR-029)
- [x] T016 (antwort-9z5.2) [P] [US2] Implement state machine validation in `pkg/api/state.go`: `ValidateResponseTransition(from, to ResponseStatus) error` enforcing: queued->in_progress, in_progress->completed|failed|cancelled, no transitions from terminal states, allow skipping queued (starting at in_progress). `ValidateItemTransition(from, to ItemStatus) error` enforcing: in_progress->completed|incomplete|failed, no transitions from terminal states. Both return `*APIError` with type `invalid_request` on invalid transitions. (FR-005, FR-024a, FR-025, FR-039)
- [x] T017 (antwort-9z5.3) [P] [US2] Implement streaming event tests in `pkg/api/events_test.go`: JSON round-trip for each event type (delta events with context fields, state machine events with response snapshots), verify sequence_number serialization, verify extension event type detection. (SC-005)
- [x] T018 (antwort-9z5.4) [P] [US2] Implement state machine tests in `pkg/api/state_test.go`: table-driven tests for all valid Response transitions (queued->in_progress, in_progress->completed, in_progress->failed, in_progress->cancelled, direct in_progress start), all valid Item transitions (in_progress->completed, in_progress->incomplete, in_progress->failed), all invalid transitions (completed->anything, failed->anything, cancelled->anything, queued->completed, incomplete->anything), verify error messages identify the invalid transition. (SC-003)

**Checkpoint**: All 15 streaming event types serialize correctly. State machine catches 100% of invalid transitions.

---

## Phase 5: User Story 3 - Stateless vs Stateful Request Classification (Priority: P2)

**Goal**: Implement the two-tier API classification. Validation correctly enforces stateless constraints.

**Independent Test**: Create requests with different `store` values. Verify stateless requests reject `previous_response_id`, default `store` is `true`.

### Implementation for User Story 3

- [x] T019 (antwort-30x.1) [US3] Add stateless/stateful validation rules to `pkg/api/validation.go`: in `ValidateRequest`, add checks for: `store: false` + `previous_response_id` returns `invalid_request` error (FR-035), `store` defaults to `true` when nil (FR-018). Add helper `IsStateless(req *CreateResponseRequest) bool`. (FR-033, FR-034, FR-035)
- [x] T020 (antwort-30x.2) [US3] Add stateless/stateful tests to `pkg/api/validation_test.go`: table-driven tests for all acceptance scenarios from User Story 3: store=false with previous_response_id rejected, store=nil defaults to stateful, store=true with previous_response_id accepted, IsStateless returns correct values. (SC-004)

**Checkpoint**: Two-tier classification enforced. Stateless requests cannot chain.

---

## Phase 6: User Story 4 - Provider Extension Types (Priority: P3)

**Goal**: Extension items, events, and request/response fields survive round-trips without data loss.

**Independent Test**: Create an Item with `provider:type` format, add extension data, serialize to JSON, deserialize back, verify exact preservation.

### Implementation for User Story 4

- [x] T021 (antwort-dfi.1) [P] [US4] Add extension validation to `pkg/api/validation.go`: in `ValidateItem`, ensure types not matching standard types AND not matching `provider:type` pattern are rejected. Extension items must have non-nil `Extension` field. Requests with unknown fields in the extensions map are accepted without schema validation. (FR-002, FR-037)
- [x] T022 (antwort-dfi.2) [P] [US4] Add extension round-trip tests to `pkg/api/types_test.go`: table-driven tests for: Item with `"acme:telemetry_chunk"` type and `json.RawMessage` extension data survives marshal/unmarshal, Request with extensions map containing opaque JSON survives round-trip, Response with extensions map survives round-trip, invalid type (no colon, not standard) rejected by validation. (SC-006)

**Checkpoint**: Provider extensions pass through without data loss. Invalid types rejected.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Package-level verification and documentation

- [x] T023 (antwort-drc.1) Verify all tests pass with `go test -v -count=1 ./pkg/api/...` and fix any failures
- [x] T024 (antwort-drc.2) Run `go vet ./pkg/api/...` and fix any issues
- [x] T025 (antwort-drc.3) Validate OpenAPI contract in `specs/001-core-protocol/contracts/openapi.yaml` matches implemented types (manual review: field names, types, required/optional, enum values)
- [x] T026 (antwort-drc.4) Run quickstart.md code examples mentally against the implemented API to verify accuracy, update quickstart.md if any function signatures changed during implementation

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Setup completion, BLOCKS all user stories
- **US1 (Phase 3)**: Depends on Foundational. BLOCKS US3 (stateless validation extends US1's validator)
- **US2 (Phase 4)**: Depends on Foundational. Can run in PARALLEL with US1 (different files: events.go/state.go vs types.go/validation.go)
- **US3 (Phase 5)**: Depends on US1 (extends validation.go)
- **US4 (Phase 6)**: Depends on US1 (extends validation.go and types_test.go). Can run in PARALLEL with US3
- **Polish (Phase 7)**: Depends on all user stories complete

### User Story Dependencies

```
Phase 1 (Setup)
    │
    ▼
Phase 2 (Foundational: id.go, errors.go)
    │
    ├──────────────────┐
    ▼                  ▼
Phase 3 (US1)     Phase 4 (US2)     ← PARALLEL
    │
    ├──────────────────┐
    ▼                  ▼
Phase 5 (US3)     Phase 6 (US4)     ← PARALLEL
    │                  │
    └────────┬─────────┘
             ▼
        Phase 7 (Polish)
```

### Parallel Opportunities

Within Phase 2:
- T003 (id.go) and T004 (errors.go) are independent files, run in parallel
- T005 (id_test.go) and T006 (errors_test.go) are independent, run in parallel

Within Phase 3 (US1):
- T007 (content types) and T008 (item data structs) can run in parallel (both contribute to types.go, but are independent sections)

Across Phases 3-4:
- US1 (Phase 3) and US2 (Phase 4) work on different files and can run in parallel

Across Phases 5-6:
- US3 (Phase 5) and US4 (Phase 6) can run in parallel if US4 extension validation is added to a separate validation function

---

## Parallel Example: Foundational Phase

```bash
# These can run simultaneously (different files):
Task: T003 "Implement ID generation in pkg/api/id.go"
Task: T004 "Implement error types in pkg/api/errors.go"

# After both complete, tests can run simultaneously:
Task: T005 "Implement ID tests in pkg/api/id_test.go"
Task: T006 "Implement error tests in pkg/api/errors_test.go"
```

## Parallel Example: US1 + US2

```bash
# After Foundational completes, these can run simultaneously:
# Developer A works on US1 (types.go, validation.go):
Task: T007-T014

# Developer B works on US2 (events.go, state.go):
Task: T015-T018
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T002)
2. Complete Phase 2: Foundational (T003-T006)
3. Complete Phase 3: User Story 1 (T007-T014)
4. **STOP and VALIDATE**: Run `go test -v ./pkg/api/...`, verify all types serialize correctly
5. This delivers a usable data model package that other specs can depend on

### Incremental Delivery

1. Setup + Foundational → Core utilities ready
2. Add US1 → Core types + validation (MVP, enough for Spec 02 and 03 to start)
3. Add US2 → Streaming events + state machine (enables Spec 02 transport work)
4. Add US3 → Stateless/stateful classification (enables Spec 05 storage work)
5. Add US4 → Extension support (enables Spec 03/08 provider work)

### Single Developer Strategy

Execute phases sequentially: 1 → 2 → 3 → 4 → 5 → 6 → 7.
Total: 26 tasks across 7 phases.

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- All types must use `json:"field_name,omitempty"` tags matching OpenAI Responses API wire format
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
