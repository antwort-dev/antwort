# Feature Specification: Core Engine & Provider Abstraction

**Feature Branch**: `003-core-engine`
**Created**: 2026-02-17
**Status**: Draft
**Input**: User description: "Core Engine & Provider Abstraction for the Antwort OpenResponses gateway, covering three layers: protocol-agnostic provider interface, core orchestration engine implementing ResponseCreator, and vLLM Chat Completions adapter as the first provider implementation."

## Overview

This specification defines the core orchestration engine and provider abstraction layer for Antwort. Together, these components bridge the transport layer (Spec 002) to LLM inference backends by implementing the `ResponseCreator` interface and delegating inference to pluggable provider adapters.

The design introduces three layers:

1. **Provider interface**: A protocol-agnostic abstraction for LLM backend communication. Each adapter handles its own backend protocol internally (Chat Completions, Responses API, or future protocols). The interface defines capabilities, inference operations (streaming and non-streaming), and model management.

2. **Core engine**: Implements `transport.ResponseCreator` from Spec 002. The engine translates incoming `CreateResponseRequest` objects into provider-specific requests, invokes the provider, maps provider events back to OpenResponses streaming events, and writes results through the `ResponseWriter`. The engine uses nil-safe composition for optional capabilities (response storage, future tool execution) so features degrade gracefully when their dependencies are absent.

3. **vLLM adapter**: The first provider implementation. Translates Antwort's provider requests to the `/v1/chat/completions` protocol, sends them to a vLLM (or any OpenAI-compatible) backend, and translates responses back. Handles SSE chunk parsing, streaming event normalization, and backend-specific quirks.

## Clarifications

### Session 2026-02-17

- Q: Should the Provider interface assume a specific backend protocol (Chat Completions)? -> A: No. The interface is protocol-agnostic. Each adapter handles its own backend protocol internally. This enables both Chat Completions adapters (vLLM) and Responses API proxy adapters (forwarding to /v1/responses backends) as separate implementations behind the same interface.
- Q: Who maps ProviderEvent to api.StreamEvent? -> A: The core engine. The provider returns ProviderEvent types; the engine maps them to api.StreamEvent and writes them via ResponseWriter. This keeps providers independent of the transport layer.
- Q: How should optional engine capabilities (storage, tools) be handled? -> A: Nil-safe composition. The engine accepts optional interfaces as parameters. When nil, the corresponding feature is disabled. Methods on nil receivers return gracefully (no-op or appropriate error). No feature flags needed.
- Q: Should conversation history reconstruction (previous_response_id) be part of this spec? -> A: Define the reconstruction logic against the ResponseStore interface. Testable with mocks. Not functional in production until Spec 005 provides a real store implementation.
- Q: Should GET /v1/models be added? -> A: No. ListModels() exists on the Provider interface for internal use (model validation), but the HTTP endpoint is deferred.
- Q: How should retries be handled? -> A: Simple configuration (max retries count). No prescribed backoff strategy or retryable-error classification. Implementation decides.
- Q: Should the spec define explicit translation rules for Item types to Chat Completions messages? -> A: Yes. Every Item type must have a documented mapping, including edge cases like system instructions, multimodal content encoding, tool results, and consecutive same-role messages.
- Q: The Response type lacks an Input field. How should conversation reconstruction access input Items from prior turns? -> A: Add `Input []Item` to the Response type, matching the OpenResponses specification. This requires a backwards-compatible addition to Spec 001's data model. The engine stores both input and output in each Response, enabling full conversation reconstruction from response chains.
- Q: How should the adapter handle Chat Completions responses with multiple choices (n > 1)? -> A: Use only the first choice (index 0). The adapter always sets n=1 in outbound requests. The OpenResponses API produces exactly one response per request, so multiple choices have no clear mapping.

## User Scenarios & Testing

### User Story 1 - Submit a Non-Streaming Request Through the Full Stack (Priority: P1)

A developer sends a `POST /v1/responses` request with `stream: false`. The transport layer deserializes the request, passes it to the core engine, which translates it to a provider request, calls the vLLM backend via Chat Completions, translates the response back to OpenResponses format, and returns it as JSON. The developer receives a complete `Response` object with the model's output as Items.

**Why this priority**: This is the first end-to-end path from HTTP request to LLM backend and back. It validates the engine's orchestration logic, provider request translation, Chat Completions communication, response translation, and the wiring between transport, engine, and provider.

**Independent Test**: Can be tested by running the engine with a mock HTTP server that speaks Chat Completions, sending a request with a text prompt, and verifying the response contains correctly structured output Items with valid status and usage statistics.

**Acceptance Scenarios**:

1. **Given** an engine configured with a vLLM provider pointing at a Chat Completions backend, **When** a valid CreateResponseRequest with a text input Item is submitted, **Then** the engine returns a Response with status `completed`, containing at least one assistant message output Item
2. **Given** a request with `instructions` set, **When** the engine translates the request, **Then** the instructions appear as the first message (system role) in the Chat Completions messages array
3. **Given** a request with multiple input Items (user message, function_call_output), **When** the engine translates the request, **Then** each Item maps to the correct Chat Completions message format (user role, tool role respectively)
4. **Given** a backend that returns usage statistics, **When** the response is translated back, **Then** the Response includes accurate input_tokens, output_tokens, and total_tokens values

---

### User Story 2 - Submit a Streaming Request and Receive Events (Priority: P1)

A developer sends a `POST /v1/responses` request with `stream: true`. The engine invokes the provider's streaming interface, receives a channel of provider events (text deltas, tool call deltas, completion signals), maps each to an OpenResponses streaming event, and writes them to the ResponseWriter. The developer receives properly ordered SSE events including lifecycle events (`response.created`, `response.in_progress`, `output_item.added`, text deltas, `output_item.done`, `response.completed`).

**Why this priority**: Streaming is the primary consumption mode for LLM inference. Without correct event translation and ordering, streaming responses are unusable. This is co-equal with US1 because most production workloads use streaming.

**Independent Test**: Can be tested by running the engine with a mock HTTP server that returns Chat Completions SSE chunks, sending a streaming request, and verifying the output events follow the correct OpenResponses event sequence with proper types, ordering, and content.

**Acceptance Scenarios**:

1. **Given** an engine with a vLLM provider, **When** a streaming request is submitted, **Then** the engine emits events in this order: `response.created`, `response.in_progress`, `response.output_item.added`, `response.content_part.added`, zero or more `response.output_text.delta`, `response.output_text.done`, `response.content_part.done`, `response.output_item.done`, `response.completed`
2. **Given** a backend streaming text token by token, **When** each Chat Completions SSE chunk arrives with `delta.content`, **Then** the engine emits a `response.output_text.delta` event with the text fragment
3. **Given** a backend that returns `finish_reason: "length"`, **When** the stream completes, **Then** the engine emits `response.completed` with the Response status set to `incomplete` (output truncated)
4. **Given** a streaming request where the client disconnects (context cancelled), **When** the engine detects cancellation, **Then** it stops reading from the provider stream and cleans up resources

---

### User Story 3 - Provider Returns Tool Call Results in Streaming Mode (Priority: P1)

A developer sends a request with tool definitions. The backend decides to call a tool and returns tool call chunks in the Chat Completions stream. The engine correctly assembles incremental tool call arguments, produces the appropriate OpenResponses events (`response.output_item.added` with function_call type, `response.function_call_arguments.delta`, `response.function_call_arguments.done`, `response.output_item.done`), and marks the response as completed with function_call Items in the output.

**Why this priority**: Tool calling is essential for agentic workflows. Even though server-side tool execution is deferred to Spec 004, the engine must correctly translate tool call results from the backend so that client-side tool execution works immediately.

**Independent Test**: Can be tested with a mock backend that returns Chat Completions chunks containing `delta.tool_calls` with incremental JSON argument fragments, verifying the engine buffers fragments correctly and emits complete function_call Items.

**Acceptance Scenarios**:

1. **Given** a request with tool definitions and a backend that decides to call a tool, **When** the streaming response includes `delta.tool_calls` chunks, **Then** the engine buffers the incremental JSON argument fragments and emits `response.function_call_arguments.delta` events for each fragment
2. **Given** a tool call with arguments split across 5 SSE chunks, **When** the final chunk arrives with `finish_reason: "tool_calls"`, **Then** the engine emits `response.function_call_arguments.done` with the fully assembled arguments JSON string, followed by `response.output_item.done` with a complete function_call Item
3. **Given** a backend that returns multiple tool calls in a single response, **When** the stream completes, **Then** each tool call is represented as a separate function_call output Item, each with its own call_id, name, and arguments

---

### User Story 4 - Conversation Chaining via previous_response_id (Priority: P2)

A developer sends a follow-up request with `previous_response_id` set. The engine loads the referenced response (and its chain of previous responses) from the response store, reconstructs the full conversation history as a flat messages array, appends the new input Items, and sends the complete context to the backend. The developer receives a response that reflects awareness of the entire conversation.

**Why this priority**: Conversation chaining is a core stateful feature of the OpenResponses API. While the response store implementation is deferred to Spec 005, the reconstruction logic must be defined and testable now. Without it, multi-turn conversations are not possible.

**Independent Test**: Can be tested with a mock ResponseStore that returns predetermined response chains, verifying the engine reconstructs the correct message sequence and sends it to the provider.

**Acceptance Scenarios**:

1. **Given** a mock store with responses A -> B -> C (where C references B, B references A), **When** a request arrives referencing response C, **Then** the engine reconstructs messages from A, B, and C in chronological order, appends the new input, and sends the full sequence to the provider
2. **Given** a request with `previous_response_id` but no response store configured (nil), **When** the engine processes the request, **Then** it returns an error indicating that conversation chaining requires a response store
3. **Given** a request with `previous_response_id` referencing a non-existent response, **When** the engine queries the store, **Then** it returns a `not_found` error
4. **Given** a chain of responses where earlier responses included `instructions`, **When** the engine reconstructs the conversation, **Then** only the most recent `instructions` value is used as the system message (not duplicated from each response)

---

### User Story 5 - Provider Capability Validation (Priority: P2)

A developer sends a request that requires capabilities the configured provider does not support (for example, image inputs to a text-only model, or tool definitions to a provider that cannot do tool calling). The engine checks the provider's declared capabilities before sending the request and returns a clear error identifying the unsupported feature.

**Why this priority**: Early validation prevents wasted inference calls and provides actionable error messages. Without capability checking, the backend would receive invalid requests and return cryptic errors.

**Independent Test**: Can be tested by creating a provider with specific capabilities disabled (e.g., Vision: false) and submitting requests that require those capabilities, verifying the engine rejects them with descriptive errors before calling the provider.

**Acceptance Scenarios**:

1. **Given** a provider with `Vision: false`, **When** a request includes an input Item with image content, **Then** the engine returns an `invalid_request` error stating the provider does not support image inputs
2. **Given** a provider with `ToolCalling: false`, **When** a request includes tool definitions, **Then** the engine returns an `invalid_request` error stating the provider does not support tool calling
3. **Given** a provider with `Streaming: false`, **When** a streaming request (`stream: true`) is submitted, **Then** the engine returns an `invalid_request` error stating the provider does not support streaming
4. **Given** a provider with `MaxContextWindow: 8192`, **When** the engine can estimate that the input exceeds this window, **Then** the engine either applies truncation (if `truncation: "auto"`) or returns an error (if `truncation: "disabled"`)

---

### User Story 6 - Multimodal Content Translation (Priority: P3)

A developer sends a request containing multimodal input Items (text mixed with images). The vLLM adapter translates these into the Chat Completions content array format, encoding images appropriately. The backend processes the multimodal input and the response is translated back normally.

**Why this priority**: Multimodal support is required for vision-capable models but not for the most common text-only use case. It extends the translation logic without changing the core flow.

**Independent Test**: Can be tested by sending a request with image content parts to a mock backend that accepts multimodal Chat Completions requests, verifying the content array contains the correct image encoding format.

**Acceptance Scenarios**:

1. **Given** a request with a user message containing text and an image URL, **When** the adapter translates the request, **Then** the Chat Completions content array includes a text part and an image_url part with the correct URL
2. **Given** a request with a user message containing an inline base64-encoded image, **When** the adapter translates the request, **Then** the Chat Completions content array includes an image_url part with a data URI containing the base64 data and media type
3. **Given** a request with audio or video content parts, **When** the provider's capabilities indicate these are not supported, **Then** the engine returns an `invalid_request` error before calling the adapter

---

### Edge Cases

- What happens when the backend returns an empty response (no choices)? The engine returns a `server_error` indicating the backend produced no output.
- What happens when the backend returns a malformed SSE chunk? The engine skips the malformed chunk, logs a warning, and continues processing the stream. If the stream cannot recover, the engine emits a `response.failed` event.
- What happens when the backend returns HTTP 429 (rate limited)? The adapter maps this to a `too_many_requests` error. Retry behavior is implementation-defined based on config.
- What happens when the backend is unreachable (connection refused, DNS failure)? The adapter returns a `server_error` with a descriptive message. The engine propagates this as a failed response.
- What happens when a streaming response produces both text content and tool calls? The engine emits events for both: text delta events for the content portion and function_call events for the tool calls. Each output Item is independent.
- What happens when `previous_response_id` forms a cycle (A references B, B references A)? The engine detects the cycle during chain traversal (by tracking visited IDs) and returns an `invalid_request` error.
- How does the system handle reasoning tokens from models like DeepSeek R1? The vLLM adapter detects reasoning content (via `reasoning_content` field or provider-specific markers) and maps it to a `reasoning` output Item. If the model does not produce reasoning, no reasoning Item is emitted.
- What happens when the adapter receives a Chat Completions response with an unknown `finish_reason`? The engine treats it as `completed` and logs a warning about the unrecognized finish reason.
- What happens when consecutive input Items have the same role? The adapter preserves them as separate messages in the Chat Completions array. It does not merge consecutive same-role messages, since the backend may interpret them differently than a merged message.

## Requirements

### Functional Requirements

**Provider Interface**

- **FR-001**: System MUST define a Provider interface with operations for non-streaming inference, streaming inference, model listing, capability declaration, and resource cleanup
- **FR-002**: The Provider interface MUST be protocol-agnostic. It MUST NOT assume any specific backend protocol. Each adapter implementation handles its own protocol translation internally.
- **FR-003**: System MUST define a ProviderCapabilities structure declaring what features the backend supports: streaming, tool calling, vision, audio, reasoning, maximum context window, supported models, and provider-specific extensions
- **FR-004**: System MUST define provider-level request, response, and event types that represent the translation boundary between the engine and the backend. These types are stripped of transport and storage concerns.
- **FR-005**: The streaming inference operation MUST return a channel of ProviderEvent values. The channel MUST be closed by the provider when the stream completes or errors.
- **FR-006**: ProviderEvent MUST support event types for: text delta, text done, tool call delta, tool call done, reasoning delta, reasoning done, stream done, and stream error

**Core Engine**

- **FR-007**: System MUST implement the `ResponseCreator` interface defined in Spec 002 (transport layer)
- **FR-008**: For non-streaming requests, the engine MUST translate the CreateResponseRequest to a ProviderRequest, call the provider's Complete method, translate the ProviderResponse to a Response object, and write it via ResponseWriter
- **FR-009**: For streaming requests, the engine MUST translate the request, call the provider's Stream method, consume events from the channel, map each ProviderEvent to one or more api.StreamEvent values, and write them via ResponseWriter in the correct order
- **FR-010**: The engine MUST generate synthetic lifecycle events that the backend does not produce: `response.created`, `response.in_progress`, `response.output_item.added`, `response.content_part.added`, `response.content_part.done`, `response.output_item.done`, and `response.completed`/`response.failed`/`response.cancelled`
- **FR-011**: The engine MUST accept an optional ResponseStore (nil-safe). When provided and the request has `store: true`, the engine MUST store the completed response including both the original input Items and the generated output Items. When nil, store operations are skipped silently.
- **FR-012**: The engine MUST validate incoming requests against the provider's declared capabilities before calling the provider. Requests requiring unsupported capabilities MUST be rejected with an `invalid_request` error identifying the specific unsupported feature.
- **FR-013**: The engine MUST generate unique response IDs (prefixed `resp_`) and item IDs (prefixed `item_`) for all responses and output Items it creates, using the ID generation functions from Spec 001
- **FR-014**: The engine MUST populate the Response's `created_at` timestamp, `model` field (using the actual model returned by the provider, which may differ from the requested model), `status`, and `usage` fields

**Conversation History Reconstruction**

- **FR-015**: When a request includes `previous_response_id` and a ResponseStore is available, the engine MUST load the referenced response and follow the chain of `previous_response_id` links to reconstruct the full conversation history
- **FR-016**: The reconstructed history MUST be ordered chronologically (oldest first), extracting input and output Items from each stored Response, and flattened into the ProviderRequest messages array, followed by the current request's input Items
- **FR-017**: When reconstructing conversation history, the engine MUST use only the most recent `instructions` value as the system message. Earlier instructions in the chain are superseded.
- **FR-018**: The engine MUST detect cycles in the response chain (by tracking visited response IDs) and return an `invalid_request` error if a cycle is found
- **FR-019**: When `previous_response_id` is set but no ResponseStore is configured (nil), the engine MUST return an `invalid_request` error indicating that conversation chaining requires a response store

**Request Translation (Engine to Provider)**

- **FR-020**: The engine MUST translate `CreateResponseRequest.Instructions` to a system-role message placed first in the ProviderRequest messages array
- **FR-021**: The engine MUST translate each input Item to a ProviderMessage according to these rules:
  - `message` Item with role `user` -> message with role `user` and content from the Item's content parts
  - `message` Item with role `assistant` -> message with role `assistant` and text content from the Item's output parts
  - `message` Item with role `system` -> message with role `system` and text content
  - `function_call` Item -> assistant message with a tool_calls array entry containing the function name, call_id, and arguments
  - `function_call_output` Item -> message with role `tool`, tool_call_id matching the originating call, and the output as content
  - `reasoning` Item -> skipped (not sent to Chat Completions backends; reasoning is model-generated, not user-provided)
- **FR-022**: The engine MUST translate multimodal content parts to the Chat Completions content array format: text parts as `{type: "text", text: "..."}`, image URL parts as `{type: "image_url", image_url: {url: "..."}}`, and inline image data as `{type: "image_url", image_url: {url: "data:<media_type>;base64,<data>"}}`
- **FR-023**: The engine MUST map tool definitions from `CreateResponseRequest.Tools` to Chat Completions tool format and map `ToolChoice` values directly (auto, required, none, or specific function)
- **FR-024**: The engine MUST map inference parameters: `temperature`, `top_p`, `max_output_tokens` (to `max_tokens`), and `stop` sequences. Parameters that are nil/unset MUST be omitted from the provider request.
- **FR-025**: The engine MUST include `stream_options: {include_usage: true}` in streaming Chat Completions requests so that usage statistics are returned in the final chunk

**Response Translation (Provider to Engine)**

- **FR-026**: The vLLM adapter MUST always set `n=1` in outbound Chat Completions requests and use only `choices[0]` from the response. The adapter MUST translate `choices[0].message.content` to an assistant message output Item with `output_text` content
- **FR-027**: The vLLM adapter MUST translate Chat Completions response `choices[].message.tool_calls` to separate function_call output Items, each with a unique item ID, the function name, call_id, and arguments
- **FR-028**: The vLLM adapter MUST map `finish_reason` values: `stop` to response status `completed`, `length` to `incomplete`, `tool_calls` to `completed` (with function_call Items in output)
- **FR-029**: The vLLM adapter MUST translate Chat Completions `usage` to OpenResponses `Usage` (prompt_tokens to input_tokens, completion_tokens to output_tokens, total_tokens to total_tokens)

**Streaming Event Translation**

- **FR-030**: The vLLM adapter MUST parse Chat Completions SSE chunks (`data: {...}` lines) and produce ProviderEvent values. The `data: [DONE]` sentinel MUST produce a ProviderEventDone event.
- **FR-031**: The adapter MUST buffer incremental tool call arguments across multiple SSE chunks and emit ProviderEventToolCallDelta for each fragment, followed by ProviderEventToolCallDone when the tool call is complete (indicated by finish_reason or a new tool call starting)
- **FR-032**: The adapter MUST handle the first SSE chunk specially: when a chunk contains a `role` field (indicating a new message), the adapter MUST produce events that the engine uses to emit `output_item.added` and `content_part.added` lifecycle events
- **FR-033**: The adapter MUST normalize backend-specific quirks. For example, if the backend sends per-token `content_index` values, the adapter MUST normalize them to a consistent content part index.

**Error Handling**

- **FR-034**: The adapter MUST map HTTP error responses from the backend to OpenResponses error types: 400 to `invalid_request`, 401/403 to `server_error` (backend auth is a server-side concern), 404 to `not_found` (model not found), 429 to `too_many_requests`, 500+ to `server_error`
- **FR-035**: Network-level errors (connection refused, timeout, DNS failure) MUST be mapped to `server_error` with a descriptive message identifying the backend as the source
- **FR-036**: When a streaming response encounters an error after events have been emitted, the engine MUST emit a `response.failed` event containing the error details, rather than dropping the stream silently

**Configuration**

- **FR-037**: The vLLM adapter MUST be configurable with: backend base URL, optional API key, request timeout, and maximum retry count
- **FR-038**: The engine MUST be configurable with an optional default model (used when the request omits the model field, if allowed by deployment policy)

**Context and Cancellation**

- **FR-039**: The engine MUST propagate the request context to all provider calls. When the context is cancelled (client disconnect, explicit cancellation), the provider MUST stop processing and return promptly.
- **FR-040**: When context cancellation occurs during streaming, the engine MUST emit a `response.cancelled` event (if events have already been sent) or return a cancellation error (if no events have been sent yet)

### Key Entities

- **Provider**: An abstraction over an LLM inference backend. Protocol-agnostic. Declares its capabilities, accepts provider-level requests, returns provider-level responses or streams of events. Each adapter implementation handles a specific backend protocol (Chat Completions, Responses API, etc).
- **ProviderCapabilities**: A declaration of what features a specific provider instance supports: streaming, tool calling, vision, audio, reasoning, context window size, supported models, and extensions. Used by the engine for early request validation.
- **ProviderRequest**: The backend-facing request type. Contains model, messages, tools, tool choice, inference parameters, and an extension map for provider-specific options. Stripped of transport and storage concerns.
- **ProviderResponse**: The backend's complete non-streaming response. Contains output Items (already translated to OpenResponses types), usage statistics, and the actual model used.
- **ProviderEvent**: A single streaming event from the backend. Typed (text delta, tool call delta, reasoning delta, completion, error). Carries incremental data, completed Items, usage, or error information.
- **ProviderMessage**: A message in the provider's conversation format. Has a role (system, user, assistant, tool), content (string or multimodal parts), optional tool calls, and optional tool_call_id.
- **Engine**: The orchestration component. Implements ResponseCreator. Translates requests, invokes the provider, maps events, manages lifecycle, and writes results. Accepts optional capabilities (store, future tools) via nil-safe composition.
- **Translator**: Request translation logic that converts CreateResponseRequest to ProviderRequest. Handles Item-to-message mapping, multimodal content encoding, tool format conversion, and inference parameter mapping. Implemented per provider adapter.

## Success Criteria

### Measurable Outcomes

- **SC-001**: A complete non-streaming request-response cycle (transport -> engine -> provider -> backend mock -> provider -> engine -> transport) produces a valid OpenResponses Response with correctly structured output Items, usage statistics, and appropriate status
- **SC-002**: A complete streaming request-response cycle produces the full OpenResponses event sequence in correct order: `response.created` through `response.completed`, with all intermediate lifecycle and delta events present and correctly typed
- **SC-003**: Every input Item type (user message, assistant message, system message, function_call, function_call_output, reasoning) translates to the correct Chat Completions message format, verified by inspecting the outbound request to the mock backend
- **SC-004**: Incremental tool call arguments spread across multiple SSE chunks are correctly buffered and reassembled into complete function_call output Items with valid JSON arguments
- **SC-005**: Capability validation rejects requests requiring unsupported features (vision, tools, streaming, audio) with specific, actionable error messages before any backend call is made
- **SC-006**: Conversation history reconstruction from a chain of 3+ stored responses produces the correct chronological message sequence with only the most recent instructions as the system message
- **SC-007**: The provider interface supports at least two conceptually different adapter implementations (vLLM Chat Completions adapter and a test/mock adapter) without interface changes, confirming protocol-agnostic design
- **SC-008**: Context cancellation during streaming stops provider communication and produces either a `response.cancelled` event or a cancellation error within 1 second, consistent with Spec 002's cancellation guarantee
- **SC-009**: Backend errors (HTTP 4xx/5xx, network failures, malformed responses) are mapped to the correct OpenResponses error types and surfaced to the client with descriptive messages

## Assumptions

- The vLLM adapter targets the OpenAI-compatible Chat Completions API (`/v1/chat/completions`). Any server that implements this API (vLLM, Ollama, LiteLLM, TGI) works with this adapter.
- The Chat Completions SSE format follows the OpenAI specification: each chunk is `data: {json}\n\n` with a `data: [DONE]\n\n` sentinel at the end.
- The vLLM adapter is the only provider implementation in this spec. A Responses API proxy adapter (forwarding to `/v1/responses` backends directly) is a future addition enabled by the protocol-agnostic interface but not implemented here.
- Server-side tool execution (the agentic loop where the engine calls tools and feeds results back to the model) is deferred to Spec 004 (Tool System). This spec handles only the translation of tool call results from the backend, enabling client-side tool execution.
- The ResponseStore interface from Spec 002 is used for conversation history reconstruction. A real store implementation is provided by Spec 005 (Storage). Until then, the reconstruction logic is testable with mock stores.
- The engine uses zero external dependencies, consistent with Specs 001 and 002. The vLLM adapter uses only `net/http` and `encoding/json` from the standard library.
- Model validation (checking that the requested model is served by the provider) uses `ListModels()` but does not cache the model list. Caching is a future optimization.
- Reasoning token detection from models like DeepSeek R1 depends on provider-specific response fields (`reasoning_content`). The adapter checks for this field but does not fail if it is absent.

## Dependencies

- **Spec 001 (Core Protocol & Data Model)**: All Item, Response, StreamEvent, Error, and Usage types are defined in `pkg/api`. The engine and provider depend on these types. **Requires amendment**: The `Response` type needs an `Input []Item` field added (backwards-compatible) to support conversation reconstruction from stored response chains.
- **Spec 002 (Transport Layer)**: The `ResponseCreator`, `ResponseStore`, and `ResponseWriter` interfaces are defined in `pkg/transport`. The engine implements ResponseCreator and optionally consumes ResponseStore.

## Scope Boundaries

### In Scope

- Provider interface definition with capability negotiation (protocol-agnostic)
- Provider-level request, response, and event types
- Core engine implementing ResponseCreator
- Engine-to-provider request translation with explicit per-Item-type rules
- Provider-to-engine response and event translation
- Streaming event mapping (Chat Completions SSE to OpenResponses events)
- Synthetic lifecycle event generation by the engine
- Tool call argument buffering and reassembly
- Conversation history reconstruction logic (against ResponseStore interface)
- Capability-based request validation
- vLLM adapter for Chat Completions protocol
- Error mapping from backend HTTP errors to OpenResponses error types
- Context propagation and cancellation handling
- Nil-safe composition for optional engine capabilities

### Out of Scope

- Server-side tool execution and agentic loop (Spec 004)
- Response persistence / storage implementation (Spec 005)
- Authentication and authorization (Spec 006)
- GET /v1/models HTTP endpoint (deferred)
- Responses API proxy adapter (future, enabled by protocol-agnostic interface)
- LiteLLM adapter (Spec 008)
- Model list caching or refresh strategies
- Retry backoff strategy specification (config only, behavior is implementation-defined)
- gRPC transport support
- Metrics, tracing, or observability instrumentation (Spec 007)
