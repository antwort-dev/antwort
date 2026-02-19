# Feature Specification: Core Protocol & Data Model

**Feature Branch**: `001-core-protocol`
**Created**: 2026-02-16
**Status**: Draft
**Input**: User description: "Core Protocol and Data Model for OpenResponses API, based on the OpenResponses specification (openresponses.org)"

## Overview

This specification defines the foundational data model and protocol types for Antwort, an OpenResponses-compliant API proxy. All other specs (transport, provider, tools, storage, auth, deployment) depend on these types.

The core protocol defines Items (the atomic unit of context), Content models (user input and model output), Request/Response schemas, State machines governing object lifecycles, Error types, and Extension mechanisms for provider-specific customization.

Antwort introduces a two-tier API classification (stateless vs stateful) that determines which features require persistence and which can operate in a zero-state mode suitable for embedding in proxies like Envoy.

## Clarifications

### Session 2026-02-16

- Q: FR-027 defines `response.queued` event but FR-024 didn't include `queued` as a valid response status. Should `queued` be added? → A: Yes, add `queued` as a valid response status with state machine transition `queued -> in_progress -> terminal`. Supports async/batch processing via `service_tier`.
- Q: What format should response and item IDs follow? → A: Prefixed format: `resp_` for responses, `item_` for items, followed by a random alphanumeric string. Matches OpenAI convention for drop-in client compatibility.
- Q: What triggers the `cancelled` response state? → A: Cancellation is transport-driven (client disconnect, transport timeout). The core protocol only defines `cancelled` as a valid terminal state; the trigger mechanism is a transport-layer concern (Spec 02).
- Q: What is the default value for `tool_choice` when omitted? → A: `auto` (model decides whether to invoke tools). Matches OpenResponses and OpenAI convention.
- Q: Should the spec define upper bounds on input size? → A: Yes, configurable limits. The spec requires enforcement (max input Items, max content size per part) but values are deployment-configurable, not hardcoded in the protocol.

### Evolution 2026-02-19 (Conformance Testing)

Running the official OpenResponses compliance suite revealed several schema requirements not captured in the original spec:

- **Response schema expansion**: The OpenResponses Zod schema requires 30+ fields on Response (not just the 12 originally specified). Added fields echo back request parameters: tools, tool_choice, truncation, temperature, top_p, presence_penalty, frequency_penalty, top_logprobs, parallel_tool_calls, text config, reasoning config, max_output_tokens, max_tool_calls, store, background, service_tier, metadata, safety_identifier, prompt_cache_key, completed_at, incomplete_details.
- **Item wire format**: The OpenResponses wire format uses a flat structure for Items (role and content at the item top level), not a nested wrapper. Internal representation remains nested for engine logic; custom JSON marshaling produces the flat wire format.
- **PreviousResponseID**: Changed from `string` to `*string` (nullable) to match the Zod schema that requires the field to be present with null value rather than omitted.
- **OutputContentPart**: `annotations` and `logprobs` arrays are required by the Zod schema (not optional). Empty arrays are serialized as `[]`, never as `null`.
- **ToolDefinition**: Added `strict` boolean field required by the Zod schema.
- **Usage details**: Added `input_tokens_details` (cached_tokens) and `output_tokens_details` (reasoning_tokens) sub-objects.
- **StreamEvent serialization**: Each event type requires specific fields at the top level. Custom MarshalJSON produces the correct fields per event type (output_index, item_id, content_index, logprobs as needed).
- **requires_action status**: Added as terminal response status (Spec 004 amendment). State machine: `in_progress -> requires_action`, no outgoing transitions.
- **incomplete status**: Added to response status enum for max-turns scenarios.

## User Scenarios & Testing

### User Story 1 - Submit a Prompt and Receive a Response (Priority: P1)

A developer sends a prompt to the Antwort API and receives a structured response containing the model's output as Items. The response follows the OpenResponses schema exactly, so any OpenAI Responses API-compatible client works without modification.

**Why this priority**: This is the fundamental interaction. Without correct request parsing and response serialization, nothing else works. Every other spec depends on these types being right.

**Independent Test**: Can be fully tested by constructing a request object with input Items, processing it, and verifying the response contains correctly structured output Items with valid state transitions. Delivers the core value of spec-compliant data exchange.

**Acceptance Scenarios**:

1. **Given** a request with a model name and one text input Item, **When** the request is validated, **Then** it is accepted as valid with all required fields present
2. **Given** a request with no model name, **When** the request is validated, **Then** it is rejected with an `invalid_request` error identifying the missing field
3. **Given** a request with input Items of mixed types (message, function_call output), **When** the request is validated, **Then** all item types are correctly parsed and their type-specific fields are populated

---

### User Story 2 - Receive Streaming Events with Correct State Transitions (Priority: P1)

A developer submits a streaming request and receives a sequence of events that follow the OpenResponses streaming protocol. Each event carries the correct type, and the response and its items transition through valid state machine states (in_progress to completed, failed, or cancelled).

**Why this priority**: Streaming is the primary consumption mode for LLM APIs. The event types and state machine rules defined here are used directly by the transport layer (Spec 02).

**Independent Test**: Can be tested by simulating a sequence of streaming events and validating that each event type is correctly structured, state transitions follow the allowed paths, and no mutations occur after terminal states.

**Acceptance Scenarios**:

1. **Given** a response in `in_progress` state, **When** the response completes, **Then** it transitions to `completed` and no further modifications are accepted
2. **Given** a response in `in_progress` state, **When** an error occurs, **Then** it transitions to `failed` with a structured error object containing type, code, and message
3. **Given** an Item in `completed` state, **When** an attempt is made to modify it, **Then** the modification is rejected because terminal states are immutable
4. **Given** a streaming response, **When** items are emitted, **Then** the event sequence follows the correct order: `output_item.added`, content deltas, `output_item.done`

---

### User Story 3 - Stateless vs Stateful Request Classification (Priority: P2)

A developer submits a request with `store: false` for a fire-and-forget inference call (stateless tier). A different developer submits a request with `store: true` (or omits the field, defaulting to `true`) for a conversation that should be persisted and chainable via `previous_response_id` (stateful tier).

**Why this priority**: The two-tier classification is an architectural decision unique to Antwort. It determines which features are available and whether persistence is required, directly affecting deployment flexibility (e.g., ext_proc mode is stateless only).

**Independent Test**: Can be tested by creating requests with different `store` values and verifying that stateless requests reject `previous_response_id`, while stateful requests allow it.

**Acceptance Scenarios**:

1. **Given** a request with `store: false` and a `previous_response_id`, **When** the request is validated, **Then** it is rejected because stateless requests cannot reference previous responses
2. **Given** a request with `store` omitted, **When** the request is validated, **Then** it defaults to stateful mode (`store: true`)
3. **Given** a stateless request, **When** the response is returned, **Then** it does not include a response ID suitable for chaining

---

### User Story 4 - Provider Extension Types (Priority: P3)

A provider-specific integration sends Items or request parameters with custom types prefixed by a provider slug (e.g., `"acme:telemetry_chunk"`). The core protocol accepts and preserves these extensions without understanding their contents, passing them through as opaque data.

**Why this priority**: Extension support enables provider-specific features (vLLM guided decoding, LiteLLM fallback configuration) without modifying the core protocol. This is important for production use but not required for basic operation.

**Independent Test**: Can be tested by constructing Items with provider-prefixed types and verifying they survive serialization round-trips with their extension data intact.

**Acceptance Scenarios**:

1. **Given** an Item with type `"acme:telemetry_chunk"` and opaque extension data, **When** it is serialized and deserialized, **Then** the type and extension data are preserved exactly
2. **Given** a request with extension fields in the extensions map, **When** it is validated, **Then** extension fields are accepted without schema validation (they are opaque to the core)
3. **Given** an Item with an unrecognized type that does not follow the `provider:type` pattern, **When** it is validated, **Then** it is rejected as an invalid type

---

### Edge Cases

- What happens when an Item type is a valid `provider:type` format but the provider slug is unknown? Accept it, since the core protocol does not maintain a provider registry.
- How does the system handle a request where `input` is an empty array? Reject with `invalid_request` error: at least one input item is required.
- What happens when `tool_choice` is set to a structured object referencing a tool not present in the `tools` array? Reject with `invalid_request` error identifying the missing tool.
- How does the system handle `max_output_tokens` set to zero? Reject with `invalid_request` error: must be a positive integer if provided.
- What happens when a response contains both output Items and an error? The error takes precedence; response status is `failed`, and any partial output Items have `incomplete` status.
- What triggers the `cancelled` response state? Cancellation is a transport-layer concern (client disconnect, transport timeout). The core protocol defines `cancelled` as a valid terminal state but does not define a cancel operation. When cancelled, any in-progress Items transition to `incomplete`.

## Requirements

### Functional Requirements

**Item System**

- **FR-001**: System MUST support four standard Item types: `message`, `function_call`, `function_call_output`, and `reasoning`
- **FR-002**: System MUST support provider-extension Item types using the `provider:type` naming convention (e.g., `"acme:custom_item"`)
- **FR-003**: Every Item MUST contain three required fields: a unique identifier (prefixed format: `item_` followed by a random alphanumeric string), a type discriminator, and a lifecycle status
- **FR-004**: Item status MUST be one of: `in_progress`, `incomplete`, `completed`, or `failed`
- **FR-005**: Items in terminal states (`completed`, `incomplete`, `failed`) MUST be immutable; no further modifications are allowed

**Content Models**

- **FR-006**: User content MUST support multimodal input: text, image, audio, and video
- **FR-007**: Model output content MUST support two content types: `output_text` (primary text output with token-level log probabilities) and `summary_text` (reasoning summaries safe for user display)
- **FR-007a**: Output content parts MUST include `annotations` (array, required, empty when no annotations) and `logprobs` (array, required, empty when no log probabilities) fields. These MUST serialize as `[]` not `null`.
- **FR-008**: User content and model output content MUST be asymmetric (different schemas), reflecting the difference between what users provide and what models produce

**Message Items**

- **FR-009**: Message Items MUST carry a role: `user`, `assistant`, or `system`
- **FR-010**: User and system messages MUST contain user content (multimodal input)
- **FR-011**: Assistant messages MUST contain model output content (text)

**Function Call Items**

- **FR-012**: Function call Items MUST contain a function name, a call identifier, and arguments (as a JSON-encoded string)
- **FR-012b**: Tool definitions MUST include a `strict` boolean field indicating whether the tool uses strict schema validation

**Function Call Output Items**

- **FR-012a**: Function call output Items MUST contain a call identifier (matching the originating function call) and the tool's output string, used by clients to return tool execution results in multi-turn tool use

**Reasoning Items**

- **FR-013**: Reasoning Items MUST support three optional fields: raw content (the thinking trace), encrypted content (provider-opaque protected reasoning), and a summary (safe for user display)
- **FR-014**: All three reasoning fields MUST be optional, since providers vary in what they expose

**Request Schema**

- **FR-015**: Requests MUST require a model identifier
- **FR-016**: Requests MUST require at least one input Item
- **FR-017**: Requests MUST support optional parameters: instructions, tools, tool_choice, allowed_tools, store, stream, previous_response_id, truncation, service_tier, max_output_tokens, temperature, and top_p
- **FR-018**: The `store` field MUST default to `true` when omitted
- **FR-019**: The `truncation` field MUST accept two values: `auto` (server may shorten context) and `disabled` (fail rather than truncate)
- **FR-020**: The `tool_choice` field MUST accept: `auto`, `required`, `none`, or a structured object specifying a tool by name. The default MUST be `auto` when omitted.
- **FR-021**: Requests MUST support an extensions map for provider-specific parameters that are opaque to the core protocol

**Response Schema**

- **FR-022**: Responses MUST contain all fields required by the OpenResponses Zod schema: id, object, created_at, completed_at (nullable), status, incomplete_details (nullable), model, previous_response_id (nullable), instructions (nullable), output (array), error (nullable), tools (array), tool_choice, truncation, parallel_tool_calls, text config, top_p, presence_penalty, frequency_penalty, top_logprobs, temperature, reasoning (nullable), usage (nullable), max_output_tokens (nullable), max_tool_calls (nullable), store, background, service_tier, metadata, safety_identifier (nullable), prompt_cache_key (nullable)
- **FR-022a**: Response MUST echo back request parameters (tools, tool_choice, truncation, temperature, top_p, store, etc.) so clients can verify what settings were used
- **FR-023**: Usage statistics MUST include input_tokens, output_tokens, total_tokens, input_tokens_details (with cached_tokens), and output_tokens_details (with reasoning_tokens)
- **FR-024**: Response status MUST be one of: `queued`, `in_progress`, `completed`, `incomplete`, `failed`, `cancelled`, or `requires_action`
- **FR-024a**: The response state machine transitions are: `queued` -> `in_progress` -> `completed` | `incomplete` | `failed` | `cancelled` | `requires_action`. A response MAY skip `queued` and start directly in `in_progress` for synchronous processing. `requires_action` is terminal (Spec 004).
- **FR-025**: Responses in terminal states (`completed`, `failed`, `cancelled`) MUST be immutable

**Streaming Event Types**

- **FR-026**: System MUST define delta events for incremental content delivery: item added, content part added/done, text delta/done, function call arguments delta/done
- **FR-027**: System MUST define state machine events for lifecycle transitions: `response.created`, `response.queued`, `response.in_progress`, `response.completed`, `response.failed`, `response.cancelled`
- **FR-028**: Each streaming event MUST carry a type identifier, a monotonically increasing sequence number for ordering, and the relevant payload. The payload fields vary by event type: lifecycle events carry a response snapshot, item events carry output_index and item, content part events carry item_id/output_index/content_index/part, text delta events carry item_id/output_index/content_index/delta/logprobs.
- **FR-028a**: Delta events MUST include context fields (item identifier, output index, and content index) to enable client-side correlation of incremental updates with the correct item and content part. Text delta and text done events MUST include a `logprobs` array (empty when no log probabilities are available).
- **FR-029**: Extension streaming events MUST be supported using the `provider:event_type` naming convention

**Wire Format**

- **FR-029a**: Items MUST serialize to a flat wire format where type-specific fields are at the top level (not nested in a wrapper). Message items serialize with `role` and `content` at the top level. Function call items serialize with `call_id`, `name`, and `arguments` at the top level. Internal representation MAY use nested wrappers for engine logic.
- **FR-029b**: Arrays that are part of the OpenResponses schema (tools, output, annotations, logprobs, content) MUST serialize as `[]` when empty, never as `null`
- **FR-029c**: Nullable fields (previous_response_id, completed_at, error, reasoning, etc.) MUST be present in the JSON with `null` value, not omitted

**Error System**

- **FR-030**: Errors MUST be structured objects with a type, an optional code, an optional parameter reference, and a human-readable message
- **FR-031**: Error types MUST include: `server_error`, `invalid_request`, `not_found`, `model_error`, and `too_many_requests`
- **FR-032**: Validation failures MUST produce `invalid_request` errors that identify the specific field or parameter that failed

**Stateless/Stateful Classification**

- **FR-033**: Requests with `store: false` MUST operate in stateless mode: no persistence, no response retrieval, no `previous_response_id` support
- **FR-034**: Requests with `store: true` (or omitted) MUST operate in stateful mode: responses are persisted and can be retrieved, deleted, or chained
- **FR-035**: Stateless requests that include a `previous_response_id` MUST be rejected with an `invalid_request` error

**Validation**

- **FR-036**: System MUST validate all required fields on requests before processing
- **FR-037**: System MUST validate that Item types are either standard types or follow the `provider:type` extension pattern
- **FR-038**: System MUST validate `tool_choice` against the provided tools list when a specific tool is forced
- **FR-039**: System MUST enforce state machine transitions, rejecting any invalid state change
- **FR-040**: System MUST enforce configurable limits on request size (maximum number of input Items, maximum content size per part). Limit values are deployment-configurable, not fixed in the protocol. Requests exceeding limits MUST be rejected with an `invalid_request` error.

### Key Entities

- **Item**: The atomic unit of context. Polymorphic, discriminated by type. Can represent a message, function call, reasoning trace, or provider extension. Has a lifecycle governed by a state machine.
- **Content (User)**: Multimodal input provided by the client. Supports text, images, audio, and video. Multiple modalities can appear in a single message.
- **Content (Model)**: Text output generated by the model. Narrower than user content by design. May include token-level log probabilities.
- **Request**: The input object for creating a response. Contains model selection, input items, and optional parameters for controlling inference behavior.
- **Response**: The output object containing the model's generated Items, usage statistics, and lifecycle status. Governed by a state machine.
- **Streaming Event**: A typed event representing either an incremental content delta or a state machine transition. Defined here, serialized by the transport layer (Spec 02).
- **Error**: A structured error object with a classified type, enabling consistent error handling across different transports and providers.
- **Extensions**: Opaque data containers (on Items, Requests, and Responses) that allow providers to attach custom data without modifying the core schema.

## Success Criteria

### Measurable Outcomes

- **SC-001**: All OpenResponses-compliant client libraries can construct valid requests and parse valid responses using Antwort's data model without modification
- **SC-002**: Every Item type (message, function_call, reasoning, and at least one provider extension type) survives a serialization round-trip (encode to JSON, decode back) with zero data loss
- **SC-003**: State machine enforcement catches 100% of invalid transitions (e.g., modifying a completed Item) and returns a structured error
- **SC-004**: Request validation identifies and reports all missing required fields, invalid field values, and constraint violations (like `previous_response_id` on stateless requests) with specific, actionable error messages
- **SC-005**: The streaming event type catalog covers the full OpenResponses event protocol, and each event type can be constructed, serialized, and deserialized correctly
- **SC-006**: Provider extension types (custom items, custom events, custom request/response fields) pass through the core protocol without data loss or schema modification

## Assumptions

- The OpenResponses specification (as published at openresponses.org) is the normative reference. Where the spec is ambiguous, we follow OpenAI's Responses API behavior as the de facto standard.
- The `function_call_output` input type (used by clients to submit tool results) is modeled as a standard input Item type, not a separate schema.
- Token log probabilities on model output content are optional and provider-dependent; the core protocol defines the field but does not require providers to populate it.
- The `service_tier` field is a hint, not a guarantee. Its enforcement is left to the auth/rate-limiting layer (Spec 06).
