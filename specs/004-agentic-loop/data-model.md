# Data Model: Agentic Loop & Tool Orchestration

**Feature**: 004-agentic-loop
**Date**: 2026-02-18

## New Types

### ToolExecutor (Interface)

The pluggable contract for tool execution backends.

| Method | Signature | Description |
|--------|-----------|-------------|
| Kind | `() -> ToolKind` | Returns the kind of tools this executor handles |
| CanExecute | `(tool ToolDefinition) -> bool` | Checks if this executor can handle the given tool |
| Execute | `(ctx, call ToolCall) -> (*ToolResult, error)` | Runs the tool and returns the result |

### ToolKind (Enum)

Classifies how a tool is hosted and executed.

| Value | Description |
|-------|-------------|
| `ToolKindFunction` | Client-executed. Returns to client with `requires_action`. |
| `ToolKindMCP` | Server-executed via MCP server connection. |
| `ToolKindSandbox` | Server-executed in isolated Kubernetes sandbox pod. |

### ToolCall (Value)

A model's request to invoke a tool.

| Field | Type | Description |
|-------|------|-------------|
| ID | string | Unique call identifier (from the model, e.g., `call_abc123`) |
| Name | string | Tool function name |
| Arguments | string | JSON-encoded arguments string |

Maps from: `api.Item` with `Type=function_call` -> `FunctionCall.CallID`, `FunctionCall.Name`, `FunctionCall.Arguments`

### ToolResult (Value)

The output of a tool execution.

| Field | Type | Description |
|-------|------|-------------|
| CallID | string | Matches the originating ToolCall.ID |
| Output | string | Tool output content (text) |
| IsError | bool | If true, the output is an error message |

Maps to: `api.Item` with `Type=function_call_output` -> `FunctionCallOutput.CallID`, `FunctionCallOutput.Output`

## Amended Types

### ResponseStatus (Spec 001 Amendment)

Add new terminal status value.

| Value | New? | Description |
|-------|------|-------------|
| `queued` | No | Waiting to be processed |
| `in_progress` | No | Currently being processed |
| `completed` | No | Successfully completed |
| `incomplete` | No | Output truncated or turn limit reached |
| `failed` | No | Error occurred |
| `cancelled` | No | Client or system cancelled |
| **`requires_action`** | **Yes** | Paused, waiting for client to provide function call results |

### State Machine Transitions (Spec 001 Amendment)

| From | To (allowed) |
|------|-------------|
| (initial) | queued, in_progress |
| queued | in_progress |
| in_progress | completed, failed, cancelled, **requires_action** |
| completed | (terminal) |
| incomplete | (terminal) |
| failed | (terminal) |
| cancelled | (terminal) |
| **requires_action** | **(terminal)** |

### Engine Config (Spec 003 Extension)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| DefaultModel | string | "" | Existing field |
| **MaxAgenticTurns** | **int** | **10** | Maximum turns in the agentic loop before returning incomplete |
| **Executors** | **[]ToolExecutor** | **nil** | Registered tool executors. Nil = single-shot fallback. |

## Relationships

```
Engine
  ├── has Provider (required, from Spec 003)
  ├── has ResponseStore (optional, from Spec 003)
  ├── has []ToolExecutor (optional, new)
  │    ├── FunctionToolExecutor (built-in, signals client execution)
  │    ├── MCPExecutor (Spec 10, future)
  │    └── SandboxExecutor (Spec 11, future)
  └── has Config
       ├── DefaultModel
       └── MaxAgenticTurns

ToolExecutor
  ├── receives ToolCall (from api.Item function_call)
  └── returns ToolResult (becomes api.Item function_call_output)
```
