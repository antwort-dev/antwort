# Brainstorm 13: Web Search Provider

**Dependencies**: Brainstorm 12 (Function Registry)
**Package**: `pkg/tools/builtins/websearch/`

## Purpose

Built-in `web_search` tool matching OpenAI's capability. The model calls `web_search` with a query, antwort searches the web via a configurable search backend, and returns results for the model to synthesize.

## Tool Definition

```json
{
  "type": "web_search_preview",
  "name": "web_search",
  "description": "Search the web for current information"
}
```

Note: OpenAI uses `type: "web_search_preview"` not `type: "function"`. We should support this tool type for API compatibility, while also working as a standard function tool.

## Search Backend Options

| Backend | Self-hosted? | API Key? | Free Tier |
|---------|-------------|----------|-----------|
| **SearXNG** | Yes | No | Unlimited |
| **Brave Search** | No | Yes | 2000/month |
| **Google CSE** | No | Yes | 100/day |
| **Bing Search** | No | Yes | 1000/month |

**Recommendation**: Support a generic search adapter interface. Ship with SearXNG adapter (self-hosted, no API key) and Brave adapter.

## Execute Flow

1. Model calls `web_search(query="latest news about kubernetes")`
2. Provider sends query to search backend
3. Backend returns ranked results (title, URL, snippet)
4. Provider formats results as tool output
5. Model synthesizes answer from results

## Management API

Minimal or none. Web search doesn't need file management. Possibly:
- `GET /v1/web_search/config` (current search backend status)

## Configuration

```yaml
builtins:
  web_search:
    enabled: true
    backend: searxng          # or "brave", "google_cse"
    url: http://searxng:8080  # SearXNG instance URL
    api_key_file: /run/secrets/search/api-key  # for Brave/Google
    max_results: 5
```

## Deliverables

- [ ] WebSearchProvider implementing FunctionProvider
- [ ] SearXNG search adapter
- [ ] Brave Search adapter (optional P2)
- [ ] Configuration in config.yaml
- [ ] Tests with mock search backend
