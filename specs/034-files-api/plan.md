# Implementation Plan: Files API & Document Ingestion

**Branch**: `034-files-api` | **Date**: 2026-03-02 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/034-files-api/spec.md`

## Summary

Add a Files API and document ingestion pipeline to antwort. Users upload files via multipart REST, then add them to vector stores to trigger asynchronous ingestion: content extraction (via Docling for complex formats, passthrough for plain text), chunking, embedding, and vector indexing. This closes the RAG ingestion loop left open by Spec 018 (File Search), which handles query-time search but defers document ingestion.

The implementation follows the existing FunctionProvider pattern. A new `pkg/files/` package defines pluggable interfaces for file storage, content extraction, chunking, and vector indexing. The Docling adapter communicates with docling-serve over HTTP. The ingestion pipeline runs asynchronously via a goroutine worker pool. All file operations are user-scoped via the existing auth identity system.

## Technical Context

**Language/Version**: Go 1.22+ (consistent with all existing specs)
**Primary Dependencies**: Go standard library only for core. S3 backend requires AWS SDK (adapter package only). Docling adapter uses stdlib `net/http`.
**Storage**: File content: filesystem (default), S3, in-memory. File metadata: in-memory (default), PostgreSQL (future, via existing pgx adapter).
**Testing**: `go test` with table-driven tests. Integration tests using real HTTP server + test fixtures. Docling tests use a mock HTTP server.
**Target Platform**: Linux containers on Kubernetes
**Project Type**: Web service (extension to existing antwort server)
**Performance Goals**: File operations (upload, list, get, delete) under 2 seconds. End-to-end ingestion (10-page PDF) under 60 seconds.
**Constraints**: Max upload size 50 MB (configurable). Concurrent ingestions limited (default: 4 workers).
**Scale/Scope**: Single-instance deployment. File counts per user in the hundreds to low thousands.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Pre-Design | Post-Design | Notes |
|-----------|-----------|-------------|-------|
| I. Interface-First Design | PASS | PASS | 6 interfaces, all 1-5 methods: FileStore(3), FileMetadataStore(5), ContentExtractor(2), Chunker(1), VectorIndexer(2), VectorStoreFileStore(5) |
| II. Zero External Dependencies (Core) | PASS | PASS | Core `pkg/files/` uses only stdlib. S3 adapter isolated in `pkg/files/s3/`. Docling adapter uses stdlib HTTP. |
| III. Nil-Safe Composition | PASS | PASS | FilesProvider is nil-safe: not registered when config disabled. Docling extractor nil when not configured, falls back to passthrough. |
| IV. Typed Error Domain | PASS | PASS | Uses existing APIError types: NewInvalidRequestError, NewNotFoundError. Ingestion errors recorded as human-readable strings on file records. |
| V. Validate Early, Fail Fast | PASS | PASS | File size validated at upload. Purpose validated before storage. MIME type checked before extraction. |
| VI. Protocol-Agnostic Provider | N/A | N/A | Not a provider feature. |
| VII. Streaming First-Class | N/A | N/A | Ingestion is async batch, not streaming. |
| VIII. Context Carries Cross-Cutting | PASS | PASS | User identity from auth context for scoping. Request ID for tracing. |
| IX. Kubernetes-Native | PASS | PASS | PVC for filesystem storage, S3 for scalable storage. Docling deployed as sidecar or service. |

No violations. No complexity tracking needed.

## Design Decisions

### D1: Package Layout

New `pkg/files/` package following the filesearch pattern (cohesive single package with adapter sub-packages for external dependencies).

```
pkg/files/
├── types.go              # File, Chunk, ExtractionResult, VectorPoint types
├── filestore.go          # FileStore interface + filesystem + memory backends
├── metadata.go           # FileMetadataStore interface + in-memory implementation
├── extractor.go          # ContentExtractor interface + passthrough implementation
├── docling.go            # Docling extraction adapter (stdlib HTTP)
├── chunker.go            # Chunker interface + fixed-size implementation
├── indexer.go            # VectorIndexer interface (implemented by Qdrant adapter)
├── pipeline.go           # IngestionPipeline orchestrator
├── vsfiles.go            # VectorStoreFileRecord types + VectorStoreFileStore interface + in-memory
├── provider.go           # FilesProvider (FunctionProvider implementation)
├── api.go                # HTTP handlers for /files endpoints
├── vsfiles_api.go        # HTTP handlers for /vector_stores/{id}/files endpoints
├── *_test.go             # Tests for each component
└── s3/                   # S3 FileStore adapter (external dependency)
    └── s3.go
```

### D2: VectorIndexer Interface (New)

The existing `VectorStoreBackend` (filesearch) provides read operations (Search). The ingestion pipeline needs write operations. Rather than extending the existing interface (breaking change), define a new `VectorIndexer` interface in `pkg/files/`:

```go
type VectorIndexer interface {
    UpsertPoints(ctx context.Context, collection string, points []VectorPoint) error
    DeletePointsByFile(ctx context.Context, collection string, fileID string) error
}
```

The existing `QdrantBackend` in filesearch implements both interfaces. At server wiring time, the same Qdrant instance is passed to both filesearch (for search) and files (for indexing).

### D3: FunctionProvider Registration

`FilesProvider` implements `registry.FunctionProvider` with:
- `Name()` = "files"
- `Tools()` = empty (Files API is not a tool)
- `Routes()` = all file management and vector store file endpoints
- Registered via config like other providers, mounted under `/builtin/`

Route ownership:
- **FilesProvider**: `/files`, `/files/{file_id}`, `/files/{file_id}/content`, `/vector_stores/{store_id}/files`, `/vector_stores/{store_id}/files/{file_id}`, `/vector_stores/{store_id}/file_batches`
- **FileSearchProvider** (unchanged): `/vector_stores`, `/vector_stores/{store_id}`

### D4: Docling Integration

The Docling adapter calls `POST /v1/convert/file` on docling-serve:
- Sends file as multipart form data with `to_formats=md` and `do_ocr` configurable
- Auth via optional `X-Api-Key` header
- Parses `md_content` field from JSON response
- Configurable timeout (default 300s) for large documents
- Returns `ExtractionResult` with Markdown text

The adapter uses only stdlib `net/http` (no external dependency).

### D5: Ingestion Pipeline

The pipeline runs as a goroutine worker pool (configurable concurrency, default 4):

```
File added to vector store
  → Pipeline.Ingest(ctx, file, vectorStoreID)
    → goroutine (from worker pool):
      1. Update status → processing
      2. Read file content from FileStore
      3. Extract text (ContentExtractor)
      4. Chunk text (Chunker)
      5. Embed chunks (EmbeddingClient from filesearch)
      6. Upsert vectors (VectorIndexer)
      7. Update status → completed (or failed with error)
```

Each stage failure updates the file status to `failed` with the error detail.

### D6: File Deletion Cascade

When a file is deleted:
1. Query VectorStoreFileStore.ListByFile to find all stores containing this file
2. For each store: call VectorIndexer.DeletePointsByFile to remove chunks
3. Delete VectorStoreFileRecords
4. Delete file content from FileStore
5. Delete file metadata from FileMetadataStore

### D7: Configuration

```yaml
providers:
  files:
    enabled: true
    settings:
      max_upload_size: 52428800  # 50 MB
      store_type: filesystem     # filesystem | s3 | memory
      store_path: /data/files    # filesystem base directory
      s3_bucket: ""              # S3 bucket name
      s3_region: ""              # S3 region
      s3_endpoint: ""            # S3/MinIO endpoint (optional)
      docling_url: ""            # docling-serve URL (empty = disabled)
      docling_api_key: ""        # docling-serve API key (optional)
      docling_timeout: 300s      # per-file extraction timeout
      docling_ocr: true          # enable OCR
      chunk_size: 800            # max tokens per chunk
      chunk_overlap: 200         # token overlap between chunks
      pipeline_workers: 4        # concurrent ingestion workers
```

Follows the existing provider settings pattern: `map[string]interface{}` parsed in `New()`.

## Project Structure

### Documentation (this feature)

```text
specs/034-files-api/
├── spec.md
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── contracts/
│   └── files-api.md     # API contract
├── review-summary.md    # Spec review
├── checklists/
│   └── requirements.md  # Quality checklist
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
pkg/files/
├── types.go              # File, Chunk, ExtractionResult, VectorPoint, FileStatus
├── filestore.go          # FileStore interface + FilesystemStore + MemoryFileStore
├── metadata.go           # FileMetadataStore interface + MemoryMetadataStore
├── extractor.go          # ContentExtractor interface + PassthroughExtractor
├── docling.go            # DoclingExtractor (HTTP client to docling-serve)
├── chunker.go            # Chunker interface + FixedSizeChunker
├── indexer.go            # VectorIndexer interface
├── pipeline.go           # IngestionPipeline (worker pool orchestrator)
├── vsfiles.go            # VectorStoreFileRecord + VectorStoreFileStore + in-memory
├── provider.go           # FilesProvider (FunctionProvider)
├── api.go                # File HTTP handlers (upload, list, get, content, delete)
├── vsfiles_api.go        # Vector store file HTTP handlers (add, list, remove)
├── types_test.go
├── filestore_test.go
├── metadata_test.go
├── extractor_test.go
├── docling_test.go
├── chunker_test.go
├── pipeline_test.go
├── vsfiles_test.go
├── api_test.go
├── vsfiles_api_test.go
└── s3/
    ├── s3.go             # S3FileStore (AWS SDK adapter)
    └── s3_test.go
```

Extends existing files:
- `cmd/server/main.go`: Add `files` provider case to `createFunctionRegistry`
- `pkg/tools/builtins/filesearch/qdrant.go`: Add `UpsertPoints` and `DeletePointsByFile` methods
- `pkg/config/config.go`: No changes needed (providers already use `map[string]interface{}`)

**Structure Decision**: Single `pkg/files/` package following the filesearch precedent. All core code uses stdlib only. S3 adapter isolated in sub-package per constitution Principle II.
