# Tasks: Annotation & Citation Generation

**Input**: Design documents from `/specs/035-annotations/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md

**Tests**: Tests are included inline with each user story, following project testing standards.

**Organization**: Tasks grouped by user story. Each story is independently testable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2)

## Phase 1: Setup

**Purpose**: Extend existing types and add annotation infrastructure

- [ ] T001 [P] Extend `Annotation` struct with `FileID`, `Quote`, `URL`, `Title` fields (all `omitempty`) in `pkg/api/types.go`
- [ ] T002 [P] Add `Metadata map[string]string` field to `ToolResult` struct in `pkg/tools/executor.go`
- [ ] T003 [P] Add `EventAnnotationAdded = "response.output_text.annotation.added"` constant to `pkg/api/events.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Annotation generator and source context tracking

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T004 Define `SourceContext` struct (ToolName, FileID, URL, Title, Content) and `AnnotationGenerator` interface (Generate method) in `pkg/engine/annotations.go`
- [ ] T005 Implement `SubstringMatcher` (finds longest common substrings between source content and output text, builds Annotation entries with character positions, configurable minimum match length) in `pkg/engine/annotations.go`
- [ ] T006 Write tests for `SubstringMatcher` in `pkg/engine/annotations_test.go` (table-driven: exact match, partial match, no match fallback, multiple sources, overlapping ranges, empty text, unicode)

**Checkpoint**: Annotation generator works in isolation with test fixtures.

---

## Phase 3: User Story 1 - File Search Citations (Priority: P1) MVP

**Goal**: file_search tool results produce file_citation annotations on the response output text.

**Independent Test**: Upload a document, search it, verify file_citation annotations appear with correct file_id and quote.

### Implementation for User Story 1

- [ ] T007 [US1] Populate `ToolResult.Metadata` in file_search provider: set `tool=file_search`, `file_id` from `SearchMatch.Metadata["file_id"]`, `content` from `SearchMatch.Content` in `pkg/tools/builtins/filesearch/provider.go`
- [ ] T008 [US1] Add source context extraction helper: convert `ToolResult.Metadata` into `[]SourceContext` in `pkg/engine/annotations.go`
- [ ] T009 [US1] Integrate annotation generator into non-streaming engine loop: after final provider response, extract source contexts from tool results, generate annotations, attach to `OutputContentPart.Annotations` in `pkg/engine/loop.go`
- [ ] T010 [US1] Accept optional `AnnotationGenerator` in engine constructor, wire through to the loop in `pkg/engine/engine.go`
- [ ] T011 [US1] Write tests for file_citation generation in `pkg/engine/annotations_test.go` (file_search metadata produces file_citation with file_id and quote, no metadata produces no annotations)
- [ ] T012 [US1] Write integration test: mock provider returns text containing file_search content, verify response has file_citation annotations with correct positions in `pkg/engine/loop_test.go`

**Checkpoint**: Non-streaming responses with file_search include file_citation annotations.

---

## Phase 4: User Story 2 - Web Search Citations (Priority: P1)

**Goal**: web_search tool results produce url_citation annotations on the response output text.

**Independent Test**: Trigger web_search, verify url_citation annotations appear with URL and title.

### Implementation for User Story 2

- [ ] T013 [US2] Populate `ToolResult.Metadata` in web_search provider: set `tool=web_search`, `url`, `title`, `content` from `SearchResult` fields in `pkg/tools/builtins/websearch/provider.go`
- [ ] T014 [US2] Write tests for url_citation generation in `pkg/engine/annotations_test.go` (web_search metadata produces url_citation with url and title)
- [ ] T015 [US2] Write integration test: mock provider returns text using web_search results, verify response has url_citation annotations in `pkg/engine/loop_test.go`

**Checkpoint**: Non-streaming responses with web_search include url_citation annotations.

---

## Phase 5: User Story 3 - Mixed Citations (Priority: P2)

**Goal**: Responses using both file_search and web_search produce both annotation types correctly.

**Independent Test**: Trigger both tools, verify mixed file_citation and url_citation annotations coexist.

### Implementation for User Story 3

- [ ] T016 [US3] Write test for mixed annotations in `pkg/engine/annotations_test.go` (both file_search and web_search sources produce both annotation types, non-overlapping ranges)
- [ ] T017 [US3] Handle annotation range conflict resolution in `SubstringMatcher`: when sources overlap, prefer the longest match, split at sentence boundaries in `pkg/engine/annotations.go`

**Checkpoint**: Mixed citation responses produce correct, non-overlapping annotations.

---

## Phase 6: User Story 4 - No Citations Without Tools (Priority: P2)

**Goal**: Responses without search tools produce empty annotations arrays.

**Independent Test**: Generate a response without any tools, verify annotations is empty.

### Implementation for User Story 4

- [ ] T018 [US4] Write test confirming no annotations when no tool results have metadata in `pkg/engine/annotations_test.go`
- [ ] T019 [US4] Write test confirming no annotations when tool results exist but LLM output doesn't match any source content in `pkg/engine/annotations_test.go`

**Checkpoint**: No false citations generated.

---

## Phase 7: Streaming Support

**Purpose**: SSE event emission for annotations during streaming responses

- [ ] T020 Add `MarshalJSON` case for `EventAnnotationAdded` in `pkg/api/events.go` (payload: annotation_index, annotation object with all fields)
- [ ] T021 Integrate annotation generation into streaming engine loop: after text accumulation in `consumeStreamTurn()`, generate annotations, emit `EventAnnotationAdded` events after `EventOutputTextDone` and before `EventContentPartDone` in `pkg/engine/loop.go`
- [ ] T022 Write test for streaming annotation events in `pkg/engine/loop_test.go` (verify EventAnnotationAdded emitted with correct sequence numbering and annotation data)

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Edge cases, documentation, verification

- [ ] T023 [P] Handle edge case: source content in tool result but no match in output (annotate entire output segment with source reference, no quote) in `pkg/engine/annotations.go`
- [ ] T024 [P] Handle edge case: same source cited multiple times (each occurrence gets its own annotation with distinct positions) in `pkg/engine/annotations.go`
- [ ] T025 Add Files API reference page update: document annotation fields in response output in `docs/modules/reference/pages/files-api.adoc`
- [ ] T026 Add annotation documentation to API reference: document file_citation and url_citation annotation types in `docs/modules/reference/pages/api-reference.adoc`
- [ ] T027 Verify `go vet ./pkg/api/... ./pkg/engine/... ./pkg/tools/...` pass with no errors

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately. T001-T003 all parallel.
- **Foundational (Phase 2)**: Depends on Phase 1 (needs extended types)
- **US1 (Phase 3)**: Depends on Phase 2 (needs annotation generator)
- **US2 (Phase 4)**: Depends on Phase 2 (needs annotation generator). Can run parallel with US1.
- **US3 (Phase 5)**: Depends on US1 and US2 (tests mixed annotations)
- **US4 (Phase 6)**: Depends on Phase 2 only. Can run parallel with US1/US2.
- **Streaming (Phase 7)**: Depends on US1 (needs non-streaming working first)
- **Polish (Phase 8)**: Depends on all prior phases

### Parallel Opportunities

- **Phase 1**: T001, T002, T003 all parallel (different files)
- **US1 and US2**: Can run in parallel after Phase 2 (different providers, same generator)
- **US4**: Can run parallel with US1/US2 (only tests, no implementation)
- **Phase 8**: T023, T024 parallel (independent edge cases)

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Phase 1: Extend types (T001-T003)
2. Phase 2: Annotation generator (T004-T006)
3. Phase 3: File search citations (T007-T012)
4. **STOP and VALIDATE**: Response with file_search has file_citation annotations

### Incremental Delivery

1. Setup + Foundational: Types and generator ready
2. US1: File citations in non-streaming responses
3. US2: Web citations (parallel with US1)
4. US3+US4: Mixed and negative cases
5. Streaming: SSE annotation events
6. Polish: Edge cases, docs

---

## Notes

- All changes are in existing packages (pkg/api, pkg/engine, pkg/tools). No new packages.
- The `AnnotationGenerator` is injected as nil-safe optional (constitution Principle III)
- S3 FileStore (FR-010 from spec 034) is not affected by this feature
- Annotation generation is best-effort: failures are logged, never fatal

## Beads Task Management

This project uses beads (`bd`) for persistent task tracking across sessions:
- Run `/sdd:beads-task-sync` to create bd issues from this file
- `bd ready --json` returns unblocked tasks (dependencies resolved)
- `bd close <id>` marks a task complete (use `-r "reason"` for close reason)
- `bd sync` persists state to git
