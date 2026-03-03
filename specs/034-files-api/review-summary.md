# Review Summary: 034-files-api

**Reviewed**: 2026-03-02 | **Verdict**: APPROVED - Ready for implementation | **Spec Version**: Draft

## For Reviewers

This spec adds a Files API and document ingestion pipeline to antwort. Users upload files via REST, content is extracted (via Docling for complex formats, passthrough for simple ones), chunked, embedded, and indexed in vector stores. This closes the RAG ingestion loop that Spec 018 (File Search) left open.

### Key Areas to Review

1. **VectorIndexer interface design** (plan.md D2, research.md R1): New interface in `pkg/files/` rather than extending `VectorStoreBackend`. The Qdrant adapter implements both. Verify this avoids import cycles at wiring time.

2. **Ingestion pipeline concurrency** (plan.md D5, research.md R8): Worker pool pattern with configurable concurrency (default 4). Verify goroutine lifecycle management and status update atomicity.

3. **FunctionProvider without tools** (plan.md D3, research.md R6): `FilesProvider` registers as FunctionProvider with empty `Tools()`, contributing only HTTP routes. This reuses auth middleware and metrics wrapping from the registry.

4. **Docling adapter** (plan.md D4, research.md R4): Synchronous HTTP call within async pipeline goroutine. Configurable timeout (300s). Verify error wrapping distinguishes timeout from connection refused.

5. **File deletion cascade** (plan.md D6): Delete file, query all stores containing it, remove chunks per store, then remove records and content. Verify ordering prevents orphaned chunks.

## Coverage Matrix

### Functional Requirements to Tasks

| FR | Description | Task(s) | Status |
|----|-------------|---------|--------|
| FR-001 | Files API endpoints (upload, list, get, content, delete) | T018, T027, T028, T029, T030 | COVERED |
| FR-002 | File ID format (file_ prefix) | T002 | COVERED |
| FR-003 | File metadata fields | T003 | COVERED |
| FR-004 | File status values | T003 | COVERED |
| FR-005 | User-scoped file access | T010, T018, T027-T031 | COVERED |
| FR-006 | Max upload size | T018, T021 | COVERED |
| FR-007 | Purpose field values | T018 | COVERED |
| FR-008 | FileStore interface | T004 | COVERED |
| FR-009 | Filesystem backend | T013 | COVERED |
| FR-010 | S3 backend | deferred | NOTED (see notes) |
| FR-011 | Memory backend | T012 | COVERED |
| FR-012 | User-scoped storage isolation | T013, T023 | COVERED |
| FR-013 | ContentExtractor interface | T006 | COVERED |
| FR-014 | Docling adapter | T032 | COVERED |
| FR-015 | Docling Markdown output | T032 | COVERED |
| FR-016 | Docling configuration | T032, T021 | COVERED |
| FR-017 | Passthrough extractor | T014 | COVERED |
| FR-018 | Complex format error without extractor | T033, T036 | COVERED |
| FR-019 | Chunker interface | T007 | COVERED |
| FR-020 | Fixed-size chunker | T015 | COVERED |
| FR-021 | Chunk metadata (index, offsets) | T003, T015 | COVERED |
| FR-022 | Ingestion pipeline orchestration | T017 | COVERED |
| FR-023 | Async ingestion | T017, T019 | COVERED |
| FR-024 | Status updates at each stage | T017 | COVERED |
| FR-025 | Error detail recording | T017, T049 | COVERED |
| FR-026 | Reuse EmbeddingClient (Spec 018) | T017, T022 | COVERED |
| FR-027 | Reuse VectorStoreBackend (Spec 018) | T016, T022 | COVERED |
| FR-028 | VS file management endpoints | T019, T039, T040 | COVERED |
| FR-029 | VS file counts by status | T039 | COVERED |
| FR-030 | Batch endpoint (P3) | T042-T045 | COVERED |
| FR-031 | Configuration system | T021 | COVERED |
| FR-032 | Env var overrides | T021 | COVERED |
| FR-033 | Prometheus metrics | T046, T047 | COVERED |

**Coverage**: 32/33 FRs covered. FR-010 (S3 backend) explicitly deferred to follow-up.

### User Stories to Tasks

| Story | Priority | Tasks | Test Tasks |
|-------|----------|-------|------------|
| US1 - Upload and Search | P1 | T012-T022 | T023-T026 |
| US2 - File Lifecycle | P1 | T027-T030 | T031 |
| US3 - Multi-Format Extraction | P1 | T032-T033 | T034-T035 |
| US4 - Graceful Degradation | P2 | T036-T037 | T038 |
| US5 - VS File Management | P2 | T039-T040 | T041 |
| US6 - Batch Ingestion | P3 | T042-T044 | T045 |

### Success Criteria Coverage

| SC | Description | Verified By |
|----|-------------|-------------|
| SC-001 | Upload PDF, search within 60s | T026 (integration test) |
| SC-002 | Multi-format extraction with structure | T034, T035 |
| SC-003 | User isolation at every layer | T023, T031 (cross-user 404) |
| SC-004 | Graceful degradation | T038 |
| SC-005 | Status trackable with error detail | T025 (status transitions) |
| SC-006 | file_search unchanged | T026 (uses existing EmbeddingClient) |
| SC-007 | File ops under 2s | Implicit (in-memory stores, no blocking I/O) |

## Task Summary

| Phase | Story | Tasks | Tests | Parallel |
|-------|-------|-------|-------|----------|
| 1. Setup | - | 2 | 0 | 1 parallel |
| 2. Foundational | - | 9 | 0 | 7 parallel, 2 sequential |
| 3. US1 (P1) | Upload+Search | 11 | 4 | 4 parallel impl, 2 parallel tests |
| 4. US2 (P1) | File Lifecycle | 4 | 1 | 3 parallel handlers |
| 5. US3 (P1) | Multi-Format | 2 | 2 | Sequential |
| 6. US4 (P2) | Degradation | 2 | 1 | Sequential |
| 7. US5 (P2) | VS File Mgmt | 2 | 1 | 1 parallel |
| 8. US6 (P3) | Batch | 3 | 1 | 1 parallel |
| 9. Polish | - | 6 | 0 | 2 parallel |
| **Total** | | **41** | **10** | **51 total** |

## Key Strengths

- Clean interface-first design with 6 small interfaces (all within constitution's 1-5 method limit)
- VectorIndexer keeps filesearch interface stable while adding write operations
- Incremental delivery: MVP (US1) delivers text ingestion with just Phase 1-3
- Passthrough extractor enables useful operation without Docling
- Thorough test coverage with table-driven tests for each component

## Risks

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Docling API changes | Low | Adapter isolated in single file; version-pin container image |
| Worker pool goroutine leaks | Medium | T048 handles cancellation; T025 tests concurrent ingestion |
| Character-based chunking inaccuracy | Low | Acceptable for v1; Chunker interface allows swapping later |
| Qdrant implementing two interfaces | Low | Same struct, additional methods only; tested in T016 |
| S3 backend deferred | N/A | Explicitly deferred; filesystem + memory cover dev and test |

## Red Flags

None found. All tasks:
- Have exact file paths
- Follow the checklist format (checkbox, ID, labels)
- Are scoped to a single concern
- Have clear acceptance criteria via test tasks

## Notes

- **FR-010 (S3 backend)** explicitly deferred per tasks.md note. This is acceptable because filesystem covers Kubernetes (via PVC) and memory covers testing. S3 can be added as a follow-up task when needed.
- **Duplicate edge case coverage**: T033 (extractor selection) and T036 (nil Docling handling) overlap on the "no Docling + complex format" scenario. This is intentional (US3 establishes the routing logic, US4 hardens the error paths).
- **T012 and T013 in same file**: Both MemoryFileStore and FilesystemStore are in `filestore.go`. The [P] marker is valid because they implement independent code paths, but care needed to avoid merge conflicts if truly parallel.

## Reviewer Guidance

When reviewing the implementation PR:
1. Verify all 6 interfaces match the data-model.md signatures
2. Check that VectorIndexer is implemented on QdrantBackend without modifying VectorStoreBackend
3. Confirm pipeline status transitions match the state diagram (uploaded, processing, completed, failed)
4. Verify user-scoped isolation in MemoryMetadataStore.List and all HTTP handlers
5. Test graceful degradation: start server without docling_url, upload text (should work), upload PDF (should fail with clear error)
6. Run `go vet ./pkg/files/...` to verify no import cycles between pkg/files and filesearch
