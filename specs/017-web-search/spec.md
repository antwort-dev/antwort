# Feature Specification: Web Search Provider

**Feature Branch**: `017-web-search`
**Created**: 2026-02-20
**Status**: Draft

## Overview

This specification adds a built-in `web_search` tool to antwort. When the model needs current information, it calls the web_search tool with a query. Antwort searches the web via a configurable search backend and returns ranked results (title, URL, snippet) for the model to synthesize into an answer with source citations.

This is the first concrete implementation of the FunctionProvider interface (Spec 016). It validates the registry framework end-to-end with a real provider.

## Clarifications

### Session 2026-02-20

- Q: Tool type? -> A: Register as standard function tool (`type: "function"`, `name: "web_search"`). OpenAI's `web_search_preview` type compatibility can be added later in the request parser.
- Q: Search backend for MVP? -> A: SearXNG (self-hosted, no API key, works in Kubernetes). Brave adapter follows as P2.
- Q: Config location? -> A: Uses `providers` map from Spec 016: `providers.web_search.enabled`, `providers.web_search.settings.url`, etc.
- Q: Management API? -> A: None. Web search has no state to manage. Routes() returns nil.
- Q: Result format? -> A: Structured text with title, URL, and snippet per result. The model uses these to cite sources.

## User Scenarios & Testing

### User Story 1 - Model Searches the Web (Priority: P1)

A user asks a question requiring current information. The model calls the web_search tool. Antwort queries the search backend and returns results. The model synthesizes an answer citing the sources.

**Acceptance Scenarios**:

1. **Given** web_search is enabled and a search backend is configured, **When** the model calls `web_search(query="...")`, **Then** antwort queries the backend and returns ranked results
2. **Given** search results are returned, **When** the model produces an answer, **Then** the answer can reference URLs from the search results
3. **Given** the search backend returns no results, **When** the tool result is fed back, **Then** the model receives an empty result and can inform the user

---

### User Story 2 - Search Backend Failure (Priority: P1)

The search backend is unreachable. The tool call returns an error result. The model informs the user that search is unavailable.

**Acceptance Scenarios**:

1. **Given** the search backend is unreachable, **When** web_search is called, **Then** an error result is returned to the model
2. **Given** web_search is disabled in config, **When** a request arrives, **Then** the web_search tool does not appear in the model's tool list

---

### Edge Cases

- What happens when the search query is empty? The provider returns an error result ("empty query").
- What happens when the backend returns more results than max_results? The provider truncates to the configured limit.
- What happens when results contain HTML in snippets? The provider strips HTML tags from snippets before returning.

## Requirements

### Functional Requirements

**Web Search Provider**

- **FR-001**: The system MUST provide a WebSearchProvider implementing the FunctionProvider interface (Spec 016)
- **FR-002**: The provider MUST register a `web_search` tool with a `query` parameter (string, required)
- **FR-003**: The provider MUST return search results formatted as structured text: one entry per result with title, URL, and snippet
- **FR-004**: The provider MUST support a configurable maximum number of results (default: 5)

**Search Backend Adapter**

- **FR-005**: The system MUST define a SearchAdapter interface with a `Search(ctx, query, maxResults) ([]SearchResult, error)` method
- **FR-006**: The system MUST provide a SearXNG adapter that queries a SearXNG instance via its JSON API
- **FR-007**: The SearXNG adapter MUST be configurable with the instance URL

**Configuration**

- **FR-008**: The provider MUST be configurable via the `providers` map in the config system (Spec 016):
```yaml
providers:
  web_search:
    enabled: true
    settings:
      backend: searxng
      url: http://searxng:8080
      max_results: 5
```
- **FR-009**: The provider MUST read its configuration from the `Settings` map passed by the registry

**Error Handling**

- **FR-010**: Search backend failures MUST be returned as tool error results (not server errors)
- **FR-011**: Empty queries MUST be rejected with an error result

**Metrics**

- **FR-012**: The provider MUST register custom Prometheus metrics: `antwort_websearch_queries_total{backend, status}` and `antwort_websearch_results_returned{backend}` (histogram)

### Key Entities

- **WebSearchProvider**: FunctionProvider implementation for web search.
- **SearchAdapter**: Interface for pluggable search backends.
- **SearchResult**: A single search result with title, URL, and snippet.

## Success Criteria

- **SC-001**: The model calls web_search, receives results, and produces an answer citing sources
- **SC-002**: A SearXNG instance returns results that the provider formats correctly
- **SC-003**: Search backend failures are handled gracefully (error result, not crash)
- **SC-004**: The provider registers with the FunctionRegistry and its tool appears in model requests

## Assumptions

- SearXNG is the MVP search backend. Its JSON API returns results at `/search?q=...&format=json`.
- The Brave Search adapter is P2 (separate follow-up).
- The provider has no management API (Routes() returns nil).
- Custom metrics are registered via the Collectors() method on FunctionProvider.

## Dependencies

- **Spec 016 (Function Registry)**: FunctionProvider interface and FunctionRegistry.
- **Spec 013 (Observability)**: Prometheus metrics for custom collectors.
- **Spec 012 (Configuration)**: Providers config map.

## Scope Boundaries

### In Scope

- WebSearchProvider implementing FunctionProvider
- SearchAdapter interface
- SearXNG adapter
- Configuration via providers map
- Custom Prometheus metrics
- Error handling (backend failure, empty query)

### Out of Scope

- Brave Search adapter (P2)
- Google CSE adapter (P2)
- Search result caching
- Rate limiting per search backend
- `web_search_preview` tool type (OpenAI compatibility, future)
