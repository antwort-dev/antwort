# Brainstorm 19: Annotation and Custom Tool Streaming Events

## Problem

The upstream spec defines events for text annotations (citations from file_search/web_search results) and custom tool input streaming. Antwort produces annotations in the final response but doesn't stream them as they're discovered.

## Missing SSE Events (3)

- `response.output_text.annotation.added` - a citation/annotation was added to the output text
- `response.custom_tool_call.input.delta` - incremental custom tool input
- `response.custom_tool_call.input.done` - custom tool input complete

## Current State

- Annotations are populated in `OutputContentPart.Annotations` by the file search provider
- Custom tools are handled via the `FunctionProvider` interface, which doesn't distinguish "custom" from "built-in"
- There's no streaming of annotation discovery

## What's Needed

1. Emit annotation events when file_search or web_search results produce citations
2. Custom tool events would be for provider-specific tools that aren't standard function calls (e.g., a tool that the provider runs server-side with streamed input)

## Complexity

Low for annotations (emit after tool results are incorporated). The custom tool events are less clear since Antwort treats all tools uniformly via the ToolExecutor interface.

## Priority

Low. Annotations are a polish feature. Custom tool streaming is only relevant if we add provider-specific server-side tools beyond what FunctionProvider covers.
