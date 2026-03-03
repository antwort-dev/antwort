# Feature Specification: Files API & Document Ingestion

**Feature Branch**: `034-files-api`
**Created**: 2026-03-02
**Status**: Draft
**Input**: User description: "Files API and Document Ingestion - upload, store, extract, chunk, embed, and index documents for RAG"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Upload and Search a Document (Priority: P1)

A user uploads a document (PDF, DOCX, or plain text), adds it to a vector store, and searches its content via the existing file_search tool. The system stores the file, extracts its content, splits it into chunks, embeds each chunk, and indexes them. The document becomes searchable alongside any previously indexed content.

**Why this priority**: This is the core value proposition. Without upload-to-search, the Files API has no purpose. It closes the RAG loop that Spec 018 (File Search) left open by deferring ingestion to external pipelines.

**Independent Test**: Can be fully tested by uploading a single PDF, adding it to a vector store, waiting for completion, and querying for known content from the document. Delivers immediate value: users can ingest documents without external tooling.

**Acceptance Scenarios**:

1. **Given** a running system with content extraction and embedding services available, **When** a user uploads a PDF via the Files API, **Then** the file is stored and its status is `uploaded`
2. **Given** an uploaded file, **When** the user adds it to a vector store, **Then** ingestion starts asynchronously and the file status transitions to `processing`
3. **Given** a file being processed, **When** extraction, chunking, and embedding complete successfully, **Then** the file status becomes `completed` and the vector store's file counts are updated
4. **Given** a completed file in a vector store, **When** a user performs a file_search query with terms matching the document content, **Then** relevant chunks appear in the search results with source attribution back to the original file

---

### User Story 2 - File Lifecycle Management (Priority: P1)

A user manages files through their full lifecycle: upload, list, inspect, download, and delete. Deleting a file removes it from storage and cleans up any chunks indexed in vector stores.

**Why this priority**: File management is essential for any production use. Users need to list what they've uploaded, inspect status, download originals, and clean up files they no longer need.

**Independent Test**: Can be tested by uploading several files, listing them, retrieving metadata and content for one, then deleting it and confirming removal. Delivers value independently of search functionality.

**Acceptance Scenarios**:

1. **Given** multiple uploaded files, **When** the user lists files, **Then** all files are returned with metadata (name, size, status, creation time) in a paginated response
2. **Given** a file ID, **When** the user retrieves file metadata, **Then** the response includes ingestion status, size, MIME type, and timestamps
3. **Given** a file ID, **When** the user downloads the file content, **Then** the original file bytes are returned with the correct content type
4. **Given** a file that has been indexed in one or more vector stores, **When** the user deletes the file, **Then** the file is removed from storage and its chunks are removed from all associated vector stores
5. **Given** a file ID that belongs to a different user, **When** the user attempts to retrieve or delete it, **Then** the request is rejected (the file appears as not found)

---

### User Story 3 - Multi-Format Document Extraction (Priority: P1)

A user uploads documents in various formats (PDF, DOCX, PPTX, images, plain text, Markdown). The system extracts structured content from each, preserving tables and headings where applicable.

**Why this priority**: Supporting multiple document formats is critical for real-world adoption. Users have heterogeneous document collections and should not need to pre-convert files.

**Independent Test**: Can be tested by uploading one file of each supported format, ingesting each, and verifying the extracted text is accurate and preserves document structure (tables, headings).

**Acceptance Scenarios**:

1. **Given** a PDF upload, **When** processed for extraction, **Then** text, tables, and headings are extracted as structured content
2. **Given** a DOCX upload, **When** processed for extraction, **Then** content is extracted preserving document structure
3. **Given** an image upload (PNG, JPEG), **When** processed with OCR enabled, **Then** text is extracted from the image
4. **Given** a plain text or Markdown file, **When** processed for extraction, **Then** the content is used directly without an external extraction service (passthrough)

---

### User Story 4 - Graceful Degradation Without Extraction Service (Priority: P2)

When the external content extraction service is not configured or unavailable, the system still handles simple formats (plain text, Markdown, CSV) using built-in extractors. Complex formats (PDF, DOCX, images) fail with a clear error explaining what is needed.

**Why this priority**: Enables a useful minimal deployment without the extraction service, while providing clear guidance for users who need richer format support.

**Independent Test**: Can be tested by running the system without an extraction service, uploading a text file (succeeds), and uploading a PDF (fails with descriptive error).

**Acceptance Scenarios**:

1. **Given** no extraction service configured, **When** a text file is uploaded and ingested, **Then** the file content is used directly as extracted text and ingestion succeeds
2. **Given** no extraction service configured, **When** a PDF is uploaded and ingestion is attempted, **Then** the file status becomes `failed` with an error indicating that PDF extraction requires an external extraction service
3. **Given** the extraction service becomes unreachable during ingestion, **When** a file is being processed, **Then** the file status becomes `failed` with a descriptive error and previously indexed files remain searchable

---

### User Story 5 - Vector Store File Management (Priority: P2)

A user manages which files are included in a vector store: adding files individually, listing files with their ingestion status, and removing files (which deletes their chunks from the store).

**Why this priority**: Extends the Vector Store API (Spec 018) with file-level management. Users need control over which documents feed into which vector stores.

**Independent Test**: Can be tested by creating a vector store, adding two files, listing the store's files to see status, removing one file, and confirming its chunks are gone while the other file's chunks remain.

**Acceptance Scenarios**:

1. **Given** a vector store and an uploaded file, **When** the user adds the file to the store, **Then** ingestion starts and the file appears in the store's file list with status `in_progress`
2. **Given** a vector store with multiple files, **When** the user lists the store's files, **Then** each file's ingestion status is shown (in_progress, completed, failed)
3. **Given** a file indexed in a vector store, **When** the user removes the file from the store, **Then** all chunks belonging to that file are deleted from the store, but the file itself remains in file storage
4. **Given** the same file added to two different vector stores, **When** it is removed from one store, **Then** the other store's copy of the chunks is unaffected

---

### User Story 6 - Batch File Ingestion (Priority: P3)

A user adds multiple files to a vector store in a single operation and tracks overall batch progress.

**Why this priority**: Convenience feature for bulk document ingestion. Individual file addition (P2) covers the core need; batching improves the experience for large collections.

**Independent Test**: Can be tested by uploading five files, creating a batch to add all five to a vector store, and polling batch status until all are completed.

**Acceptance Scenarios**:

1. **Given** multiple uploaded files, **When** the user creates a file batch for a vector store, **Then** all files are queued for ingestion and a batch ID is returned
2. **Given** a batch operation in progress, **When** the user checks batch status, **Then** progress is reported (counts of in_progress, completed, failed files)

---

### Edge Cases

- What happens when a file is deleted while ingestion is in progress? Ingestion is cancelled (best-effort) and any partial chunks are cleaned up.
- What happens when the same file is added to multiple vector stores? Each store gets its own copy of the chunks. Operations on one store do not affect the other.
- What happens with very large files? A configurable maximum file size is enforced at upload time (default: 50 MB). Files exceeding the limit are rejected before storage.
- What happens when content extraction produces empty output? The file status becomes `failed` with an error indicating no extractable content was found.
- What happens when the embedding service is down during ingestion? The file status becomes `failed`. Previously indexed files remain searchable. The file can be re-added to the vector store to retry.
- What happens when a user uploads a file with an unsupported MIME type? The upload succeeds (the file is stored), but ingestion fails with a descriptive error about the unsupported format.

## Requirements *(mandatory)*

### Functional Requirements

**Files API**

- **FR-001**: The system MUST provide a Files API with endpoints for: uploading a file (multipart), listing files (paginated), retrieving file metadata, downloading file content, and deleting a file
- **FR-002**: File identifiers MUST follow the project's ID format conventions (prefix + random characters)
- **FR-003**: Each file MUST track: identifier, filename, size in bytes, MIME type, purpose, status, owner identity, and timestamps (created, updated)
- **FR-004**: File status values MUST include: `uploaded`, `processing`, `completed`, `failed`
- **FR-005**: Files MUST be scoped to the authenticated user. Cross-user file access MUST be prevented at every layer
- **FR-006**: File upload MUST enforce a configurable maximum file size (default: 50 MB)
- **FR-007**: The `purpose` field MUST support at minimum: `assistants` (for RAG ingestion), `batch`, `fine-tune`, `vision`

**File Storage**

- **FR-008**: The system MUST define a pluggable storage interface for file content persistence
- **FR-009**: The system MUST provide a filesystem-based storage backend (default) using a configurable base directory
- **FR-010**: The system MUST provide an S3-compatible storage backend supporting standard S3 and MinIO-compatible endpoints
- **FR-011**: The system MUST provide an in-memory storage backend for testing
- **FR-012**: All storage backends MUST enforce user-scoped isolation (files stored under user-specific paths)

**Content Extraction**

- **FR-013**: The system MUST define a pluggable content extraction interface that accepts a file and returns structured text
- **FR-014**: The system MUST provide an extraction adapter for the Docling extraction service (docling-serve) communicating over its REST API
- **FR-015**: The Docling adapter MUST request structured Markdown output for downstream chunking
- **FR-016**: The Docling adapter MUST support configuration of: service endpoint, optional credentials, per-file timeout, and OCR toggle
- **FR-017**: The system MUST provide a passthrough extractor for plain text, Markdown, and CSV files that requires no external service
- **FR-018**: When no extraction service is configured, extraction of complex formats (PDF, DOCX, PPTX, images) MUST fail with a descriptive error

**Chunking**

- **FR-019**: The system MUST define a pluggable chunking interface that splits text into segments suitable for embedding
- **FR-020**: The system MUST provide a fixed-size chunker with configurable maximum chunk size (default: 800 tokens) and overlap (default: 200 tokens)
- **FR-021**: Each chunk MUST include: text content, sequential index, and character offsets (start, end) within the source document

**Ingestion Pipeline**

- **FR-022**: The system MUST orchestrate an ingestion pipeline consisting of: extract, chunk, embed, and index stages
- **FR-023**: Ingestion MUST run asynchronously after a file is added to a vector store (the add-file request returns immediately)
- **FR-024**: The pipeline MUST update the file's status at each stage transition and on completion or failure
- **FR-025**: Failed ingestion MUST record a human-readable error detail on the file record
- **FR-026**: The pipeline MUST reuse the existing embedding capability from Spec 018 for chunk embedding
- **FR-027**: The pipeline MUST reuse the existing vector store capability from Spec 018 for chunk indexing

**Vector Store File Management (extends Spec 018)**

- **FR-028**: The Vector Store API MUST be extended with endpoints for: adding a file to a store (triggers ingestion), listing files in a store with status, and removing a file from a store (deletes its chunks)
- **FR-029**: Vector store metadata MUST track file counts by status (in_progress, completed, failed, cancelled)
- **FR-030**: The system SHOULD provide a batch endpoint for adding multiple files to a vector store in one request (P3)

**Configuration**

- **FR-031**: All Files API settings MUST be configurable via the project's configuration system, including: file storage backend and options, extraction service endpoint and options, and chunking parameters
- **FR-032**: Configuration MUST support environment variable overrides following the project's naming conventions

**Observability**

- **FR-033**: The system MUST expose metrics for: files uploaded (counter, by MIME type), ingestion duration (histogram, by MIME type), ingestion outcomes (counter, by status), extraction duration (histogram), and chunks produced (counter)

### Key Entities

- **File**: An uploaded file with metadata (name, size, MIME type, purpose), status tracking (uploaded, processing, completed, failed), owner identity, and timestamps
- **FileStore**: Pluggable storage interface for persisting and retrieving file content (filesystem, S3-compatible, in-memory)
- **ContentExtractor**: Pluggable interface for converting file content into structured text (Docling adapter for complex formats, passthrough for simple formats)
- **Chunker**: Pluggable interface for splitting extracted text into embedding-sized segments with positional metadata
- **IngestionPipeline**: Orchestrator that coordinates the extract, chunk, embed, index workflow asynchronously with status tracking
- **Chunk**: A text segment with sequential index and character offsets, ready for embedding and vector store insertion
- **ExtractionResult**: Output of content extraction, containing structured text and metadata about the extraction (page count, method used)

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can upload a typical 10-page PDF, add it to a vector store, and search its content via file_search within 60 seconds
- **SC-002**: Text, DOCX, and PDF formats are extracted correctly with tables and headings preserved in the output
- **SC-003**: User isolation prevents cross-user file access at every layer (upload, storage, metadata retrieval, search results)
- **SC-004**: The system operates in a useful degraded mode when the extraction service is unavailable: simple formats (text, Markdown, CSV) still work, complex formats fail with actionable error messages
- **SC-005**: File ingestion status is trackable at every stage and errors are reported with enough detail for users to understand and resolve the issue
- **SC-006**: The existing file_search tool (Spec 018) works unchanged with files ingested through this pipeline, with no modifications to the search interface
- **SC-007**: All file operations (upload, list, get, delete) complete in under 2 seconds for files within the size limit, excluding ingestion processing time

## Assumptions

- An external content extraction service (docling-serve) is available as a container image for complex format support
- An embedding service compatible with the existing embedding interface (Spec 018) is available
- A vector database compatible with the existing vector store interface (Spec 018) is available
- Kubernetes persistent storage (PVC) or S3-compatible object storage is available for file persistence in production deployments
- The authentication and identity system (Spec 007) is operational for user-scoped file isolation

## Dependencies

- **Spec 018 (File Search)**: Vector store backend, embedding client, and Vector Store API that this spec extends
- **Spec 016 (Function Registry)**: FunctionProvider interface used by the file_search tool
- **Spec 005 (Storage)**: File metadata persistence
- **Spec 007 (Auth)**: User identity and authentication for file scoping
- **Spec 012 (Configuration)**: Configuration system for Files API settings
- **Spec 013 (Observability)**: Metrics registration and exposition

## Scope Boundaries

### In Scope

- Files API (upload, list, get, download, delete)
- Pluggable file storage (filesystem, S3-compatible, in-memory)
- Pluggable content extraction (Docling adapter, passthrough for simple formats)
- Pluggable chunking (fixed-size with overlap)
- Asynchronous ingestion pipeline with status tracking
- Vector Store file management endpoints (add, list, remove)
- User-scoped file isolation
- Observability metrics for file operations and ingestion
- Configuration for files, extraction, and chunking

### Out of Scope

- Semantic or AI-based chunking strategies (future enhancement)
- Annotation and citation generation from chunks (separate feature)
- Document versioning (re-uploading creates a new file)
- File format conversion for download (files are returned in their original format)
- Deployment automation for the extraction service (operational concern)
- Streaming upload progress indication
- Real-time ingestion status push notifications (polling only)
