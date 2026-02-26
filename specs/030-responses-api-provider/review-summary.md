# Review Summary: 030-responses-api-provider

**Reviewed**: 2026-02-26 | **Verdict**: PASS | **Spec Version**: Draft

## For Reviewers

This spec adds a Responses API provider that forwards inference requests using the Responses API wire format instead of Chat Completions. The gateway continues to own the agentic loop, state management, and tool execution. As a prerequisite, the built-in tool type expansion logic moves from the engine to the provider layer (fixing a Constitution Principle VI violation).

### Key Areas to Review

1. **Phase 1 (expandBuiltinTools migration)**: Moves protocol-specific logic from engine to provider layer. This is a refactoring that affects existing providers (vLLM, LiteLLM). Can be merged independently.

2. **Provider wire format (Phase 2)**: The Responses API adapter translates between internal types and the Responses API format. Key design: always sends `store: false` to the backend, assembles conversation history as `input` items.

3. **SSE event mapping (Phase 2)**: Native Responses API events map near-1:1 to internal ProviderEvent types. This eliminates the custom synthesis logic used by Chat Completions adapters.

4. **Assumption: vLLM maturity**: The spec assumes vLLM's `/v1/responses` endpoint is stable enough. Research confirms availability since v0.10.0 (July 2025) with ongoing improvements. Known limitations are documented in research.md.

### Coverage

- **14/14** requirements have corresponding tasks
- **19 tasks** across 6 phases
- Phase 1 is a standalone refactoring (can be merged first)

### Red Flags

None. The plan resolves a known constitution violation (Principle VI) while adding new capability.

### Task Summary

| Phase | Tasks | What |
|-------|-------|------|
| 1. Foundational | T001-T003 | Move expandBuiltinTools to provider layer |
| 2. US1: Inference | T004-T010 | Responses API adapter with streaming |
| 3. US2: State | T011-T012 | Verify stateful operations |
| 4. US3: Tools | T013-T014 | Verify agentic loop with tool calls |
| 5. US4: Migration | T015-T016 | Startup validation, conformance |
| 6. Polish | T017-T019 | Vet, full suite, spec update |
