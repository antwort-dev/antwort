# Review Summary: 038-agent-profiles

**Reviewed**: 2026-03-03 | **Verdict**: PASS | **Spec Version**: Draft

## For Reviewers

This spec adds server-side agent profiles to antwort. Profiles are named configuration bundles (model, instructions with `{{variable}}` templates, tools, constraints) loaded from the config file. The `agent` field on create response references profiles by name. The OpenAI `prompt` parameter maps to profiles as a compatibility shim, enabling standard SDK usage.

### Key Areas to Review

1. **Merge semantics (FR-008/FR-009)**: Request fields override profile defaults. Tools are merged (union). This is the most important behavioral contract, well-defined in US3.

2. **Prompt parameter compatibility (FR-005/FR-007)**: The `prompt` object (`id`, `version`, `variables`) maps directly to profile resolution. `version` is accepted but ignored in v1. This provides forward compatibility.

3. **Template substitution (FR-011/FR-013)**: Simple `{{variable}}` only. Undefined variables are left as literal text (no error). This is safe and predictable.

4. **No runtime CRUD**: Profiles are config-file-only. No `/v1/profiles` CRUD endpoint. This keeps v1 simple. A future spec can add API-managed profiles and versioning.

### Constitution Compliance

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Interface-First | PASS | Profile resolution behind an interface |
| II. Zero External Deps | PASS | Template substitution is string replacement |
| III. Nil-Safe | PASS | No profiles configured = agent/prompt fields rejected |
| V. Validate Early | PASS | Profile validation at startup (FR-018) |
| Documentation | PASS | FR-019/020/021 mandate reference + tutorial + developer docs |

### Coverage

- 4 user stories (3x P1, 1x P2)
- 21 functional requirements (including 3 for docs)
- 6 success criteria
- 4 edge cases

### Red Flags

None.
