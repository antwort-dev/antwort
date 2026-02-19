# Feature Specification: LiteLLM Provider Adapter

**Feature Branch**: `008-provider-litellm`
**Created**: 2026-02-19
**Status**: Draft
**Input**: User description: "LiteLLM provider adapter with shared OpenAI-compatible translation base, model mapping, and LiteLLM-specific extensions."

## Overview

This specification adds LiteLLM as a second provider backend for antwort. Since both vLLM and LiteLLM use the OpenAI Chat Completions wire format, the core translation logic is extracted into a shared base that both adapters reuse. The LiteLLM adapter adds model name mapping, multi-model support (LiteLLM routes to 100+ backends), and LiteLLM-specific extensions (fallbacks, cost tracking metadata).

This spec validates a key architectural decision: the provider interface from Spec 003 is generic enough to support multiple backends with minimal per-provider code. The shared translation base ensures consistent behavior across providers and reduces maintenance.

## Clarifications

### Session 2026-02-19

- Q: Should antwort query LiteLLM's /model/info for per-model capabilities? -> A: No. Start with static capabilities (all true). LiteLLM routes to many backends with different capabilities. The backend returns an error if a capability isn't supported, which surfaces correctly via the existing error mapping.
- Q: How to handle model name normalization? -> A: Use a configurable ModelMapping. The user sends "claude-3", the config maps it to "anthropic/claude-3-sonnet". If no mapping exists, pass through as-is.
- Q: Should the shared translator refactoring happen in Spec 03 or here? -> A: Here. The vLLM adapter works fine. This spec extracts the shared base and both adapters embed it. Backward-compatible.
- Q: How to surface LiteLLM cost tracking? -> A: Map to Response.Extensions under the "litellm:" prefix. Follows the extension pattern from Spec 001.
- Q: Provider router for multi-provider? -> A: Out of scope. The engine takes a single provider. Operators choose vLLM or LiteLLM at deployment time. Multi-provider routing is a future spec.

## User Scenarios & Testing

### User Story 1 - Extract Shared Translation Base (Priority: P1)

A developer refactors the vLLM adapter to extract the OpenAI Chat Completions translation logic (request building, response parsing, SSE streaming, error mapping) into a shared package. Both vLLM and LiteLLM adapters embed this shared base. The vLLM adapter continues to work identically after the refactoring.

**Why this priority**: The shared base eliminates code duplication and ensures consistent translation across providers. Without it, every new OpenAI-compatible provider duplicates hundreds of lines.

**Independent Test**: After refactoring, all existing vLLM tests pass. The shared base can be instantiated independently and handles a round-trip (request -> Chat Completions JSON -> response).

**Acceptance Scenarios**:

1. **Given** the existing vLLM adapter, **When** the shared translation base is extracted, **Then** all existing vLLM tests continue to pass without modification
2. **Given** the shared base, **When** a non-streaming request is translated, **Then** the Chat Completions JSON is identical to what the vLLM adapter produced
3. **Given** the shared base, **When** a streaming response is parsed, **Then** the SSE events are handled identically to the vLLM adapter

---

### User Story 2 - LiteLLM Non-Streaming (Priority: P1)

A developer configures antwort to use a LiteLLM proxy as its backend. Non-streaming requests are translated to Chat Completions format, sent to LiteLLM, and responses are parsed back. Model names are optionally mapped via configuration.

**Why this priority**: Non-streaming is the simplest path to validate the LiteLLM adapter works.

**Independent Test**: Start a mock LiteLLM server, send a request through antwort, verify the response is correct.

**Acceptance Scenarios**:

1. **Given** a LiteLLM backend URL and model, **When** a non-streaming request is sent, **Then** the response contains valid output items
2. **Given** a model mapping {"gpt-4": "openai/gpt-4"}, **When** a request for "gpt-4" is sent, **Then** the backend receives "openai/gpt-4"
3. **Given** no model mapping, **When** a request is sent, **Then** the model name is passed through as-is

---

### User Story 3 - LiteLLM Streaming (Priority: P1)

A developer sends a streaming request through antwort to LiteLLM. SSE events are parsed and forwarded as OpenResponses streaming events.

**Why this priority**: Streaming is the primary consumption mode for LLM responses.

**Independent Test**: Start a mock LiteLLM server returning SSE, verify antwort produces correct OpenResponses streaming events.

**Acceptance Scenarios**:

1. **Given** a streaming request to LiteLLM, **When** the backend returns SSE chunks, **Then** antwort emits corresponding OpenResponses streaming events
2. **Given** a streaming request, **When** the backend returns tool call chunks, **Then** antwort correctly reassembles tool call arguments

---

### User Story 4 - LiteLLM Extensions (Priority: P2)

A developer sends a request with LiteLLM-specific parameters (fallback models, cost tracking metadata) via the extensions map. These parameters are forwarded to LiteLLM's extra_body. LiteLLM-specific response data (cost, model info) is returned in the response extensions.

**Why this priority**: Extensions differentiate LiteLLM from generic OpenAI-compatible backends but aren't required for basic functionality.

**Independent Test**: Send a request with `litellm:fallbacks` in extensions, verify it reaches LiteLLM's extra_body.

**Acceptance Scenarios**:

1. **Given** a request with extensions `{"litellm:fallbacks": ["gpt-3.5-turbo"]}`, **When** sent to LiteLLM, **Then** the fallbacks parameter is included in the request body
2. **Given** a LiteLLM response with cost metadata headers, **When** parsed by antwort, **Then** cost info appears in response extensions under `litellm:cost`

---

### Edge Cases

- What happens when the LiteLLM proxy is unreachable? The adapter returns a server error, same as the vLLM adapter (shared error mapping).
- What happens when a mapped model name doesn't exist in LiteLLM? LiteLLM returns its own error, which is mapped through the shared error handler.
- What happens when LiteLLM returns provider-specific error formats? The shared error mapping handles standard OpenAI error format. Non-standard errors are wrapped as server errors.
- What happens when the vLLM adapter is refactored? All existing tests must continue to pass. The refactoring is purely structural (extract, don't modify).

## Requirements

### Functional Requirements

**Shared Translation Base**

- **FR-001**: The system MUST extract the OpenAI Chat Completions translation logic from the vLLM adapter into a shared package that can be embedded by multiple providers
- **FR-002**: The shared base MUST handle: request translation (messages, tools, tool_choice, parameters), response parsing (items, usage, status), SSE streaming (chunk parsing, tool call reassembly), and error mapping (HTTP status codes to API errors)
- **FR-003**: The shared base MUST support customization points: model name mapping, extra request parameters, and response extension extraction
- **FR-004**: After extraction, all existing vLLM tests MUST continue to pass without modification

**LiteLLM Adapter**

- **FR-005**: The system MUST provide a LiteLLM provider adapter that implements the provider interface using the shared translation base
- **FR-006**: The LiteLLM adapter MUST support configurable model name mapping (user-facing name to LiteLLM model identifier)
- **FR-007**: The LiteLLM adapter MUST support non-streaming and streaming requests
- **FR-008**: The LiteLLM adapter MUST support the `/v1/models` endpoint for model discovery
- **FR-009**: The LiteLLM adapter MUST support API key authentication to the LiteLLM proxy

**LiteLLM Extensions**

- **FR-010**: The adapter MUST forward LiteLLM-specific request parameters from the extensions map (prefixed with `litellm:`) to LiteLLM's request body
- **FR-011**: The adapter MUST extract LiteLLM-specific response metadata (cost, model info) into the response extensions map

**Server Integration**

- **FR-012**: The server binary MUST support selecting the LiteLLM provider via environment variable (e.g., `ANTWORT_PROVIDER=litellm`)
- **FR-013**: The server MUST support provider-specific environment variables for LiteLLM configuration (URL, API key, model mapping)

### Key Entities

- **Shared Translation Base**: Reusable OpenAI Chat Completions translation logic embedded by both vLLM and LiteLLM adapters.
- **LiteLLM Adapter**: Provider implementation connecting to a LiteLLM proxy instance.
- **Model Mapping**: Configuration that translates user-facing model names to LiteLLM model identifiers.

## Success Criteria

### Measurable Outcomes

- **SC-001**: All existing vLLM tests pass after the shared base extraction (zero regressions)
- **SC-002**: The LiteLLM adapter handles non-streaming and streaming requests correctly with a mock LiteLLM server
- **SC-003**: Model name mapping translates user-facing names to LiteLLM identifiers
- **SC-004**: LiteLLM-specific extensions (fallbacks, metadata) flow through request/response without loss
- **SC-005**: The conformance test suite passes with LiteLLM as the backend (same score as vLLM)
- **SC-006**: The server binary supports switching between vLLM and LiteLLM via configuration

## Assumptions

- LiteLLM is deployed as a separate proxy service. Antwort connects to it via HTTP, same as vLLM.
- LiteLLM's API is OpenAI Chat Completions compatible. The shared translation base handles 95%+ of the wire format.
- Model mapping is static configuration. Dynamic model discovery from LiteLLM's `/model/info` endpoint is a future enhancement.
- The provider router (choosing between vLLM and LiteLLM per request) is out of scope. Operators choose one provider at deployment time.
- The shared base extraction is a refactoring of existing code, not a rewrite. It preserves all existing behavior.

## Dependencies

- **Spec 003 (Core Engine)**: Provider interface, vLLM adapter (source of shared translation logic).
- **Spec 006 (Conformance)**: Server binary that needs provider selection support.

## Scope Boundaries

### In Scope

- Shared OpenAI Chat Completions translation base (extracted from vLLM)
- LiteLLM provider adapter (non-streaming, streaming, models)
- Model name mapping configuration
- LiteLLM-specific extensions (fallbacks, cost metadata)
- Server binary provider selection (ANTWORT_PROVIDER env var)
- vLLM adapter refactoring to use shared base

### Out of Scope

- Running LiteLLM itself (antwort connects to an existing proxy)
- Provider router for multi-provider selection
- Per-model dynamic capability checking
- LiteLLM SDK embedding (HTTP API only)
- Cost-based routing or budget enforcement
