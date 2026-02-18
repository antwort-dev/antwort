# Spec 04: Agentic Loop & Tool Orchestration

**Branch**: `spec/04-agentic-loop`
**Dependencies**: Spec 01 (Core Protocol), Spec 03 (Core Engine)
**Package**: `pkg/engine` (loop), `pkg/tools` (types, executors, filtering)

## Purpose

Implement the agentic inference-tool execution cycle that transforms antwort from a single-shot inference proxy into an agentic execution engine. This spec covers tool type definitions, tool choice enforcement, allowed_tools filtering, and the core loop that orchestrates multi-turn inference with tool invocation.

The agentic loop is the orchestration layer. It does not execute tools itself. It delegates execution to pluggable `ToolExecutor` implementations (function/client-executed, MCP, sandbox) defined in separate specs.

## Scope

### In Scope

- Tool type definitions (ToolKind, ToolDefinition, ToolCall, ToolResult)
- ToolExecutor interface (the contract that MCP and sandbox specs implement)
- `tool_choice` enforcement (auto, required, none, forced function)
- `allowed_tools` filtering (restrict which tools the model may invoke)
- Agentic loop orchestration (inference -> tool calls -> execute -> feed back -> inference)
- Turn management and termination conditions
- `requires_action` response status for client-executed function tools
- Streaming behavior across multiple agentic turns
- Maximum turns safety limit (configurable)
- Integration with engine's `CreateResponse` flow

### Out of Scope

- MCP client implementation (Spec 10)
- Sandbox pod execution (Spec 11)
- Specific tool implementations (file search, web search, code interpreter)
- Provider-specific tool formats (handled by Translator in Spec 03)

## Tool Type Hierarchy

```go
// ToolKind classifies how a tool is hosted and executed.
type ToolKind int

const (
    // ToolKindFunction is a client-executed tool. The model produces
    // function_call items, the response is returned to the client with
    // status "requires_action" (in agentic mode), and the client sends
    // back function_call_output items in a follow-up request.
    ToolKindFunction ToolKind = iota

    // ToolKindMCP is a tool connected via the Model Context Protocol.
    // Antwort connects to the MCP server and executes the tool
    // server-side within the agentic loop.
    ToolKindMCP

    // ToolKindSandbox is a tool executed in an isolated Kubernetes
    // sandbox pod. Antwort delegates execution to a pod from the
    // sandbox pool and collects the result.
    ToolKindSandbox
)
```

## ToolExecutor Interface

```go
// ToolExecutor executes tool calls. Implementations exist for each
// ToolKind: function (returns to client), MCP (calls MCP server),
// sandbox (delegates to k8s pod).
type ToolExecutor interface {
    // Kind returns the type of tools this executor handles.
    Kind() ToolKind

    // CanExecute checks if this executor can handle the given tool.
    CanExecute(tool ToolDefinition) bool

    // Execute runs the tool and returns the result.
    // For function tools, this returns an error indicating the call
    // must be delegated to the client (the loop handles this by
    // pausing and returning requires_action).
    Execute(ctx context.Context, call ToolCall) (*ToolResult, error)
}
```

The function tool "executor" is special: it doesn't execute anything. It signals to the agentic loop that execution must pause and return control to the client.

## Tool Choice & Filtering

```go
// Tool choice enforcement:
// - "auto": model decides whether to call tools (default)
// - "required": model must call at least one tool
// - "none": model must not call tools (tools still visible for context)
// - forced: model must call a specific named tool

// AllowedTools filtering:
// - If allowed_tools is set, only those tools are callable
// - All tools are still sent for context (model sees them)
// - Calls to non-allowed tools are rejected by the engine
```

Filtering happens at two levels:
1. **Before inference**: The engine sends all tools to the provider (for context) but marks the allowed subset
2. **After inference**: If the model calls a non-allowed tool, the engine rejects it with an error (rather than sending it to the provider and hoping it complies)

## Agentic Loop

### Loop Flow

```
Request arrives
    │
    ▼
┌──────────────────┐
│ Translate request │
│ (with tools)      │
└────────┬─────────┘
         │
    ┌────▼────┐
    │ Provider │◄──────────────────────┐
    │ Inference│                       │
    └────┬────┘                        │
         │                             │
    ┌────▼──────────┐                  │
    │ Tool calls in  │                 │
    │ response?      │                 │
    └──┬──────────┬──┘                 │
       │          │                    │
    Yes│          │No                  │
       │          │                    │
  ┌────▼────┐  ┌──▼──────────┐        │
  │ Classify │  │ Return      │        │
  │ by Kind  │  │ final       │        │
  └──┬───┬──┘  │ response    │        │
     │   │     └─────────────┘        │
     │   │                            │
     │   ├── Function? ──► Pause,     │
     │   │                 return     │
     │   │                 requires_  │
     │   │                 action     │
     │   │                            │
     │   ├── MCP? ──► Call MCP ──┐    │
     │   │            server     │    │
     │   │                       │    │
     │   └── Sandbox? ► Execute  │    │
     │                  in pod ──┤    │
     │                           │    │
     │   ┌───────────────────────┘    │
     │   │                            │
     │   ▼                            │
     │ Collect results                │
     │ Append to conversation         │
     │ Check turn limit               │
     └───────────────────────────────►┘
```

### Termination Conditions

The loop terminates when:
1. **Final answer**: The model produces only message items (no tool calls)
2. **Client-executed tools**: A function tool call requires client action (return `requires_action`)
3. **Max turns reached**: Safety limit exceeded (configurable, default: 10)
4. **Context cancelled**: Client disconnect or timeout
5. **Unrecoverable error**: Provider error, tool execution failure

### Streaming Across Turns

During a multi-turn agentic loop with streaming enabled, the client receives a continuous stream of events from all turns. Each turn's events flow through without explicit turn delimiters.

The event sequence for a 2-turn agentic loop (text -> tool call -> tool result -> final text):

```
Turn 1:
  response.created
  response.in_progress
  output_item.added (function_call)
  function_call_arguments.delta ...
  function_call_arguments.done
  output_item.done

[tool execution happens server-side, invisible to client]

Turn 2:
  output_item.added (message)
  content_part.added
  output_text.delta ...
  output_text.done
  content_part.done
  output_item.done
  response.completed
```

The response.created and response.in_progress happen once (at the start). The response.completed happens once (at the end). Intermediate tool execution is invisible to the client for server-executed tools (MCP, sandbox).

For client-executed function tools, the stream pauses after the function_call items and the response status is `requires_action`.

## Integration with Engine

The agentic loop extends the engine's `CreateResponse` method. When tools are present in the request and tool executors are configured, the engine uses the loop instead of single-shot inference.

The loop uses nil-safe composition: if no tool executors are registered, the engine falls back to single-shot behavior (tool calls are returned to the client as function_call items, exactly as Spec 003 does today).

## Open Questions

- Should parallel tool execution be supported (multiple tool calls in one turn executed concurrently)?
- How should the loop handle mixed tool kinds in a single turn (e.g., one function call + one MCP call)?
- Should there be per-tool timeout configuration, or just the overall request context timeout?
- How should tool execution errors be represented in the event stream (as failed items, as error events, or both)?
