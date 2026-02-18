# Quickstart: Agentic Loop & Tool Orchestration

**Feature**: 004-agentic-loop
**Date**: 2026-02-18

## Single-Shot Tool Calls (No Executors)

Without any executors registered, the engine behaves exactly as Spec 003: tool calls are returned to the client as `function_call` items with `completed` status.

```go
// Create engine without executors (single-shot mode).
eng, err := engine.New(myProvider, nil, engine.Config{
    DefaultModel: "llama-3-8b",
})

// Submit request with tools.
req := &api.CreateResponseRequest{
    Model: "llama-3-8b",
    Input: []api.Item{...},
    Tools: []api.ToolDefinition{
        {Type: "function", Name: "get_weather", ...},
    },
}

// Response contains function_call items directly.
// Status: "completed"
eng.CreateResponse(ctx, req, writer)
```

## Agentic Loop with Mock Executor

Register an executor to enable the agentic loop. The engine dispatches tool calls to the executor and feeds results back to the model.

```go
// Create a mock executor that handles all tools.
mockExec := &MockExecutor{
    canExecute: func(tool api.ToolDefinition) bool { return true },
    execute: func(ctx context.Context, call tools.ToolCall) (*tools.ToolResult, error) {
        return &tools.ToolResult{
            CallID: call.ID,
            Output: `{"temperature": 22, "unit": "celsius"}`,
        }, nil
    },
}

// Create engine with executor (agentic mode).
eng, err := engine.New(myProvider, nil, engine.Config{
    DefaultModel:    "llama-3-8b",
    MaxAgenticTurns: 5,
    Executors:       []tools.ToolExecutor{mockExec},
})

// The engine now runs the agentic loop:
// 1. Model produces function_call items
// 2. Engine calls mockExec.Execute for each
// 3. Results fed back to model
// 4. Model produces final text answer
eng.CreateResponse(ctx, req, writer)
```

## Client-Executed Function Tools (requires_action)

When no executor can handle a tool call, the engine returns `requires_action`:

```go
// Engine has executors, but none handle "function" type tools.
eng, err := engine.New(myProvider, nil, engine.Config{
    Executors: []tools.ToolExecutor{mcpExecutor}, // only handles MCP tools
})

// Model calls a function tool -> no executor matches -> requires_action
// Response status: "requires_action"
// Response output: [function_call items]

// Client executes the function and sends follow-up:
followUp := &api.CreateResponseRequest{
    Model:              "llama-3-8b",
    PreviousResponseID: resp.ID, // points to requires_action response
    Input: []api.Item{
        {
            Type: api.ItemTypeFunctionCallOutput,
            FunctionCallOutput: &api.FunctionCallOutputData{
                CallID: "call_abc123",
                Output: `{"result": "success"}`,
            },
        },
    },
}
eng.CreateResponse(ctx, followUp, writer)
```

## Allowed Tools Filtering

Restrict which tools the model may invoke:

```go
req := &api.CreateResponseRequest{
    Model: "llama-3-8b",
    Input: []api.Item{...},
    Tools: []api.ToolDefinition{
        {Type: "function", Name: "get_weather"},
        {Type: "function", Name: "delete_account"},
        {Type: "function", Name: "search_docs"},
    },
    AllowedTools: []string{"get_weather", "search_docs"},
    // "delete_account" is visible to the model but cannot be called.
    // If the model calls it, an error is fed back.
}
```

## Streaming Agentic Loop

Multi-turn streaming produces a single continuous event stream:

```
// Turn 1: Model calls a tool
response.created          (once)
response.in_progress      (once)
output_item.added         (function_call)
function_call_arguments.delta ...
function_call_arguments.done
output_item.done

// [server-side tool execution, invisible to client]

// Turn 2: Model answers with text
output_item.added         (message)
content_part.added
output_text.delta ...
output_text.done
content_part.done
output_item.done
response.completed        (once)
```
