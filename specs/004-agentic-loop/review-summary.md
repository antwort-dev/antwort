# Plan Review Summary: Agentic Loop & Tool Orchestration

**Feature**: 004-agentic-loop
**Review Date**: 2026-02-18
**Artifacts Reviewed**: spec.md, plan.md, tasks.md, research.md, data-model.md, quickstart.md

## Coverage Matrix

### FR-to-Task Mapping

| FR | Description | Task(s) | Coverage |
|----|-------------|---------|----------|
| FR-001 | ToolExecutor interface definition | T004 | Full |
| FR-002 | Multiple executor implementations | T004 | Full |
| FR-003 | CanExecute capability check | T004 | Full |
| FR-004 | Constructor injection, nil-safe fallback | T006, T007 | Full |
| FR-005 | tool_choice enforcement (auto/required/none/forced) | T009 | Full |
| FR-006 | tool_choice "none" prevents loop entry | T009 | Full |
| FR-007 | Forced tool_choice validation | (existing Spec 001) | Already done |
| FR-008 | allowed_tools sends all tools, validates calls | T005, T021 | Full |
| FR-009 | Non-allowed tool rejected with error | T005, T021 | Full |
| FR-010 | Empty allowed_tools = all allowed | T005, T022 | Full |
| FR-011 | Agentic loop entry condition | T010, T013 | Full |
| FR-012 | Loop terminates on final answer | T010, T015 | Full |
| FR-013 | Loop terminates on max turns | T010, T023 | Full |
| FR-014 | Loop terminates on context cancellation | T010, T024 | Full |
| FR-015 | Max turns configurable, default 10 | T006 | Full |
| FR-016 | Concurrent tool execution | T011 | Full |
| FR-017 | Tool results appended with call_id | T010, T012 | Full |
| FR-018 | Unhandled tools = client-executed | T017 | Full |
| FR-019 | requires_action status for function tools | T017 | Full |
| FR-020 | requires_action status amendment | T001, T002 | Full |
| FR-021 | Follow-up with previous_response_id | T020 | Full |
| FR-022 | Single continuous stream across turns | T014, T016 | Full |
| FR-023 | response.created/in_progress once at start | T014, T016 | Full |
| FR-024 | Terminal event once at end | T014, T019 | Full |
| FR-025 | No lifecycle events between turns | T014 | Full |
| FR-026 | Streaming requires_action terminal event | T019 | Full |
| FR-027 | Non-streaming loop uses Complete per turn | T010 | Full |
| FR-028 | Cumulative usage across turns | T010 | Full |
| FR-029 | Tool errors fed back with is_error | T012, T015 | Full |
| FR-030 | Unrecoverable errors terminate loop | T010, T025 | Full |
| FR-031 | Resilient execution, log and continue | T012 | Full |

**Coverage: 31/31 FRs mapped to tasks (100%)**

### SC-to-Task Mapping

| SC | Description | Task(s) | Coverage |
|----|-------------|---------|----------|
| SC-001 | Multi-turn loop completes with all items | T015 | Full |
| SC-002 | Concurrent tool calls in single turn | T011, T015 | Full |
| SC-003 | Streaming across turns, single lifecycle | T016 | Full |
| SC-004 | requires_action + follow-up | T020 | Full |
| SC-005 | allowed_tools filtering | T022 | Full |
| SC-006 | Turn limit produces incomplete | T023 | Full |
| SC-007 | Tool errors fed back gracefully | T015 | Full |
| SC-008 | No executors = single-shot behavior | T008 | Full |
| SC-009 | Two executor implementations | T004 | Full |

**Coverage: 9/9 SCs mapped to tasks (100%)**

### Edge Case Coverage

| Edge Case | Task | Coverage |
|-----------|------|----------|
| CanExecute false for all tools | T026 | Full |
| Unknown tool call (not in request) | T027 | Full |
| Mixed text + tool calls | T028 | Full |
| tool_choice "required" no tool calls | T008 | Full |
| All tool calls fail with errors | T015 | Full |
| requires_action follow-up without outputs | (covered by existing history.go) | Implicit |

## Red Flag Scan

### Potential Issues

**1. Task T010 is very large (Severity: Low)**

T010 covers the entire non-streaming agentic loop in a single task, including the main loop, turn counting, result accumulation, usage aggregation, and termination conditions. This is a lot of logic for one task.

**Mitigation**: T011 (concurrent execution) and T012 (executor dispatch) factor out the two most complex subsystems. T010 handles the orchestration shell. The task is large but cohesive, and splitting further would create artificial boundaries.

**Verdict**: Acceptable. Monitor during implementation.

**2. Streaming multi-turn (T014) complexity (Severity: Low)**

The streaming agentic loop needs to manage event state across multiple provider.Stream calls. This requires careful coordination with the existing streamState, sequence numbers, and lifecycle events.

**Mitigation**: Research R3 addresses this directly with a clear architectural decision (loop wraps streaming, delegates per-turn to existing infrastructure). Tests in T016 verify the event sequence.

**Verdict**: Acceptable. Well-researched.

**3. No explicit `EventResponseRequiresAction` stream event type in tasks (Severity: Low)**

The spec mentions streaming `requires_action` terminal events (FR-026), and T001 adds `requires_action` to the SSE terminal events map. However, there's no explicit `EventResponseRequiresAction` constant added. The implementation will likely use `EventResponseCompleted` with a `requires_action` status in the response payload, which is what T019 describes.

**Verdict**: Acceptable. The approach (status in response payload rather than separate event type) is simpler and consistent.

## Task Quality Assessment

| Criterion | Status | Notes |
|-----------|--------|-------|
| All tasks have IDs | PASS | T001-T030 sequential |
| All tasks have file paths | PASS | Every task references specific Go files |
| All US tasks have story labels | PASS | [US1]-[US5] correctly applied |
| Parallel markers appropriate | PASS | [P] only on independent tasks |
| Phase dependencies documented | PASS | Clear dependency graph |
| Checkpoints after each phase | PASS | Every phase has a checkpoint |
| Tests included with implementation | PASS | Go convention (test alongside source) |
| FR traceability in task descriptions | PASS | FR references in parentheses |
| Beads IDs assigned | PASS | All tasks have beads IDs |

## Constitution Compliance

| Principle | Plan Status | Notes |
|-----------|-------------|-------|
| I. Interface-First | PASS | T004 defines ToolExecutor (3 methods) |
| II. Zero Dependencies | PASS | stdlib only, sync.WaitGroup for concurrency |
| III. Nil-Safe Composition | PASS | T006/T007: nil executors = single-shot |
| IV. Typed Error Domain | PASS | Tool errors via is_error on function_call_output |
| V. Validate Early | PASS | T005: allowed_tools filter before execution |
| VI. Protocol-Agnostic | PASS | Loop uses provider.Complete/Stream only |
| VII. Streaming First-Class | PASS | T014/T016: multi-turn streaming |
| VIII. Context Propagation | PASS | T024: context cancellation terminates loop |
| IX. Kubernetes-Native | N/A | Interface only, no k8s execution |
| Layer Dependencies | PASS | pkg/tools -> api only; engine -> tools + existing |

## Overall Assessment

**Score: 97%** (1 minor observation, no blocking issues)

### Strengths

- Complete FR coverage (31/31)
- Complete SC coverage (9/9)
- Clear phase structure with checkpoints
- Well-documented dependency graph
- Research decisions are thorough (6 decisions with alternatives)
- Parallel opportunities identified
- Edge cases covered in dedicated phase
- Beads integration complete (30 tasks synced)

### One Observation

The plan could benefit from a quickstart validation task that specifically tests the code examples against the actual API (T030 mentions this but is vague). During implementation, the quickstart.md examples should be compiled as Go test code to verify they work.

### Recommendation

**Ready for implementation.** No blocking issues. The plan is comprehensive, well-structured, and fully covers the specification.

## Dependency Graph

```
Phase 1 (Setup: T001-T003)
  └── Phase 2 (Foundational: T004-T007)
       ├── Phase 3 (US1: T008-T009)
       └── Phase 4 (US2: T010-T016) ←── Core
            ├── Phase 5 (US3: T017-T020)
            │    └── Phase 8 (Polish: T026-T030)
            ├── Phase 6 (US4: T021-T022)
            └── Phase 7 (US5: T023-T025)
```
