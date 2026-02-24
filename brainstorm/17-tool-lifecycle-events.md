# Brainstorm 17: Tool Lifecycle SSE Events

## Problem

When Antwort executes tools during the agentic loop (MCP, file_search, web_search), the client only sees the final `function_call` and `function_call_output` items. There's no visibility into the tool execution progress. The upstream OpenResponses spec defines lifecycle events for each built-in tool type so clients can show "searching...", "executing MCP tool...", etc.

## Missing SSE Events (17)

### MCP tool execution (8)
- `response.mcp_call.in_progress` - MCP tool call started
- `response.mcp_call_arguments.delta` - incremental MCP arguments
- `response.mcp_call_arguments.done` - MCP arguments complete
- `response.mcp_call.completed` - MCP tool execution finished
- `response.mcp_call.failed` - MCP tool execution failed
- `response.mcp_list_tools.in_progress` - MCP tool discovery started
- `response.mcp_list_tools.completed` - MCP tool discovery finished
- `response.mcp_list_tools.failed` - MCP tool discovery failed

### File search (3)
- `response.file_search_call.in_progress` - file search started
- `response.file_search_call.searching` - actively searching
- `response.file_search_call.completed` - file search finished

### Web search (3)
- `response.web_search_call.in_progress` - web search started
- `response.web_search_call.searching` - actively searching
- `response.web_search_call.completed` - web search finished

### Code interpreter (5)
- `response.code_interpreter_call.in_progress` - code execution started
- `response.code_interpreter_call.interpreting` - actively running
- `response.code_interpreter_call.code.delta` - incremental code output
- `response.code_interpreter_call.code.done` - code output complete
- `response.code_interpreter_call.completed` - code execution finished

## Current State

- The agentic loop in `pkg/engine/loop.go` executes tools via `executeTools()` but emits no progress events during execution
- In streaming mode, the loop emits `output_item.added` / `output_item.done` for tool call results, but nothing in between
- MCP execution happens in `pkg/tools/mcp/executor.go`
- File search execution happens in `pkg/tools/builtins/filesearch/provider.go`
- Web search execution happens in `pkg/tools/builtins/websearch/provider.go`

## What's Needed

1. Add 17+ new `StreamEventType` constants
2. Modify the `ToolExecutor` interface or add an optional `ToolExecutorWithProgress` interface that emits progress events
3. Update `executeTools()` / `executeToolsSequentially()` in the loop to emit lifecycle events around each tool call
4. For MCP specifically: emit events during `MCPClient.CallTool()` (connect, execute, result)
5. For file_search/web_search: emit events during `FunctionProvider.Execute()`

## Design Considerations

- **Interface extension**: The current `ToolExecutor.Execute()` returns `(ToolResult, error)`. To emit progress events, either:
  - Accept a callback/channel for progress events
  - Return a channel of progress events
  - Use an optional `ToolExecutorWithProgress` interface (checked via type assertion)
- **Streaming vs non-streaming**: Tool lifecycle events only make sense in streaming mode. In non-streaming mode, the client waits for the complete response anyway.
- **Code interpreter**: Antwort delegates code execution to sandbox pods (agent-sandbox). The lifecycle events would come from the sandbox pod's execution progress. This is the most complex case and depends on Spec 011 (Sandbox).

## Phasing

1. **Phase 1**: MCP lifecycle events (most common tool type, 8 events)
2. **Phase 2**: File search + web search events (6 events, simpler since they're HTTP calls)
3. **Phase 3**: Code interpreter events (5 events, depends on sandbox infrastructure)

## Complexity

Medium for MCP and built-in tools. High for code interpreter (depends on sandbox).

## Dependencies

- Spec 004 (Agentic Loop) - tool execution during streaming
- Spec 011 (MCP Client) - MCP tool execution
- Spec 016 (Function Registry) - built-in tool execution
- Spec 017 (Web Search) - web search provider
- Spec 018 (File Search) - file search provider
