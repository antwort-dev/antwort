# Tasks: Responses API Provider

## Phase 1: Foundational (expandBuiltinTools Migration)

**Purpose**: Move built-in tool type expansion from the engine to the provider layer. This is a prerequisite for the Responses API provider and fixes the Constitution Principle VI violation.

- [x] T001 (antwort-a4j.1) Create shared utility `pkg/provider/tools.go`: extract built-in tool expansion logic from `pkg/engine/engine.go` into a function `ExpandBuiltinTools(tools []ProviderTool, definitions []ProviderTool) []ProviderTool` that replaces built-in type stubs with function definitions. Used by both Chat Completions and Responses API providers (FR-008, FR-009)
- [x] T002 (antwort-a4j.2) Remove `expandBuiltinTools()` function and its call from `pkg/engine/engine.go`. The engine must preserve `tool.Type` as-is in `translateRequest()` at `pkg/engine/translate.go`. Both vLLM/LiteLLM and Responses API providers call the shared utility from their translation layers (FR-010)
- [x] T003 (antwort-a4j.3) Update existing tests to verify tool types are preserved through the engine and expanded in both provider adapters via the shared utility. Verify no regressions in `go test ./pkg/engine/ ./pkg/provider/...` (FR-008, FR-009, FR-010)

**Checkpoint**: Built-in tool types flow through the engine unchanged. Both provider types expand them via shared utility. Existing tests pass.

---

## Phase 2: User Story 1 - Native Responses API Inference (Priority: P1)

**Goal**: A provider adapter that forwards inference requests via the Responses API wire format and streams native SSE events back.

**Independent Test**: Configure the provider with a mock HTTP server that speaks Responses API. Send a streaming request and verify native SSE events arrive.

- [x] T004 (antwort-1k1.1) [P] [US1] Create `pkg/provider/responses/types.go`: Responses API wire format types for request, response, SSE events, and error objects (FR-001, FR-003)
- [x] T005 (antwort-1k1.2) [P] [US1] Create `pkg/provider/responses/translate.go`: translate `ProviderRequest` to Responses API request (input items, tools, model, `store: false`) and `Responses API response` back to `ProviderResponse` (FR-002, FR-003)
- [x] T006 (antwort-1k1.3) [US1] Create `pkg/provider/responses/stream.go`: SSE event parser that reads the backend's event stream and maps Responses API events to `ProviderEvent` types (FR-005, FR-006)
- [x] T007 (antwort-1k1.4) [US1] Create `pkg/provider/responses/provider.go`: `ResponsesProvider` implementing `Provider` interface with `CreateResponse` (non-streaming) and `StreamResponse` (streaming). Constructor takes backend URL, API key, model. Uses shared `ExpandBuiltinTools` utility to expand built-in types before forwarding (FR-001, FR-004, FR-008)
- [x] T008 (antwort-1k1.5) [P] [US1] Create `pkg/provider/responses/translate_test.go`: table-driven tests for request/response translation. Verify `store: false` is always set, built-in tool types are expanded to function definitions, input items are correctly formatted (FR-002, FR-003, FR-008)
- [x] T009 (antwort-1k1.6) [P] [US1] Create `pkg/provider/responses/stream_test.go`: tests for SSE event parsing with mock event data. Cover text delta, function call, error events, unknown event types (FR-005, FR-006)
- [x] T010 (antwort-1k1.7) [US1] Wire `"vllm-responses"` provider type in `cmd/server/main.go`: add case to provider factory that creates `ResponsesProvider` from config. Coexists with existing `"vllm"` and `"litellm"` providers (FR-011, FR-012)

**Checkpoint**: Responses API provider compiles. Unit tests pass for translation and streaming. Provider is selectable via config.

---

## Phase 3: User Story 2 - Stateful Enrichment (Priority: P1)

**Goal**: State management (persistence, conversation chaining) works with the Responses API provider.

**Independent Test**: Send two requests where the second uses `previous_response_id`. Verify the backend receives the full conversation history.

- [x] T011 (antwort-45g.1) [US2] Verify `pkg/engine/engine.go` conversation history reconstruction works with the Responses API provider. The engine assembles history and passes it via `ProviderRequest.Messages`, which the provider translates to `input` items. No code changes expected, just verification (FR-002)
- [x] T012 (antwort-45g.2) [US2] Create integration test in `pkg/provider/responses/integration_test.go`: start a mock Responses API server (httptest), wire the provider, verify stateful operations (store, retrieve, conversation chaining) work through the engine (FR-002, FR-003)

**Checkpoint**: Stateful operations work with the Responses API provider. Conversation chaining passes full history to the backend.

---

## Phase 4: User Story 3 - Server-Side Tool Execution (Priority: P1)

**Goal**: Code interpreter, MCP, and function provider tools execute server-side through the agentic loop with the Responses API provider.

**Independent Test**: Send a request with code_interpreter enabled. Verify the model calls it, code executes, and the result feeds back.

- [x] T013 (antwort-cmj.1) [US3] Verify the Responses API provider correctly handles tool call responses from the backend. The provider must parse function_call items from the Responses API response and map them to `ProviderResponse.ToolCalls` for the engine's agentic loop (FR-003, FR-006)
- [x] T014 (antwort-cmj.2) [US3] Add tool execution round-trip test to `pkg/provider/responses/integration_test.go`: mock backend returns a tool call, engine executes it, provider sends the result back. Verify the multi-turn loop completes (FR-003)

**Checkpoint**: Agentic loop works end-to-end with the Responses API provider including tool calls.

---

## Phase 5: User Story 4 - Migration from Chat Completions (Priority: P2)

**Goal**: Switching from Chat Completions to Responses API provider preserves all behavior.

**Independent Test**: Run the conformance test suite against the Responses API provider.

- [x] T015 (antwort-wuh.1) [US4] Add startup validation in `pkg/provider/responses/provider.go`: probe the backend's `/v1/responses` endpoint during construction. Return a clear error if the endpoint is not available (FR-013)
- [x] T016 (antwort-wuh.2) [US4] Run `make api-test` with the Responses API provider configured against a mock backend. Verify all conformance tests pass (FR-014)

**Checkpoint**: Conformance suite passes with the Responses API provider.

---

## Phase 6: Polish

- [x] T017 (antwort-6n0.1) Run `go vet ./pkg/... ./cmd/...` and verify clean
- [x] T018 (antwort-6n0.2) Run full test suite (`go test ./pkg/... ./test/integration/`) and verify zero regressions
- [x] T019 (antwort-6n0.3) Update spec 016 FR-007a to reference the provider layer instead of the engine

**Checkpoint**: All tests green. Responses API provider ready for deployment.

---

## Dependencies

- Phase 1: No dependencies (can start immediately)
- Phase 2: Depends on Phase 1 (tool expansion must be in provider layer before Responses API provider)
- Phase 3: Depends on Phase 2 (needs the provider to exist)
- Phase 4: Depends on Phase 2 (needs the provider to exist)
- Phase 3 and Phase 4: Can run in parallel (independent concerns)
- Phase 5: Depends on Phases 2, 3, 4 (needs full provider functionality)
- Phase 6: Depends on all previous phases

## Implementation Strategy

### MVP First (Phase 1 + Phase 2)

1. Complete Phase 1: Move expandBuiltinTools to provider layer
2. Complete Phase 2: Basic Responses API provider with streaming
3. **STOP and VALIDATE**: Test with a vLLM backend that supports `/v1/responses`

### Incremental Delivery

1. Phase 1 (foundational refactor) can be merged independently
2. Phase 2 delivers the core provider (MVP)
3. Phases 3+4 verify the agentic loop and state management (parity with Chat Completions)
4. Phase 5 validates full migration path
