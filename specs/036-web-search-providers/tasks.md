# Tasks: Brave & Tavily Web Search Providers

**Input**: Design documents from `/specs/036-web-search-providers/`
**Prerequisites**: plan.md (required), spec.md (required), research.md

**Organization**: Tasks grouped by user story.

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup

**Purpose**: No setup needed. Existing package and interface.

---

## Phase 2: User Story 1 - Brave Search (Priority: P1)

**Goal**: Brave Search adapter implementing SearchAdapter.

**Independent Test**: Configure with Brave API key, trigger web_search, verify results returned.

- [ ] T001 [P] [US1] Implement BraveAdapter (GET to Brave Web Search API, X-Subscription-Token auth, parse web.results into SearchResult, handle 401/429 errors) in `pkg/tools/builtins/websearch/brave.go`
- [ ] T002 [US1] Write tests for BraveAdapter in `pkg/tools/builtins/websearch/brave_test.go` (table-driven with httptest: successful search, empty results, auth error 401, rate limit 429, malformed response, max_results limit)

**Checkpoint**: Brave adapter works in isolation with mock server.

---

## Phase 3: User Story 2 - Tavily Search (Priority: P1)

**Goal**: Tavily adapter implementing SearchAdapter.

**Independent Test**: Configure with Tavily API key, trigger web_search, verify results returned.

- [ ] T003 [P] [US2] Implement TavilyAdapter (POST to Tavily Search API, Bearer auth, JSON body with query and max_results, parse results into SearchResult, handle 401/429 errors) in `pkg/tools/builtins/websearch/tavily.go`
- [ ] T004 [US2] Write tests for TavilyAdapter in `pkg/tools/builtins/websearch/tavily_test.go` (table-driven with httptest: successful search, empty results, auth error 401, rate limit 429, malformed response, max_results in request body)

**Checkpoint**: Tavily adapter works in isolation with mock server.

---

## Phase 4: User Story 3 - Backend Selection (Priority: P1)

**Goal**: Configuration wiring for backend selection.

- [ ] T005 [US3] Add `brave` and `tavily` cases to the backend switch in `New()`, extract `api_key` from settings, validate required fields, create adapters in `pkg/tools/builtins/websearch/provider.go`
- [ ] T006 [US3] Write tests for backend selection in `pkg/tools/builtins/websearch/provider_test.go` (brave with api_key works, tavily with api_key works, brave without api_key fails, unknown backend fails, searxng unchanged)

**Checkpoint**: All three backends selectable via configuration.

---

## Phase 5: Polish

- [ ] T007 [P] Add Brave and Tavily to API reference documentation in `docs/modules/reference/pages/api-reference.adoc` (web_search backend options)
- [ ] T008 [P] Update configuration reference with brave and tavily settings in `docs/modules/reference/pages/config-reference.adoc`
- [ ] T009 Verify `go vet ./pkg/tools/builtins/websearch/...` and `go test ./pkg/tools/builtins/websearch/...` pass

---

## Dependencies & Execution Order

- **US1 and US2**: Can run in parallel (different files, no interdependency)
- **US3**: Depends on US1 and US2 (wires both adapters into provider)
- **Polish**: Depends on US3

## Implementation Strategy

### MVP: Brave Only

1. T001-T002: Brave adapter + tests
2. T005 (brave case only): Wire into provider
3. Validate end-to-end

### Full Delivery

1. T001-T004 in parallel: Both adapters + tests
2. T005-T006: Configuration wiring
3. T007-T009: Docs and verification
