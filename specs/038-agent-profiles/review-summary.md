# Review Summary: 038-agent-profiles

**Reviewed**: 2026-03-03 | **Verdict**: APPROVED - Ready for implementation | **Spec Version**: Draft

## For Reviewers

This spec adds server-side agent profiles to antwort. Profiles are named configuration bundles (model, instructions with `{{variable}}` templates, tools, constraints) loaded from `config.yaml`. The `agent` field and OpenAI `prompt` parameter on create response resolve profiles by name. A new `pkg/agent/` package isolates all profile logic.

### Key Areas to Review

1. **Merge semantics (FR-008/FR-009)**: Request fields override profile defaults. Tools are merged (union, not replacement). This is well-defined in US3 with 5 acceptance scenarios.

2. **Prompt parameter mapping (FR-005/FR-006)**: `prompt.id` maps to profile name. `prompt.variables` become template substitutions. `prompt.version` accepted but ignored in v1. Provides OpenAI SDK forward compatibility.

3. **Template substitution (FR-011/FR-013)**: Simple `{{variable}}` string replacement. Undefined variables left as literal text. No template engine dependency (stdlib only).

4. **Config-only in v1**: No runtime CRUD API for profiles. Profiles are managed via config file (ConfigMap in K8s). This keeps scope manageable. API-managed profiles and versioning can be added in a future spec.

## Coverage Matrix

| FR | Description | Task(s) | Status |
|----|-------------|---------|--------|
| FR-001 | Profile definition in config | T001, T003, T006 | COVERED |
| FR-002 | `agent` field on request | T002, T010 | COVERED |
| FR-003 | Profile resolution + merge | T005, T010 | COVERED |
| FR-004 | Unknown profile error | T006, T009, T012 | COVERED |
| FR-005 | `prompt` parameter | T002, T013 | COVERED |
| FR-006 | Prompt resolves profile + vars | T004, T013 | COVERED |
| FR-007 | OpenAI SDK compatibility | T014 | COVERED |
| FR-008 | Request overrides profile | T005, T008, T015 | COVERED |
| FR-009 | Tool union | T005, T008, T015 | COVERED |
| FR-010 | No agent/prompt = unchanged | T015 | COVERED |
| FR-011 | `{{variable}}` syntax | T004, T007 | COVERED |
| FR-012 | Variable substitution | T004, T007 | COVERED |
| FR-013 | Undefined vars as literal | T007 | COVERED |
| FR-014 | List profiles endpoint | T016, T017 | COVERED |
| FR-015 | Summary only in list | T016, T018 | COVERED |
| FR-016 | Config file `agents` section | T003, T011 | COVERED |
| FR-017 | Unique profile names | T006, T011 | COVERED |
| FR-018 | Startup validation | T006, T009, T011 | COVERED |
| FR-019 | API reference docs | T019 | COVERED |
| FR-020 | Tutorial docs | T021 | COVERED |
| FR-021 | Developer docs | T022 | COVERED |

**Coverage**: 21/21 FRs covered. All 6 success criteria verifiable.

## Task Summary

| Phase | Story | Tasks | Tests | Parallel |
|-------|-------|-------|-------|----------|
| 1. Setup | - | 3 | 0 | 3 parallel |
| 2. Foundational | - | 3 | 3 | 2 parallel |
| 3. US1 (P1) | Define+Use | 3 | 0 | Sequential |
| 4. US2 (P1) | Prompt param | 1 | 1 | Sequential |
| 5. US3 (P1) | Merge logic | 0 | 1 | Sequential |
| 6. US4 (P2) | List profiles | 2 | 1 | Sequential |
| 7. Docs | - | 5 | 0 | 4 parallel |
| 8. Polish | - | 2 | 0 | Sequential |
| **Total** | | **19** | **6** | **25 total** |

## Key Strengths

- Clean separation: all agent logic in `pkg/agent/`, no pollution of engine internals
- ProfileResolver interface (1 method) enables future CRD-based resolver without changing the engine
- Prompt parameter is a thin adapter on profile resolution, no separate infrastructure
- Config-only in v1 keeps scope tight while delivering full user value

## Risks

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Config reload needed for profile changes | Expected | Document clearly, hot-reload in future spec |
| Template injection via variables | Low | Variables substitute into instructions only, not into tool configs |
| Prompt.version ignored silently | Low | Accept but log warning, add versioning in future spec |

## Red Flags

None.

## Reviewer Guidance

When reviewing the implementation PR:
1. Verify ProfileResolver is injected into engine as nil-safe optional
2. Confirm merge logic: request fields override, tools union
3. Check template substitution handles edge cases (empty vars, no `{{}}`, nested `{{}}`)
4. Verify `GET /v1/agents` does not expose full instructions (security)
5. Run `go test ./pkg/agent/...` to verify all profile tests pass
