# Feature Specification: Brave & Tavily Web Search Providers

**Feature Branch**: `036-web-search-providers`
**Created**: 2026-03-03
**Status**: Draft
**Input**: User description: "Add Brave Search and Tavily as web search backend options alongside SearXNG"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Web Search via Brave (Priority: P1)

A user configures antwort with a Brave Search API key. When the LLM uses the web_search tool, search queries are sent to Brave's API and results (title, URL, snippet) are returned to the LLM for grounding its response.

**Why this priority**: Brave Search is a hosted API that works out of the box with an API key. Unlike SearXNG (which requires self-hosting), Brave removes the infrastructure barrier for getting started with web search. This is the fastest path to a working web_search deployment.

**Independent Test**: Configure antwort with a Brave API key, ask a question that triggers web_search, verify the response includes information from Brave search results with url_citation annotations.

**Acceptance Scenarios**:

1. **Given** antwort configured with `backend: brave` and a valid API key, **When** the LLM triggers a web_search tool call, **Then** search results from Brave are returned with title, URL, and snippet for each result
2. **Given** a valid Brave configuration, **When** a search returns results, **Then** the results are formatted identically to SearXNG results (same SearchResult structure) so the rest of the system works unchanged
3. **Given** an invalid or expired Brave API key, **When** a search is attempted, **Then** the tool returns a clear error message indicating authentication failure
4. **Given** the Brave API is unreachable, **When** a search is attempted, **Then** the tool returns a clear error message indicating the search service is unavailable

---

### User Story 2 - Web Search via Tavily (Priority: P1)

A user configures antwort with a Tavily API key. When the LLM uses the web_search tool, search queries are sent to Tavily's API and results are returned to the LLM.

**Why this priority**: Tavily is designed specifically for AI agent use cases. It returns clean, extraction-optimized results that work well as LLM context. Having two hosted options gives users a choice based on their needs and existing subscriptions.

**Independent Test**: Configure antwort with a Tavily API key, ask a question that triggers web_search, verify results are returned.

**Acceptance Scenarios**:

1. **Given** antwort configured with `backend: tavily` and a valid API key, **When** the LLM triggers a web_search tool call, **Then** search results from Tavily are returned with title, URL, and snippet
2. **Given** a valid Tavily configuration, **When** a search returns results, **Then** the results are formatted identically to SearXNG and Brave results
3. **Given** an invalid Tavily API key, **When** a search is attempted, **Then** the tool returns a clear error message indicating authentication failure

---

### User Story 3 - Backend Selection via Configuration (Priority: P1)

A user selects which web search backend to use through configuration. The choice is made at deployment time, not per-request. All three backends (SearXNG, Brave, Tavily) produce identical output through the same tool interface, so switching backends requires only a configuration change.

**Why this priority**: Users need a clean way to choose and switch between backends. The web_search tool's behavior and output format must be identical regardless of which backend is configured.

**Independent Test**: Deploy with each backend in sequence, verify the same query produces structurally identical results (title, URL, snippet) from each.

**Acceptance Scenarios**:

1. **Given** the configuration specifies `backend: brave` with an API key, **When** antwort starts, **Then** web search queries use the Brave backend
2. **Given** the configuration specifies `backend: tavily` with an API key, **When** antwort starts, **Then** web search queries use the Tavily backend
3. **Given** the configuration specifies `backend: searxng` with a URL, **When** antwort starts, **Then** web search queries use SearXNG (existing behavior, unchanged)
4. **Given** the configuration specifies an unknown backend name, **When** antwort starts, **Then** startup fails with a clear error identifying the invalid backend name and listing available options

---

### Edge Cases

- What happens when the search API rate-limits the request? The tool returns an error message indicating rate limiting. The LLM can retry or answer without search results.
- What happens when the search API returns zero results? The tool returns "No results found for [query]" (existing behavior, unchanged).
- What happens when both API key and URL are configured for a hosted backend? The API key takes precedence; the URL is ignored (hosted backends use fixed endpoints).
- What happens when the API key is configured but empty? Startup fails with a validation error indicating the API key is required for the selected backend.

## Requirements *(mandatory)*

### Functional Requirements

**Brave Search Backend**

- **FR-001**: The system MUST provide a Brave Search adapter that sends queries to the Brave Search API and returns results in the standard SearchResult format (title, URL, snippet)
- **FR-002**: The Brave adapter MUST authenticate using an API key provided via configuration
- **FR-003**: The Brave adapter MUST respect the configured maximum results limit
- **FR-004**: The Brave adapter MUST handle API errors (authentication failure, rate limiting, service unavailable) with descriptive error messages

**Tavily Search Backend**

- **FR-005**: The system MUST provide a Tavily adapter that sends queries to the Tavily API and returns results in the standard SearchResult format
- **FR-006**: The Tavily adapter MUST authenticate using an API key provided via configuration
- **FR-007**: The Tavily adapter MUST respect the configured maximum results limit
- **FR-008**: The Tavily adapter MUST handle API errors with descriptive error messages

**Backend Selection**

- **FR-009**: The system MUST support selecting the web search backend via configuration with values: `searxng` (existing, default), `brave`, `tavily`
- **FR-010**: The system MUST validate that required configuration is present for the selected backend (URL for SearXNG, API key for Brave and Tavily) and fail at startup with a clear error if missing
- **FR-011**: All three backends MUST produce identical SearchResult output (title, URL, snippet) so that downstream processing (formatting, citation generation) works unchanged

**Configuration**

- **FR-012**: Brave backend MUST be configurable with: API key (required)
- **FR-013**: Tavily backend MUST be configurable with: API key (required)
- **FR-014**: Configuration MUST support the existing pattern: settings map with `backend`, `api_key`, `url`, and `max_results` fields

### Key Entities

- **SearchAdapter**: Pluggable interface for web search backends (already exists). One method: Search(query, maxResults) returns []SearchResult.
- **SearchResult**: Standard result format with Title, URL, Snippet (already exists). All backends produce this.
- **BraveAdapter**: New adapter for Brave Search API.
- **TavilyAdapter**: New adapter for Tavily API.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can switch between SearXNG, Brave, and Tavily by changing one configuration value, with no other changes required
- **SC-002**: Search results from all three backends are structurally identical (same fields, same format) so that citations, formatting, and downstream processing work without modification
- **SC-003**: Brave and Tavily searches complete within 5 seconds for typical queries
- **SC-004**: Authentication errors produce actionable messages that tell the user what went wrong and how to fix it (e.g., "Brave API key is invalid. Check your configuration.")
- **SC-005**: The existing SearXNG backend continues to work exactly as before with no behavioral changes

## Assumptions

- Brave Search API is available at `https://api.search.brave.com/res/v1/web/search` and authenticates via `X-Subscription-Token` header
- Tavily API is available at `https://api.tavily.com/search` and authenticates via `api_key` in the request body
- Both APIs return JSON responses with title, URL, and snippet/content fields
- Users obtain API keys from the respective provider's developer portal

## Dependencies

- **Spec 017 (Web Search)**: SearchAdapter interface and WebSearchProvider that this spec extends
- **Spec 012 (Configuration)**: Configuration system for backend selection and API key management
- **Spec 035 (Annotations)**: url_citation generation works with any backend (no changes needed)

## Scope Boundaries

### In Scope

- Brave Search adapter implementing SearchAdapter
- Tavily adapter implementing SearchAdapter
- Backend selection via configuration (`backend` field)
- API key configuration for both backends
- Error handling for authentication, rate limiting, and service unavailability
- Configuration validation at startup

### Out of Scope

- Per-request backend selection (backend is deployment-time, not request-time)
- Search result caching (can be added later as a cross-cutting concern)
- Search result ranking or re-ranking across backends
- Fallback chains (e.g., try Brave, fall back to Tavily)
- API key rotation or multi-key support
