# Tasks: Files API & Document Ingestion

**Input**: Design documents from `/specs/034-files-api/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Tests are included inline within each user story phase, following the project's testing standards (table-driven, real components, fakes only at infrastructure boundaries).

**Organization**: Tasks grouped by user story. Each story is independently testable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Create package structure and file ID generation

- [ ] T001 Create `pkg/files/` package directory and initial `doc.go`
- [ ] T002 [P] Add `NewFileID()` and `ValidateFileID()` functions to `pkg/api/id.go` following existing `resp_`/`item_` pattern with `file_` prefix

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core types and interfaces that ALL user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T003 [P] Define core types (File, FileStatus, Chunk, ExtractionResult, VectorPoint) in `pkg/files/types.go` per data-model.md
- [ ] T004 [P] Define FileStore interface (Store, Retrieve, Delete) in `pkg/files/filestore.go` per data-model.md
- [ ] T005 [P] Define FileMetadataStore interface (Save, Get, List, Delete, Update) in `pkg/files/metadata.go` per data-model.md
- [ ] T006 [P] Define ContentExtractor interface (Extract, SupportedFormats) in `pkg/files/extractor.go` per data-model.md
- [ ] T007 [P] Define Chunker interface (Chunk) in `pkg/files/chunker.go` per data-model.md
- [ ] T008 [P] Define VectorIndexer interface (UpsertPoints, DeletePointsByFile) in `pkg/files/indexer.go` per data-model.md and research R1
- [ ] T009 [P] Define VectorStoreFileRecord type and VectorStoreFileStore interface (Save, Get, List, Delete, ListByFile) in `pkg/files/vsfiles.go` per data-model.md
- [ ] T010 Implement MemoryMetadataStore (in-memory FileMetadataStore with RWMutex, user-scoped List) in `pkg/files/metadata.go`
- [ ] T011 [P] Implement MemoryVectorStoreFileStore (in-memory VectorStoreFileStore with RWMutex) in `pkg/files/vsfiles.go`

**Checkpoint**: All interfaces defined, in-memory stores ready. User story implementation can begin.

---

## Phase 3: User Story 1 - Upload and Search a Document (Priority: P1) MVP

**Goal**: A user uploads a file, adds it to a vector store, and searches its content via file_search.

**Independent Test**: Upload a text file, add to vector store, query for known content, verify chunks appear in results.

### Implementation for User Story 1

- [ ] T012 [P] [US1] Implement MemoryFileStore (in-memory FileStore for testing) in `pkg/files/filestore.go`
- [ ] T013 [P] [US1] Implement FilesystemStore (filesystem FileStore with user-scoped subdirectories, configurable base dir) in `pkg/files/filestore.go`
- [ ] T014 [P] [US1] Implement PassthroughExtractor for text, Markdown, and CSV (reads content directly, returns ExtractionResult) in `pkg/files/extractor.go`
- [ ] T015 [P] [US1] Implement FixedSizeChunker (character-based with ~4 chars/token ratio, whitespace-aware boundaries, configurable size and overlap) in `pkg/files/chunker.go`
- [ ] T016 [US1] Add UpsertPoints and DeletePointsByFile methods to QdrantBackend in `pkg/tools/builtins/filesearch/qdrant.go` implementing the VectorIndexer interface from `pkg/files/`
- [ ] T017 [US1] Implement IngestionPipeline (goroutine worker pool: extract, chunk, embed, index with status updates at each stage) in `pkg/files/pipeline.go`
- [ ] T018 [US1] Implement file upload handler (POST /files: multipart parsing, size validation, purpose validation, MIME detection, store file, save metadata) in `pkg/files/api.go`
- [ ] T019 [US1] Implement add-file-to-store handler (POST /vector_stores/{store_id}/files: validate file and store exist, create VectorStoreFileRecord, trigger pipeline) in `pkg/files/vsfiles_api.go`
- [ ] T020 [US1] Implement FilesProvider (FunctionProvider with Name="files", empty Tools, Routes for all endpoints) in `pkg/files/provider.go`
- [ ] T021 [US1] Implement New() constructor for FilesProvider (parse settings map, create FileStore, MetadataStore, extractors, chunker, pipeline, VectorStoreFileStore) in `pkg/files/provider.go`
- [ ] T022 [US1] Wire FilesProvider into server: add "files" case to `createFunctionRegistry` in `cmd/server/main.go`, pass EmbeddingClient and VectorIndexer from shared Qdrant backend
- [ ] T023 [US1] Write tests for MemoryFileStore and FilesystemStore in `pkg/files/filestore_test.go` (table-driven: store/retrieve/delete, user isolation, missing file errors)
- [ ] T024 [US1] Write tests for FixedSizeChunker in `pkg/files/chunker_test.go` (table-driven: short text, exact boundary, overlap, whitespace splitting, empty input)
- [ ] T025 [US1] Write tests for IngestionPipeline in `pkg/files/pipeline_test.go` (status transitions, failure at each stage, concurrent ingestions, worker pool limits)
- [ ] T026 [US1] Write integration test for upload-to-search flow in `pkg/files/api_test.go` (upload text file, add to store, verify status=completed, search via EmbeddingClient mock)

**Checkpoint**: User Story 1 fully functional. Upload text file, ingest into vector store, search content.

---

## Phase 4: User Story 2 - File Lifecycle Management (Priority: P1)

**Goal**: Users can list, inspect, download, and delete files. Deletion cascades to vector store chunks.

**Independent Test**: Upload files, list them, get metadata, download content, delete and verify removal from storage and vector stores.

### Implementation for User Story 2

- [ ] T027 [P] [US2] Implement list files handler (GET /files: pagination via ListOptions, user-scoped, purpose filter) in `pkg/files/api.go`
- [ ] T028 [P] [US2] Implement get file handler (GET /files/{file_id}: user-scoped metadata retrieval) in `pkg/files/api.go`
- [ ] T029 [P] [US2] Implement content download handler (GET /files/{file_id}/content: stream file bytes with Content-Type and Content-Disposition) in `pkg/files/api.go`
- [ ] T030 [US2] Implement delete file handler with cascade (DELETE /files/{file_id}: query VectorStoreFileStore.ListByFile, delete chunks via VectorIndexer.DeletePointsByFile per store, remove VectorStoreFileRecords, delete from FileStore, delete from FileMetadataStore) in `pkg/files/api.go`
- [ ] T031 [US2] Write tests for file lifecycle handlers in `pkg/files/api_test.go` (table-driven: list pagination, get metadata, download content, delete cascade, cross-user 404)

**Checkpoint**: Full file CRUD works. Upload, list, get, download, delete with cascade.

---

## Phase 5: User Story 3 - Multi-Format Document Extraction (Priority: P1)

**Goal**: Users upload PDF, DOCX, PPTX, and images. The system extracts structured content via Docling.

**Independent Test**: Upload files of each format, ingest, verify extracted text preserves structure.

### Implementation for User Story 3

- [ ] T032 [US3] Implement DoclingExtractor (HTTP client to docling-serve POST /v1/convert/file: multipart form, to_formats=md, do_ocr config, X-Api-Key auth, parse md_content from response, configurable timeout) in `pkg/files/docling.go`
- [ ] T033 [US3] Implement extractor selection logic in pipeline: route to DoclingExtractor for PDF/DOCX/PPTX/XLSX/HTML/images, PassthroughExtractor for text/Markdown/CSV. When DoclingExtractor is nil and format requires it, fail with descriptive error in `pkg/files/pipeline.go`
- [ ] T034 [US3] Write tests for DoclingExtractor in `pkg/files/docling_test.go` (table-driven with httptest.Server mock: successful Markdown extraction, OCR toggle, API key auth, timeout, error responses, empty content)
- [ ] T035 [US3] Write tests for extractor selection in `pkg/files/pipeline_test.go` (table-driven: PDF routes to docling, text routes to passthrough, unknown format error)

**Checkpoint**: Multi-format extraction works. PDF, DOCX, images via Docling; text/MD via passthrough.

---

## Phase 6: User Story 4 - Graceful Degradation Without Extraction Service (Priority: P2)

**Goal**: System works for simple formats without Docling. Complex formats fail with clear errors when Docling unavailable.

**Independent Test**: Run without Docling configured, upload text (succeeds), upload PDF (fails with descriptive error).

### Implementation for User Story 4

- [ ] T036 [US4] Add nil-safe Docling handling to pipeline extractor selection: when DoclingExtractor is nil, return `NewInvalidRequestError` with message "PDF/DOCX extraction requires an external extraction service (docling-serve)" in `pkg/files/pipeline.go`
- [ ] T037 [US4] Add connection error handling in DoclingExtractor: wrap HTTP errors with context ("extraction service unreachable"), distinguish timeout from connection refused in `pkg/files/docling.go`
- [ ] T038 [US4] Write tests for degradation scenarios in `pkg/files/pipeline_test.go` (table-driven: nil docling + text succeeds, nil docling + PDF fails with descriptive error, docling unreachable + PDF fails with connection error)

**Checkpoint**: Graceful degradation verified. Simple formats work without Docling; complex formats produce actionable errors.

---

## Phase 7: User Story 5 - Vector Store File Management (Priority: P2)

**Goal**: Users add, list, and remove files from vector stores with full status visibility.

**Independent Test**: Create store, add two files, list files in store, remove one, verify chunks deleted from removed file only.

### Implementation for User Story 5

- [ ] T039 [P] [US5] Implement list files in store handler (GET /vector_stores/{store_id}/files: pagination, status filter) in `pkg/files/vsfiles_api.go`
- [ ] T040 [US5] Implement remove file from store handler (DELETE /vector_stores/{store_id}/files/{file_id}: delete chunks via VectorIndexer, delete VectorStoreFileRecord, file itself remains) in `pkg/files/vsfiles_api.go`
- [ ] T041 [US5] Write tests for vector store file management in `pkg/files/vsfiles_api_test.go` (table-driven: add file triggers ingestion, list with status filter, remove deletes chunks only, multi-store isolation)

**Checkpoint**: Vector store file management works. Add, list, remove files per store with independent chunk lifecycle.

---

## Phase 8: User Story 6 - Batch File Ingestion (Priority: P3)

**Goal**: Users add multiple files to a vector store in one operation and track batch progress.

**Independent Test**: Upload five files, create batch, poll status until completed.

### Implementation for User Story 6

- [ ] T042 [P] [US6] Define FileBatch and FileBatchCounts types in `pkg/files/types.go` and add `NewBatchID()` to `pkg/api/id.go` with `batch_` prefix
- [ ] T043 [US6] Implement batch create handler (POST /vector_stores/{store_id}/file_batches: validate file IDs, create batch record, queue all files for ingestion via pipeline) in `pkg/files/vsfiles_api.go`
- [ ] T044 [US6] Implement batch status handler (GET /vector_stores/{store_id}/file_batches/{batch_id}: compute file counts from VectorStoreFileStore) in `pkg/files/vsfiles_api.go`
- [ ] T045 [US6] Write tests for batch operations in `pkg/files/vsfiles_api_test.go` (batch creation, status polling, partial failures, all completed)

**Checkpoint**: Batch ingestion works. Bulk add with progress tracking.

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Observability, edge cases, and hardening

- [ ] T046 [P] Register Prometheus metrics (files_uploaded_total, ingestion_duration_seconds, ingestion_status_total, extraction_duration_seconds, chunking_chunks_total) in `pkg/files/provider.go` via Collectors() method
- [ ] T047 [P] Add metrics instrumentation to pipeline stages (record extraction duration, ingestion duration, chunk count, upload count by MIME type) in `pkg/files/pipeline.go` and `pkg/files/api.go`
- [ ] T048 Handle edge case: file deleted during active ingestion (check file existence before each pipeline stage, cancel and clean up partial chunks) in `pkg/files/pipeline.go`
- [ ] T049 Handle edge case: empty extraction result (set status=failed with "no extractable content found") in `pkg/files/pipeline.go`
- [ ] T050 [P] Write tests for edge cases in `pkg/files/pipeline_test.go` (delete during ingestion, empty extraction, embedding service down)
- [ ] T051 Verify `go vet` and `go test ./pkg/files/...` pass with no errors

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (T001, T002)
- **US1 (Phase 3)**: Depends on Phase 2 (all interfaces and in-memory stores)
- **US2 (Phase 4)**: Depends on US1 (needs upload handler and FileStore)
- **US3 (Phase 5)**: Depends on US1 (needs pipeline and extractor selection)
- **US4 (Phase 6)**: Depends on US3 (builds on extractor selection logic)
- **US5 (Phase 7)**: Depends on US1 (needs add-file handler and VectorStoreFileStore)
- **US6 (Phase 8)**: Depends on US5 (extends vector store file operations)
- **Polish (Phase 9)**: Depends on US1-US5 (metrics instrument all paths)

### User Story Dependencies

- **US1 (P1)**: After Phase 2. No story dependencies. MVP.
- **US2 (P1)**: After US1 (shares upload handler and stores). Independent test: list/get/delete without search.
- **US3 (P1)**: After US1 (needs pipeline). Independent test: upload + extract multiple formats.
- **US4 (P2)**: After US3 (extends extractor selection). Independent test: run without Docling.
- **US5 (P2)**: After US1 (needs add-file handler). Can run parallel with US2-US4.
- **US6 (P3)**: After US5 (extends VS file management). Independent test: batch add + poll.

### Parallel Opportunities

- **Phase 2**: T003-T009 are all parallel (different files, independent interfaces)
- **Phase 3**: T012-T015 are parallel (different implementations), T023-T024 parallel (different test files)
- **Phase 4**: T027-T029 are parallel (independent handler methods)
- **US2 and US5** can run in parallel after US1 (no interdependency)

---

## Parallel Example: User Story 1

```bash
# After Phase 2, launch parallel implementation tasks:
Task: "Implement MemoryFileStore in pkg/files/filestore.go"          # T012
Task: "Implement FilesystemStore in pkg/files/filestore.go"          # T013
Task: "Implement PassthroughExtractor in pkg/files/extractor.go"     # T014
Task: "Implement FixedSizeChunker in pkg/files/chunker.go"           # T015

# After T012-T016, launch sequential pipeline + handlers:
Task: "Implement IngestionPipeline in pkg/files/pipeline.go"         # T017
Task: "Implement upload handler in pkg/files/api.go"                 # T018
Task: "Implement add-file-to-store handler in pkg/files/vsfiles_api.go" # T019

# After handlers, launch parallel tests:
Task: "Tests for FileStore in pkg/files/filestore_test.go"           # T023
Task: "Tests for Chunker in pkg/files/chunker_test.go"               # T024
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T002)
2. Complete Phase 2: Foundational (T003-T011)
3. Complete Phase 3: User Story 1 (T012-T026)
4. **STOP and VALIDATE**: Upload text file, add to vector store, search content via file_search
5. Deploy if ready (text-only ingestion is immediately useful)

### Incremental Delivery

1. Setup + Foundational: Foundation ready
2. US1 (upload + search text): MVP, deployable
3. US2 (list/get/download/delete): Full file management
4. US3 (Docling extraction): Multi-format support
5. US4 (graceful degradation): Production hardening
6. US5 (VS file management): Fine-grained control
7. US6 (batch ingestion): Bulk operations
8. Polish: Metrics, edge cases

### Parallel Team Strategy

With multiple developers after Phase 2:
- Developer A: US1 (MVP path, blocks others)
- After US1:
  - Developer A: US3 + US4 (extraction pipeline)
  - Developer B: US2 + US5 (file/store management)
  - Developer C: US6 (batch operations)

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story
- Each user story is independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- S3 FileStore (FR-010) deferred to a follow-up since it requires external SDK dependency

## Beads Task Management

This project uses beads (`bd`) for persistent task tracking across sessions:
- Run `/sdd:beads-task-sync` to create bd issues from this file
- `bd ready --json` returns unblocked tasks (dependencies resolved)
- `bd close <id>` marks a task complete (use `-r "reason"` for close reason, NOT `--comment`)
- `bd comments add <id> "text"` adds a detailed comment to an issue
- `bd sync` persists state to git
- `bd create "DISCOVERED: [short title]" --labels discovered` tracks new work
  - Keep titles crisp (under 80 chars); add details via `bd comments add <id> "details"`
- Run `/sdd:beads-task-sync --reverse` to update checkboxes from bd state
- **Always use `jq` to parse bd JSON output, NEVER inline Python one-liners**
