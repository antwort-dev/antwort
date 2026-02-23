# Feature Specification: File Search Provider

**Feature Branch**: `018-file-search`
**Created**: 2026-02-23
**Status**: Draft

## Overview

This specification adds a built-in `file_search` tool with a Vector Store management API to antwort. Users upload documents, which are parsed, chunked, embedded (via an external embedding service), and stored in an external vector database. When the model calls `file_search`, antwort queries the vector DB and returns relevant document chunks for retrieval-augmented generation (RAG).

All compute (embedding, vector search) happens outside the antwort process. Antwort orchestrates the pipeline but delegates heavy work to external services, consistent with the constitution's principle that antwort never executes compute-intensive code itself.

The provider implements the FunctionProvider interface (Spec 016) and exposes OpenAI-compatible management endpoints via the registry's route system.

## Clarifications

### Session 2026-02-23

- Q: Where does embedding happen? -> A: External service. Any OpenAI-compatible `/v1/embeddings` endpoint (TEI, LiteLLM, dedicated model). No in-process embedding.
- Q: Where are vectors stored? -> A: External vector database via a pluggable VectorStoreBackend interface. Qdrant adapter for P1. Milvus, pgvector adapters for P2.
- Q: Where are files stored? -> A: File metadata in antwort's existing storage. File content chunked and embedded, original content stored as metadata alongside vectors in the vector DB.
- Q: Sync or async indexing? -> A: Synchronous for P1. Upload blocks until indexing completes. Async with status tracking for P2.
- Q: Tenant scoping? -> A: Yes. Each tenant has their own vector stores, scoped via auth context.

## User Scenarios & Testing

### User Story 1 - Upload and Search Documents (Priority: P1)

A user creates a vector store, uploads a document, and asks the model a question about it. The model calls `file_search`, antwort retrieves relevant chunks from the vector DB, and the model answers with information from the document.

**Acceptance Scenarios**:

1. **Given** a vector store with an uploaded document, **When** the model calls `file_search(query="...")`, **Then** antwort returns relevant chunks with source metadata
2. **Given** an uploaded PDF, **When** it is indexed, **Then** the text is extracted, chunked, embedded via the external service, and stored in the vector DB
3. **Given** search results, **When** the model produces an answer, **Then** it can cite the source document and chunk position

---

### User Story 2 - Vector Store Management API (Priority: P1)

An operator manages vector stores and files via REST API. Create stores, upload files, list contents, delete stores and files.

**Acceptance Scenarios**:

1. **Given** the management API, **When** a vector store is created, **Then** it returns an ID and can be listed
2. **Given** a vector store, **When** a file is uploaded, **Then** it is parsed, chunked, and indexed
3. **Given** a vector store with files, **When** a file is deleted, **Then** its vectors are removed from the DB
4. **Given** auth is enabled, **When** a user accesses the API, **Then** only their tenant's stores are visible

---

### User Story 3 - Pluggable Vector Backend (Priority: P1)

An operator configures the vector database backend. The system supports multiple backends via a pluggable interface, with Qdrant as the default.

**Acceptance Scenarios**:

1. **Given** Qdrant is configured, **When** documents are indexed, **Then** vectors are stored in Qdrant collections
2. **Given** a different backend is configured, **When** the same operations run, **Then** they work identically (interface compliance)

---

### Edge Cases

- What happens when the embedding service is unreachable? File upload fails with a clear error. Existing searches continue to work against already-indexed content.
- What happens when the vector DB is unreachable? file_search returns an error result to the model. The model can inform the user.
- What happens when a file format is unsupported? Upload returns an error listing supported formats.
- What happens when a file exceeds the size limit? Upload is rejected before processing.
- What happens when a vector store is deleted while a search is in progress? The search completes with whatever results are available. Subsequent searches return empty.

## Requirements

### Functional Requirements

**File Search Provider**

- **FR-001**: The system MUST provide a FileSearchProvider implementing the FunctionProvider interface (Spec 016)
- **FR-002**: The provider MUST register a `file_search` tool with a `query` parameter and optional `vector_store_ids` parameter
- **FR-003**: The provider MUST return search results formatted as structured text with source document name, chunk content, and relevance score

**Vector Store Management API**

- **FR-004**: The provider MUST expose OpenAI-compatible management endpoints via the FunctionProvider Routes() method: POST/GET/DELETE for vector stores and files (7 endpoints total)
- **FR-005**: Management endpoints MUST be protected by auth and tenant-scoped (automatically via the registry middleware)

**VectorStoreBackend Interface**

- **FR-006**: The system MUST define a VectorStoreBackend interface with methods for: creating/deleting collections, upserting/deleting documents, and searching by vector
- **FR-007**: The system MUST provide a Qdrant adapter implementing VectorStoreBackend
- **FR-008**: The backend MUST be configurable via the providers config map

**Embedding Client Interface**

- **FR-009**: The system MUST define an EmbeddingClient interface with methods for embedding text and reporting vector dimensions
- **FR-010**: The system MUST provide an HTTP client that calls any OpenAI-compatible `/v1/embeddings` endpoint
- **FR-011**: The embedding service URL MUST be configurable

**Document Processing**

- **FR-012**: The system MUST support parsing plain text files (.txt, .md)
- **FR-013**: The system MUST support parsing PDF files
- **FR-014**: Parsed text MUST be split into chunks with configurable size and overlap
- **FR-015**: Each chunk MUST be embedded via the external embedding service and stored in the vector DB with source metadata

**Configuration**

- **FR-016**: The provider MUST be configurable via the `providers` map in config.yaml

**Metrics**

- **FR-017**: The provider MUST register custom Prometheus metrics: document count, chunk count, search latency, embedding latency

**Tenant Scoping**

- **FR-018**: Vector stores MUST be scoped to the authenticated tenant
- **FR-019**: Cross-tenant access to vector stores MUST be prevented

### Key Entities

- **FileSearchProvider**: FunctionProvider implementation for document search.
- **VectorStoreBackend**: Interface for pluggable vector databases (Qdrant, Milvus, pgvector).
- **EmbeddingClient**: Interface for external embedding services.
- **VectorStore**: A named collection of indexed documents, tenant-scoped.
- **VectorStoreFile**: A document uploaded to a vector store, parsed and indexed.

## Success Criteria

- **SC-001**: A document uploaded via the management API is searchable via the file_search tool
- **SC-002**: The model uses file_search results to answer questions about uploaded documents with source citations
- **SC-003**: The Vector Store API is compatible with OpenAI's endpoint structure
- **SC-004**: Tenant isolation prevents cross-tenant document access
- **SC-005**: The Qdrant adapter stores and retrieves vectors correctly

## Assumptions

- An external embedding service is available at a configurable URL.
- An external vector database (Qdrant) is available at a configurable URL.
- No compute-intensive operations (embedding, vector math) run in the antwort process.
- File metadata (store name, file name, status) is stored in antwort's existing storage system.
- PDF parsing uses a library, not an external service.

## Dependencies

- **Spec 016 (Function Registry)**: FunctionProvider interface and registry.
- **Spec 005 (Storage)**: Metadata storage for vector stores and files.
- **Spec 007 (Auth)**: Tenant scoping for management API.
- **Spec 013 (Observability)**: Custom Prometheus metrics.

## Scope Boundaries

### In Scope

- FileSearchProvider implementing FunctionProvider
- VectorStoreBackend interface + Qdrant adapter
- EmbeddingClient interface + OpenAI-compatible HTTP client
- Vector Store management API (7 endpoints)
- Text and PDF file parsing
- Chunking with configurable size/overlap
- file_search tool execution
- Tenant-scoped vector stores
- Custom Prometheus metrics
- Configuration via providers map

### Out of Scope

- Milvus adapter (P2)
- pgvector adapter (P2)
- DOCX, HTML, CSV parsing (P2)
- Async indexing with status tracking (P2)
- File content caching
- Search result ranking customization
