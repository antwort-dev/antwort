# Tasks: File Search Provider

## Phase 1: Interfaces (P1)

- [ ] T001 [US3] Create `pkg/tools/builtins/filesearch/backend.go`: VectorStoreBackend interface (CreateCollection, DeleteCollection, Search), VectorDocument, SearchMatch types (FR-010).
- [ ] T002 [US1] Create `pkg/tools/builtins/filesearch/embedding.go`: EmbeddingClient interface (Embed, Dimensions), OpenAI-compatible HTTP client implementation (FR-004, FR-005, FR-006, FR-007).
- [ ] T003 [US1] Create `pkg/tools/builtins/filesearch/store.go`: VectorStore metadata type (ID, Name, TenantID, CreatedAt, CollectionName), in-memory metadata store for vector store records.

**Checkpoint**: Interfaces and types defined.

---

## Phase 2: Qdrant Adapter (P1)

- [ ] T004 [US3] Create `pkg/tools/builtins/filesearch/qdrant.go`: Qdrant adapter implementing VectorStoreBackend via Qdrant HTTP API (create/delete collection, search by vector). Configurable URL (FR-011, FR-012).
- [ ] T005 [US3] Write `pkg/tools/builtins/filesearch/qdrant_test.go`: mock Qdrant HTTP server (httptest), test create collection, search, delete.

**Checkpoint**: Qdrant adapter works with mock.

---

## Phase 3: Management API + Provider (P1)

- [ ] T006 [US2] Create `pkg/tools/builtins/filesearch/api.go`: HTTP handlers for Vector Store CRUD: POST/GET/GET/{id}/DELETE vector stores. Tenant-scoped via context. Returns routes for FunctionProvider (FR-008, FR-009).
- [ ] T007 [US1] Create `pkg/tools/builtins/filesearch/provider.go`: FileSearchProvider implementing FunctionProvider. file_search tool with query + vector_store_ids params. Execute: embed query via EmbeddingClient, search via VectorStoreBackend, format results. Routes() returns API handlers. Collectors() returns custom metrics (FR-001, FR-002, FR-003, FR-014).
- [ ] T008 [US1] [US2] Write `pkg/tools/builtins/filesearch/provider_test.go`: mock backend + mock embedding, test tool execution, test API endpoints, test tenant isolation.

**Checkpoint**: File search provider works end-to-end with mocks.

---

## Phase 4: Server Integration + Config

- [ ] T009 Wire FileSearchProvider into `cmd/server/main.go`: when `providers.file_search.enabled=true`, create Qdrant backend + embedding client from settings, register provider with FunctionRegistry (FR-013).
- [ ] T010 [P] Run `go vet ./...` and `go test ./...`.

---

## Dependencies

- Phase 1: No dependencies.
- Phase 2: Depends on Phase 1 (backend interface).
- Phase 3: Depends on Phase 1 + 2 (backend + embedding + metadata).
- Phase 4: Depends on Phase 3.
