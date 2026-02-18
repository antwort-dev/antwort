# Feature Specification: Agentic Loop & Tool Orchestration

**Feature Branch**: `004-agentic-loop`
**Created**: 2026-02-18
**Status**: Draft
**Input**: User description: "Agentic Loop and Tool Orchestration for the Antwort OpenResponses gateway, covering tool type definitions, ToolExecutor interface, tool choice enforcement, allowed_tools filtering, and the core agentic inference-tool execution cycle."

## Overview

This specification defines the agentic loop that transforms antwort from a single-shot inference proxy into an agentic execution engine. The agentic loop orchestrates multi-turn cycles of model inference and tool execution: the model produces tool call requests, the engine delegates execution to pluggable executors, feeds results back to the model, and repeats until the model produces a final answer or a termination condition is reached.

The spec introduces three concepts:

1. **Tool executor interface**: A pluggable contract for tool execution backends. Implementations include client-executed function tools (built-in), MCP server tools (Spec 10), and Kubernetes sandbox tools (Spec 11). Only the interface and the function tool executor are defined here.

2. **Tool choice and filtering**: Enforcement of `tool_choice` (auto/required/none/forced) and `allowed_tools` restrictions, ensuring the model only invokes permitted tools.

3. **Agentic loop**: The core inference-tool cycle that runs inside the engine's `CreateResponse` flow. It handles multi-turn execution for server-side tools (MCP, sandbox) and pauses with `requires_action` status for client-executed function tools.

## Clarifications

### Session 2026-02-18

- Q: Should parallel tool execution be supported? -> A: Yes. When the model returns multiple tool calls in one turn, execute them concurrently. Use fan-out/fan-in to dispatch calls to executors in parallel, collect all results, and feed them back in the next turn.
- Q: How should mixed tool kinds in a single turn be handled? -> A: If any tool call in a turn is client-executed (function kind), the entire turn pauses with `requires_action`. Server-side tools are not executed partially. This avoids the complexity of partial results and ensures the client sees a consistent state.
- Q: Should there be per-tool timeout configuration? -> A: No. Use the request context timeout only. Per-tool timeouts add configuration complexity with little benefit.
- Q: How should tool execution errors be represented? -> A: Feed errors back to the model as `function_call_output` items with `is_error: true`. The model decides whether to retry, try a different approach, or report the error. Only unrecoverable errors (all executors failed, context cancelled) cause the loop itself to fail with `response.failed`.
- Q: Where does the agentic loop live architecturally? -> A: As a private method on Engine, called from `CreateResponse` when tools and executors are present. Not a separate type.
- Q: Where do tool executors get registered? -> A: Constructor injection. The engine constructor accepts an optional slice of executors. If empty/nil, single-shot behavior (tool calls returned as function_call items, same as Spec 003).
- Q: Does `requires_action` exist as a response status? -> A: Not yet. This spec requires adding it to the Response status enum (Spec 001 amendment, similar to how `Input []Item` was added for Spec 003).
- Q: Is `requires_action` a terminal status? -> A: Yes. A response with `requires_action` is complete from the server's perspective. The client must create a NEW request with `previous_response_id` pointing to the `requires_action` response and include `function_call_output` items. The state machine allows transition from `in_progress` to `requires_action`, and `requires_action` is terminal (no outgoing transitions).
- Q: How does the agentic loop work in non-streaming mode? -> A: Identically to streaming, except the engine calls `provider.Complete` in a loop instead of `provider.Stream`. The final `Response` is returned only when the loop completes, containing all accumulated output items (intermediate tool calls, results, and the final answer) in a single response object.
- Q: What happens to tool calls when `tool_choice: "none"` is set? -> A: Tool calls from the model are still included in the response output (they are not stripped). The engine simply does not enter the agentic loop or execute them. The response status is `completed`, not `requires_action`.
- Q: How is usage accumulated across turns? -> A: Usage (input_tokens, output_tokens, total_tokens) is summed across all turns of the agentic loop. The final response contains cumulative usage for the entire multi-turn execution.

## User Scenarios & Testing

### User Story 1 - Single-Turn Function Tool Call (Priority: P1)

A developer sends a request with tool definitions and a prompt that triggers the model to call a tool. The engine detects the tool calls in the response, determines they are function-kind (client-executed), and returns the response with status `completed` containing `function_call` output items. The developer receives the tool call details and can execute the function client-side.

This is the baseline behavior that already works from Spec 003. No agentic loop is involved. The engine returns tool calls directly to the client without attempting to execute them.

**Why this priority**: This is the foundation. It validates that existing single-shot tool call behavior is preserved when no server-side executors are registered.

**Independent Test**: Send a request with tools to an engine with no executors registered, verify the response contains function_call items with status `completed`.

**Acceptance Scenarios**:

1. **Given** an engine with no tool executors registered, **When** a request with tool definitions produces a response with function_call items, **Then** the response has status `completed` and contains the function_call items directly (single-shot behavior)
2. **Given** a request with `tool_choice: "none"`, **When** tools are present in the request, **Then** the model receives the tools for context but the engine does not enter the agentic loop even if the model produces tool calls
3. **Given** a request with `tool_choice: "required"`, **When** the model responds without tool calls, **Then** the engine returns the response as-is (enforcement is advisory, the model's response is respected)

---

### User Story 2 - Multi-Turn Agentic Loop with Server-Side Execution (Priority: P1)

A developer sends a request with tool definitions. The engine has server-side tool executors registered (MCP or sandbox). The model produces tool call items. The engine dispatches each tool call to the appropriate executor, collects results, feeds them back to the model as `function_call_output` items, and calls the model again. This cycle repeats until the model produces a final text answer without tool calls. The client receives the complete response with all output items (including intermediate tool calls and results).

**Why this priority**: This is the core agentic capability. Without the loop, server-side tool execution is not possible.

**Independent Test**: Create a mock executor that returns fixed results. Send a request that triggers tool calls. Verify the engine calls the executor, feeds results back, and the model produces a final answer. Verify the response contains the complete chain of tool calls, results, and final answer.

**Acceptance Scenarios**:

1. **Given** an engine with a mock executor and a provider that first returns a tool call then returns a text answer, **When** a request is submitted, **Then** the engine executes the tool call via the executor, feeds the result back, and returns the final text answer with status `completed`
2. **Given** a 3-turn agentic loop (tool call A, result A, tool call B, result B, final answer), **When** streaming is enabled, **Then** events from all turns are emitted in a single continuous stream with `response.created` at the start and `response.completed` at the end
3. **Given** a provider that returns multiple tool calls in one turn, **When** the engine processes the turn, **Then** it executes all tool calls concurrently, collects results, and feeds them all back in the next inference call
4. **Given** a tool execution that returns an error, **When** the error is fed back as a `function_call_output` with `is_error: true`, **Then** the model receives the error and can decide how to proceed

---

### User Story 3 - Client-Executed Tool Calls with requires_action (Priority: P1)

A developer sends a request with function-type tool definitions. The engine has server-side executors registered but none that can handle function-type tools. The model produces function_call items. The engine detects that these are client-executed tools and returns the response with status `requires_action`. The developer executes the functions and submits a follow-up request with `previous_response_id` and `function_call_output` input items. The engine loads the conversation history, appends the tool results, and continues the inference.

**Why this priority**: Client-executed function calling is the standard OpenAI/OpenResponses pattern and must work correctly alongside the agentic loop.

**Independent Test**: Send a request that triggers function tool calls, verify the response has `requires_action` status. Send a follow-up with results, verify the engine produces a final answer.

**Acceptance Scenarios**:

1. **Given** a request producing function_call items with no matching server-side executor, **When** the engine processes the response, **Then** it returns status `requires_action` with the function_call items in the output
2. **Given** a follow-up request with `previous_response_id` and `function_call_output` input items, **When** the engine processes it, **Then** it reconstructs the conversation (including the original function_call items from the stored response) and sends the complete context to the model
3. **Given** a streaming request that produces function_call items requiring client execution, **When** the stream completes the function_call events, **Then** the terminal event uses `requires_action` status (not `completed`)

---

### User Story 4 - Allowed Tools Filtering (Priority: P2)

A developer sends a request with tool definitions and an `allowed_tools` list that restricts which tools the model may invoke. The engine sends all tools to the model (for context awareness) but validates that the model's tool calls only reference allowed tools. If the model calls a non-allowed tool, the engine rejects the call and feeds an error back to the model.

**Why this priority**: Tool filtering is important for security and control but is not required for basic agentic functionality.

**Independent Test**: Send a request with 3 tools but `allowed_tools` containing only 1. Mock the model to call a non-allowed tool. Verify the engine feeds an error back and does not execute the disallowed tool.

**Acceptance Scenarios**:

1. **Given** a request with tools [A, B, C] and `allowed_tools: ["A"]`, **When** the model calls tool A, **Then** the engine executes tool A normally
2. **Given** a request with tools [A, B, C] and `allowed_tools: ["A"]`, **When** the model calls tool B, **Then** the engine does not execute tool B and feeds an error result back to the model stating the tool is not allowed
3. **Given** a request with `allowed_tools` empty (or not set), **When** the model calls any tool, **Then** all tools are allowed (no restriction)

---

### User Story 5 - Safety Limits and Loop Termination (Priority: P2)

A developer sends a request that triggers an agentic loop. The loop has a configurable maximum turns limit (default: 10). If the model keeps producing tool calls without reaching a final answer, the loop terminates after the limit and returns an `incomplete` response with a descriptive reason.

**Why this priority**: Safety limits prevent runaway loops but are secondary to core agentic functionality.

**Independent Test**: Configure a max turns of 2. Mock the model to always produce tool calls. Verify the loop terminates after 2 turns with an `incomplete` response.

**Acceptance Scenarios**:

1. **Given** a max turns limit of 2, **When** the model produces tool calls on both turns without a final answer, **Then** the engine terminates the loop and returns status `incomplete` with the tool call results accumulated so far
2. **Given** a streaming agentic loop where the context is cancelled mid-turn, **When** the engine detects cancellation, **Then** it emits `response.cancelled` and stops the loop
3. **Given** a provider error during an intermediate turn of the loop, **When** the engine catches the error, **Then** it emits `response.failed` with the error details

---

### Edge Cases

- What happens when a tool executor is registered but `CanExecute` returns false for all tools in the request? The engine treats tool calls as client-executed (function behavior), returning them to the client.
- What happens when the model returns a tool call referencing a tool not in the request's tool list? The engine feeds an error back to the model as a `function_call_output` with `is_error: true`.
- What happens when all tool calls in a turn fail with errors? The errors are fed back to the model as `function_call_output` items with `is_error: true`. The model receives the errors and can decide to retry or give up. The loop does not terminate on tool errors alone.
- What happens when the model returns both text content and tool calls in the same response? Both are included in the output. The tool calls trigger the agentic loop (or `requires_action` for function tools). Text content is accumulated alongside tool results.
- What happens when `tool_choice: "required"` is set but the model returns no tool calls? The engine respects the model's response. Enforcement is best-effort since the engine cannot force the model to produce tool calls.
- What happens when `previous_response_id` points to a response with `requires_action` status but the follow-up request does not include `function_call_output` items? The engine reconstructs the conversation and sends it to the model. The model may call the same tools again or produce a different response.

## Requirements

### Functional Requirements

**Tool Executor Interface**

- **FR-001**: The system MUST define a tool executor interface with operations for checking executability and executing tool calls
- **FR-002**: The executor interface MUST support multiple executor implementations behind the same contract. At minimum, a function-tool executor (returns control to client) and a mock executor (for testing) MUST be implementable without interface changes.
- **FR-003**: The executor interface MUST allow each executor to declare which tools it can handle via a capability check operation
- **FR-004**: Tool executors MUST be registered via the engine constructor. When no executors are provided, the engine MUST fall back to single-shot behavior (returning tool calls to the client as function_call items)

**Tool Choice Enforcement**

- **FR-005**: The system MUST enforce `tool_choice` values: "auto" (model decides), "required" (model should call tools), "none" (tools visible for context but not callable), and forced (specific tool must be called)
- **FR-006**: When `tool_choice: "none"` is set, the engine MUST still send tool definitions to the provider for context but MUST NOT enter the agentic loop or execute any tool calls. Tool calls produced by the model are included in the response output as-is. The response status is `completed`.
- **FR-007**: When a forced `tool_choice` specifies a tool name, the engine MUST validate that the tool exists in the request's tool list (already implemented in Spec 001 validation)

**Allowed Tools Filtering**

- **FR-008**: When `allowed_tools` is set, the engine MUST send all tool definitions to the provider (for context) but MUST validate that the model's tool calls only reference tools in the allowed list
- **FR-009**: When the model calls a tool not in the `allowed_tools` list, the engine MUST NOT execute the tool. Instead, it MUST feed an error result back to the model as a `function_call_output` with `is_error: true`
- **FR-010**: When `allowed_tools` is empty or not set, all tools in the request are allowed (no filtering)

**Agentic Loop**

- **FR-011**: When tool executors are registered and the model produces tool call items, the engine MUST enter the agentic loop: execute tool calls, feed results back, and re-invoke the model
- **FR-012**: The agentic loop MUST terminate when the model produces a response with no tool calls (final answer)
- **FR-013**: The agentic loop MUST terminate when the maximum turns limit is reached, returning the response with status `incomplete`
- **FR-014**: The agentic loop MUST terminate when the request context is cancelled, emitting `response.cancelled`
- **FR-015**: The maximum turns limit MUST be configurable with a default of 10
- **FR-016**: When multiple tool calls are produced in a single turn, the engine MUST execute them concurrently and collect all results before proceeding to the next inference call
- **FR-017**: Tool execution results MUST be appended to the conversation as `function_call_output` items with the corresponding `call_id` from the original `function_call`

**Client-Executed Function Tools**

- **FR-018**: When the model produces tool calls that no registered executor can handle (all executors return false from capability check), the engine MUST treat them as client-executed function calls
- **FR-019**: For client-executed function calls, the engine MUST return the response with status `requires_action` and include the function_call items in the output
- **FR-020**: The `requires_action` status MUST be added to the response status enum (Spec 001 amendment). It MUST be a terminal status (no outgoing transitions). The state machine MUST allow transition from `in_progress` to `requires_action`.
- **FR-021**: When a follow-up request includes `previous_response_id` referencing a `requires_action` response, the engine MUST reconstruct the conversation history (including the function_call items) and continue processing with the client-provided `function_call_output` items

**Streaming**

- **FR-022**: During a multi-turn agentic loop with streaming enabled, the engine MUST emit a single continuous stream of events across all turns
- **FR-023**: The `response.created` and `response.in_progress` events MUST be emitted exactly once at the start of the stream
- **FR-024**: The terminal event (`response.completed`, `response.requires_action`, `response.failed`, or `response.cancelled`) MUST be emitted exactly once at the end of the stream
- **FR-025**: Intermediate tool execution (between turns) MUST NOT produce separate response lifecycle events. Tool call and result items are emitted as `output_item.added` / `output_item.done` events within the continuous stream.
- **FR-026**: For client-executed function tools in streaming mode, the terminal event MUST use `requires_action` status in the response payload

**Non-Streaming Loop**

- **FR-027**: In non-streaming mode, the agentic loop MUST call `provider.Complete` for each turn and accumulate all output items (tool calls, tool results, and the final answer) into a single Response returned at the end of the loop
- **FR-028**: Usage statistics MUST be summed across all turns of the agentic loop. The final Response MUST contain cumulative input_tokens, output_tokens, and total_tokens for the entire multi-turn execution.

**Error Handling**

- **FR-029**: Tool execution errors MUST be fed back to the model as `function_call_output` items with `is_error: true` and the error message as the output content
- **FR-030**: Only unrecoverable errors (provider failure, context cancellation, all executors failed for a required tool) MUST cause the loop to terminate with `response.failed`
- **FR-031**: When a tool executor returns an error, the engine MUST log the error details and continue the loop (resilient execution)

### Key Entities

- **ToolExecutor**: A pluggable backend for tool execution. Declares which tools it can handle and executes tool calls. Implementations exist for function tools (built-in, returns to client), MCP (Spec 10), and sandbox (Spec 11).
- **ToolCall**: A model's request to invoke a tool. Contains the call ID, tool name, and JSON arguments string.
- **ToolResult**: The output of a tool execution. Contains the call ID, output content, and an error flag.
- **AgenticTurn**: A single cycle of the agentic loop: one inference call followed by zero or more tool executions. Not a formal type, but a logical unit tracked by the loop for turn counting.

## Success Criteria

### Measurable Outcomes

- **SC-001**: A multi-turn agentic loop (2+ turns) completes successfully, producing a final response that includes all intermediate tool call items, result items, and the final answer text
- **SC-002**: Multiple tool calls in a single turn are executed concurrently, with all results fed back before the next inference call
- **SC-003**: A streaming agentic loop emits the complete OpenResponses event sequence with `response.created` exactly once at the start and `response.completed` exactly once at the end, regardless of the number of turns
- **SC-004**: Client-executed function tools produce `requires_action` status, and a follow-up request with results continues the conversation correctly
- **SC-005**: The `allowed_tools` filter prevents execution of non-allowed tools and feeds clear error messages back to the model
- **SC-006**: The safety turn limit terminates runaway loops with `incomplete` status
- **SC-007**: Tool execution errors are gracefully fed back to the model without terminating the loop, allowing the model to recover
- **SC-008**: An engine with no executors registered preserves existing single-shot behavior (tool calls returned as function_call items with `completed` status)
- **SC-009**: The tool executor interface supports at least two implementations (function executor and mock executor) without interface changes

## Assumptions

- The `requires_action` response status will be added to `pkg/api/types.go` as a backwards-compatible amendment to Spec 001, following the same pattern as the `Input []Item` addition for Spec 003. It is a terminal status in the state machine (like `completed` or `failed`).
- The state machine in `pkg/api/state.go` will be amended to allow `in_progress` -> `requires_action` as a valid transition, and `requires_action` will have no outgoing transitions.
- The conversation history reconstruction from Spec 003 (Phase 6, `history.go`) correctly handles responses with `requires_action` status, reconstructing function_call items from the stored response output.
- Tool definitions from the request are already translated to provider format by Spec 003's `translateRequest`. The agentic loop reuses this translation.
- The engine's existing streaming infrastructure (events.go, stream state tracking) is extended for multi-turn streaming, not replaced.
- Server-side tool executors (MCP, sandbox) are not implemented in this spec. Only the interface and the function-tool executor are delivered. The agentic loop is tested with mock executors.
- In non-streaming mode, the agentic loop calls `provider.Complete` per turn (not `provider.Stream`). The complete response is returned only after the loop terminates.
- Usage is cumulative across all turns. Each turn's usage is added to the running total.

## Dependencies

- **Spec 001 (Core Protocol)**: All Item, Response, StreamEvent, Error, and Usage types. **Requires amendment**: Add `requires_action` to `ResponseStatus`.
- **Spec 003 (Core Engine)**: The engine structure, `CreateResponse` flow, streaming infrastructure, conversation history reconstruction, and provider abstraction.

## Scope Boundaries

### In Scope

- Tool executor interface definition
- Function-tool executor (returns to client with `requires_action`)
- `tool_choice` enforcement (auto, required, none, forced)
- `allowed_tools` filtering and rejection
- Agentic loop orchestration (multi-turn inference-tool cycle)
- Turn management and termination conditions
- Concurrent tool execution within a turn
- Streaming across multiple agentic turns
- Maximum turns safety limit
- Error feeding (tool errors back to model)
- Integration with engine's `CreateResponse` flow
- `requires_action` response status amendment

### Out of Scope

- MCP client and executor implementation (Spec 10)
- Kubernetes sandbox executor implementation (Spec 11)
- Specific tool implementations (file search, code interpreter, web search)
- Provider-specific tool format translation (handled by Spec 03 Translator)
- Tool result validation or sanitization
- Tool execution retry policies (beyond feeding errors back to the model)
- Tool execution metrics or observability (Spec 07)
