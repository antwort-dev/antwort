# Review Summary: Core Protocol & Data Model

**Feature**: 001-core-protocol
**Date**: 2026-02-16
**Artifacts Reviewed**: spec.md, plan.md, tasks.md, research.md, data-model.md, contracts/openapi.yaml

## Overall Assessment: Pass (9/10)

The plan and task breakdown are strong. Full coverage of all 40 functional requirements and 6 success criteria. Clean dependency graph with legitimate parallel opportunities. One minor red flag around same-file parallelism.

## Coverage Matrix

### Functional Requirements → Tasks

| FR | Description | Task(s) | Status |
|----|-------------|---------|--------|
| FR-001 | Four standard Item types | T008, T009 | Covered |
| FR-002 | Extension Item types | T009, T021 | Covered |
| FR-003 | Item required fields + ID format | T003, T009, T012 | Covered |
| FR-004 | Item status values | T008 | Covered |
| FR-005 | Terminal state immutability | T016 | Covered |
| FR-006 | Multimodal user input | T007 | Covered |
| FR-007 | output_text + summary_text | T007 | Covered |
| FR-007a | Annotations on content | T007 | Covered |
| FR-008 | Asymmetric content schemas | T007 | Covered |
| FR-009 | Message roles | T008 | Covered |
| FR-010 | User/system content | T008 | Covered |
| FR-011 | Assistant output content | T008 | Covered |
| FR-012 | Function call fields | T008 | Covered |
| FR-012a | Function call output fields | T008 | Covered |
| FR-013 | Reasoning fields | T008 | Covered |
| FR-014 | Reasoning fields optional | T008 | Covered |
| FR-015 | Model required | T012 | Covered |
| FR-016 | Input required (min 1) | T012 | Covered |
| FR-017 | Optional request params | T011 | Covered |
| FR-018 | store defaults true | T019 | Covered |
| FR-019 | Truncation values | T012 | Covered |
| FR-020 | tool_choice values + default | T010, T012 | Covered |
| FR-021 | Extensions map | T011 | Covered |
| FR-022 | Response fields + resp_ ID | T003, T011 | Covered |
| FR-023 | Optional response fields | T011 | Covered |
| FR-024 | Response status + queued | T011 | Covered |
| FR-024a | Response state machine | T016 | Covered |
| FR-025 | Response terminal immutability | T016 | Covered |
| FR-026 | Delta events | T015 | Covered |
| FR-027 | State machine events | T015 | Covered |
| FR-028 | Event fields + sequence_number | T015 | Covered |
| FR-028a | Delta context fields | T015 | Covered |
| FR-029 | Extension events | T015 | Covered |
| FR-030 | Error structure | T004 | Covered |
| FR-031 | Error types | T004 | Covered |
| FR-032 | Validation error messages | T004, T012 | Covered |
| FR-033 | Stateless mode | T019 | Covered |
| FR-034 | Stateful mode | T019 | Covered |
| FR-035 | Stateless + previous_response_id | T019 | Covered |
| FR-036 | Validate required fields | T012 | Covered |
| FR-037 | Validate Item types | T012, T021 | Covered |
| FR-038 | Validate tool_choice vs tools | T012 | Covered |
| FR-039 | Enforce state transitions | T016 | Covered |
| FR-040 | Configurable size limits | T012 | Covered |

**Result**: 40/40 FRs covered (100%)

### Success Criteria → Tasks

| SC | Description | Task(s) | Status |
|----|-------------|---------|--------|
| SC-001 | Client compatibility | T013 | Covered |
| SC-002 | Round-trip fidelity | T013 | Covered |
| SC-003 | State machine enforcement | T018 | Covered |
| SC-004 | Validation completeness | T014, T020 | Covered |
| SC-005 | Streaming event coverage | T017 | Covered |
| SC-006 | Extension pass-through | T022 | Covered |

**Result**: 6/6 SCs covered (100%)

### Acceptance Scenarios → Tests

| Story | Scenario | Test Task | Status |
|-------|----------|-----------|--------|
| US1.1 | Valid request accepted | T014 | Covered |
| US1.2 | Missing model rejected | T014 | Covered |
| US1.3 | Mixed item types parsed | T013, T014 | Covered |
| US2.1 | in_progress -> completed | T018 | Covered |
| US2.2 | in_progress -> failed with error | T018 | Covered |
| US2.3 | completed Item immutable | T018 | Covered |
| US2.4 | Event sequence order | T017 | Covered |
| US3.1 | store:false + previous_response_id rejected | T020 | Covered |
| US3.2 | store omitted defaults to true | T020 | Covered |
| US3.3 | Stateless response has no chainable ID | T020 | Covered |
| US4.1 | Extension round-trip | T022 | Covered |
| US4.2 | Extension fields accepted without validation | T022 | Covered |
| US4.3 | Invalid type rejected | T022 | Covered |

**Result**: 13/13 acceptance scenarios covered (100%)

## Red Flag Scan

### RF-1: Same-file parallelism (Low Risk)

**Finding**: T007 and T008 are both marked `[P]` but write to the same file (`pkg/api/types.go`). If executed by parallel agents literally writing simultaneously, this causes conflicts.

**Impact**: Low. In practice, a single developer writes both sequentially. An AI agent would handle them as sequential writes to the same file. The [P] marker is conceptually correct (independent sections) but operationally misleading.

**Recommendation**: Remove [P] from T008, making it depend on T007. Or note in tasks.md that T007/T008 parallelism requires merge coordination.

### RF-2: No explicit `go.sum` or dependency management task (No Risk)

**Finding**: T001 initializes `go mod init` but there are no external dependencies, so `go.sum` will be empty. This is correct for a stdlib-only package.

**Impact**: None. Noted for completeness.

## Task Quality Audit

| Criterion | Status | Notes |
|-----------|--------|-------|
| All tasks have IDs (T001-T026) | Pass | Sequential, no gaps |
| All tasks have checkboxes | Pass | `- [ ]` format |
| All US tasks have [Story] labels | Pass | US1-US4 correctly labeled |
| Setup/Foundation tasks lack [Story] labels | Pass | Correct |
| Polish tasks lack [Story] labels | Pass | Correct |
| All tasks have file paths | Pass | Exact paths provided |
| [P] markers justified | Warn | T007/T008 same-file issue (RF-1) |
| Dependencies documented | Pass | Phase dependency graph included |
| Checkpoints defined | Pass | After each phase |
| MVP scope identified | Pass | Phase 3 (US1) marked as MVP |

## Plan Quality Assessment

| Criterion | Status |
|-----------|--------|
| Technical context complete | Pass |
| Research decisions documented with rationale | Pass (5 decisions in research.md) |
| Data model matches spec entities | Pass |
| OpenAPI contract matches data model | Pass |
| Project structure is minimal and justified | Pass (single pkg/api package) |
| No over-engineering | Pass (zero dependencies, validation functions not library) |
| Complexity tracking empty (no violations) | Pass |

## Cross-Artifact Consistency

| Check | Status |
|-------|--------|
| Spec entity list matches data-model.md entities | Pass |
| Data model fields match OpenAPI schema fields | Pass |
| Task file paths match plan.md structure | Pass |
| Research decisions reflected in task descriptions | Pass |
| Clarification answers integrated into spec | Pass (5 clarifications applied) |
| Edge cases from spec have corresponding validation tasks | Pass |

## Recommendation

**Proceed to implementation.** The plan is comprehensive, the task breakdown covers all requirements, and the dependency structure is sound.

Minor action before starting:
- Consider removing `[P]` from T008 to avoid same-file parallel confusion (RF-1).

## Reviewer Guidance

For spec PR reviewers, focus on:

1. **Data model completeness**: Compare `data-model.md` entity fields against the OpenResponses spec at openresponses.org
2. **OpenAPI contract accuracy**: Verify `contracts/openapi.yaml` schema types, required fields, and enum values match the spec
3. **State machine correctness**: Verify the `queued` -> `in_progress` -> terminal transitions in spec.md FR-024a cover all valid paths
4. **Stateless/stateful boundary**: Confirm FR-033 through FR-035 correctly define the two-tier split and that no stateful features leak into stateless mode
5. **Extension pattern**: Verify `provider:type` naming convention is consistently defined across Items (FR-002), Events (FR-029), and Request/Response extensions (FR-021)
