# Tasks: Web Search Provider

## Phase 1: Provider + Adapter (P1)

- [ ] T001 [US1] Create `pkg/tools/builtins/websearch/adapter.go`: SearchAdapter interface (Search method), SearchResult struct (Title, URL, Snippet) (FR-005).
- [ ] T002 [US1] Create `pkg/tools/builtins/websearch/searxng.go`: SearXNG adapter implementing SearchAdapter. Queries /search?q=...&format=json, parses JSON response (FR-006, FR-007).
- [ ] T003 [US1] Create `pkg/tools/builtins/websearch/provider.go`: WebSearchProvider implementing FunctionProvider. Registers web_search tool with query param. Execute parses args, calls adapter, formats results as structured text. Custom Prometheus metrics. Routes() returns nil (FR-001 to FR-004, FR-009 to FR-012).
- [ ] T004 [US1] [US2] Write tests in `pkg/tools/builtins/websearch/provider_test.go`: mock search backend (httptest), verify tool registration, search execution, result formatting, empty query error, backend failure error, metrics collection.

**Checkpoint**: Web search provider works with mock backend.

---

## Phase 2: Server Integration

- [ ] T005 Wire WebSearchProvider into `cmd/server/main.go`: when `providers.web_search.enabled` is true, create provider from settings, register with FunctionRegistry (FR-008).
- [ ] T006 [P] Run `go vet ./...` and `go test ./...`.

---

## Dependencies

- Phase 1: No dependencies (uses FunctionProvider from Spec 016).
- Phase 2: Depends on Phase 1.
