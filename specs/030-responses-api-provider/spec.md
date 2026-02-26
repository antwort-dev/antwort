# Feature Specification: Responses API Provider

**Feature Branch**: `030-responses-api-provider`
**Created**: 2026-02-26
**Status**: Draft

## Overview

The gateway currently translates between two protocols: clients send OpenResponses API requests, and the backend receives Chat Completions API requests. This translation is lossy. The engine synthesizes Responses API lifecycle events from Chat Completions delta chunks, and every new feature (reasoning, code_interpreter, structured output) requires custom synthesis logic.

Backends like vLLM, SGLang, and Ollama are adding native Responses API support. This spec adds a provider adapter that forwards inference requests using the Responses API wire format. The gateway still owns the agentic loop, state management, and server-side tool execution. Only the inference call changes protocol, yielding native SSE events and simpler translation.

## Clarifications

### Session 2026-02-26

- Q: Should the gateway delegate the full request or just the inference call? A: Inference only. The gateway owns the agentic loop, state, and tools. The provider only handles a single inference call per turn.
- Q: What's the primary value? A: Adding statefulness (persistence, conversation chaining) and server-side tool execution (code_interpreter, MCP, web_search) to a stateless Responses API backend.
- Q: Where do built-in tool type expansions happen? A: In the provider layer. Chat Completions adapters expand `code_interpreter` to function definitions. The Responses API adapter passes them through natively. This fixes the current violation of Constitution Principle VI.

## User Scenarios & Testing

### User Story 1 - Native Responses API Inference (Priority: P1)

An operator deploys the gateway in front of a vLLM instance that supports the Responses API. Clients send OpenResponses API requests. The gateway forwards inference calls using the Responses API wire format instead of translating to Chat Completions. SSE events from the backend flow through natively without synthesis.

**Why this priority**: This is the core capability. Without it, every new feature requires custom event synthesis logic in the gateway.

**Independent Test**: Configure the gateway with a Responses API provider pointing at a vLLM instance. Send a streaming request and verify that SSE events arrive with the correct Responses API event types.

**Acceptance Scenarios**:

1. **Given** a backend supporting the Responses API, **When** a streaming request is sent, **Then** the client receives native Responses API SSE events (response.created, output_item.added, content_part.delta, response.completed)
2. **Given** a backend supporting the Responses API, **When** a non-streaming request is sent, **Then** the response contains the complete output in the standard format
3. **Given** a request with `{"type": "code_interpreter"}` in the tools array, **When** forwarded to a Responses API backend, **Then** the built-in tool type is preserved (not expanded to a function definition)

---

### User Story 2 - Stateful Enrichment (Priority: P1)

A developer sends requests with `store: true` and `previous_response_id` to chain multi-turn conversations. The backend is stateless (Responses API without persistence). The gateway adds state management on top: storing responses, reconstructing conversation history, and enabling CRUD operations on stored responses.

**Why this priority**: Statefulness is the gateway's primary value proposition over a raw backend.

**Independent Test**: Send two requests where the second references the first via `previous_response_id`. Verify the backend receives the full conversation history even though it has no state.

**Acceptance Scenarios**:

1. **Given** `store: true`, **When** a response is created, **Then** the response is persisted and retrievable via GET
2. **Given** a `previous_response_id`, **When** a new request is sent, **Then** the gateway reconstructs the conversation history and sends it to the backend
3. **Given** `store: false`, **When** a response is created, **Then** no persistence occurs (pass-through behavior)

---

### User Story 3 - Server-Side Tool Execution (Priority: P1)

A developer sends a request with `code_interpreter` and MCP tools enabled. The model produces a tool call. The gateway executes the tool server-side (sandbox pod for code_interpreter, MCP server for MCP tools) and feeds the result back to the backend for the next turn.

**Why this priority**: Server-side tool execution is what makes the gateway more than a proxy.

**Independent Test**: Send a request with code_interpreter enabled. Verify the model can call it, the code executes, and the result is fed back for a multi-turn response.

**Acceptance Scenarios**:

1. **Given** code_interpreter enabled, **When** the model calls code_interpreter, **Then** the gateway executes the code in a sandbox and returns the result as a tool output for the next inference turn
2. **Given** MCP tools enabled, **When** the model calls an MCP tool, **Then** the gateway executes it via the MCP server
3. **Given** multiple tool calls in one turn, **When** parallel_tool_calls is true, **Then** all tools execute concurrently

---

### User Story 4 - Migration from Chat Completions Provider (Priority: P2)

An operator switches from the existing Chat Completions provider (`vllm`) to the Responses API provider (`vllm-responses`) by changing the `provider` field in the configuration. All existing functionality (streaming, tools, state) continues to work without client changes. Both providers remain available; the operator chooses which one to use.

**Why this priority**: Seamless migration ensures adoption without disruption.

**Independent Test**: Run the conformance test suite against both provider types and verify identical results.

**Acceptance Scenarios**:

1. **Given** an existing deployment using the Chat Completions provider, **When** the operator changes the provider type to Responses API, **Then** all existing API behavior is preserved
2. **Given** the Responses API provider, **When** the conformance test suite runs, **Then** all tests pass

---

### Edge Cases

- What happens when the backend doesn't support the Responses API? The provider returns a clear error at startup indicating the backend URL does not support the Responses API endpoint.
- What happens when the backend returns an unexpected SSE event type? Unknown event types are logged and skipped without breaking the stream.
- What happens when the backend's Responses API is partially implemented (e.g., no streaming)? The provider falls back to non-streaming mode and reports a capability limitation.
- What happens when the backend returns errors in Responses API format? The provider maps backend errors to the gateway's typed error domain.

## Requirements

### Functional Requirements

**Provider Adapter**

- **FR-001**: The system MUST support a Responses API provider type that forwards inference requests using the Responses API wire format
- **FR-002**: The provider MUST send inference requests to the backend's `/v1/responses` endpoint with `store: false` (the gateway manages state, not the backend)
- **FR-003**: The provider MUST translate between the gateway's internal types and the Responses API wire format
- **FR-004**: The provider MUST implement the same Provider interface used by Chat Completions adapters (no interface changes)

**Streaming**

- **FR-005**: The provider MUST support streaming inference by consuming the backend's native Responses API SSE events
- **FR-006**: The provider MUST map backend SSE events to the gateway's internal event types
- **FR-007**: The provider MUST support non-streaming inference as a fallback when the backend does not support streaming

**Built-in Tool Types**

- **FR-008**: The Responses API provider MUST pass built-in tool types (`code_interpreter`, `file_search`, `web_search_preview`) through to the backend without expansion
- **FR-009**: The Chat Completions providers MUST expand built-in tool types to function definitions (moving this logic from the engine to the provider layer)
- **FR-010**: The engine MUST stop expanding built-in tool types and preserve them as-is for the provider to handle

**Configuration**

- **FR-011**: The provider MUST be a separate provider type (e.g., `provider: vllm-responses`) that coexists with the existing Chat Completions providers (`vllm`, `litellm`). The operator selects one provider per deployment. The existing providers are not modified or replaced.
- **FR-012**: The provider MUST accept the same configuration fields as the existing providers (backend_url, api_key, default_model)

**Error Handling**

- **FR-013**: The provider MUST validate at startup that the backend supports the Responses API endpoint
- **FR-014**: The provider MUST map backend error responses to the gateway's typed error domain

## Success Criteria

### Measurable Outcomes

- **SC-001**: A streaming request through the Responses API provider returns the same user-visible output as the Chat Completions provider for the same model and input
- **SC-002**: The conformance test suite passes identically against both provider types
- **SC-003**: Built-in tool types (code_interpreter, web_search) work through the agentic loop with the Responses API provider
- **SC-004**: New streaming features supported by the backend (e.g., reasoning events) are forwarded to clients without custom synthesis code in the gateway
- **SC-005**: Migration from Chat Completions to Responses API provider requires only a configuration change, no client modifications

## Assumptions

- The target backend (vLLM) has a stable Responses API implementation at `/v1/responses` that supports at least basic inference (messages + model + stream).
- The backend's Responses API produces SSE events compatible with the OpenResponses specification.
- The gateway's internal types (ProviderRequest, ProviderResponse, ProviderEvent) are close enough to the Responses API format that translation is minimal.
- The existing Provider interface (CreateResponse, StreamResponse, Capabilities) is sufficient for the Responses API adapter without changes.
- Built-in tool types in the tools array are recognized and handled by the backend's Responses API (or ignored gracefully if unsupported).

## Dependencies

- **Spec 003 (Core Engine)**: Engine must stop expanding built-in tools (FR-010)
- **Spec 006 (Provider Interface)**: No changes needed, but the provider must implement the existing interface
- **Spec 016 (Function Registry)**: FR-007a (built-in tool expansion) moves from engine to provider layer (FR-009)

## Scope Boundaries

### In Scope

- Responses API provider adapter (inference only, single-turn per call)
- SSE event mapping (backend events to gateway events)
- Built-in tool type passthrough for Responses API, expansion for Chat Completions
- Migration of `expandBuiltinTools` from engine to provider layer
- Configuration for provider type selection
- Startup validation of backend Responses API support

### Out of Scope

- Full request passthrough (delegating the agentic loop to the backend)
- Backend-side state management (the gateway always manages state)
- Auto-detection of provider type based on backend capabilities (explicit configuration only)
- Multi-backend routing or load balancing
- Backend-specific extensions beyond the standard Responses API
