# Brainstorm 30: Responses API Provider

## Problem

Antwort currently translates between two protocols: it accepts OpenResponses API requests from clients and converts them to Chat Completions API requests for the backend (vLLM, LiteLLM). This translation is lossy.

The engine synthesizes Responses API lifecycle events (response.created, output_item.added, content_part.delta, etc.) from Chat Completions delta chunks. Every new feature (reasoning tokens, code_interpreter events, structured output) requires custom synthesis logic. The translation layer is growing and becoming fragile.

Meanwhile, vLLM (and other backends like Ollama, SGLang) are adding native Responses API support. If the backend speaks the same protocol as the client, the translation becomes near-identity and the provider layer gets simpler instead of more complex.

## Core Idea

Add a Responses API provider that forwards inference requests using the Responses API wire format instead of Chat Completions. Antwort still owns the agentic loop, state management, and server-side tool execution. Only the inference call changes protocol.

```
Today:
  Client (Responses API) → Engine → Provider (Chat Completions) → vLLM
                                     ↑ lossy, synthesized events

Proposed:
  Client (Responses API) → Engine → Provider (Responses API) → vLLM
                                     ↑ near-identity, native events
```

## What Antwort Adds (Value Proposition)

vLLM's Responses API is stateless. Antwort adds:

1. **Persistence and conversation chaining**: `store: true`, `previous_response_id`, response CRUD
2. **Server-side tool execution**: code_interpreter (sandbox pods), MCP tools, web_search, file_search
3. **Agentic loop**: Multi-turn inference-tool execution cycle with configurable max turns
4. **Auth and multi-tenancy**: API keys, JWT, tenant scoping
5. **Observability**: Metrics, tracing, structured logging across turns

These are the things that make antwort more than a proxy.

## Architecture Decision: Inference Only

The provider delegates only the inference call. Not the full request.

**Antwort owns:**
- Agentic loop (multi-turn tool calling)
- State management (store, previous_response_id, history reconstruction)
- Tool execution (code_interpreter, MCP, function providers)
- SSE event lifecycle (response.created through response.completed)
- Auth, metrics, tenant scoping

**The provider does:**
- Single inference call per turn (send messages + tools, receive response/events)
- Translate between antwort's internal types and the wire format
- Stream back events or return a complete response

This matches Constitution Principle VI: the provider handles protocol translation, the engine handles orchestration.

## Impact on expandBuiltinTools

Currently `expandBuiltinTools()` in `engine.go` converts `{"type": "code_interpreter"}` to `{"type": "function", "function": {...}}` before provider translation. This is a Chat Completions-specific concern in the wrong layer.

With a Responses API provider, built-in tool types should pass through natively. The fix:

1. **Remove** `expandBuiltinTools()` from the engine
2. **Engine**: Preserve `tool.Type` as-is when translating to `ProviderTool`
3. **Chat Completions adapters** (openaicompat): Expand built-in types to function definitions
4. **Responses API adapter**: Pass through built-in types natively

This restores constitutional compliance. Each adapter handles its own protocol's tool format requirements.

## Provider Interface Fit

The current `Provider` interface works without changes:

```go
type Provider interface {
    CreateResponse(ctx, req ProviderRequest) (ProviderResponse, error)
    StreamResponse(ctx, req ProviderRequest, ch chan<- ProviderEvent) error
    Capabilities() ProviderCapabilities
}
```

A Responses API adapter implements the same interface. The difference is internal:
- Chat Completions adapter translates to/from `/v1/chat/completions`
- Responses API adapter translates to/from `/v1/responses` (with `store: false`, no loop)

The `ProviderRequest` and `ProviderResponse` types are already modeled after the Responses API, so the Responses API adapter's translation is minimal.

## Streaming Benefit

The biggest gain is streaming fidelity. Today, the engine:
1. Receives Chat Completions delta chunks (role, content, tool_calls)
2. Synthesizes Responses API events (output_item.added, content_part.delta, etc.)
3. Tracks state (which output item is current, argument buffering, content indexing)

With a Responses API backend:
1. Receives native Responses API events (already in the right format)
2. Maps to `ProviderEvent` (near 1:1)
3. Engine emits them with minimal transformation

New features (reasoning streaming, code_interpreter events) work automatically if the backend supports them, without custom synthesis logic in antwort.

## Open Questions

1. **How does the provider know the tool definitions for expansion?** The Chat Completions adapter needs the FunctionProvider tool definitions to expand built-in types. Should this be passed via `ProviderRequest`, injected at construction time, or resolved via an interface?

2. **vLLM Responses API maturity**: What's actually implemented in vLLM's `/v1/responses`? Is it stable enough to target? Does it support streaming? Tool definitions?

3. **Request format differences**: vLLM's Responses API might not match OpenAI's exactly. Are there field name differences, missing features, or extensions?

4. **Fallback strategy**: If the backend doesn't support Responses API, should the provider fall back to Chat Completions? Or should this be a separate provider type?

5. **Configuration**: How does the user select the provider type? `provider: vllm-responses` vs `provider: vllm`? Or auto-detect by probing the backend?

## Dependencies

- Spec 002 (Transport Layer): No changes needed
- Spec 003 (Core Engine): Engine must stop expanding built-in tools
- Spec 006 (Provider Interface): No interface changes needed
- Spec 016 (Function Registry): FR-007a moves from engine to provider layer

## Relationship to Other Brainstorms

- **Brainstorm 24 (Config-File Agents)**: Agent profiles define tool sets. The Responses API provider must support the same tool resolution.
- **Brainstorm 11 (Sandbox)**: code_interpreter tool type passes through natively to Responses API backends.
- **Brainstorm 20 (Server Agentic Platform)**: The Responses API provider simplifies the inference layer, letting antwort focus on orchestration.
