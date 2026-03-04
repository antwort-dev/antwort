# Tasks: Resource Ownership

**Input**: Design documents from `/specs/040-resource-ownership/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md

**Tests**: Included. Ownership is a security-critical feature requiring integration tests.

**Organization**: Tasks grouped by user story for independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: Auth infrastructure changes needed before any ownership filtering

- [ ] T001 (antwort-ggy4.1) Add `RolesClaim` config field and role extraction logic to JWT authenticator in `pkg/auth/jwt/jwt.go`
- [ ] T002 (antwort-ggy4.2) [P] Add `IsAdmin(identity *Identity, adminRole string) bool` helper to `pkg/auth/auth.go` that checks `Identity.Metadata["roles"]` for the configured admin role name
- [ ] T003 (antwort-ggy4.3) [P] Add `ownerFromCtx(ctx) string` helper function to `pkg/storage/owner.go` that extracts `Identity.Subject` from context (follows `userFromCtx` pattern from Files API)
- [ ] T004 (antwort-ggy4.4) [P] Add `isAdmin(ctx, adminRole) bool` helper to `pkg/storage/owner.go` that combines identity extraction with admin role check

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Database migration and configuration that MUST be complete before user story work

**Warning**: No user story work can begin until this phase is complete

- [ ] T005 (antwort-bon0.1) Create PostgreSQL migration `pkg/storage/postgres/migrations/002_add_owner.sql` adding `owner TEXT NOT NULL DEFAULT ''` column and index to responses, conversations, and vector_stores tables
- [ ] T006 (antwort-bon0.2) Add `authorization.admin_role` and `jwt.roles_claim` fields to configuration struct in `pkg/config/` (or wherever config is defined)

**Checkpoint**: Foundation ready, owner helpers and migration in place

---

## Phase 3: User Story 1 - User Data Isolation (Priority: P1)

**Goal**: Alice and Bob on the same instance cannot see each other's responses, conversations, vector stores, or files.

**Independent Test**: Authenticate as Alice, create a response. Authenticate as Bob, list responses. Alice's response must not appear.

### Implementation for User Story 1

- [ ] T007 (antwort-z2r2.1) [P] [US1] Add `owner` field to memory store `entry` struct and set from `ownerFromCtx(ctx)` in `SaveResponse` in `pkg/storage/memory/memory.go`
- [ ] T008 (antwort-z2r2.2) [P] [US1] Add `owner` field to memory store `convEntry` struct and set from `ownerFromCtx(ctx)` in `SaveConversation` in `pkg/storage/memory/conversations.go`
- [ ] T009 (antwort-z2r2.3) [US1] Add owner filtering to `GetResponse` in `pkg/storage/memory/memory.go`: extract owner from context, return `ErrNotFound` if owner mismatch (skip if owner empty or no identity)
- [ ] T010 (antwort-z2r2.4) [US1] Add owner filtering to `ListResponses` in `pkg/storage/memory/memory.go`: filter results by owner (skip if no identity)
- [ ] T011 (antwort-z2r2.5) [US1] Add owner filtering to `DeleteResponse` in `pkg/storage/memory/memory.go`: return `ErrNotFound` if owner mismatch
- [ ] T012 (antwort-z2r2.6) [US1] Add owner filtering to `GetResponseForChain` in `pkg/storage/memory/memory.go`: return `ErrNotFound` if `previous_response_id` belongs to different owner
- [ ] T013 (antwort-z2r2.7) [US1] Add owner filtering to `GetInputItems` in `pkg/storage/memory/memory.go`: check parent response owner
- [ ] T014 (antwort-z2r2.8) [US1] Add owner filtering to `GetConversation`, `ListConversations`, `DeleteConversation` in `pkg/storage/memory/conversations.go`
- [ ] T015 (antwort-z2r2.9) [US1] Add owner filtering to `AddItems` and `ListItems` in `pkg/storage/memory/conversations.go`: verify conversation owner before allowing item operations
- [ ] T016 (antwort-z2r2.10) [US1] Add `owner` column to PostgreSQL `SaveResponse` insert and owner filtering to `getResponse`, `ListResponses`, `DeleteResponse` in `pkg/storage/postgres/postgres.go`
- [ ] T017 (antwort-z2r2.11) [US1] Add owner filtering to vector store management endpoints (create sets owner, list/get/delete filter by owner) in `pkg/transport/http/vectorstores.go` or equivalent handler
- [ ] T018 (antwort-z2r2.12) [US1] Add debug-level logging for ownership denials via `slog.Debug` in each store method that filters by owner (subject, resource ID, operation)

### Tests for User Story 1

- [ ] T019 (antwort-z2r2.13) [US1] Integration test: two users creating and listing responses in isolation in `tests/integration/ownership_test.go`
- [ ] T020 (antwort-z2r2.14) [P] [US1] Integration test: two users creating and listing conversations in isolation in `tests/integration/ownership_test.go`
- [ ] T021 (antwort-z2r2.15) [P] [US1] Integration test: user cannot GET another user's response by ID (returns 404) in `tests/integration/ownership_test.go`
- [ ] T022 (antwort-z2r2.16) [P] [US1] Integration test: `previous_response_id` chaining fails with 404 when referencing another user's response in `tests/integration/ownership_test.go`

**Checkpoint**: User data isolation fully functional and tested

---

## Phase 4: User Story 2 - Admin Read and Delete Override (Priority: P1)

**Goal**: Admin user can read and delete any resource within their tenant. Cannot modify other users' resources. Cannot access resources in other tenants.

**Independent Test**: Authenticate as admin Carol, list responses. All responses from Carol's tenant appear. Carol can delete Alice's response. Carol cannot add items to Alice's conversation.

### Implementation for User Story 2

- [ ] T023 (antwort-mvz1.1) [US2] Add admin bypass to owner filtering in `GetResponse`, `ListResponses`, `DeleteResponse` in `pkg/storage/memory/memory.go`: if `isAdmin(ctx, adminRole)` and same tenant, skip owner check for read/delete
- [ ] T024 (antwort-mvz1.2) [US2] Add admin bypass to owner filtering in `GetConversation`, `ListConversations`, `DeleteConversation` in `pkg/storage/memory/conversations.go`
- [ ] T025 (antwort-mvz1.3) [US2] Ensure admin CANNOT bypass owner check for write operations: `AddItems` in conversations must still check owner even for admin
- [ ] T026 (antwort-mvz1.4) [US2] Add admin bypass to PostgreSQL store queries (read and delete only, not write) in `pkg/storage/postgres/postgres.go`
- [ ] T027 (antwort-mvz1.5) [US2] Add admin bypass to vector store management (read and delete) in handlers
- [ ] T028 (antwort-mvz1.6) [US2] Pass admin role name from configuration through to storage helpers (wire config to storage layer)

### Tests for User Story 2

- [ ] T029 (antwort-mvz1.7) [US2] Integration test: admin can list all responses in their tenant in `tests/integration/ownership_test.go`
- [ ] T030 (antwort-mvz1.8) [P] [US2] Integration test: admin can delete another user's response in `tests/integration/ownership_test.go`
- [ ] T031 (antwort-mvz1.9) [P] [US2] Integration test: admin cannot add items to another user's conversation (returns 404) in `tests/integration/ownership_test.go`
- [ ] T032 (antwort-mvz1.10) [P] [US2] Integration test: admin in tenant-a cannot see resources from tenant-b in `tests/integration/ownership_test.go`

**Checkpoint**: Admin override functional with proper tenant scoping

---

## Phase 5: User Story 3 - Backward Compatibility (Priority: P1)

**Goal**: No-auth deployments (NoOp authenticator) continue to work unchanged. Existing data accessible after migration.

**Independent Test**: Start antwort with no auth config, create and list responses. All operations work as before.

### Implementation for User Story 3

- [ ] T033 (antwort-9i8e.1) [US3] Verify all owner filtering checks handle nil identity (no-op when `IdentityFromContext` returns nil) across all modified store methods
- [ ] T034 (antwort-9i8e.2) [US3] Verify empty owner string on existing data matches all authenticated users in query logic

### Tests for User Story 3

- [ ] T035 (antwort-9i8e.3) [US3] Integration test: NoOp authenticator, create and list responses without ownership filtering in `tests/integration/ownership_test.go`
- [ ] T036 (antwort-9i8e.4) [P] [US3] Integration test: resources with empty owner field are accessible to all authenticated users in `tests/integration/ownership_test.go`

**Checkpoint**: Backward compatibility verified

---

## Phase 6: User Story 4 - Owner Identity from Authentication (Priority: P2)

**Goal**: Owner is automatically set from `Identity.Subject` at creation time. Immutable. Not settable via API.

**Independent Test**: Authenticate as Alice, create response. Verify owner is "alice". Try to override via `user` field; owner stays "alice".

### Implementation for User Story 4

- [ ] T037 (antwort-bdu7.1) [US4] Verify owner is always set from `Identity.Subject` and never from request body fields (`user`, `metadata`, etc.) in all create handlers
- [ ] T038 (antwort-bdu7.2) [US4] Verify owner field is not included in any update/modify paths (immutable)

### Tests for User Story 4

- [ ] T039 (antwort-bdu7.3) [US4] Integration test: owner is set from Identity.Subject, not from request `user` field in `tests/integration/ownership_test.go`

**Checkpoint**: Owner auto-assignment verified

---

## Phase 7: User Story 5 - Consistent 404 for Unauthorized Access (Priority: P2)

**Goal**: Non-owner access returns 404 indistinguishable from genuinely non-existent resources.

**Independent Test**: As Bob, GET a response owned by Alice. Compare 404 response body with GET for a non-existent ID. They must be identical.

### Implementation for User Story 5

- [ ] T040 (antwort-rz0h.1) [US5] Verify all ownership-denied responses use the same `storage.ErrNotFound` error path as genuinely missing resources (no distinct error code or message)

### Tests for User Story 5

- [ ] T041 (antwort-rz0h.2) [US5] Integration test: compare 404 response body for non-owner access vs non-existent resource in `tests/integration/ownership_test.go`

**Checkpoint**: Security hardening verified

---

## Phase 8: Polish and Cross-Cutting Concerns

**Purpose**: Documentation and final validation

- [ ] T042 (antwort-nu79.1) [P] Update config reference documentation with `authorization.admin_role` and `jwt.roles_claim` settings in `docs/modules/reference/`
- [ ] T043 (antwort-nu79.2) [P] Update operations guide with ownership model explanation in `docs/modules/operations/`
- [ ] T044 (antwort-nu79.3) [P] Update multi-user quickstart (quickstart 03) to demonstrate ownership isolation in `quickstarts/03-multi-user/`
- [ ] T045 (antwort-nu79.4) Run existing conformance and SDK tests to verify no regressions

---

## Dependencies and Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Setup completion, BLOCKS all user stories
- **US1 (Phase 3)**: Depends on Foundational. Core isolation, MVP.
- **US2 (Phase 4)**: Depends on US1 (admin adds logic on top of owner filtering)
- **US3 (Phase 5)**: Depends on US1 (verifies backward compat of owner filtering)
- **US4 (Phase 6)**: Can run in parallel with US2/US3 after US1
- **US5 (Phase 7)**: Can run in parallel with US2/US3/US4 after US1
- **Polish (Phase 8)**: Depends on all user stories being complete

### User Story Dependencies

- **US1 (Data Isolation)**: Foundation only. No story dependencies. **MVP.**
- **US2 (Admin Override)**: Depends on US1 (extends owner filtering with admin bypass)
- **US3 (Backward Compat)**: Depends on US1 (validates the nil-identity path works)
- **US4 (Owner Auto-Assignment)**: Depends on US1 (verifies how owner is set)
- **US5 (Consistent 404)**: Depends on US1 (verifies error responses)

### Parallel Opportunities

- T001, T002, T003, T004 can run in parallel (Setup phase, different files)
- T007, T008 can run in parallel (memory store entry structs, different files)
- T019-T022 integration tests can run in parallel
- T029-T032 integration tests can run in parallel
- US4 and US5 can run in parallel after US1
- T042, T043, T044 documentation tasks can run in parallel

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T004)
2. Complete Phase 2: Foundational (T005-T006)
3. Complete Phase 3: User Story 1 (T007-T022)
4. **STOP and VALIDATE**: Run isolation tests independently
5. Deploy/demo if ready

### Incremental Delivery

1. Setup + Foundational -> Foundation ready
2. Add US1 (Data Isolation) -> Test independently -> MVP!
3. Add US2 (Admin Override) -> Test independently -> Admin features
4. Add US3 (Backward Compat) -> Test independently -> Safe upgrade path
5. Add US4 + US5 in parallel -> Verify auto-assignment and 404 consistency
6. Polish -> Documentation and final validation

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Owner filtering follows the exact same pattern as existing tenant filtering
- Files API (`pkg/files/metadata.go`) is the reference implementation; do not modify it
- All store methods that filter by owner must log denials at `slog.Debug` level
- Empty owner string means "accessible to all" (backward compatibility for pre-migration data)

<!-- SDD-TRAIT:beads -->
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
