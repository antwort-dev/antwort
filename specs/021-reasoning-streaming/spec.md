# Feature Specification: Reasoning Streaming Events

**Feature Branch**: `021-reasoning-streaming`
**Created**: 2026-02-24
**Status**: Draft

## Overview

Reasoning-capable models (DeepSeek R1, Qwen QwQ, OpenAI o-series) produce chain-of-thought tokens before generating the final answer. The gateway already handles reasoning content in non-streaming responses, but during streaming it silently drops reasoning tokens. This specification adds the OpenResponses reasoning SSE events so clients can display the model's thinking process in real-time.

## User Scenarios & Testing

### User Story 1 - Streaming Reasoning Visibility (Priority: P1)

A developer sends a streaming request to a reasoning-capable model. As the model thinks through the problem, the client receives reasoning token deltas in real-time, followed by the final answer text. The client can display a "thinking..." section that updates progressively, then show the answer when it arrives.

**Why this priority**: Reasoning models are increasingly common. Without streaming reasoning events, clients see nothing until the final answer starts, which for complex reasoning can mean many seconds of silence. This creates a poor user experience that undermines the value of streaming.

**Independent Test**: Send a streaming request to a reasoning model, verify reasoning delta events are received before text delta events.

**Acceptance Scenarios**:

1. **Given** a streaming request to a reasoning model, **When** the model produces reasoning tokens, **Then** the client receives `response.reasoning.delta` events with incremental reasoning content
2. **Given** a streaming response with reasoning, **When** the reasoning phase completes, **Then** the client receives a `response.reasoning.done` event before text generation begins
3. **Given** a streaming response with reasoning, **When** the response completes, **Then** the reasoning content appears as a reasoning item in the response output alongside the text output
4. **Given** a streaming request to a model that does not produce reasoning, **When** the response streams, **Then** no reasoning events are emitted (backward compatible)

---

### User Story 2 - Reasoning in Non-Streaming Responses (Priority: P1)

A developer sends a non-streaming request to a reasoning model. The response includes reasoning items showing the model's chain-of-thought alongside the final answer.

**Why this priority**: Non-streaming reasoning already partially works (the item is created), but the reasoning output item should be properly ordered in the response output (reasoning before text).

**Independent Test**: Send a non-streaming request to a reasoning model, verify reasoning item appears in the output.

**Acceptance Scenarios**:

1. **Given** a non-streaming request to a reasoning model, **When** the response is returned, **Then** the output includes a reasoning item with the chain-of-thought content
2. **Given** a non-streaming request to a non-reasoning model, **When** the response is returned, **Then** no reasoning items appear in the output

---

### User Story 3 - Reasoning in Agentic Loops (Priority: P2)

A developer runs an agentic request with tools against a reasoning model. Each turn of the agentic loop may produce reasoning tokens before deciding which tool to call. The reasoning events are emitted during each turn.

**Why this priority**: Agentic use cases with reasoning models benefit from seeing the model's decision process at each turn, but this is a secondary use case that builds on the core streaming support.

**Independent Test**: Send a streaming agentic request with tools to a reasoning model, verify reasoning events are emitted on each turn.

**Acceptance Scenarios**:

1. **Given** a streaming agentic request with tools, **When** the model reasons before making a tool call, **Then** reasoning delta events are emitted before the function call arguments
2. **Given** a multi-turn agentic loop, **When** the model reasons on the second turn, **Then** reasoning events are emitted with correct sequence numbers continuing from the previous turn

---

### Edge Cases

- What happens when reasoning content is empty? No reasoning events are emitted (same as non-reasoning models).
- What happens when reasoning and text content arrive in the same streaming chunk? Both reasoning delta and text delta events are emitted from the same chunk.
- What happens when the model produces reasoning but no final answer (e.g., max tokens reached during reasoning)? The reasoning done event is emitted and the response completes with status "incomplete".
- What happens when reasoning content is very long? No truncation; all reasoning tokens are streamed as deltas.

## Requirements

### Functional Requirements

**Streaming Events**

- **FR-001**: The system MUST emit `response.reasoning.delta` events containing incremental reasoning tokens during streaming
- **FR-002**: The system MUST emit `response.reasoning.done` events when the reasoning phase of a streaming response completes
- **FR-003**: Reasoning events MUST include the correct `item_id`, `output_index`, and `content_index` fields matching the reasoning output item
- **FR-004**: Reasoning events MUST have monotonically increasing `sequence_number` values consistent with other event types in the same stream

**Output Items**

- **FR-005**: Reasoning content MUST appear as a reasoning-type output item in both streaming and non-streaming responses
- **FR-006**: When both reasoning and text content are produced, the reasoning item MUST appear before the text message item in the output array

**Backward Compatibility**

- **FR-007**: Models that do not produce reasoning content MUST NOT trigger any reasoning events or items (zero behavioral change for non-reasoning models)
- **FR-008**: The existing text streaming event sequence (response.created, output_item.added, content_part.added, text deltas, text done, content_part.done, output_item.done, response.completed) MUST remain unchanged

**Spec Alignment**

- **FR-009**: The OpenAPI specification MUST be updated to include the new reasoning event types
- **FR-010**: The SSE event count MUST increase from 15 to at least 17 (adding reasoning.delta and reasoning.done)

## Success Criteria

- **SC-001**: Streaming requests to reasoning models produce visible reasoning token deltas before the final answer begins
- **SC-002**: All existing streaming and non-streaming tests continue to pass with zero regressions
- **SC-003**: The conformance test suite score remains at or above the current level
- **SC-004**: A reasoning model (e.g., DeepSeek R1 via vLLM) produces the expected event sequence: reasoning deltas, reasoning done, then text deltas, then text done

## Assumptions

- Reasoning-capable backends expose reasoning tokens via the `reasoning_content` field in Chat Completions streaming chunks (this is the convention used by DeepSeek R1 and supported by vLLM).
- Reasoning summaries (the `response.reasoning_summary.*` events from the upstream spec) are deferred. No open-source model currently produces separate reasoning summaries. This spec focuses on the raw reasoning content events.
- The `reasoning` configuration field on the request (added in Spec 020) controls whether reasoning is enabled. When reasoning is not requested, the backend doesn't produce reasoning tokens and no reasoning events are emitted.
- Reasoning items use the same `item_` ID format as other output items.

## Dependencies

- **Spec 003 (Core Engine)**: Streaming event emission infrastructure
- **Spec 020 (API Compliance)**: Reasoning configuration field on the request

## Scope Boundaries

### In Scope

- Mapping provider reasoning events to OpenResponses SSE event types
- Emitting reasoning output items in both streaming and non-streaming responses
- Updating the OpenAPI spec with new event types
- Integration tests with a mock backend that produces reasoning content

### Out of Scope

- Reasoning summary events (`response.reasoning_summary.*`), deferred until models produce summaries
- Reasoning effort configuration (already handled as passthrough in Spec 020)
- Encrypted reasoning content (provider-specific, not applicable to open-source models)
- Reasoning token counting in usage breakdowns (already handled via `output_tokens_details.reasoning_tokens`)
