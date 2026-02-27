# Tasks: State Management & Storage

**Input**: Design documents from `/specs/005-storage/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, quickstart.md

**Tests**: Tests are included as part of implementation tasks (Go convention: test file alongside source file).

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Interface Extension & Package Structure)

**Purpose**: Extend ResponseStore interface and create package structure

- [x] T001 (antwort-vr3.1) (antwort-1yu.1) Extend `ResponseStore` interface in `pkg/transport/handler.go` with `SaveResponse(ctx, *Response) error`, `GetResponseForChain(ctx, id) (*Response, error)`, `HealthCheck(ctx) error`, and `Close() error`. Update mock in engine tests to satisfy new interface (FR-001, FR-004, FR-005, FR-023).
- [x] T002 (antwort-vr3.2) (antwort-1yu.2) [P] Create `pkg/storage/doc.go` with package documentation. Create `pkg/storage/errors.go` with `ErrNotFound` and `ErrConflict` sentinel errors.
- [x] T003 (antwort-vr3.3) (antwort-1yu.3) [P] Create `pkg/storage/tenant.go` with `SetTenant(ctx, tenantID) context.Context` and `GetTenant(ctx) string` using a private context key. Write tests in `pkg/storage/tenant_test.go` (FR-018, FR-019).

---

## Phase 2: User Story 1 - Persist and Retrieve (Priority: P1) MVP

**Goal**: Save responses after inference and retrieve them by ID.

**Independent Test**: Save a response, retrieve by ID, verify all fields. Delete, verify not-found.

### Implementation for User Story 1 (In-Memory First)

- [x] T004 (antwort-58h.1) (antwort-67x.1) [US1] Implement in-memory store in `pkg/storage/memory/memory.go`: SaveResponse, GetResponse, GetResponseForChain, DeleteResponse (soft delete), HealthCheck, Close. Use sync.RWMutex for concurrency. Store responses in a map with soft-delete tracking. Write comprehensive tests in `pkg/storage/memory/memory_test.go` covering: save+get, get not-found, delete+get (soft delete), duplicate save (conflict), health check (FR-001, FR-002, FR-003, FR-009, FR-010, FR-012).
- [x] T005 (antwort-58h.2) (antwort-67x.2) [US1] Integrate SaveResponse into engine's `CreateResponse` in `pkg/engine/engine.go`: after WriteResponse (non-streaming) or after terminal event (streaming), call store.SaveResponse if store is configured and request has store=true. Populate Response.Input from request items before saving. Log warning on save failure, do not fail client response (FR-006, FR-007, FR-008, FR-024).
- [x] T006 (antwort-58h.3) (antwort-67x.3) [US1] Update engine's `history.go` to use `GetResponseForChain` instead of `GetResponse` for chain traversal, so soft-deleted responses are included in chain reconstruction (FR-022, FR-023).
- [x] T007 (antwort-58h.4) (antwort-67x.4) [US1] Write engine storage integration tests in `pkg/engine/engine_test.go`: verify save is called after non-streaming inference, verify save is skipped for store=false, verify save is skipped when store is nil, verify save failure does not affect client response.

**Checkpoint**: Responses are persisted and retrievable using in-memory store.

---

## Phase 3: User Story 2 - Conversation Chaining (Priority: P1)

**Goal**: Verify chain reconstruction works with stored responses (including soft-deleted).

**Independent Test**: Store 3 chained responses, request referencing the last, verify full chain reconstruction.

### Implementation for User Story 2

- [x] T008 (antwort-nuy.1) (antwort-8kk.1) [US2] Write chain reconstruction tests in `pkg/engine/engine_test.go` using in-memory store: store 3 chained responses (A->B->C), send request with previous_response_id=C, verify engine reconstructs full conversation. Test soft-deleted intermediate response still accessible for chain. Test non-existent previous_response_id returns not-found.
- [x] T009 (antwort-nuy.2) (antwort-8kk.2) [US2] Write soft-delete chain integrity tests in `pkg/storage/memory/memory_test.go`: save A->B->C, delete B, verify GetResponse(B) returns not-found but GetResponseForChain(B) returns B.

**Checkpoint**: Chain reconstruction works with soft-deleted responses.

---

## Phase 4: User Story 3 - In-Memory Store LRU Eviction (Priority: P2)

**Goal**: Support LRU eviction when max size is reached.

**Independent Test**: Create store with max size 3, save 4 responses, verify oldest is evicted.

### Implementation for User Story 3

- [x] T010 (antwort-7km.1) (antwort-bvm.1) [US3] Implement LRU eviction in `pkg/storage/memory/memory.go`: when SaveResponse is called and the store is at max capacity, evict the least recently accessed response. Use a doubly-linked list for access ordering. Write tests in `pkg/storage/memory/memory_test.go` covering: eviction at capacity, access updates order, max size 0 means unlimited (FR-011).

**Checkpoint**: LRU eviction prevents unbounded memory growth.

---

## Phase 5: User Story 4 - PostgreSQL Adapter (Priority: P2)

**Goal**: Implement PostgreSQL storage with migrations and health checks.

**Independent Test**: Start PostgreSQL via testcontainers, run migrations, save and retrieve responses.

### Implementation for User Story 4

- [x] T011 (antwort-me1.1) (antwort-3m9.1) [US4] [P] Create `pkg/storage/postgres/config.go` with PostgresConfig struct (DSN, MaxConns, MaxIdleConns, ConnMaxLifetime, MigrateOnStart, TLS options) (FR-014, FR-017).
- [x] T012 (antwort-me1.2) (antwort-3m9.2) [US4] [P] Create `pkg/storage/postgres/migrations/001_create_responses.sql` with the schema from data-model.md (responses table, indexes, schema_migrations table).
- [x] T013 (antwort-me1.3) (antwort-3m9.3) [US4] Create `pkg/storage/postgres/migrations.go` with embedded SQL migration runner using `//go:embed`. Track applied versions in schema_migrations table (FR-015).
- [x] T014 (antwort-me1.4) (antwort-3m9.4) [US4] Implement PostgreSQL adapter in `pkg/storage/postgres/postgres.go`: New() constructor with pgxpool, SaveResponse (INSERT with ON CONFLICT), GetResponse (SELECT WHERE deleted_at IS NULL), GetResponseForChain (SELECT without deleted_at filter), DeleteResponse (UPDATE SET deleted_at), HealthCheck (pool.Ping), Close (pool.Close). Handle JSONB serialization for input/output/error/extensions (FR-013, FR-014, FR-016).
- [x] T015 (antwort-me1.5) (antwort-3m9.5) [US4] Write PostgreSQL integration tests in `pkg/storage/postgres/postgres_test.go` using testcontainers-go: start PostgreSQL container, run migrations, test full CRUD cycle, test soft delete, test chain retrieval, test health check, test duplicate save conflict (FR-013).
- [x] T016 (antwort-me1.6) (antwort-3m9.6) [US4] Add `pgx/v5` and `testcontainers-go` dependencies to go.mod.

**Checkpoint**: PostgreSQL adapter fully functional with schema migrations.

---

## Phase 6: User Story 5 - Multi-Tenant Isolation (Priority: P3)

**Goal**: Scope all storage operations to the tenant from context.

**Independent Test**: Store responses for tenant A and B, verify cross-tenant isolation.

### Implementation for User Story 5

- [x] T017 (antwort-ues.1) (antwort-zaj.1) [US5] Add tenant scoping to in-memory store in `pkg/storage/memory/memory.go`: extract tenant from context via storage.GetTenant, scope SaveResponse (store tenant_id), GetResponse (filter by tenant), DeleteResponse (filter by tenant). Empty tenant = no filtering. Write tests in `pkg/storage/memory/memory_test.go` (FR-018, FR-019, FR-020, FR-021).
- [x] T018 (antwort-ues.2) (antwort-zaj.2) [US5] Add tenant scoping to PostgreSQL adapter in `pkg/storage/postgres/postgres.go`: add `tenant_id` to INSERT, add `WHERE tenant_id = $tenant` to SELECT/UPDATE (when tenant is not empty). Write tests in `pkg/storage/postgres/postgres_test.go` (FR-018, FR-021).

**Checkpoint**: Tenant isolation prevents cross-tenant access.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Edge cases, validation, and final verification

- [x] T019 (antwort-7az.1) (antwort-inj.1) [P] Handle edge case: SaveResponse with duplicate ID returns ErrConflict in both adapters.
- [x] T020 (antwort-7az.2) (antwort-inj.2) [P] Handle edge case: database unreachable during SaveResponse logs warning, doesn't fail client.
- [x] T021 (antwort-7az.3) (antwort-inj.3) [P] Handle edge case: in-memory store concurrent access (verify mutex correctness with race detector).
- [x] T022 (antwort-7az.4) (antwort-inj.4) Run `go vet ./...` and `go test ./...` across all packages to verify compilation and test passing.
- [x] T023 (antwort-7az.5) (antwort-inj.5) Run `go test -race ./pkg/storage/...` to verify concurrent access safety.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies. Start immediately.
- **Phase 2 (US1)**: Depends on Phase 1 (interface + errors + tenant context).
- **Phase 3 (US2)**: Depends on Phase 2 (save/get must work for chain tests).
- **Phase 4 (US3)**: Depends on Phase 2 (in-memory store must exist). Independent of Phase 3.
- **Phase 5 (US4)**: Depends on Phase 1 (interface). Independent of Phases 2-4.
- **Phase 6 (US5)**: Depends on Phase 2 and Phase 5 (both adapters must exist).
- **Phase 7 (Polish)**: Depends on all phases.

### Parallel Opportunities

Within Phase 1:
- T002, T003 can run in parallel

Within Phase 5:
- T011, T012 can run in parallel

After Phase 1:
- US1 (Phase 2) and US4 (Phase 5) can start in parallel

Within Phase 7:
- T019, T020, T021 can run in parallel

---

## Implementation Strategy

### MVP First (User Stories 1 + 2)

1. Complete Phase 1: Setup (interface extension, package structure)
2. Complete Phase 2: US1 (save/get with in-memory store)
3. Complete Phase 3: US2 (chain reconstruction with soft delete)
4. **STOP and VALIDATE**: Full stateful API works with in-memory store

### Incremental Delivery

1. Setup -> Interface and types ready
2. US1 -> Save/retrieve works (in-memory MVP)
3. US2 -> Chain reconstruction verified
4. US3 -> LRU eviction for memory safety
5. US4 -> PostgreSQL production backend
6. US5 -> Multi-tenant isolation
7. Polish -> Edge cases and race detection

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- In-memory store is implemented first (US1) so the engine integration can be tested without PostgreSQL
- PostgreSQL adapter (US4) can be developed in parallel with US2/US3
- Multi-tenancy (US5) depends on both adapters being complete
- Go convention: test files sit alongside source files (`*_test.go`)
- PostgreSQL tests use testcontainers-go for real database testing
