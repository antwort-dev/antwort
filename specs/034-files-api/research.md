# Research: 034-files-api

**Date**: 2026-03-02

## R1: VectorStoreBackend Extension for Write Operations

**Decision**: Define a new `VectorIndexer` interface in `pkg/files/` rather than extending the existing `VectorStoreBackend` in filesearch.

**Rationale**: The current `VectorStoreBackend` (filesearch/backend.go) has 3 methods: `CreateCollection`, `DeleteCollection`, `Search`. Adding write methods (upsert, delete-by-filter) would change the interface and all its implementations. Defining a separate `VectorIndexer` interface in the files package keeps the filesearch interface stable and avoids import cycles. The Qdrant adapter already speaks HTTP to Qdrant, so implementing both interfaces on the same struct is straightforward.

**Alternatives considered**:
- Extend `VectorStoreBackend` directly: Would break the filesearch interface contract and force all backends to implement write methods even if unused for search-only deployments.
- Shared `pkg/vectorstore/` package: Over-engineering for two consumers. Can be factored out later if more packages need it.

## R2: File Metadata Storage Pattern

**Decision**: Define `FileMetadataStore` interface in `pkg/files/` with in-memory implementation as default. Follow the same pattern as filesearch's `MetadataStore`.

**Rationale**: The filesearch `MetadataStore` is an in-memory store with a simple interface. File metadata follows the same pattern: CRUD + list + status updates. A PostgreSQL adapter can be added later (in `pkg/storage/postgres/` alongside the existing response store).

**Alternatives considered**:
- Extend `ResponseStore` (transport.handler.go): Wrong abstraction; files are not responses.
- Add to filesearch's `MetadataStore`: Creates coupling between files and filesearch packages.

## R3: File Content Storage (FileStore) Design

**Decision**: Define `FileStore` interface with `Store`, `Retrieve`, `Delete` methods. Three backends: filesystem (default), in-memory (testing), S3 (production, adapter package).

**Rationale**: File content is binary and potentially large (up to 50 MB). Storage needs are different from metadata: streaming reads/writes, user-scoped paths, backend-specific optimizations. The interface uses `io.Reader`/`io.ReadCloser` for memory efficiency.

**Alternatives considered**:
- Store file bytes in the metadata database: Not scalable for large files, wastes database resources.
- Only S3: Filesystem is simpler for development and single-node deployments.

## R4: Docling-Serve Integration

**Decision**: Use the synchronous `POST /v1/convert/file` endpoint with multipart form data. Request Markdown output format.

**Rationale**: The synchronous endpoint is simpler than the async flow (submit + poll + retrieve). Since antwort's ingestion pipeline already runs asynchronously, the per-file extraction call can be synchronous within the pipeline goroutine. The configurable timeout (default 300s) handles large documents. Markdown output is structured enough for chunking while being simpler to process than JSON.

**Key API details**:
- Endpoint: `POST /v1/convert/file`
- Content-Type: multipart/form-data
- Form fields: `files` (file upload), `do_ocr` (boolean), `to_formats` (comma-separated, use "md")
- Auth: Optional `X-Api-Key` header
- Response: JSON with `md_content` field containing the Markdown text
- Error: HTTP 504 on timeout, HTTP 401 on invalid API key

**Alternatives considered**:
- Async endpoint (submit + poll): More complex, adds polling logic, needed only for very large documents which the file size limit already prevents.
- Request JSON output instead of Markdown: JSON provides more structure but is harder to chunk. Markdown preserves headings and tables in a text-friendly format.

## R5: Package Structure

**Decision**: Single `pkg/files/` package for core types, interfaces, built-in implementations, pipeline, and HTTP handlers. Separate adapter sub-packages for external dependencies only.

```
pkg/files/
├── types.go            # File, Chunk, ExtractionResult types
├── filestore.go        # FileStore interface + filesystem + memory implementations
├── metadata.go         # FileMetadataStore interface + in-memory implementation
├── extractor.go        # ContentExtractor interface + passthrough implementation
├── docling.go          # Docling extraction adapter (stdlib HTTP, no external deps)
├── chunker.go          # Chunker interface + fixed-size implementation
├── indexer.go          # VectorIndexer interface
├── pipeline.go         # IngestionPipeline orchestrator
├── vsfiles.go          # VectorStoreFileRecord types + in-memory store
├── provider.go         # FilesProvider (FunctionProvider implementation)
├── api.go              # HTTP handlers for /files endpoints
├── vsfiles_api.go      # HTTP handlers for /vector_stores/{id}/files endpoints
└── s3/                 # S3 FileStore adapter (external dependency: AWS SDK)
    └── s3.go
```

**Rationale**: Follows the filesearch package pattern (everything in one package). Docling adapter uses only stdlib `net/http`, so no need for a separate adapter package. S3 requires an external SDK, so it gets its own sub-package per constitution Principle II.

**Alternatives considered**:
- Separate packages per concern (pkg/files/extraction/, pkg/files/chunking/): Over-segmented. Each would have 1-2 files. A single cohesive package is simpler.
- Put docling adapter in separate package: It only uses stdlib HTTP, so there's no external dependency to isolate.

## R6: FunctionProvider Registration Pattern

**Decision**: Register `FilesProvider` as a `FunctionProvider` with empty `Tools()`. It contributes only HTTP routes, no tool definitions.

**Rationale**: The FunctionProvider pattern provides automatic auth middleware, metrics wrapping, and consistent route mounting under `/builtin/`. Using it for files routes avoids duplicating this infrastructure. The filesearch provider already demonstrates the pattern of contributing both tools and management API routes.

**Route ownership**:
- FilesProvider owns: `/files`, `/files/{file_id}`, `/files/{file_id}/content`, `/vector_stores/{store_id}/files`, `/vector_stores/{store_id}/files/{file_id}`, `/vector_stores/{store_id}/file_batches`
- FileSearchProvider keeps: `/vector_stores`, `/vector_stores/{store_id}`

**Alternatives considered**:
- Mount directly on main mux: Requires manual auth middleware setup and metrics wrapping.
- New "APIProvider" interface: Unnecessary abstraction for one use case.

## R7: Chunking Strategy

**Decision**: Fixed-size chunker splitting by character count with configurable overlap. Default: 800 tokens max chunk size, 200 tokens overlap. Use a simple token-to-character ratio (1 token ~ 4 characters) for the initial implementation.

**Rationale**: Token-accurate chunking requires a tokenizer library (tiktoken or equivalent), which would be an external dependency. A character-based approximation is sufficient for the first version. The chunker interface allows swapping in a token-accurate implementation later.

**Chunk boundaries**: Split at the nearest whitespace or sentence boundary within the target size to avoid breaking words.

**Alternatives considered**:
- Token-accurate chunking (tiktoken): Requires external dependency or CGO. Can be added as an alternative chunker later.
- Semantic chunking: Out of scope per spec. Requires NLP models.

## R8: Async Ingestion Concurrency Model

**Decision**: Use a goroutine per file ingestion with a configurable concurrency limit (default: 4 concurrent ingestions). Use a channel-based worker pool pattern.

**Rationale**: Ingestion involves I/O-bound operations (HTTP calls to Docling, embedding service, vector DB). A worker pool prevents unbounded goroutine creation while allowing concurrent processing. The concurrency limit is configurable because it depends on the capacity of downstream services.

**Alternatives considered**:
- Single sequential goroutine: Too slow for batch operations.
- Unbounded goroutines: Risks overwhelming downstream services.
- External job queue: Over-engineering for the current scope.

## R9: Vector Store File Count Tracking

**Decision**: File counts on the vector store metadata response are computed on-the-fly from the `VectorStoreFileStore`, not cached on the vector store record.

**Rationale**: Keeping counts in sync with the actual state is error-prone (concurrent updates, crash recovery). Computing from the source of truth (the file records) is simpler and always accurate. The file list is small enough that counting is fast.

**Alternatives considered**:
- Cached counters on VectorStore: Requires atomic updates and recovery logic. Premature optimization.
