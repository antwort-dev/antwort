# Feature Specification: Structured Output (text.format Passthrough)

**Feature Branch**: `029-structured-output`
**Created**: 2026-02-25
**Status**: Draft

## Overview

The gateway accepts `text.format` in requests and echoes it in responses, but never forwards the format constraint to the inference backend. This means constrained decoding (JSON mode, JSON schema mode) has no effect. The model ignores the requested format and returns free-form text.

Structured output is widely used across the ecosystem. The OpenAI SDK's `client.responses.parse()`, LangChain's `response_format`, CrewAI's structured output, Haystack's `text_format`, and Codex CLI's `--response-schema` all depend on the format constraint reaching the model. Without this, Antwort cannot serve any framework that requires guaranteed JSON output.

This specification completes the passthrough pipeline so that `text.format` flows from the Responses API request through to the Chat Completions `response_format` parameter, enabling constrained decoding at the model level.

## User Scenarios & Testing

### User Story 1 - JSON Object Mode (Priority: P1)

A developer wants the model to always return valid JSON. They set `text.format: {"type": "json_object"}` on the request. The model's output is guaranteed to be parseable JSON, eliminating fragile regex or string extraction from free-form text.

**Why this priority**: JSON mode is the simplest constrained decoding mode and the foundation for all structured output. Most frameworks try this first.

**Independent Test**: Send a request with `text.format: {"type": "json_object"}`, verify the backend receives `response_format: {"type": "json_object"}`, and the response contains valid JSON output.

**Acceptance Scenarios**:

1. **Given** a request with `text.format: {"type": "json_object"}`, **When** the request is processed, **Then** the inference backend receives `response_format: {"type": "json_object"}` and the response output contains valid JSON
2. **Given** a request with `text.format: {"type": "text"}` (the default), **When** the request is processed, **Then** no `response_format` parameter is sent to the backend
3. **Given** a request with no `text.format` field, **When** the request is processed, **Then** no `response_format` parameter is sent to the backend (same as default)

---

### User Story 2 - JSON Schema Mode (Priority: P1)

A developer wants the model to output JSON conforming to a specific schema. They provide `text.format: {"type": "json_schema", "json_schema": {"name": "...", "schema": {...}}}`. The model's output is guaranteed to match the schema structure, enabling direct deserialization into typed objects.

**Why this priority**: Schema mode is the most powerful constrained decoding feature. It enables `client.responses.parse()` in the OpenAI SDK and typed output in LangChain, Pydantic AI, and CrewAI. Equal priority with JSON mode because both are needed for ecosystem compatibility.

**Independent Test**: Send a request with a json_schema format, verify the backend receives the schema in `response_format`, and the response output matches the schema.

**Acceptance Scenarios**:

1. **Given** a request with `text.format: {"type": "json_schema", "json_schema": {"name": "person", "schema": {"type": "object", "properties": {"name": {"type": "string"}}}}}`, **When** the request is processed, **Then** the inference backend receives the complete schema in `response_format`
2. **Given** a json_schema format with `strict: true`, **When** the request is processed, **Then** the strict flag is forwarded to the backend unchanged
3. **Given** a json_schema format with a complex nested schema, **When** the request is processed, **Then** the schema flows through without modification or re-interpretation

---

### User Story 3 - SDK parse() Compatibility (Priority: P2)

A developer uses the OpenAI Python SDK's `client.responses.parse()` method to get typed Python objects from model responses. The SDK sends a json_schema format, receives the response, and parses the output text against the schema client-side. The gateway must not interfere with this flow.

**Why this priority**: parse() is the primary way Python developers consume structured output. It works entirely client-side once the format constraint reaches the model, so no server-side changes are needed beyond the passthrough. Testing validates end-to-end compatibility.

**Independent Test**: Use the OpenAI Python SDK to call `client.responses.parse()` against the gateway, verify a typed result is returned.

**Acceptance Scenarios**:

1. **Given** a client using `client.responses.parse()` with a Pydantic model, **When** the response is received, **Then** the SDK successfully parses the output into the typed model
2. **Given** the response includes `text.format` echoed in the response body, **When** the SDK reads the response, **Then** the echoed format matches what was sent in the request

---

### Edge Cases

- What happens when the backend doesn't support `response_format`? The backend returns its own error. The gateway forwards the error to the client unchanged.
- What happens with an invalid JSON schema? The backend validates the schema and returns an error. The gateway does not validate schemas.
- What happens when `text.format.type` is an unknown value? The gateway forwards it as-is. The backend decides whether to accept or reject it.
- What happens during streaming with structured output? The format constraint applies to the complete output. Streaming tokens still arrive incrementally. No special handling needed.

## Requirements

### Functional Requirements

**Format Forwarding**

- **FR-001**: The system MUST forward `text.format` from the Responses API request to the Chat Completions `response_format` parameter on the inference backend
- **FR-002**: When `text.format.type` is `"json_object"`, the system MUST send `response_format: {"type": "json_object"}` to the backend
- **FR-003**: When `text.format.type` is `"json_schema"`, the system MUST send the complete format object (including `json_schema.name`, `json_schema.schema`, and `json_schema.strict`) as `response_format` to the backend
- **FR-004**: When `text.format.type` is `"text"` or `text.format` is absent, the system MUST NOT send a `response_format` parameter to the backend
- **FR-005**: The system MUST NOT validate, modify, or re-interpret the `text.format` payload. It is forwarded as-is to the backend (passthrough semantics)

**Schema Payload**

- **FR-006**: The `text.format` object MUST support the following fields: `type` (required), `name` (for json_schema), `strict` (for json_schema), and `schema` (for json_schema)
- **FR-007**: The `schema` field MUST carry the JSON Schema object through the pipeline without parsing, rewriting, or validating its structure

**Response Echoing**

- **FR-008**: The response MUST continue to echo `text.format` as received in the request (existing behavior, must not regress)

**Streaming Compatibility**

- **FR-009**: Structured output MUST work identically in both streaming and non-streaming modes. The format constraint is applied by the backend, not the gateway

## Success Criteria

- **SC-001**: A request with `text.format: {"type": "json_object"}` results in the inference backend receiving `response_format: {"type": "json_object"}`
- **SC-002**: A request with a json_schema format results in the complete schema reaching the inference backend unchanged
- **SC-003**: The OpenAI Python SDK's `client.responses.parse()` returns a correctly typed object when used against the gateway
- **SC-004**: All existing tests continue to pass with zero regressions

## Assumptions

- The gateway does not validate `text.format` values. Validation is the backend's responsibility. This is consistent with the existing passthrough philosophy for all inference parameters (temperature, top_p, etc.).
- The `schema` field contains a standard JSON Schema object. The gateway treats it as opaque bytes.
- Backend providers (vLLM, LiteLLM) already support `response_format` in the Chat Completions API. No provider-side changes are needed.
- The SDK parse() test (SC-003) may require a mock backend that produces schema-conforming output. This is a test infrastructure concern, not a gateway change.

## Dependencies

- **Spec 003 (Core Engine)**: Engine translation pipeline (translateRequest)
- **Spec 008 (Provider LiteLLM)**: Shared openaicompat translation layer

## Scope Boundaries

### In Scope

- Forwarding `text.format` to Chat Completions `response_format`
- Supporting `text`, `json_object`, and `json_schema` format types
- Carrying `name`, `strict`, and `schema` fields for json_schema mode
- Updating the OpenAPI spec to reflect the full TextFormat schema
- Integration tests for format passthrough
- SDK compatibility validation

### Out of Scope

- Schema validation or compilation at the gateway level
- Logprobs support (separate spec)
- Refusal handling for structured output (the `refusal` field when content policy rejects output)
- Server-side JSON parsing or transformation of model output
- Supporting format types beyond what the Chat Completions API accepts
