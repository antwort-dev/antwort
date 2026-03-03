# Research: 036-web-search-providers

**Date**: 2026-03-03

## R1: Brave Search API Contract

**Decision**: Use the Web Search endpoint (`GET https://api.search.brave.com/res/v1/web/search`) with `X-Subscription-Token` header auth.

**Key details**:
- Method: GET with query parameters (`q`, `count`)
- Auth: `X-Subscription-Token: <api_key>` header
- Response: JSON with `web.results[]` array, each having `title`, `url`, `description`
- Rate limit: HTTP 429 with `X-RateLimit-Remaining` headers
- Error: JSON with `error.detail` and `error.code` fields

**Alternatives considered**:
- LLM Context endpoint (`/v1/llm/context`): Newer, but returns different format. Web Search is more standard and established.

## R2: Tavily Search API Contract

**Decision**: Use the Search endpoint (`POST https://api.tavily.com/search`) with Bearer token auth.

**Key details**:
- Method: POST with JSON body (`query`, `max_results`)
- Auth: `Authorization: Bearer <api_key>` header
- Response: JSON with `results[]` array, each having `title`, `url`, `content`
- Rate limit: HTTP 429 (100 RPM dev, 1000 RPM prod)
- Error: HTTP status code based (401/403 auth, 429 rate limit)

**Alternatives considered**:
- Include `include_answer: true` for LLM-generated answers: Adds latency, not needed since antwort's LLM generates answers from search results.

## R3: Field Mapping

**Decision**: Map Brave's `description` and Tavily's `content` to the existing `Snippet` field in `SearchResult`.

| SearchResult field | Brave field | Tavily field | SearXNG field |
|-------------------|-------------|--------------|---------------|
| Title             | title       | title        | title         |
| URL               | url         | url          | url           |
| Snippet           | description | content      | content       |

## R4: Configuration Pattern

**Decision**: Extend the existing settings map with `api_key` field. Backend selection via `backend` field (existing). No new config structure needed.

```yaml
providers:
  web_search:
    enabled: true
    settings:
      backend: brave        # or "tavily" or "searxng"
      api_key: "your-key"   # required for brave/tavily
      url: ""               # required for searxng only
      max_results: 5
```
