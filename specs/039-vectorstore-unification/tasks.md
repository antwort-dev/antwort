# Tasks: Vector Store Unification & New Backends

**Input**: Design documents from `/specs/039-vectorstore-unification/`

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Unified Interface

- [x] T001 Create `pkg/vectorstore/backend.go` with unified Backend interface (5 methods), SearchMatch, and VectorPoint types
- [x] T002 Write compile-time interface checks and basic type tests in `pkg/vectorstore/backend_test.go`

---

## Phase 2: Qdrant Migration (US3)

- [x] T003 [US3] Move QdrantBackend from `pkg/tools/builtins/filesearch/qdrant.go` to `pkg/vectorstore/qdrant/qdrant.go`, implement unified Backend interface
- [x] T004 [US3] Update `pkg/tools/builtins/filesearch/` to import Backend and types from `pkg/vectorstore/` instead of local definitions
- [x] T005 [US3] Update `pkg/files/` to import Backend and VectorPoint from `pkg/vectorstore/` instead of local VectorIndexer and Embedder
- [x] T006 [US3] Remove old `VectorStoreBackend` interface from `pkg/tools/builtins/filesearch/backend.go` (keep only re-exports if needed)
- [x] T007 [US3] Remove old `VectorIndexer` interface from `pkg/files/indexer.go` (replace with import from vectorstore)
- [x] T008 [US3] Update server wiring in `cmd/server/main.go` to use `pkg/vectorstore/` types
- [x] T009 [US3] Verify all existing tests pass after migration: `go test ./pkg/tools/builtins/filesearch/... ./pkg/files/... ./pkg/engine/...`

**Checkpoint**: All existing functionality works with unified interface. Zero behavioral changes.

---

## Phase 3: In-Memory Backend (US2)

- [x] T010 [P] [US2] Implement in-memory Backend (map-based collections, brute-force cosine similarity search) in `pkg/vectorstore/memory/memory.go`
- [x] T011 [US2] Write tests for in-memory Backend in `pkg/vectorstore/memory/memory_test.go` (create/delete collection, upsert, search returns correct order, delete by file, cosine similarity accuracy)

**Checkpoint**: In-memory backend passes all vector store operations without external services.

---

## Phase 4: pgvector Backend (US1)

- [x] T012 [US1] Implement pgvector Backend (collection as table, HNSW index, cosine search, pool sharing) in `pkg/vectorstore/pgvector/pgvector.go`
- [x] T013 [US1] Add pgvector extension validation at startup in `pkg/vectorstore/pgvector/pgvector.go`
- [x] T014 [US1] Write tests for pgvector Backend in `pkg/vectorstore/pgvector/pgvector_test.go` (schema creation, upsert, search, delete, extension check)

**Checkpoint**: pgvector backend works with PostgreSQL. Note: integration tests require PostgreSQL with pgvector, may be skipped in CI.

---

## Phase 5: Backend Selection (US4)

- [x] T015 [US4] Add backend selection logic to file_search provider: `qdrant` (default), `pgvector`, `memory` in `pkg/tools/builtins/filesearch/provider.go`
- [x] T016 [US4] Wire pgvector backend to use the existing PostgreSQL pool from storage config in `cmd/server/main.go`
- [x] T017 [US4] Write test for backend selection in `pkg/tools/builtins/filesearch/provider_test.go`

**Checkpoint**: Backend selectable via config.

---

## Phase 6: Documentation

- [x] T018 [P] Update file search reference docs with backend selection in `docs/modules/reference/pages/files-api.adoc`
- [x] T019 [P] Add developer guide for custom vector store backends in `docs/modules/developer/pages/vectorstore.adoc`
- [x] T020 Update nav.adoc for developer module

---

## Phase 7: Polish

- [x] T021 Verify `go vet ./pkg/vectorstore/... ./pkg/tools/builtins/filesearch/... ./pkg/files/... ./pkg/engine/...`
- [x] T022 Verify `go test ./...` passes (all packages)

---

## Dependencies

- **Phase 2** depends on Phase 1 (needs unified interface)
- **Phase 3** depends on Phase 1 (needs unified interface)
- **Phase 4** depends on Phase 1 (needs unified interface)
- **Phase 3 and 4** can run in parallel
- **Phase 5** depends on Phase 2+3+4 (needs all backends)
- **Phase 6+7** depend on Phase 5

## Implementation Strategy

### MVP: Unified Interface + In-Memory

1. Phase 1: Create unified interface (T001-T002)
2. Phase 2: Migrate Qdrant (T003-T009)
3. Phase 3: Add in-memory (T010-T011)
4. Validate: all tests pass with no external services

### Full: Add pgvector

5. Phase 4: pgvector backend (T012-T014)
6. Phase 5: Backend selection (T015-T017)
7. Phase 6-7: Docs and polish
