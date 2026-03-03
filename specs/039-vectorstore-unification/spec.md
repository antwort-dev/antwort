# Feature Specification: Vector Store Unification & New Backends

**Feature Branch**: `039-vectorstore-unification`
**Created**: 2026-03-03
**Status**: Draft
**Input**: User description: "Unify the split VectorStoreBackend and VectorIndexer interfaces into a single package, add pgvector and in-memory backends"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Use PostgreSQL for Both Responses and Vectors (Priority: P1)

A user deploys antwort with PostgreSQL for response storage (existing) and now also uses the same PostgreSQL database for vector search via pgvector. No separate vector database (Qdrant) is needed. The user uploads documents via the Files API, they are embedded and stored in PostgreSQL, and file_search queries return results from the same database.

**Why this priority**: This is the highest-value addition. Most production antwort deployments already have PostgreSQL. Adding pgvector eliminates the need for a separate Qdrant deployment, reducing infrastructure cost and operational complexity significantly.

**Independent Test**: Configure antwort with `vector_backend: pgvector` pointing at the existing PostgreSQL database. Upload a text file, add to a vector store, search for known content. Verify results are returned correctly.

**Acceptance Scenarios**:

1. **Given** antwort configured with pgvector backend and PostgreSQL, **When** a user creates a vector store, uploads a file, and searches, **Then** the full RAG pipeline works end-to-end using only PostgreSQL
2. **Given** a pgvector backend, **When** search results are returned, **Then** they have the same structure (title, content, score, metadata) as Qdrant results
3. **Given** an existing PostgreSQL deployment without the pgvector extension, **When** antwort starts with pgvector backend, **Then** startup fails with a clear error explaining that the pgvector extension is required

---

### User Story 2 - Run Vector Store Tests in CI (Priority: P1)

An in-memory vector backend enables all vector store integration tests to run without any external service. Currently, vector store tests are skipped in CI because they require a running Qdrant instance. With an in-memory backend, the full test suite runs on standard CI runners.

**Why this priority**: Untested code is unreliable code. The in-memory backend is essential for development velocity and CI reliability. Every vector store operation can be tested without infrastructure.

**Independent Test**: Run the full test suite with the in-memory backend. All vector store operations (create collection, upsert, search, delete) pass without any external service.

**Acceptance Scenarios**:

1. **Given** an in-memory vector backend, **When** the test suite runs, **Then** all vector store operations work correctly without any external service
2. **Given** an in-memory backend, **When** vectors are upserted and searched, **Then** nearest-neighbor search returns correct results (by cosine similarity)
3. **Given** an in-memory backend, **When** the process restarts, **Then** all data is lost (no persistence, by design)

---

### User Story 3 - Unified Backend Interface (Priority: P1)

A developer adding a new vector store backend implements a single interface with 5 methods. There is no need to implement two separate interfaces in two different packages. The unified interface covers both read operations (search) and write operations (upsert, delete).

**Why this priority**: The current split between `VectorStoreBackend` (3 methods in filesearch) and `VectorIndexer` (2 methods in files) is confusing and was caused by an import cycle, not a meaningful separation. Unifying them makes adding new backends straightforward.

**Independent Test**: Implement a test backend using the unified interface. Verify it compiles and passes the standard backend test suite.

**Acceptance Scenarios**:

1. **Given** a unified vector store interface, **When** a developer implements it, **Then** they implement exactly one interface with 5 methods (create, delete, search, upsert, delete-by-filter)
2. **Given** the unified interface, **When** the existing Qdrant backend is migrated to it, **Then** all existing functionality works identically
3. **Given** the unified interface, **When** both filesearch and files packages use it, **Then** no import cycles exist

---

### User Story 4 - Backend Selection via Configuration (Priority: P1)

A user selects which vector store backend to use through configuration. The choice is made at deployment time. All backends (Qdrant, pgvector, in-memory) produce identical results through the same interface. Switching backends requires only a configuration change.

**Why this priority**: Users need a clean way to choose backends based on their infrastructure.

**Independent Test**: Deploy with each backend, verify the same operations produce consistent results.

**Acceptance Scenarios**:

1. **Given** `vector_backend: qdrant`, **When** antwort starts, **Then** vector operations use Qdrant (existing behavior)
2. **Given** `vector_backend: pgvector`, **When** antwort starts, **Then** vector operations use PostgreSQL with pgvector
3. **Given** `vector_backend: memory`, **When** antwort starts, **Then** vector operations use the in-memory backend
4. **Given** no `vector_backend` configured, **When** file_search is enabled, **Then** Qdrant is used as the default (backward compatible)

---

### Edge Cases

- What happens when pgvector extension is not installed in PostgreSQL? Startup fails with a clear error: "pgvector extension required. Run: CREATE EXTENSION vector;"
- What happens when the in-memory backend's data exceeds available memory? The process runs out of memory (same as any in-memory store). The in-memory backend is for testing, not production.
- What happens when switching backends on a running deployment? Vector data is not migrated. Users must re-index documents. This is documented.

## Requirements *(mandatory)*

### Functional Requirements

**Unified Interface**

- **FR-001**: The system MUST provide a single, unified vector store interface that covers both read operations (collection management, search) and write operations (upsert, delete by filter)
- **FR-002**: The unified interface MUST have at most 5 methods to comply with the constitution's interface size limit
- **FR-003**: The unified interface MUST be importable by both the file search package and the files package without import cycles
- **FR-004**: The existing Qdrant backend MUST be migrated to implement the unified interface with zero behavioral changes

**pgvector Backend**

- **FR-005**: The system MUST provide a pgvector backend that stores vectors in PostgreSQL using the pgvector extension
- **FR-006**: The pgvector backend MUST support cosine similarity for nearest-neighbor search
- **FR-007**: The pgvector backend MUST reuse the existing PostgreSQL connection pool when both response storage and vector storage use the same database
- **FR-008**: The pgvector backend MUST create the required schema (tables, indexes) at startup
- **FR-009**: The pgvector backend MUST validate that the pgvector extension is available and fail with a clear error if not

**In-Memory Backend**

- **FR-010**: The system MUST provide an in-memory vector backend for testing and development
- **FR-011**: The in-memory backend MUST implement cosine similarity search that returns correct nearest-neighbor results
- **FR-012**: The in-memory backend MUST be usable in CI without any external services

**Backend Selection**

- **FR-013**: The system MUST support selecting the vector store backend via configuration
- **FR-014**: The default backend MUST be Qdrant for backward compatibility
- **FR-015**: The system MUST validate backend configuration at startup and fail with clear errors for invalid settings

**Documentation (per constitution v1.6.0)**

- **FR-016**: The feature MUST include reference documentation for the unified interface and all backends
- **FR-017**: The feature MUST include a developer guide explaining how to implement a custom vector store backend
- **FR-018**: The existing file search and RAG documentation MUST be updated to cover backend selection

### Key Entities

- **VectorBackend**: Unified interface for all vector store operations (create/delete collection, search, upsert points, delete points by filter)
- **SearchMatch**: Standard search result with document ID, score, content, and metadata (unchanged)
- **VectorPoint**: A vector with ID, embedding, and metadata payload (unchanged, from pkg/files)

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can run the full RAG pipeline (upload, ingest, search) using only PostgreSQL, with zero additional vector database infrastructure
- **SC-002**: All vector store integration tests pass in CI without any external services (using in-memory backend)
- **SC-003**: Switching between Qdrant, pgvector, and in-memory backends requires changing one configuration value, with no code changes
- **SC-004**: The existing Qdrant backend works identically after migration to the unified interface (zero behavioral changes)
- **SC-005**: Search results from all three backends are structurally identical for the same input data
- **SC-006**: Documentation covers the unified interface, all backends, and custom backend implementation

## Assumptions

- PostgreSQL deployments can install the pgvector extension (available on all major managed PostgreSQL services and operators)
- The pgvector extension supports cosine distance (`<=>` operator) for similarity search
- In-memory cosine similarity can be computed with brute-force search (no index needed) for testing workloads
- The existing pgx/v5 PostgreSQL driver supports pgvector's vector type via custom type registration

## Dependencies

- **Spec 018 (File Search)**: VectorStoreBackend interface and Qdrant implementation being unified
- **Spec 034 (Files API)**: VectorIndexer interface being unified
- **Spec 005 (Storage)**: PostgreSQL connection pool to be shared with pgvector backend

## Scope Boundaries

### In Scope

- Unified vector store interface in a shared package
- Migration of Qdrant backend to unified interface
- pgvector backend implementation
- In-memory backend implementation
- Backend selection via configuration
- Schema creation for pgvector at startup
- Reference, developer, and updated RAG documentation

### Out of Scope

- Milvus backend (deferred, see brainstorm notes)
- Data migration between backends
- Hybrid search (combining vector and keyword search)
- Vector index tuning (HNSW parameters exposed as config)
- Automatic pgvector extension installation
