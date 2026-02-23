# Feature Specification: File Search Provider

**Feature Branch**: `018-file-search`
**Created**: 2026-02-23
**Status**: Draft

## Overview

This specification adds a built-in `file_search` tool with a Vector Store management API to antwort. The Vector Store API manages metadata (store names, file references, configuration). An external ingestion pipeline handles document processing (parsing, chunking, embedding, vector insertion). When the model calls `file_search`, antwort queries the external vector DB and returns relevant document chunks for retrieval-augmented generation (RAG).

Antwort is the query layer and metadata manager, not the ingestion engine. Document ingestion (parsing, chunking, embedding) runs as a separate process, keeping antwort focused on its gateway role.

## Clarifications

### Session 2026-02-23

- Q: Where does embedding happen? -> A: External ingestion pipeline. Not in antwort.
- Q: Where are vectors stored? -> A: External vector database via a pluggable VectorStoreBackend interface. Qdrant adapter for P1.
- Q: Who does document ingestion? -> A: An external process (separate microservice, Job, or CLI tool). Antwort provides the Vector Store API for metadata management and the file_search tool for querying. The ingestion pipeline reads from the same vector DB.
- Q: What does antwort's file upload endpoint do? -> A: Stores file metadata and makes the file available for the external ingestion pipeline to process. Antwort does NOT parse, chunk, or embed files itself.
- Q: Tenant scoping? -> A: Yes. Vector stores are tenant-scoped. The external ingestion pipeline must respect tenant boundaries (via collection naming or metadata filtering).

## User Scenarios & Testing

### User Story 1 - Search Indexed Documents (Priority: P1)

A user has documents indexed in a vector store (by the external ingestion pipeline). The model calls `file_search`, antwort queries the vector DB, and returns relevant chunks for the model to synthesize an answer.

**Acceptance Scenarios**:

1. **Given** a vector store with indexed documents, **When** the model calls `file_search(query="...")`, **Then** antwort queries the vector DB and returns relevant chunks with source metadata
2. **Given** search results, **When** the model produces an answer, **Then** it can cite the source document and chunk position
3. **Given** an empty vector store, **When** file_search is called, **Then** the tool returns an empty result (not an error)

---

### User Story 2 - Vector Store Management API (Priority: P1)

An operator manages vector store metadata via REST API. Create stores (which create collections in the vector DB), list stores, delete stores.

**Acceptance Scenarios**:

1. **Given** the management API, **When** a vector store is created, **Then** a corresponding collection is created in the vector DB and the store is listed
2. **Given** a vector store, **When** it is deleted, **Then** the collection is removed from the vector DB
3. **Given** auth is enabled, **When** a user accesses the API, **Then** only their tenant's stores are visible

---

### User Story 3 - Pluggable Vector Backend (Priority: P1)

An operator configures the vector database backend. The system supports multiple backends via a pluggable interface, with Qdrant as the default.

**Acceptance Scenarios**:

1. **Given** Qdrant is configured, **When** a vector store is created, **Then** a Qdrant collection is created
2. **Given** a search query, **When** file_search executes, **Then** it queries the Qdrant collection and returns results

---

### Edge Cases

- What happens when the vector DB is unreachable? file_search returns an error result to the model.
- What happens when the embedding service (used by the external pipeline) is down? Ingestion fails, but existing searches continue to work.
- What happens when a vector store is deleted while a search is in progress? The search completes with whatever results are available.
- What happens when no vector_store_ids are specified in file_search? Search across all of the user's stores (tenant-scoped).

## Requirements

### Functional Requirements

**File Search Provider**

- **FR-001**: The system MUST provide a FileSearchProvider implementing the FunctionProvider interface (Spec 016)
- **FR-002**: The provider MUST register a `file_search` tool with a `query` parameter and optional `vector_store_ids` parameter
- **FR-003**: The provider MUST return search results formatted as structured text with source document name, chunk content, and relevance score

**Embedding Client (for query-time embedding)**

- **FR-004**: The system MUST define an EmbeddingClient interface with methods for embedding text and reporting vector dimensions
- **FR-005**: The system MUST provide an HTTP client that calls any OpenAI-compatible `/v1/embeddings` endpoint
- **FR-006**: The embedding service URL MUST be configurable
- **FR-007**: The embedding client is used only at query time (to embed the search query). Document embedding is handled by the external ingestion pipeline.

**Vector Store Management API**

- **FR-008**: The provider MUST expose management endpoints via Routes():
  - POST `/v1/vector_stores` (create store + vector DB collection)
  - GET `/v1/vector_stores` (list stores)
  - GET `/v1/vector_stores/{id}` (get store details)
  - DELETE `/v1/vector_stores/{id}` (delete store + vector DB collection)
- **FR-009**: Management endpoints MUST be protected by auth and tenant-scoped (via registry middleware)

**VectorStoreBackend Interface**

- **FR-010**: The system MUST define a VectorStoreBackend interface with methods for: creating/deleting collections and searching by vector
- **FR-011**: The system MUST provide a Qdrant adapter implementing VectorStoreBackend
- **FR-012**: The backend MUST be configurable via the providers config map

**Configuration**

- **FR-013**: The provider MUST be configurable via the `providers` map:
```yaml
providers:
  file_search:
    enabled: true
    settings:
      embedding_url: http://embedding-service:8080/v1/embeddings
      vector_backend: qdrant
      max_results: 10
      qdrant:
        url: http://qdrant:6333
```

**Metrics**

- **FR-014**: The provider MUST register custom Prometheus metrics: search latency, query embedding latency, results returned count

**Tenant Scoping**

- **FR-015**: Vector stores MUST be scoped to the authenticated tenant
- **FR-016**: Cross-tenant access to vector stores MUST be prevented

### Key Entities

- **FileSearchProvider**: FunctionProvider implementation for document search (query layer only).
- **VectorStoreBackend**: Interface for pluggable vector databases (query + collection management).
- **EmbeddingClient**: Interface for query-time embedding (external service).
- **VectorStore**: Metadata for a named collection, tenant-scoped. Maps to a vector DB collection.

## Success Criteria

- **SC-001**: Documents indexed by the external pipeline are searchable via the file_search tool
- **SC-002**: The model uses file_search results to answer questions about documents with source citations
- **SC-003**: The Vector Store API creates and deletes collections in the external vector DB
- **SC-004**: Tenant isolation prevents cross-tenant document access
- **SC-005**: The Qdrant adapter queries vectors correctly

## Assumptions

- An external embedding service is available for query-time embedding.
- An external vector database (Qdrant) is available and pre-populated by the ingestion pipeline.
- Document ingestion (parsing, chunking, embedding, vector insertion) is handled by a separate process, not antwort.
- Antwort only embeds the search query at query time (one embedding call per search, not per document).
- Vector store metadata (name, tenant, creation time) is stored in antwort's existing storage system.

## Dependencies

- **Spec 016 (Function Registry)**: FunctionProvider interface and registry.
- **Spec 005 (Storage)**: Metadata storage for vector store records.
- **Spec 007 (Auth)**: Tenant scoping for management API.
- **Spec 013 (Observability)**: Custom Prometheus metrics.

## Scope Boundaries

### In Scope

- FileSearchProvider implementing FunctionProvider
- VectorStoreBackend interface + Qdrant adapter (query + collection management)
- EmbeddingClient interface + HTTP client (query-time embedding only)
- Vector Store management API (4 endpoints: create, list, get, delete)
- file_search tool execution (embed query, search vector DB, return chunks)
- Tenant-scoped vector stores
- Custom Prometheus metrics
- Configuration via providers map

### Out of Scope

- Document ingestion pipeline (parsing, chunking, embedding, insertion). This is a separate service/tool.
- File upload endpoints (files are ingested by the external pipeline, not uploaded to antwort)
- Milvus adapter (P2)
- pgvector adapter (P2)
- Async indexing
- File content caching
