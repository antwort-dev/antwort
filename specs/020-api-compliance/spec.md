# Feature Specification: OpenResponses API Compliance

**Feature Branch**: `020-api-compliance`
**Created**: 2026-02-23
**Status**: Draft

## Overview

Antwort currently implements a subset of the OpenResponses request and response schema. Several standard request fields are silently ignored because they are not declared in the request type, and some response fields that clients expect are missing. This specification closes the gap by adding the missing passthrough fields so that clients built against the upstream OpenResponses SDK work without modification.

The work is organized in three tiers: quick-win passthrough fields that just need echoing, medium-effort fields that require some behavioral logic, and larger features that need architectural discussion.

## Clarifications

### Session 2026-02-23

- Q: Should fields we don't actively use still be accepted? A: Yes. Accepting and echoing them is essential for SDK compatibility. Clients send these fields; silently dropping them causes confusion.
- Q: Should we validate enum values for passthrough fields? A: No. Passthrough fields are forwarded to the provider as-is. The provider validates them.
- Q: What about `background` and `conversation`? A: Out of scope for this spec. They require significant new infrastructure (async jobs, conversation storage).

## User Scenarios & Testing

### User Story 1 - Request Field Passthrough (Priority: P1)

A developer sends an OpenResponses-compliant request that includes fields like `metadata`, `user`, `reasoning`, or `top_logprobs`. The gateway accepts all upstream-defined fields, passes relevant ones through to the inference provider, and echoes them back in the response.

**Why this priority**: SDK clients send these fields by default. Silently dropping them breaks round-trip consistency and causes debugging confusion.

**Independent Test**: Send a request with all supported passthrough fields populated, verify each appears in the response.

**Acceptance Scenarios**:

1. **Given** a request with `metadata`, **When** the response is returned, **Then** the `metadata` field in the response matches the request value
2. **Given** a request with `user`, **When** the response is returned, **Then** the `user` field appears in the response
3. **Given** a request with `frequency_penalty`, `presence_penalty`, `top_logprobs`, **When** sent to the provider, **Then** the values are included in the provider request
4. **Given** a request with `reasoning`, `text`, **When** sent to the provider, **Then** the values are forwarded to the provider
5. **Given** a request with `parallel_tool_calls: false`, **When** a single inference response contains multiple tool calls, **Then** the engine executes them one at a time instead of concurrently
6. **Given** a request with `max_tool_calls`, **When** the agentic loop runs, **Then** the loop terminates after the specified number of tool call rounds

---

### User Story 2 - Response Verbosity Control (Priority: P2)

A developer includes the `include` field in a request to control which optional sections appear in the response (e.g., excluding usage data or reasoning content for bandwidth savings).

**Why this priority**: Reduces response payload size for clients that don't need all fields, improving perceived latency.

**Independent Test**: Send a request with `include` filters, verify excluded sections are omitted from the response.

**Acceptance Scenarios**:

1. **Given** a request with `include` omitting usage, **When** the response is returned, **Then** the `usage` field is absent
2. **Given** a request without `include`, **When** the response is returned, **Then** all fields are included (backward compatible)

---

### User Story 3 - Stream Configuration (Priority: P2)

A developer sends `stream_options` to configure streaming behavior, such as requesting usage statistics in streaming responses.

**Why this priority**: Some clients need token usage during streaming to track costs in real-time.

**Independent Test**: Send a streaming request with `stream_options` requesting usage, verify usage appears in the final streaming event.

**Acceptance Scenarios**:

1. **Given** a streaming request with `stream_options.include_usage: true`, **When** the stream completes, **Then** usage data is included in the response.completed event
2. **Given** a streaming request without `stream_options`, **When** the stream completes, **Then** behavior is unchanged from current implementation

---

### Edge Cases

- What happens when a client sends an unknown field not in the spec? Unknown fields are silently ignored (standard JSON unmarshaling behavior).
- What happens when `parallel_tool_calls` is false but no tools are used? The field is echoed in the response but has no effect.
- What happens when `max_tool_calls` is set to 0? Treated as "no limit" (same as omitting it).
- What happens when `include` requests a filter that doesn't exist? Unknown filter values are ignored.

## Requirements

### Functional Requirements

**P1: Quick-Win Passthrough Fields**

- **FR-001**: The request type MUST accept the `metadata` field (key-value map) and echo it in the response
- **FR-002**: The request type MUST accept the `user` field (string identifier) and echo it in the response
- **FR-003**: The request type MUST accept `frequency_penalty` (number) and forward it to the inference provider
- **FR-004**: The request type MUST accept `presence_penalty` (number) and forward it to the inference provider
- **FR-005**: The request type MUST accept `top_logprobs` (integer) and forward it to the inference provider
- **FR-006**: The request type MUST accept the `reasoning` configuration and forward it to the inference provider
- **FR-007**: The request type MUST accept the `text` configuration (output format) and forward it to the inference provider
- **FR-008**: The request type MUST accept `parallel_tool_calls` (boolean, default true). When false, tool calls within a single inference response MUST be executed one at a time instead of concurrently
- **FR-009**: The request type MUST accept `max_tool_calls` (integer) and enforce the limit in the agentic loop

**P2: Medium-Effort Fields**

- **FR-010**: The request type MUST accept the `include` field (array of strings) that controls which optional response sections are returned. Valid values follow the upstream OpenResponses enum (e.g., `usage`, `reasoning`, `file_search_call.results`). When `include` is omitted, all sections are returned
- **FR-011**: The request type MUST accept `stream_options` and honor the `include_usage` setting during streaming

**Spec Alignment**

- **FR-012**: The OpenAPI specification (`api/openapi.yaml`) MUST be updated to reflect all newly supported fields
- **FR-013**: The oasdiff comparison MUST show fewer divergences after implementation (reduced warnings count)

## Success Criteria

- **SC-001**: Requests containing all P1 fields are accepted without error and the fields appear in the response
- **SC-002**: oasdiff reports zero "request-property-removed" warnings for fields that are now accepted in the request type
- **SC-003**: Existing integration tests continue to pass with zero regressions
- **SC-004**: The conformance test suite score remains at or above the current level (5/6 core tests passing)
- **SC-005**: A client using the official OpenResponses SDK can send a fully-populated request and receive all fields back

## Assumptions

- Passthrough fields that the provider doesn't support are silently included in the request but may be ignored by the provider. This is acceptable behavior.
- The `parallel_tool_calls` field defaults to `true` when omitted, matching current behavior.
- The `max_tool_calls` field uses the existing engine max-turns config as the upper bound. The request-level value cannot exceed the server-side maximum.
- The `include` field filtering happens at the transport layer (response serialization), not in the engine.
- `background` and `conversation` are documented as P3 for future consideration, not committed for this spec.

## Dependencies

- **Spec 002 (Transport Layer)**: Request/response type definitions
- **Spec 003 (Core Engine)**: Engine loop, provider request translation
- **Spec 019 (API Conformance)**: OpenAPI spec and integration tests

## Future Considerations

The following upstream features require significant new infrastructure and are candidates for dedicated specs:

- **Background Execution** (`background: true`): Asynchronous request processing with immediate acknowledgment and polling. Needs a job queue or goroutine pool with response persistence.
- **Conversation Threading** (`conversation`): Server-side grouping of responses into persistent conversation threads. Needs a conversation storage layer beyond the existing `previous_response_id` chaining.

## Scope Boundaries

### In Scope

- Adding missing request fields to the request type
- Forwarding provider-relevant fields through the translation layer
- Echoing passthrough fields in the response
- Implementing `parallel_tool_calls` sequential mode
- Implementing `max_tool_calls` limit
- Implementing `include` response filtering
- Implementing `stream_options` usage inclusion
- Updating the OpenAPI spec
- Updating integration tests

### Out of Scope

- Background execution infrastructure (P3, future spec)
- Conversation threading storage (P3, future spec)
- Auto-generation of OpenAPI spec from types
- Provider-side validation of forwarded fields
- Billing and cost tracking fields (`billing`, `cost_token`)
- Prompt caching (`prompt_cache_key`, `prompt_cache_retention`) beyond passthrough
- Safety identifier enforcement beyond passthrough
