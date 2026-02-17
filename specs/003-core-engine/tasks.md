# Tasks: Core Engine & Provider Abstraction

**Input**: Design documents from `/specs/003-core-engine/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Tests are included as part of implementation tasks (Go convention: test file alongside source file).

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Prerequisite Amendment)

**Purpose**: Amend Spec 001 Response type and create package structure

- [ ] T001 (antwort-e2c.1) Add `Input []Item` field to Response type in `pkg/api/types.go` with `json:"input,omitempty"` tag. Update any existing Response validation or serialization tests in `pkg/api/types_test.go` to include the new field.
- [ ] T002 (antwort-e2c.2) Create package directory structure: `pkg/provider/`, `pkg/provider/vllm/`, `pkg/engine/`
- [ ] T003 (antwort-e2c.3) [P] Create `pkg/provider/doc.go` with package documentation describing the protocol-agnostic provider interface
- [ ] T004 (antwort-e2c.4) [P] Create `pkg/engine/doc.go` with package documentation describing the orchestration engine
- [ ] T005 (antwort-e2c.5) [P] Create `pkg/provider/vllm/doc.go` with package documentation describing the Chat Completions adapter

---

## Phase 2: Foundational (Provider Interface & Types)

**Purpose**: Define the Provider interface, all provider-level types, and capability validation. MUST be complete before any user story work.

**CRITICAL**: No user story work can begin until this phase is complete.

- [ ] T006 (antwort-5tk.1) Define Provider interface (Name, Capabilities, Complete, Stream, ListModels, Close) and Translator interface (TranslateRequest, TranslateResponse) in `pkg/provider/provider.go` (FR-001, FR-002)
- [ ] T007 (antwort-5tk.2) [P] Define ProviderCapabilities, ProviderRequest, ProviderResponse, ProviderEvent, ProviderEventType, ProviderMessage, ProviderToolCall, ProviderFunctionCall, ProviderTool, ProviderFunctionDef, and ModelInfo types in `pkg/provider/types.go` (FR-003, FR-004, FR-005, FR-006)
- [ ] T008 (antwort-5tk.3) [P] Implement capability validation function (check request features against ProviderCapabilities) in `pkg/provider/capabilities.go` with tests in `pkg/provider/capabilities_test.go` (FR-003, FR-012)
- [ ] T009 (antwort-5tk.4) [P] Write provider type serialization and construction tests in `pkg/provider/types_test.go`
- [ ] T010 (antwort-5tk.5) [P] Define VLLMConfig struct with defaults (BaseURL, APIKey, Timeout, MaxRetries) in `pkg/provider/vllm/config.go` (FR-037)
- [ ] T011 (antwort-5tk.6) [P] Define internal Chat Completions request/response types (ChatCompletionRequest, ChatCompletionResponse, ChatCompletionChunk, ChatCompletionMessage, ChatCompletionChoice) in `pkg/provider/vllm/types.go`
- [ ] T012 (antwort-5tk.7) [P] Define EngineConfig struct (DefaultModel) in `pkg/engine/config.go` (FR-038)

**Checkpoint**: Provider interface and types ready. All user story implementation can now begin.

---

## Phase 3: User Story 1 - Non-Streaming Request (Priority: P1) MVP

**Goal**: Complete non-streaming request-response cycle from engine through vLLM adapter to mock backend and back.

**Independent Test**: Run engine with mock Chat Completions HTTP server, send a text prompt with `stream: false`, verify Response contains correctly structured output Items with valid status and usage.

### Implementation for User Story 1

- [ ] T013 (antwort-su6.1) [US1] Implement request translation (CreateResponseRequest -> ProviderRequest) in `pkg/engine/translate.go`: map instructions to system message, translate each Item type per FR-021 rules (user, assistant, system, function_call, function_call_output, reasoning=skip), map tools and tool_choice per FR-023, map inference parameters per FR-024. Write table-driven tests in `pkg/engine/translate_test.go` covering all Item types.
- [ ] T014 (antwort-su6.2) [US1] Implement Chat Completions request translation (ProviderRequest -> HTTP request body) in `pkg/provider/vllm/translate.go`: build ChatCompletionRequest from ProviderMessage array, always set n=1 (FR-026), map ProviderTool to Chat Completions tool format. Write table-driven tests in `pkg/provider/vllm/translate_test.go`.
- [ ] T015 (antwort-su6.3) [US1] Implement Chat Completions response translation (HTTP response -> ProviderResponse) in `pkg/provider/vllm/response.go`: parse choices[0].message.content to assistant Item, parse tool_calls to function_call Items (FR-027), map finish_reason to status (FR-028), map usage fields (FR-029). Write tests in `pkg/provider/vllm/response_test.go`.
- [ ] T016 (antwort-su6.4) [US1] Implement HTTP error mapping (status code -> APIError) in `pkg/provider/vllm/errors.go`: map 400, 401/403, 404, 429, 500+ per FR-034, handle network errors per FR-035. Write tests covering each status code.
- [ ] T017 (antwort-su6.5) [US1] Implement VLLMProvider.Complete method in `pkg/provider/vllm/vllm.go`: create HTTP request with JSON body, send to BaseURL + "/v1/chat/completions", parse response using response.go translation, handle errors, respect context cancellation (FR-039). Include constructor function `New(config VLLMConfig)` that creates http.Client.
- [ ] T018 (antwort-su6.6) [US1] Implement Engine struct and CreateResponse method (non-streaming path) in `pkg/engine/engine.go`: constructor accepting Provider (required) and ResponseStore (nil-safe, FR-011), translate request via translate.go, call provider.Complete, generate response ID and item IDs (FR-013), populate Response fields (FR-014), write via ResponseWriter. Include engine validation (FR-012) calling capability check from T008.
- [ ] T019 (antwort-su6.7) [US1] Write engine non-streaming integration test in `pkg/engine/engine_test.go`: create mock Provider returning a fixed ProviderResponse, create mock ResponseWriter capturing WriteResponse calls, verify complete Response object matches expected structure with correct status, Items, usage, and IDs.
- [ ] T020 (antwort-su6.8) [US1] Write vLLM adapter integration test in `pkg/provider/vllm/vllm_test.go`: start `net/http/httptest.Server` that returns a fixed Chat Completions JSON response, create VLLMProvider pointing at test server, call Complete, verify ProviderResponse contains correctly translated Items and usage.

**Checkpoint**: Non-streaming end-to-end path works. Engine + vLLM adapter produce valid Response from mock backend.

---

## Phase 4: User Story 2 - Streaming Request (Priority: P1)

**Goal**: Complete streaming request-response cycle with correct event sequence from engine through vLLM SSE parsing.

**Independent Test**: Run engine with mock Chat Completions SSE server, send a streaming request, verify events arrive in correct OpenResponses order with proper types.

### Implementation for User Story 2

- [x] T021 (antwort-3cu.1) [US2] Implement SSE chunk parser in `pkg/provider/vllm/stream.go`: use bufio.Scanner to read `data: {...}` lines from HTTP response body, parse each as ChatCompletionChunk JSON, handle `data: [DONE]` sentinel producing ProviderEventDone, handle malformed chunks (skip + log warning). Write tests in `pkg/provider/vllm/stream_test.go` covering text deltas, done sentinel, and malformed input.
- [x] T022 (antwort-3cu.2) [US2] Implement VLLMProvider.Stream method in `pkg/provider/vllm/vllm.go`: create HTTP request with stream=true and stream_options (FR-025), send request, spawn goroutine reading from response body via stream.go parser, produce ProviderEvents on channel, close channel when done or on error, respect context cancellation.
- [x] T023 (antwort-3cu.3) [US2] Implement ProviderEvent to api.StreamEvent mapping in `pkg/engine/events.go`: map ProviderEventTextDelta to EventOutputTextDelta, ProviderEventTextDone to EventOutputTextDone, ProviderEventDone to EventResponseCompleted, handle sequence number generation. Write tests in `pkg/engine/events_test.go`.
- [x] T024 (antwort-3cu.4) [US2] Implement Engine.CreateResponse streaming path in `pkg/engine/engine.go`: detect stream=true, call provider.Stream, generate synthetic lifecycle events (response.created, response.in_progress, output_item.added, content_part.added per FR-010), consume ProviderEvents from channel, map via events.go, write via ResponseWriter, emit terminal event (completed/failed/cancelled per FR-040), handle context cancellation.
- [x] T025 (antwort-3cu.5) [US2] Write engine streaming integration test in `pkg/engine/engine_test.go`: create mock Provider returning a channel of text delta events, create mock ResponseWriter capturing WriteEvent calls, verify event sequence matches expected order (created, in_progress, item.added, part.added, deltas, text.done, part.done, item.done, completed). Test finish_reason=length producing incomplete status.
- [x] T026 (antwort-3cu.6) [US2] Write vLLM streaming integration test in `pkg/provider/vllm/vllm_test.go`: start httptest.Server that returns Chat Completions SSE chunks (multiple data lines + [DONE]), create VLLMProvider, call Stream, consume all events from channel, verify correct ProviderEventType sequence with expected delta content. Test context cancellation stops stream.

**Checkpoint**: Streaming end-to-end path works. Events arrive in correct order. Context cancellation handled.

---

## Phase 5: User Story 3 - Tool Call Streaming (Priority: P1)

**Goal**: Correctly buffer and reassemble incremental tool call arguments from Chat Completions streaming, producing proper function_call Items and events.

**Independent Test**: Mock backend returns tool call SSE chunks with arguments split across 5+ chunks. Verify engine produces correct function_call_arguments.delta and function_call_arguments.done events, with fully assembled JSON arguments.

### Implementation for User Story 3

- [ ] T027 (antwort-nyy.1) [US3] Extend SSE chunk parser in `pkg/provider/vllm/stream.go` to handle delta.tool_calls: detect tool_calls array in chunk, buffer argument fragments per tool call index using map[int]*strings.Builder, emit ProviderEventToolCallDelta for each fragment, emit ProviderEventToolCallDone when finish_reason="tool_calls" or new tool call starts (FR-031). Handle first tool call chunk with function name (FR-032). Write tests in `pkg/provider/vllm/stream_test.go` for single tool call, multiple tool calls, and incremental argument assembly.
- [ ] T028 (antwort-nyy.2) [US3] Extend ProviderEvent -> StreamEvent mapping in `pkg/engine/events.go` to handle tool call events: map ProviderEventToolCallDelta to EventFunctionCallArgsDelta, map ProviderEventToolCallDone to EventFunctionCallArgsDone, generate output_item.added event with function_call type on first tool call delta per tool call index. Write additional tests in `pkg/engine/events_test.go`.
- [ ] T029 (antwort-nyy.3) [US3] Extend Engine.CreateResponse streaming path in `pkg/engine/engine.go` to handle tool call events: track active tool call items (index -> item_id mapping), emit output_item.added for each new tool call, emit output_item.done after tool call done, include function_call Items in final Response output. Handle response status=completed with function_call items signaling client-side tool execution.
- [ ] T030 (antwort-nyy.4) [US3] Write engine tool call streaming test in `pkg/engine/engine_test.go`: mock Provider emits text delta + tool call delta events, verify engine produces interleaved text and tool call events in correct order, verify final Response contains both message and function_call output Items. Test multiple parallel tool calls.

**Checkpoint**: Tool call streaming works. Arguments correctly buffered and reassembled. Client-side tool execution enabled.

---

## Phase 6: User Story 4 - Conversation Chaining (Priority: P2)

**Goal**: Reconstruct conversation history from stored response chains and build complete context for the backend.

**Independent Test**: Mock ResponseStore returns a chain of 3 responses. Verify engine reconstructs correct chronological message sequence with only the most recent instructions.

### Implementation for User Story 4

- [ ] T031 (antwort-lxi.1) [US4] Implement conversation history reconstruction in `pkg/engine/history.go`: loadResponseChain function that follows previous_response_id links iteratively using ResponseStore.GetResponse, detect cycles via visited ID set (FR-018), reverse collected responses to chronological order (FR-016), extract input and output Items from each stored Response, flatten to ProviderMessages, apply most-recent-instructions rule (FR-017). Handle nil store returning error (FR-019). Write comprehensive tests in `pkg/engine/history_test.go` covering: chain of 3 responses, cycle detection, nil store error, not-found error, single response (no chain), instructions superseding.
- [ ] T032 (antwort-lxi.2) [US4] Integrate history reconstruction into Engine.CreateResponse in `pkg/engine/engine.go`: check for previous_response_id, call history.go to reconstruct messages, prepend reconstructed messages to current request's translated messages before calling provider. Store completed response with input Items when store is available (FR-011).
- [ ] T033 (antwort-lxi.3) [US4] Write engine conversation chaining integration test in `pkg/engine/engine_test.go`: create mock ResponseStore with pre-populated response chain, send request with previous_response_id, capture the ProviderRequest sent to mock Provider, verify messages array contains full conversation history in correct order with correct roles.

**Checkpoint**: Conversation chaining works with mock store. History reconstruction produces correct message sequences.

---

## Phase 7: User Story 5 - Capability Validation (Priority: P2)

**Goal**: Reject requests requiring unsupported provider capabilities with clear error messages before any backend call.

**Independent Test**: Create providers with specific capabilities disabled, submit requests requiring those capabilities, verify rejection with descriptive errors.

### Implementation for User Story 5

- [ ] T034 (antwort-u96.1) [US5] Implement request-level capability validation in `pkg/engine/validate.go`: check request against ProviderCapabilities, detect vision requirement (image content parts), detect tool calling requirement (tools array non-empty), detect streaming requirement (stream=true), detect audio requirement. Return specific invalid_request error for each unsupported capability (FR-012). Write table-driven tests in `pkg/engine/validate_test.go` covering each capability check independently.
- [ ] T035 (antwort-u96.2) [US5] Integrate capability validation into Engine.CreateResponse in `pkg/engine/engine.go`: call validate.go checks after request translation but before provider.Complete/Stream call. Ensure validation runs for both streaming and non-streaming paths.
- [ ] T036 (antwort-u96.3) [US5] Write engine capability rejection tests in `pkg/engine/engine_test.go`: create mock Provider with specific capabilities disabled (Vision=false, ToolCalling=false, Streaming=false), submit requests requiring those features, verify engine returns without calling provider, verify error type is invalid_request with descriptive message.

**Checkpoint**: Capability validation catches unsupported features before backend calls. Clear error messages returned.

---

## Phase 8: User Story 6 - Multimodal Content Translation (Priority: P3)

**Goal**: Translate multimodal input Items (text + images) to Chat Completions content array format.

**Independent Test**: Send request with image content parts to mock backend, verify Chat Completions content array contains correct image encoding.

### Implementation for User Story 6

- [ ] T037 (antwort-93c.1) [US6] Extend request translation in `pkg/engine/translate.go` to handle multimodal ContentParts: convert input_text to `{type: "text", text: "..."}`, convert input_image with URL to `{type: "image_url", image_url: {url: "..."}}`, convert input_image with base64 data to data URI format (FR-022). Write additional tests in `pkg/engine/translate_test.go` for each content type.
- [ ] T038 (antwort-93c.2) [US6] Extend Chat Completions request translation in `pkg/provider/vllm/translate.go` to handle multimodal ProviderMessage content: when content is []ContentPart, serialize as Chat Completions content array (not string). Write tests in `pkg/provider/vllm/translate_test.go` for mixed text+image input.
- [ ] T039 (antwort-93c.3) [US6] Write engine multimodal integration test in `pkg/engine/engine_test.go`: send request with mixed text and image content parts, capture outbound ProviderRequest, verify content array formatting. Test capability rejection for audio/video when Vision-only provider.

**Checkpoint**: Multimodal content translation works for text and images. Audio/video correctly rejected.

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Error handling edge cases, reasoning token support, and documentation

- [ ] T040 (antwort-2m3.1) [P] Handle reasoning tokens in `pkg/provider/vllm/stream.go` and `pkg/provider/vllm/response.go`: detect `reasoning_content` field in Chat Completions response/chunks, produce ProviderEventReasoningDelta/Done events, translate to reasoning output Items. Write tests for reasoning token detection and mapping.
- [ ] T041 (antwort-2m3.2) [P] Handle edge cases in `pkg/provider/vllm/stream.go`: empty response (no choices -> server_error), unknown finish_reason (treat as completed + log warning), mixed text+tool_calls in single response. Write edge case tests.
- [ ] T042 (antwort-2m3.3) [P] Implement VLLMProvider.ListModels in `pkg/provider/vllm/vllm.go`: send GET to BaseURL + "/v1/models", parse response into []ModelInfo. Write test with mock server.
- [ ] T043 (antwort-2m3.4) [P] Implement VLLMProvider.Close in `pkg/provider/vllm/vllm.go`: close idle HTTP connections via http.Client transport CloseIdleConnections.
- [ ] T044 (antwort-2m3.5) Run `go vet ./...` and `go test ./...` across all new packages to verify compilation and test passing
- [ ] T045 (antwort-2m3.6) Validate quickstart.md code examples compile and match actual API signatures

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies. Start immediately.
- **Phase 2 (Foundational)**: Depends on Phase 1 (T001 for Response.Input, T002 for directories).
- **Phase 3 (US1 Non-Streaming)**: Depends on Phase 2 (interfaces and types must exist).
- **Phase 4 (US2 Streaming)**: Depends on Phase 3 (non-streaming path provides base engine structure).
- **Phase 5 (US3 Tool Calls)**: Depends on Phase 4 (streaming infrastructure must exist to extend).
- **Phase 6 (US4 Conversation)**: Depends on Phase 3 (engine structure). Independent of Phase 4/5.
- **Phase 7 (US5 Capabilities)**: Depends on Phase 2 (capability types). Independent of US1-US4 implementation.
- **Phase 8 (US6 Multimodal)**: Depends on Phase 3 (translation infrastructure). Independent of US2-US5.
- **Phase 9 (Polish)**: Depends on Phases 3-5 (core streaming and tool call infrastructure).

### User Story Dependencies

- **US1 (Non-Streaming)**: Foundation only. MVP.
- **US2 (Streaming)**: Extends US1 engine structure with streaming path.
- **US3 (Tool Calls)**: Extends US2 streaming with tool call buffering.
- **US4 (Conversation)**: Independent of US2/US3. Needs US1 engine structure only.
- **US5 (Capabilities)**: Independent of all other stories. Needs only Phase 2 types.
- **US6 (Multimodal)**: Independent of US2-US5. Extends US1 translation logic.

### Parallel Opportunities

Within Phase 2:
- T007, T008, T009, T010, T011, T012 can all run in parallel (different files)

After Phase 3 (US1):
- US4, US5, US6 can start in parallel (independent of US2/US3)

Within Phase 9:
- T040, T041, T042, T043 can all run in parallel (different concerns)

---

## Parallel Example: Phase 2 (Foundational)

```bash
# All foundational type/interface definitions can run in parallel:
Task: "Define Provider interface in pkg/provider/provider.go"
Task: "Define provider types in pkg/provider/types.go"
Task: "Implement capability validation in pkg/provider/capabilities.go"
Task: "Define VLLMConfig in pkg/provider/vllm/config.go"
Task: "Define Chat Completions types in pkg/provider/vllm/types.go"
Task: "Define EngineConfig in pkg/engine/config.go"
```

## Parallel Example: After US1 Complete

```bash
# These user stories can start in parallel:
Task: "US4 - Conversation history reconstruction in pkg/engine/history.go"
Task: "US5 - Capability validation in pkg/engine/validate.go"
Task: "US6 - Multimodal content translation in pkg/engine/translate.go"
```

---

## Implementation Strategy

### MVP First (User Stories 1 + 2)

1. Complete Phase 1: Setup (Response amendment, directory structure)
2. Complete Phase 2: Foundational (interfaces, types, config)
3. Complete Phase 3: US1 Non-Streaming (first end-to-end path)
4. **STOP and VALIDATE**: Test with mock Chat Completions server
5. Complete Phase 4: US2 Streaming (primary consumption mode)
6. **STOP and VALIDATE**: Test streaming event sequence

### Incremental Delivery

1. Setup + Foundational -> Types ready
2. US1 (Non-Streaming) -> First working end-to-end path (MVP!)
3. US2 (Streaming) -> Primary production path works
4. US3 (Tool Calls) -> Agentic workflows enabled
5. US4 (Conversation) -> Multi-turn conversations work
6. US5 (Capabilities) -> Clear error messages for unsupported features
7. US6 (Multimodal) -> Vision model support
8. Polish -> Edge cases, reasoning tokens, model listing

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- US2 and US3 extend US1 code (same files), so they must be sequential
- US4, US5, US6 work on separate files and can run in parallel after US1
- Go convention: test files sit alongside source files (`*_test.go`)
- All tests use mock HTTP servers (net/http/httptest) and mock interfaces, no external dependencies
