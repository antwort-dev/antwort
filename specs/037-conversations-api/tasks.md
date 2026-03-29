# Tasks: Conversations API

**Input**: Design documents from `/specs/037-conversations-api/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Organization**: Tasks grouped by user story.

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup

**Purpose**: Types, ID generation, interface definition

- [x] T001 [P] Add Conversation and ConversationList types to `pkg/api/types.go` (ID, Object, Name, UserID, CreatedAt, UpdatedAt, Metadata fields)
- [x] T002 [P] Add `NewConversationID()` and `ValidateConversationID()` with `conv_` prefix to `pkg/api/id.go`
- [x] T003 Add ConversationStore interface (SaveConversation, GetConversation, DeleteConversation, ListConversations, ListConversationItems) to `pkg/transport/handler.go`
- [x] T004 Add `conversation_id` optional field to `CreateResponseRequest` and `Response` types in `pkg/api/types.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: In-memory store implementation

- [x] T005 Implement in-memory ConversationStore (RWMutex, map-based, tenant-scoped, soft delete, pagination) in `pkg/storage/memory/conversations.go`
- [x] T006 Write tests for in-memory ConversationStore in `pkg/storage/memory/conversations_test.go` (table-driven: CRUD, tenant isolation, soft delete, pagination, item management)

**Checkpoint**: ConversationStore works in isolation with tests.

---

## Phase 3: User Story 1 - Create and Use Conversations (Priority: P1) MVP

**Goal**: Create conversations and link responses to them.

- [x] T007 [US1] Implement conversation HTTP handlers (create, get) in `pkg/transport/http/conversations.go`
- [x] T008 [US1] Register conversation routes in HTTP adapter mux (POST/GET /v1/conversations, GET /v1/conversations/{id}) in `pkg/transport/http/adapter.go`
- [x] T009 [US1] Accept optional ConversationStore in engine constructor, store on Engine struct in `pkg/engine/engine.go`
- [x] T010 [US1] In engine CreateResponse: when conversation_id is set and previous_response_id is not, load conversation items and convert to ProviderMessages for history in `pkg/engine/engine.go`
- [x] T011 [US1] After response completion: if conversation_id is set, add input + output items to the conversation via ConversationStore in `pkg/engine/engine.go`
- [x] T012 [US1] Wire ConversationStore into server: pass memory store to adapter and engine in `cmd/server/main.go`
- [x] T013 [US1] Write integration tests for create conversation and response with conversation_id in `pkg/transport/http/conversations_test.go`

**Checkpoint**: Create conversation, send response with conversation_id, history used as context.

---

## Phase 4: User Story 2 - List and Browse (Priority: P1)

**Goal**: List conversations with pagination.

- [x] T014 [US2] Implement list conversations handler (GET /v1/conversations: pagination, tenant-scoped) in `pkg/transport/http/conversations.go`
- [x] T015 [US2] Write tests for list with pagination and ordering in `pkg/transport/http/conversations_test.go`

**Checkpoint**: List conversations with cursor-based pagination.

---

## Phase 5: User Story 3 - Item Management (Priority: P2)

**Goal**: List and add items within conversations.

- [x] T016 [US3] Implement list items handler (GET /v1/conversations/{id}/items: pagination) in `pkg/transport/http/conversations.go`
- [x] T017 [US3] Implement add item handler (POST /v1/conversations/{id}/items: add message item directly) in `pkg/transport/http/conversations.go`
- [x] T018 [US3] Register item routes in adapter mux in `pkg/transport/http/adapter.go`
- [x] T019 [US3] Write tests for item listing and direct item addition in `pkg/transport/http/conversations_test.go`

**Checkpoint**: Add items directly, list items with pagination.

---

## Phase 6: User Story 4 - Delete Conversations (Priority: P2)

**Goal**: Delete conversations (soft delete, responses preserved).

- [x] T020 [US4] Implement delete conversation handler (DELETE /v1/conversations/{id}) in `pkg/transport/http/conversations.go`
- [x] T021 [US4] Write tests for delete (soft delete, responses still accessible) in `pkg/transport/http/conversations_test.go`

**Checkpoint**: Delete conversation, verify responses still accessible by ID.

---

## Phase 7: User Story 5 - Backward Compatibility (Priority: P1)

**Goal**: Verify previous_response_id works unchanged.

- [x] T022 [US5] Write test confirming previous_response_id without conversation_id works identically to pre-existing behavior in `pkg/engine/loop_test.go`
- [x] T023 [US5] Write test confirming both conversation_id and previous_response_id set: previous_response_id provides history in `pkg/engine/loop_test.go`

**Checkpoint**: Existing clients unaffected.

---

## Phase 8: Documentation (per constitution v1.6.0)

**Purpose**: API reference and tutorial pages.

- [x] T024 [P] Create conversations API reference page in `docs/modules/reference/pages/conversations-api.adoc` (all 6 endpoints with schemas)
- [x] T025 [P] Update existing API reference to document conversation_id field on create response in `docs/modules/reference/pages/api-reference.adoc`
- [x] T026 [P] Create conversations tutorial page in `docs/modules/tutorial/pages/conversations.adoc` (create, multi-turn, list history)
- [x] T027 [P] Update reference and tutorial nav.adoc files

---

## Phase 9: Polish

- [x] T028 Verify `go vet ./pkg/api/... ./pkg/transport/... ./pkg/engine/... ./pkg/storage/...` pass
- [x] T029 Verify `go test ./pkg/transport/http/... ./pkg/storage/memory/... ./pkg/engine/...` pass

---

## Dependencies & Execution Order

- **Phase 1**: No dependencies, start immediately
- **Phase 2**: Depends on Phase 1 (needs types and interface)
- **US1 (Phase 3)**: Depends on Phase 2 (needs store implementation)
- **US2 (Phase 4)**: Depends on US1 (needs routes registered)
- **US3 (Phase 5)**: Depends on US1 (needs conversation creation)
- **US4 (Phase 6)**: Depends on US1 (needs conversations to delete)
- **US5 (Phase 7)**: Can run parallel with US2-US4 (tests existing behavior)
- **Docs (Phase 8)**: Can run parallel after US1
- **Polish (Phase 9)**: Depends on all prior phases

## Implementation Strategy

### MVP: Create + Use (US1 only)

1. Phase 1: Types and interface (T001-T004)
2. Phase 2: In-memory store (T005-T006)
3. Phase 3: Create, get, engine integration (T007-T013)
4. Validate: create conversation, send response, history works

### Full Delivery

1. Phases 1-3: MVP
2. Phases 4-6: List, items, delete
3. Phase 7: Backward compatibility tests
4. Phase 8: Documentation
5. Phase 9: Verification
