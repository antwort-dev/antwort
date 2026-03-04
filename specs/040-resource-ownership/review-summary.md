# Review Summary: Resource Ownership (Spec 040)

**Date**: 2026-03-04
**Spec Branch**: `040-resource-ownership`
**Review rounds**: 3 (spec review x2, plan review x1)

## Spec Review Results

### Round 1 (Score: 9/10)
- **Issue 1 (Medium)**: Admin scope ambiguity (instance vs tenant). **Fixed**: FR-005 now scopes admin to own `tenant_id`.
- **Issue 2 (Low)**: Missing admin write-denied scenario. **Fixed**: Added acceptance scenario 2.6.
- **Issue 3 (Low)**: FR-011 vagueness. **Fixed**: Clarified Files API is unchanged.

### Round 2 (Score: 9/10)
- **Issue 1 (Medium)**: Admin Role entity said "instance" not "tenant". **Fixed**.
- **Issue 2 (Medium)**: SC-002 said "in the instance" not "in their tenant". **Fixed**.
- **Issue 3 (Low)**: User Story 2 description said "within the instance". **Fixed**.
- **Issue 4 (Low)**: No edge case for API key admin. **Fixed**: Added edge case.

### Clarification Round (1 question)
- Observability for ownership denials: Debug-level slog logging, no metrics. Added as FR-014.

## Plan Review Results (Score: 9.5/10)

### Coverage: 100%
All 14 functional requirements and 5 success criteria traceable to tasks.

### Gap (Low severity)
SC-002 (admin query performance) has no dedicated test task. Acceptable because owner filtering is a string comparison on existing queries.

### Constitution Alignment: Full pass
All 9 principles pass. Documentation requirement identified (config reference, operations guide, quickstart update).

## Artifacts

| File | Status |
|------|--------|
| spec.md | Complete, reviewed, 14 FRs, 5 SCs, 5 user stories, 6 edge cases |
| plan.md | Complete, constitution check passed, 3 design decisions documented |
| research.md | Complete, 5 research decisions with rationale and alternatives |
| data-model.md | Complete, entity changes and query filtering rules |
| tasks.md | Complete, 45 tasks across 8 phases, beads synced (24 dependencies) |
| review-summary.md | This file |

## Key Design Decisions

1. **Owner context pattern**: Extract from `Identity.Subject` at query time (Files API pattern), not via dedicated context helpers
2. **Admin role storage**: In `Identity.Metadata["roles"]`, extracted from configurable JWT claim path
3. **Authorization in storage layer**: Not middleware or handlers. Follows existing tenant filtering pattern.
4. **Migration**: Empty owner default, matches all when no identity present
5. **Three-level permissions**: Deferred to follow-up spec. This spec is ownership + admin only.

## Reviewer Guidance

When reviewing this spec:
1. **Check FR-005 carefully**: Admin override is scoped to the admin's own `tenant_id`, not instance-wide
2. **Verify backward compatibility**: FR-008 and US3 ensure NoOp auth continues working
3. **Files API is unchanged**: FR-011 explicitly excludes files (already implemented)
4. **Constitution terminology**: Uses updated multi-user/multi-tenant/multi-instance definitions
