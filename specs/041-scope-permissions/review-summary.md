# Review Summary: Scope-Based Authorization and Resource Permissions (Spec 041)

**Date**: 2026-03-04
**Spec Branch**: `041-scope-permissions`
**Review rounds**: 2 (spec review x1, plan review x1)

## Spec Review Results

### Round 1 (Score: 9/10)
- **Issue 1 (Medium)**: Missing file endpoint scopes. **Fixed**: Added FR-018 covering file endpoints.
- **Issue 2 (Low)**: Owner self-denial (`---|---|---`). **Fixed**: Owner level is always `rwd`, immutable.
- **Issue 3 (Low)**: Missing permission update scenario. **Fixed**: Added acceptance scenario 2.8.

## Plan Review Results (Score: 9.5/10)

### Coverage: 98%
17 of 18 FRs and all 6 SCs traceable to tasks. One low gap (FR-013 no explicit negative test for responses/conversations rejecting permissions).

### Task Breakdown
- 51 total tasks across 8 phases
- 3 unit test tasks, 20 integration test tasks
- 14 parallel opportunities identified
- Beads: 51 issues created, 30 dependencies mapped

### Constitution Alignment: Full pass
All applicable principles pass. Documentation requirement identified.

## Artifacts

| File | Status |
|------|--------|
| spec.md | Complete, 18 FRs, 6 SCs, 5 user stories, 8 edge cases |
| plan.md | Complete, constitution check passed, 5 design decisions |
| research.md | Complete, 5 research decisions with rationale |
| data-model.md | Complete, entities, permission rules, endpoint-scope map |
| tasks.md | Complete, 51 tasks, beads synced |
| review-summary.md | This file |

## Key Design Decisions

1. **Scope sources**: Union of JWT scopes + role-expanded scopes
2. **Scope denial**: 403 Forbidden (not 404). Distinct from ownership denial.
3. **Role hierarchy**: Reference-based, resolved at startup, cycle detection
4. **Endpoint-scope mapping**: Hardcoded in code, not configurable
5. **Permissions format**: Input as JSON object, output as compact string, owner always `rwd`
6. **Shareable resources**: Vector stores + files only. Responses/conversations stay private.
7. **Vector store merge**: Union with agent profile defaults

## Reviewer Guidance

When reviewing this spec:
1. **Two separate features**: Scope enforcement (US1) and permissions/sharing (US2-US3) are independent. Can be reviewed separately.
2. **Check FR-005 vs FR-010 (Spec 040)**: Scope denial returns 403. Ownership denial returns 404. These are intentionally different.
3. **Owner level immutable**: FR-010 specifies owner is always `rwd`. Attempts to change it are ignored.
4. **Backward compatibility**: FR-006 ensures unconfigured deployments work unchanged. US5 tests this.
5. **Files and vector stores coupled**: US3 explains why both need sharing (broken citations otherwise).
