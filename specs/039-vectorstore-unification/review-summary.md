# Review Summary: 039-vectorstore-unification

**Reviewed**: 2026-03-03 | **Verdict**: APPROVED - Ready for implementation | **Spec Version**: Draft

## For Reviewers

This spec unifies the split vector store interfaces into a single `Backend` interface in `pkg/vectorstore/`, migrates Qdrant, and adds pgvector (PostgreSQL) and in-memory backends. The refactoring eliminates an import cycle artifact and enables CI-friendly vector testing.

### Key Areas to Review

1. **Interface unification (FR-001/FR-003)**: 5-method `Backend` interface replaces `VectorStoreBackend` (3 methods) + `VectorIndexer` (2 methods). At the constitution's method limit.

2. **Qdrant migration (FR-004)**: Zero behavioral changes required. Moving code + updating imports. All existing tests must pass unchanged.

3. **pgvector connection sharing (FR-007)**: The pgvector backend reuses the existing `pgxpool.Pool` from the response store. This is the key infrastructure win.

4. **pgvector-go dependency (R6)**: External dependency in adapter package only, per constitution Principle II.

## Coverage Matrix

| FR | Description | Task(s) | Status |
|----|-------------|---------|--------|
| FR-001 | Unified interface | T001 | COVERED |
| FR-002 | 5 methods max | T001 | COVERED |
| FR-003 | No import cycles | T004, T005, T007 | COVERED |
| FR-004 | Qdrant migration, zero changes | T003, T009 | COVERED |
| FR-005 | pgvector backend | T012 | COVERED |
| FR-006 | Cosine similarity | T012 | COVERED |
| FR-007 | Connection pool reuse | T012, T016 | COVERED |
| FR-008 | Schema creation at startup | T012 | COVERED |
| FR-009 | pgvector extension validation | T013 | COVERED |
| FR-010 | In-memory backend | T010 | COVERED |
| FR-011 | In-memory cosine search | T010, T011 | COVERED |
| FR-012 | CI-friendly | T011 | COVERED |
| FR-013 | Backend selection via config | T015 | COVERED |
| FR-014 | Default Qdrant | T015 | COVERED |
| FR-015 | Startup validation | T013, T015 | COVERED |
| FR-016 | Reference docs | T018 | COVERED |
| FR-017 | Developer guide | T019 | COVERED |
| FR-018 | Updated RAG docs | T018 | COVERED |

**Coverage**: 18/18 FRs covered. All 6 success criteria verifiable.

## Task Summary

| Phase | Story | Tasks | Parallel |
|-------|-------|-------|----------|
| 1. Unified Interface | - | 2 | Sequential |
| 2. Qdrant Migration | US3 | 7 | Sequential |
| 3. In-Memory | US2 | 2 | 1 parallel |
| 4. pgvector | US1 | 3 | Sequential |
| 5. Backend Selection | US4 | 3 | Sequential |
| 6. Docs | - | 3 | 2 parallel |
| 7. Polish | - | 2 | Sequential |
| **Total** | | **22** | |

## Key Strengths

- Clean refactoring path: unified interface first, then migrate, then add backends
- In-memory backend unblocks CI vector testing (currently skipped)
- pgvector reuses existing PostgreSQL infrastructure (zero new infra)
- MVP path (unified + in-memory) delivers value in 12 tasks before pgvector

## Risks

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Import cycle during migration | Low | Compile-time checks at each step (T009) |
| pgvector extension missing | Expected | Startup validation with clear error (T013) |
| In-memory search accuracy | Low | Brute-force cosine is exact, no approximation |

## Red Flags

None.

## Reviewer Guidance

1. Verify the unified `Backend` interface has exactly 5 methods
2. After Phase 2, run `go test ./...` to confirm zero regressions from Qdrant migration
3. Verify in-memory cosine similarity returns results in correct score order
4. Check pgvector schema uses HNSW index (not IVFFlat) for better recall
5. Confirm the pgvector backend shares `*pgxpool.Pool` with response store
