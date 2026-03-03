# Review Summary: 036-web-search-providers

**Reviewed**: 2026-03-03 | **Verdict**: PASS | **Spec Version**: Draft

## For Reviewers

This spec adds Brave Search and Tavily as web search backend options alongside the existing SearXNG. Both are hosted APIs that require only an API key, removing the infrastructure requirement of self-hosting SearXNG.

### Key Areas to Review

1. **Interface stability**: The existing `SearchAdapter` interface is unchanged. Both new backends implement the same single-method interface, producing identical `SearchResult` output.

2. **Configuration pattern**: Backend selection via `backend` field in the settings map. API keys via `api_key` field. Follows the existing pattern established by SearXNG's `url` field.

3. **No downstream changes**: Citations (spec 035), result formatting, and the web_search tool provider all work unchanged because all backends produce the same `SearchResult` type.

### Constitution Compliance

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Interface-First Design | PASS | Uses existing SearchAdapter interface (1 method) |
| II. Zero External Dependencies | PASS | Both adapters use stdlib net/http |
| III. Nil-Safe Composition | PASS | Backend selected at startup, nil adapter = startup error |
| IV. Typed Error Domain | PASS | Uses existing ToolResult error pattern |
| V. Validate Early | PASS | API key validated at startup (FR-010) |

### Coverage

- 3 user stories (all P1), independently testable
- 14 functional requirements
- 5 success criteria
- 4 edge cases

### Red Flags

None. This is a straightforward adapter addition with no architectural risk.
