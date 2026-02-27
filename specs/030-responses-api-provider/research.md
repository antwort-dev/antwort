# Research: Responses API Provider

## R1: vLLM Responses API Status

**Decision**: Target vLLM v0.10.0+ as the primary backend.

**Rationale**: vLLM has had `/v1/responses` since v0.10.0 (July 2025). The implementation is maturing with each release. The wire format follows the OpenAI spec, the OpenAI Python SDK works as-is, and streaming SSE events are supported.

**Key findings**:
- Endpoint: `/v1/responses` (available since v0.10.0)
- Request format: OpenAI-compatible with extra vLLM-specific fields
- Streaming: Yes, native SSE events (response.created, response.output_text.delta, response.function_call_arguments.delta, response.completed, etc.)
- Store support: Optional (`VLLM_ENABLE_RESPONSES_API_STORE=1`), but antwort manages state regardless
- Tool support: Function tools work. Built-in types (code_interpreter, web_search_preview) route through MCP servers in vLLM, not handled natively.
- Multi-turn: Supported via `previous_response_id` when store is enabled, or via `previous_input_messages` for stateless mode

**Limitations to account for**:
- vLLM's built-in tools require MCP server setup (not relevant for antwort since we execute tools ourselves)
- Strict Pydantic validation on message objects (may reject messages missing `id`/`status` fields)
- Multimodal support is partial
- vLLM documentation says the implementation "does not cover the full scope"

## R2: Inference-Only Forwarding Strategy

**Decision**: Forward a single inference call per agentic loop turn. Always send `store: false`. Reconstruct conversation as `input` messages.

**Rationale**: Antwort owns state and the agentic loop. The backend should only do inference. Sending `store: false` prevents the backend from duplicating state management. Conversation history is assembled by antwort's engine and sent as the `input` field.

**Key concern**: vLLM's strict message validation. If antwort sends conversation history as input items, each message must include all required fields (`id`, `status`, `role`, `content`). The current `ProviderRequest.Messages` type may need adjustment.

**Alternatives considered**:
- Let vLLM manage state via its store: rejected, duplicates antwort's state management
- Send `previous_response_id` to vLLM: rejected, requires vLLM's store to be enabled and synced
- Full passthrough: rejected, antwort loses control over tool execution and loop

## R3: Built-in Tool Type Handling

**Decision**: Responses API provider passes tool definitions as-is. Chat Completions providers expand built-in types to function definitions. The engine preserves types without modification.

**Rationale**: Constitution Principle VI requires protocol-specific concerns in adapters. Built-in tool type expansion is Chat Completions-specific. For the Responses API, `{"type": "function"}` tools pass through directly. Built-in types like `code_interpreter` are handled by antwort's agentic loop (not forwarded to the backend for execution).

**Important nuance**: vLLM expects built-in tools to be MCP-backed. Antwort doesn't forward built-in tool types to vLLM at all. Instead, antwort strips built-in tools from the request before forwarding (they're executed server-side by antwort's tool executors). Only `{"type": "function"}` tools are forwarded to the backend so the model knows about them.

This is the same pattern used by the Chat Completions providers today: the engine registers tools with the model for tool calling, but executes them locally.

## R4: How Tool Definitions Reach the Provider

**Decision**: Tool definitions are already available in `ProviderRequest.Tools`. The Chat Completions adapter inspects each tool's `Type` field and expands built-in types using the function definitions already present in the tool list (placed there by `expandBuiltinTools` or the FunctionRegistry's `DiscoveredTools`).

**Rationale**: The migration path is incremental:
1. Phase 1: Move `expandBuiltinTools` from engine to the `openaicompat` translation layer. The tools are already in the `ProviderRequest` by that point.
2. Phase 2: The Responses API adapter skips expansion and passes tools through.
3. No new dependency injection needed. The tool definitions flow through the existing request pipeline.

## R5: Other Backend Support

**Decision**: Target vLLM first. SGLang and Ollama have Responses API support but with significant limitations.

**Findings**:
- **SGLang**: Has `/v1/responses` but does NOT support custom function tools (only built-in/MCP tools). This blocks agentic loop usage since antwort needs the model to call custom functions.
- **Ollama**: Has `/v1/responses` since v0.13.3 but is explicitly stateless. Works for basic inference but multi-turn is client-managed.
- Both could work as backends with limitations. vLLM is the most complete implementation.

## R6: vLLM Multi-Turn & Augmented Protocol (Brainstorm, Feb 2026)

**Status**: Tracking upstream, not actionable yet.

**Context**: Internal discussion (Slack, Feb 23-24 2026) between Roland, Ben Browning, and Seb Han explored how stateful Responses API should work between a proxy (antwort/openresponses-gw) and vLLM.

**Key insights from the discussion**:
- vLLM's `/v1/responses` endpoint does NOT support multi-turn conversations in the Responses API input format ([vllm#33089](https://github.com/vllm-project/vllm/issues/33089)). Pydantic validation rejects typed input items from prior turns.
- Ben: "None of Responses, Chat Completions, or Completions API provide everything needed by the thing doing the state and agentic loops. You have to go outside of any of those APIs a bit, or invent a new protocol."
- vLLM is "smuggling additional data" via extensions to the Responses API (e.g., [PR #24985](https://github.com/vllm-project/vllm/pull/24985) for `enable_response_messages`), but nothing is standardized.
- The vLLM Semantic Router has its own in-memory state management for Responses API, but that's a separate layer from vLLM core.
- Flora Feng (new RHT hire) is starting to work on Responses API and tool calling in vLLM upstream.
- A vLLM community SIG for Responses API and Tool Calling is forming.

**Roland's position**: vLLM should not frame its endpoint as a "stateless Responses API" but as a proxy protocol inspired by the Responses API. The gateway (antwort) speaks compliant Responses API to clients, and uses a purpose-specific internal protocol to the backend.

**Recommendation**: Once vLLM settles on extensions (e.g., `previous_input_messages`, custom fields for conversation history), the `vllm-responses` provider should adopt them. Until then, the current approach (sending history as `input` items via the standard Responses API format, or falling back to Chat Completions for multi-turn) works. This is a future spec, not part of 030.

**References**:
- [vLLM Issue #33089: Multi-turn conversation support](https://github.com/vllm-project/vllm/issues/33089)
- [vLLM PR #24985: Response messages output](https://github.com/vllm-project/vllm/pull/24985)
- [vLLM Semantic Router v0.1](https://blog.vllm.ai/2026/01/05/vllm-sr-iris.html)
- [openresponses-gw proposal for vSR separation](https://github.com/leseb/openresponses-gw/blob/main/docs/proposal-vsr-responses-api-separation.md)
