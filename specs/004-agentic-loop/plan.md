# Implementation Plan: Agentic Loop & Tool Orchestration

**Branch**: `004-agentic-loop` | **Date**: 2026-02-18 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/004-agentic-loop/spec.md`

## Summary

Implement the agentic inference-tool execution cycle for the Antwort engine. This extends the existing `CreateResponse` flow with a multi-turn loop that dispatches tool calls to pluggable executors, feeds results back to the model, and repeats until a final answer is produced or a termination condition is reached. The spec introduces a `ToolExecutor` interface, `tool_choice`/`allowed_tools` enforcement, the `requires_action` response status, and concurrent tool execution within turns. A prerequisite amendment adds `requires_action` to Spec 001's status enum and state machine.

## Technical Context

**Language/Version**: Go 1.22+ (consistent with Specs 001-003)
**Primary Dependencies**: None (Go standard library only: `context`, `sync`, `log/slog`, `fmt`, `strings`, `time`)
**Storage**: N/A (uses existing `transport.ResponseStore` interface for conversation chaining)
**Testing**: `go test` with table-driven tests + mock executor + mock provider (same patterns as Spec 003)
**Target Platform**: Library packages, cross-platform
**Project Type**: Single Go module (`github.com/rhuss/antwort`)
**Performance Goals**: Tool dispatch overhead negligible relative to inference and tool execution latency; concurrent tool execution within turns
**Constraints**: Zero external dependencies; must extend engine without breaking existing single-shot behavior; nil-safe composition for executors
**Scale/Scope**: ~4 new entities, 31 functional requirements, 2 packages modified + 1 new, ~12 source files

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Interface-First Design | PASS | ToolExecutor (3 methods: Kind, CanExecute, Execute). Within 1-5 limit. |
| II. Zero External Dependencies | PASS | stdlib only. `sync.WaitGroup` or `errgroup`-pattern for concurrency. |
| III. Nil-Safe Composition | PASS | Engine accepts optional executors slice. Nil/empty = single-shot fallback. |
| IV. Typed Error Domain | PASS | Tool errors fed as function_call_output with is_error. Unrecoverable errors use APIError. |
| V. Validate Early, Fail Fast | PASS | allowed_tools validated before tool execution. tool_choice checked before loop entry. |
| VI. Protocol-Agnostic Provider | PASS | Loop calls provider.Complete/Stream. No protocol knowledge in loop. |
| VII. Streaming First-Class | PASS | Multi-turn streaming with single response.created and response.completed. |
| VIII. Context Propagation | PASS | Context passed to all executor calls. Cancellation terminates loop. |
| IX. Kubernetes-Native | N/A | This spec defines interfaces only, no k8s execution. |
| Layer Dependencies | PASS | `pkg/tools` imports `pkg/api` only. `pkg/engine` imports `pkg/tools` + existing deps. |

No violations. All gates pass.

## Project Structure

### Documentation (this feature)

```text
specs/004-agentic-loop/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0: design decisions
├── data-model.md        # Phase 1: entity definitions
├── quickstart.md        # Phase 1: usage guide
└── checklists/
    └── requirements.md  # Spec quality checklist
```

### Source Code (repository root)

```text
pkg/
├── api/
│   ├── types.go                  # AMENDED: Add ResponseStatusRequiresAction
│   └── state.go                  # AMENDED: Add requires_action transitions
│
├── tools/
│   ├── doc.go                    # Package documentation
│   ├── executor.go               # ToolExecutor interface, ToolCall, ToolResult types
│   ├── filter.go                 # allowed_tools filtering logic
│   ├── filter_test.go            # Filter tests
│   └── executor_test.go          # Interface contract tests (with mock)
│
└── engine/
    ├── engine.go                 # MODIFIED: Accept executors, route to agentic loop
    ├── config.go                 # MODIFIED: Add MaxAgenticTurns config
    ├── loop.go                   # Agentic loop: multi-turn inference-tool cycle
    ├── loop_test.go              # Loop orchestration tests (mock executor + mock provider)
    └── engine_test.go            # MODIFIED: Add executor registration and requires_action tests
```

**Structure Decision**: The `pkg/tools/` package is new and contains only types and interfaces. The agentic loop logic lives in `pkg/engine/loop.go` since it extends the engine's `CreateResponse` flow. This keeps tool types separate from engine orchestration while avoiding a circular dependency.

## Complexity Tracking

No constitutional violations. No complexity justifications needed.
