# Research: Agentic Loop & Tool Orchestration

**Feature**: 004-agentic-loop
**Date**: 2026-02-18

## R1: Concurrent Tool Execution Pattern

**Decision**: Use `sync.WaitGroup` with a results channel for fan-out/fan-in within a turn.

**Rationale**: The stdlib `sync.WaitGroup` combined with a buffered channel is sufficient for collecting results from concurrent goroutines. No need for `golang.org/x/sync/errgroup` (external dependency). Each tool call spawns a goroutine that sends its result to a shared channel. The main goroutine waits for all to complete via WaitGroup, then reads all results from the channel.

**Alternatives considered**:
- `errgroup` from `golang.org/x/sync`: Violates zero-external-dependency principle for core packages.
- Sequential execution: Simpler but slower when multiple tool calls are independent.
- Worker pool: Over-engineering for the expected concurrency level (1-5 tool calls per turn).

## R2: Engine Constructor Change for Executors

**Decision**: Add an optional `executors` parameter to `engine.New()` using a variadic option pattern or by extending `Config`.

**Rationale**: The current constructor is `New(provider, store, cfg)`. Adding executors as a Config field (`Config.Executors []tools.ToolExecutor`) keeps the constructor signature stable and follows the existing pattern. Nil/empty slice means no executors (single-shot fallback).

**Alternatives considered**:
- Additional constructor parameter: `New(provider, store, executors, cfg)`. Breaking change to existing callers.
- Functional options: `New(provider, store, WithExecutors(...))`. More idiomatic Go but adds complexity for one option.
- Separate registration method: `engine.RegisterExecutor(e)`. Mutable state after construction, less testable.

**Selected**: Extend `Config` with an `Executors` field. Zero-value (nil slice) = no executors.

## R3: Agentic Loop and Streaming Integration

**Decision**: The agentic loop wraps the existing streaming path. For streaming multi-turn, the loop manages the event channel across turns: it starts the provider stream for each turn, consumes events, and when tool calls are detected, it executes tools and starts a new provider stream for the next turn. The response.created/in_progress events happen once before the first turn. The response.completed happens once after the final turn.

**Rationale**: The existing `handleStreaming` method handles a single turn. The agentic loop needs to orchestrate multiple turns. Rather than modifying `handleStreaming` heavily, the loop manages the outer turn cycle and delegates each turn's streaming to the existing infrastructure.

**Alternatives considered**:
- Refactor `handleStreaming` to be loop-aware: Mixes loop concerns into streaming, making both harder to test independently.
- New streaming method per turn: Would require significant refactoring of event state tracking.

## R4: requires_action State Machine Integration

**Decision**: Add `requires_action` as a terminal status in the state machine. Valid transition: `in_progress` -> `requires_action`. No outgoing transitions from `requires_action`.

**Rationale**: A `requires_action` response is complete from the server's perspective. The client creates a new request to continue. This is consistent with how OpenAI handles the `requires_action` status in the Assistants API. The response is stored with its current output (including function_call items), and the follow-up request uses `previous_response_id` to chain.

**Impact on existing code**:
- `pkg/api/types.go`: Add `ResponseStatusRequiresAction ResponseStatus = "requires_action"`
- `pkg/api/state.go`: Add `requires_action` to `in_progress` allowed transitions. Add `requires_action` as terminal (empty allowed transitions).
- `pkg/api/state_test.go`: Add test cases for the new transitions.
- `pkg/transport/http/sse.go`: Add `requires_action` to `terminalEvents` map (if a new event type is used).

## R5: ToolCall/ToolResult vs Existing api.Item Types

**Decision**: `ToolCall` and `ToolResult` are thin types in `pkg/tools/` that map directly to existing `api.Item` fields. They are not a parallel type hierarchy. The loop converts between `ToolCall`/`ToolResult` and `api.Item` at the boundary.

**Rationale**: The `api.Item` type with `ItemTypeFunctionCall` and `ItemTypeFunctionCallOutput` already carries all the data. `ToolCall` and `ToolResult` provide a cleaner interface for executor implementations (they don't need to know about the full Item type). The conversion is straightforward:
- `ToolCall{ID, Name, Arguments}` <- `Item.FunctionCall{CallID, Name, Arguments}`
- `ToolResult{CallID, Output, IsError}` -> `Item.FunctionCallOutput{CallID, Output}` + `is_error` flag

## R6: tool_choice Enforcement Strategy

**Decision**: `tool_choice` is enforced as follows:
- `auto`: No special handling. The model decides.
- `required`: Passed to the provider. If the model doesn't produce tool calls, the response is returned as-is (best-effort).
- `none`: Tools are sent to the provider for context. If the model produces tool calls anyway, they are included in the response but the loop does not execute them. Status is `completed`.
- Forced (specific function): Already validated in Spec 001 (`ValidateRequest`). The provider receives the forced tool_choice. No additional loop-level enforcement needed.

**Rationale**: The model's tool_choice compliance is the provider's responsibility. The engine passes tool_choice to the provider and respects whatever the model returns. The only engine-level enforcement is `tool_choice: "none"` preventing loop entry, which is a safety check.
