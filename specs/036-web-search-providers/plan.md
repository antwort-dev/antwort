# Implementation Plan: Brave & Tavily Web Search Providers

**Branch**: `036-web-search-providers` | **Date**: 2026-03-03 | **Spec**: [spec.md](spec.md)

## Summary

Add Brave Search and Tavily as web search backend options alongside SearXNG. Both implement the existing `SearchAdapter` interface with no changes to the interface or downstream processing. Two new files (`brave.go`, `tavily.go`) in the existing `pkg/tools/builtins/websearch/` package, plus a configuration update in `provider.go`.

## Technical Context

**Language/Version**: Go 1.22+
**Primary Dependencies**: Go standard library only (`net/http`, `encoding/json`, `net/url`)
**Storage**: N/A
**Testing**: `go test` with `httptest.NewServer` mocks for both APIs
**Project Type**: Web service (adapter additions to existing package)
**Performance Goals**: Search results within 5 seconds
**Constraints**: No external dependencies. Both adapters use stdlib HTTP.

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Interface-First | PASS | Uses existing SearchAdapter (1 method) |
| II. Zero External Deps | PASS | stdlib net/http only |
| III. Nil-Safe | PASS | Backend validated at startup |
| V. Validate Early | PASS | API key validated at startup |

No violations.

## Design Decisions

### D1: File Layout

Two new files in the existing package. Follows the SearXNG pattern.

### D2: Brave Adapter

- GET `https://api.search.brave.com/res/v1/web/search?q={query}&count={maxResults}`
- Header: `X-Subscription-Token: {apiKey}`
- Response: `{ "web": { "results": [{ "title", "url", "description" }] } }`
- Map `description` to `Snippet`

### D3: Tavily Adapter

- POST `https://api.tavily.com/search`
- Header: `Authorization: Bearer {apiKey}`
- Body: `{ "query": "...", "max_results": 5 }`
- Response: `{ "results": [{ "title", "url", "content" }] }`
- Map `content` to `Snippet`

### D4: Configuration Wiring

Extend the `switch` in `New()` with `brave` and `tavily` cases. Extract `api_key` from settings map.

## Project Structure

```text
pkg/tools/builtins/websearch/
├── adapter.go          # SearchAdapter interface, SearchResult (unchanged)
├── searxng.go          # SearXNG adapter (unchanged)
├── brave.go            # NEW: Brave Search adapter
├── brave_test.go       # NEW: Brave adapter tests
├── tavily.go           # NEW: Tavily adapter
├── tavily_test.go      # NEW: Tavily adapter tests
├── provider.go         # MODIFIED: add brave/tavily cases to New()
└── provider_test.go    # existing tests (unchanged)
```
