# Feature Specification: Terminal Streaming Events

**Feature Branch**: `022-terminal-events`
**Created**: 2026-02-24
**Status**: Draft

## Overview

The gateway currently uses `response.completed` for all successful stream endings and `response.failed` for all errors. The upstream OpenResponses spec distinguishes between completed (normal finish), incomplete (max tokens reached), and failed (actual error). It also defines a stream-level `error` event and refusal events for content policy rejections. This specification adds these missing terminal event types for more precise client-side error handling.

## User Scenarios & Testing

### User Story 1 - Incomplete Response Detection (Priority: P1)

A developer sends a streaming request with a low `max_output_tokens` limit. The model hits the token limit before finishing its answer. The client receives a `response.incomplete` terminal event (not `response.completed`) so it knows the output was truncated and can prompt the user to continue or increase the limit.

**Why this priority**: Without this distinction, clients can't tell whether the model finished naturally or was cut off. This is the most common terminal state confusion.

**Independent Test**: Send a streaming request with a small token limit, verify the terminal event is `response.incomplete` instead of `response.completed`.

**Acceptance Scenarios**:

1. **Given** a streaming request with a low max output tokens limit, **When** the model hits the limit, **Then** the terminal event is `response.incomplete` with `incomplete_details.reason` set to "max_output_tokens"
2. **Given** a streaming request with no token limit, **When** the model finishes naturally, **Then** the terminal event remains `response.completed` (backward compatible)
3. **Given** a non-streaming request with a low token limit, **When** the model hits the limit, **Then** the response status is "incomplete" with incomplete details

---

### User Story 2 - Stream-Level Error Event (Priority: P1)

A developer's streaming connection encounters an error (provider timeout, backend crash, malformed response). The client receives an `error` event that contains error details without a full response wrapper, allowing the client to handle the error before a response object was fully constructed.

**Why this priority**: The current `response.failed` event requires a response object wrapper, which may not be available if the error occurs before the response is created (e.g., during provider connection).

**Independent Test**: Trigger a stream error, verify an `error` event is received with error details.

**Acceptance Scenarios**:

1. **Given** a streaming request, **When** the backend produces an error during streaming, **Then** the client receives an `error` event with type, message, and code fields
2. **Given** a streaming request that errors before the first event, **When** the connection fails, **Then** the client receives an `error` event (not a `response.failed`)
3. **Given** a streaming request that succeeds, **When** the stream completes normally, **Then** no `error` events are emitted

---

### User Story 3 - Refusal Content Streaming (Priority: P3)

A developer sends a request that triggers a content policy refusal. Instead of a generic error, the client receives `response.refusal.delta` and `response.refusal.done` events containing the refusal explanation, allowing the client to display why the model declined.

**Why this priority**: Open-source models served via vLLM do not currently populate the refusal field. This is future-proofing for commercial backends and models with built-in content moderation.

**Independent Test**: Send a request that triggers refusal, verify refusal delta/done events are received.

**Acceptance Scenarios**:

1. **Given** a request that triggers a content policy refusal, **When** the backend returns refusal content, **Then** the client receives `response.refusal.delta` events followed by `response.refusal.done`
2. **Given** a normal request, **When** no refusal occurs, **Then** no refusal events are emitted

---

### Edge Cases

- What happens when the model produces some output before hitting max tokens? The partial output is included in the response, and the terminal event is `response.incomplete`.
- What happens when an error occurs after some events have been emitted? The `response.failed` event is sent as normal (error context is within a response). The standalone `error` event is only for errors before a response is created.
- What happens when refusal content is empty? No refusal events are emitted; the response completes normally.
- What happens in the non-streaming path when the model hits max tokens? The response status is "incomplete" with `incomplete_details` populated.

## Requirements

### Functional Requirements

**Incomplete Detection (P1)**

- **FR-001**: When the inference provider signals that output was truncated (max tokens reached), the streaming terminal event MUST be `response.incomplete` instead of `response.completed`
- **FR-002**: The `response.incomplete` event MUST include `incomplete_details` with a `reason` field indicating why the response is incomplete
- **FR-003**: Non-streaming responses MUST set status to "incomplete" with `incomplete_details` when the provider signals truncation

**Error Event (P1)**

- **FR-004**: The system MUST emit an `error` SSE event for stream-level errors that occur before a response object is constructed
- **FR-005**: The `error` event MUST include `type`, `message`, and optional `code` fields matching the standard error format

**Refusal Events (P3)**

- **FR-006**: When the provider returns refusal content, the system MUST emit `response.refusal.delta` events with incremental refusal text
- **FR-007**: When refusal streaming completes, the system MUST emit a `response.refusal.done` event

**Spec Alignment**

- **FR-008**: The OpenAPI specification MUST be updated to include the new event types
- **FR-009**: The SSE event count MUST increase from 17 to at least 21

**Backward Compatibility**

- **FR-010**: Normal stream completions MUST continue to use `response.completed`
- **FR-011**: Existing error handling via `response.failed` MUST remain functional for errors that occur within a response context

## Success Criteria

- **SC-001**: Streaming responses that hit max tokens produce `response.incomplete` instead of `response.completed`
- **SC-002**: All existing tests continue to pass with zero regressions
- **SC-003**: The SSE event count increases from 17 to at least 19 (incomplete + error, with refusal as bonus)
- **SC-004**: Clients can distinguish between "model finished" and "model was cut off" from the terminal event type alone

## Assumptions

- The provider signals truncation via `finish_reason: "length"` in Chat Completions. This is the standard convention across vLLM, LiteLLM, and OpenAI.
- The `error` event uses the same error object shape as `response.failed` but without the response wrapper.
- Refusal content comes from the `refusal` field in the Chat Completions message. Currently only OpenAI proprietary models populate this field; vLLM returns `null`.
- The `response.incomplete` event carries the full response object (same as `response.completed`), but with status "incomplete" and `incomplete_details` populated.

## Dependencies

- **Spec 003 (Core Engine)**: Terminal event emission
- **Spec 021 (Reasoning Streaming)**: Event type infrastructure

## Scope Boundaries

### In Scope

- Adding `response.incomplete` terminal event for max-tokens truncation
- Adding `error` stream event for pre-response errors
- Adding `response.refusal.delta` and `response.refusal.done` events
- Populating `incomplete_details` on truncated responses
- Updating the OpenAPI spec with new event types
- Integration tests for incomplete and error scenarios

### Out of Scope

- Content moderation or safety filtering (Antwort doesn't filter; it forwards provider decisions)
- Custom incomplete reasons beyond max tokens
- Retry logic for incomplete responses
- Rate limiting error events (handled by auth middleware, not streaming events)
