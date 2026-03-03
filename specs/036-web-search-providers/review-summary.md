# Review Summary: 036-web-search-providers

**Reviewed**: 2026-03-03 | **Verdict**: APPROVED - Ready for implementation | **Spec Version**: Draft

## For Reviewers

This spec adds Brave Search and Tavily as web search backend options alongside the existing SearXNG. Both are hosted APIs requiring only an API key, removing the infrastructure barrier of self-hosting SearXNG. The existing `SearchAdapter` interface is unchanged.

### Key Areas to Review

1. **Interface stability**: No changes to `SearchAdapter` or `SearchResult`. Both adapters implement the same single-method interface. Downstream processing (formatting, citation generation via spec 035) works unchanged.

2. **API key management**: Keys are passed via the settings map. For production, users should use environment variable overrides (`ANTWORT_PROVIDERS_WEB_SEARCH_API_KEY`) or Kubernetes secrets mounted as files.

3. **Error handling**: Both adapters distinguish auth errors (401/403) from rate limits (429) from service unavailability (connection errors). Error messages are descriptive and actionable.

## Coverage Matrix

| FR | Description | Task(s) | Status |
|----|-------------|---------|--------|
| FR-001 | Brave adapter, SearchResult format | T001 | COVERED |
| FR-002 | Brave API key auth | T001 | COVERED |
| FR-003 | Brave max_results | T001, T002 | COVERED |
| FR-004 | Brave error handling | T001, T002 | COVERED |
| FR-005 | Tavily adapter, SearchResult format | T003 | COVERED |
| FR-006 | Tavily API key auth | T003 | COVERED |
| FR-007 | Tavily max_results | T003, T004 | COVERED |
| FR-008 | Tavily error handling | T003, T004 | COVERED |
| FR-009 | Backend selection config | T005 | COVERED |
| FR-010 | Startup validation | T005, T006 | COVERED |
| FR-011 | Identical SearchResult output | T001-T004 | COVERED |
| FR-012 | Brave config (api_key) | T005 | COVERED |
| FR-013 | Tavily config (api_key) | T005 | COVERED |
| FR-014 | Existing config pattern | T005 | COVERED |

**Coverage**: 14/14 FRs covered. All 5 success criteria verifiable via tests.

## Task Summary

| Phase | Story | Tasks | Tests | Parallel |
|-------|-------|-------|-------|----------|
| 2. US1 (P1) | Brave | 1 | 1 | 1 parallel |
| 3. US2 (P1) | Tavily | 1 | 1 | 1 parallel |
| 4. US3 (P1) | Config | 1 | 1 | Sequential |
| 5. Polish | - | 3 | 0 | 2 parallel |
| **Total** | | **5** | **4** | **9 total** |

## Key Strengths

- Minimal scope: 2 new files, 1 modified file, no new packages
- Zero downstream impact: existing interface, existing provider, existing citation pipeline
- Clean parallel opportunity: US1 and US2 are fully independent

## Risks

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| API breaking changes | Low | Adapters isolated in single files, easy to update |
| Rate limiting in tests | None | Tests use httptest mocks, no real API calls |

## Red Flags

None. This is the simplest possible feature: two HTTP adapters implementing an existing interface.

## Reviewer Guidance

When reviewing the implementation PR:
1. Verify both adapters implement `SearchAdapter` with compile-time checks
2. Confirm `description` (Brave) and `content` (Tavily) map to `Snippet`
3. Check error messages include the backend name for debuggability
4. Verify API key validation happens in `New()` at startup, not at search time
5. Run `go test ./pkg/tools/builtins/websearch/...` to verify all tests pass
