# Feature Specification: Tool Lifecycle SSE Events

**Feature Branch**: `023-tool-lifecycle-events`
**Created**: 2026-02-24
**Status**: Draft

## Overview

During the agentic loop, the gateway executes tools (MCP servers, file search, web search) on behalf of the model, but clients have no visibility into tool execution progress. They only see the final `function_call_output` item after execution completes. This specification adds lifecycle events emitted around each tool execution so clients can display progress indicators like "searching...", "executing MCP tool...", or "search complete".

These events are emitted by the engine around tool execution calls without modifying the existing tool executor interface. The engine already knows which tools it dispatches and can wrap each call with before/after events.

## User Scenarios & Testing

### User Story 1 - Tool Execution Progress (Priority: P1)

A developer runs a streaming agentic request. The model calls a tool (e.g., web_search). Before the tool executes, the client receives an `in_progress` event. After execution completes, the client receives a `completed` event. If execution fails, the client receives a `failed` event. The client uses these to show a spinner or progress indicator.

**Why this priority**: Without progress events, tool execution creates dead time in the stream. The client sees the model's function_call but then silence until the result appears. For slow tools (web search, file search), this can last several seconds.

**Independent Test**: Send a streaming agentic request with tools, verify lifecycle events are emitted around tool execution.

**Acceptance Scenarios**:

1. **Given** a streaming agentic request with tools, **When** the model calls a tool, **Then** the client receives a tool-specific `in_progress` event before execution starts
2. **Given** a tool executing during streaming, **When** the execution completes successfully, **Then** the client receives a `completed` event
3. **Given** a tool executing during streaming, **When** the execution fails, **Then** the client receives a `failed` event
4. **Given** a streaming request without tools, **When** the response completes, **Then** no tool lifecycle events are emitted (backward compatible)

---

### User Story 2 - Search Progress (Priority: P1)

A developer runs a streaming request that triggers file_search or web_search. The client receives a `searching` event while the search is in progress, providing more specific feedback than a generic `in_progress`.

**Why this priority**: Search operations are the most common built-in tools and typically take the longest. The `searching` event lets clients display "Searching the web..." or "Searching files..." instead of a generic spinner.

**Independent Test**: Send a streaming request that triggers a search tool, verify the `searching` event is emitted.

**Acceptance Scenarios**:

1. **Given** a streaming request triggering web_search, **When** the search executes, **Then** the client receives `web_search_call.in_progress`, `web_search_call.searching`, and `web_search_call.completed` events
2. **Given** a streaming request triggering file_search, **When** the search executes, **Then** the client receives `file_search_call.in_progress`, `file_search_call.searching`, and `file_search_call.completed` events

---

### User Story 3 - MCP Tool Progress (Priority: P1)

A developer runs a streaming request that calls an MCP tool. The client receives MCP-specific lifecycle events showing the tool name and execution status.

**Why this priority**: MCP tools are remote calls to external servers. Network latency makes progress visibility essential.

**Independent Test**: Send a streaming request that calls an MCP tool, verify MCP lifecycle events are emitted.

**Acceptance Scenarios**:

1. **Given** a streaming request calling an MCP tool, **When** the tool executes, **Then** the client receives `mcp_call.in_progress` and `mcp_call.completed` events
2. **Given** a streaming request calling an MCP tool that fails, **When** execution errors, **Then** the client receives `mcp_call.failed`

---

### Edge Cases

- What happens when multiple tools execute concurrently? Each tool gets its own lifecycle events with distinct item IDs.
- What happens when `parallel_tool_calls` is false? Tools execute sequentially, and lifecycle events are emitted in order.
- What happens for non-streaming requests? No lifecycle events are emitted (non-streaming clients get the complete response at once).
- What happens for tools that are not MCP, file_search, or web_search? They get generic function call lifecycle events (the existing `output_item.added`/`output_item.done` pattern).

## Requirements

### Functional Requirements

**Tool Lifecycle Events**

- **FR-001**: The system MUST emit tool-specific lifecycle events during streaming agentic tool execution
- **FR-002**: For MCP tools, the system MUST emit `response.mcp_call.in_progress` before execution and `response.mcp_call.completed` or `response.mcp_call.failed` after
- **FR-003**: For file_search tools, the system MUST emit `response.file_search_call.in_progress`, `response.file_search_call.searching`, and `response.file_search_call.completed`
- **FR-004**: For web_search tools, the system MUST emit `response.web_search_call.in_progress`, `response.web_search_call.searching`, and `response.web_search_call.completed`
- **FR-005**: Tool lifecycle events MUST include the tool name and output index

**Event Ordering**

- **FR-006**: The `in_progress` event MUST be emitted before the tool execution begins
- **FR-007**: The `completed` or `failed` event MUST be emitted after the tool execution finishes
- **FR-008**: Tool lifecycle events MUST have monotonically increasing sequence numbers consistent with other events in the stream

**Backward Compatibility**

- **FR-009**: Non-streaming requests MUST NOT be affected by tool lifecycle events
- **FR-010**: Streaming requests without tools MUST NOT emit any tool lifecycle events
- **FR-011**: The existing `output_item.added`/`output_item.done` events for tool results MUST remain unchanged

**Spec Alignment**

- **FR-012**: The OpenAPI specification MUST be updated to include the new tool lifecycle event types
- **FR-013**: The SSE event count MUST increase from 21 to at least 30

## Success Criteria

- **SC-001**: Streaming agentic requests emit tool lifecycle events that clients can use for progress indicators
- **SC-002**: All existing tests continue to pass with zero regressions
- **SC-003**: The SSE event count reaches at least 30 (adding 9+ tool lifecycle events)
- **SC-004**: Tool lifecycle events appear in the correct order (in_progress before completed/failed)

## Assumptions

- The engine identifies tool types by matching tool names against known patterns: tools handled by the MCP executor get MCP events, tools named `file_search` or `web_search` get search-specific events, all others get generic events.
- Tool lifecycle events are emitted at the engine level (wrapping tool executor calls), not inside the executors themselves. This avoids interface changes.
- The `searching` event is emitted immediately after `in_progress` for search tools (before the actual search call), since the engine can't observe mid-search progress without interface changes.
- MCP tool discovery lifecycle events (`mcp_list_tools.*`) are deferred. Discovery happens lazily at first use and is typically fast.

## Dependencies

- **Spec 004 (Agentic Loop)**: Streaming tool execution
- **Spec 011 (MCP Client)**: MCP tool execution
- **Spec 016 (Function Registry)**: Built-in tool execution
- **Spec 022 (Terminal Events)**: Event type infrastructure

## Scope Boundaries

### In Scope

- Emitting lifecycle events around tool execution in the streaming agentic loop
- MCP tool events (in_progress, completed, failed)
- File search events (in_progress, searching, completed)
- Web search events (in_progress, searching, completed)
- Updating the OpenAPI spec with new event types
- Integration tests with mock tools

### Out of Scope

- MCP argument streaming events (`mcp_call_arguments.delta/done`), deferred (requires interface changes)
- MCP tool discovery events (`mcp_list_tools.*`), deferred (lazy discovery is fast)
- Code interpreter events, deferred (requires sandbox infrastructure)
- Modifying the ToolExecutor interface
- Tool lifecycle events for non-streaming requests
