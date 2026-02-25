# Tasks: Structured Output (text.format Passthrough)

## Phase 1: Type Extensions

**Purpose**: Extend the Go types to carry structured output fields through the pipeline.

- [X] T001 [P] Extend `TextFormat` struct in `pkg/api/types.go` with `Name` (string, omitempty), `Strict` (*bool, omitempty), and `Schema` (json.RawMessage, omitempty) fields for json_schema mode (FR-006, FR-007)
- [X] T002 [P] Add `ResponseFormat *api.TextConfig` field to `ProviderRequest` struct in `pkg/provider/types.go` (FR-001)
- [X] T003 [P] Add `ResponseFormat any` field to `ChatCompletionRequest` struct in `pkg/provider/openaicompat/types.go` (FR-001)

**Checkpoint**: All types extended, project compiles.

---

## Phase 2: Translation Pipeline

**Purpose**: Wire text.format through the engine and provider translation functions.

- [X] T004 [US1] Forward `req.Text` (when non-nil and type is not "text") as `ResponseFormat` in `translateRequest()` in `pkg/engine/translate.go` (FR-001, FR-002, FR-004)
- [X] T005 [US1] Map `req.ResponseFormat` to `cr.ResponseFormat` in `TranslateToChat()` in `pkg/provider/openaicompat/translate.go`, converting TextConfig to Chat Completions response_format structure (FR-001, FR-002, FR-003)

**Checkpoint**: text.format flows from request to Chat Completions backend.

---

## Phase 3: User Story 1 & 2 - JSON Object and JSON Schema Modes (Priority: P1)

**Goal**: json_object and json_schema format constraints reach the backend and produce constrained output.

**Independent Test**: Send requests with json_object and json_schema formats, verify the mock backend receives response_format.

- [X] T006 [US1] Update mock backend in `test/integration/helpers_test.go` to inspect and echo `response_format` from incoming Chat Completions requests, returning JSON output when json_object or json_schema is requested (SC-001, SC-002)
- [X] T007 [US1] Add integration test `TestStructuredOutputJsonObject` in `test/integration/responses_test.go`: send request with `text.format: {"type": "json_object"}`, verify response contains valid JSON and text.format is echoed (FR-002, FR-008)
- [X] T008 [US2] Add integration test `TestStructuredOutputJsonSchema` in `test/integration/responses_test.go`: send request with json_schema format including name, strict, and schema fields, verify schema flows through unchanged (FR-003, FR-006, FR-007)
- [X] T009 [US1] Add integration test `TestStructuredOutputDefaultText` in `test/integration/responses_test.go`: verify no response_format is sent when text.format is absent or type is "text" (FR-004)
- [X] T010 Run full test suite and verify zero regressions (SC-004)

**Checkpoint**: json_object and json_schema modes work end-to-end.

---

## Phase 4: OpenAPI and Polish

- [X] T011 [P] Update `api/openapi.yaml` TextFormat schema with `name`, `strict`, and `schema` properties
- [X] T012 Run `make api-test` to verify conformance

**Checkpoint**: All tests pass, OpenAPI spec updated.

---

## Dependencies

- Phase 1: No dependencies (parallel type changes)
- Phase 2: Depends on Phase 1
- Phase 3: Depends on Phase 2
- Phase 4: Can run in parallel with Phase 3 (T011), but T012 depends on Phase 3
