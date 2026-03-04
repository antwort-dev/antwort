# Tasks: Scope-Based Authorization and Resource Permissions

**Input**: Design documents from `/specs/041-scope-permissions/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md

**Tests**: Included. Authorization is security-critical.

**Organization**: Tasks grouped by user story for independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: Shared types and configuration for scope enforcement and permissions

- [ ] T001 (antwort-77ha.1) [P] Create `Permissions` value type with `ParsePermissions(string) Permissions`, `String() string`, and `CanRead/CanWrite/CanDelete` check methods in `pkg/authz/permissions.go`
- [ ] T002 (antwort-77ha.2) [P] Create role expansion logic with cycle detection in `pkg/auth/scope/roles.go`: `ExpandRoles(config map[string][]string) (map[string]map[string]bool, error)` resolving references and detecting cycles at startup
- [ ] T003 (antwort-77ha.3) [P] Add `RoleScopes map[string][]string` field to `AuthorizationConfig` in `pkg/config/config.go`
- [ ] T004 (antwort-77ha.4) [P] Add `VectorStoreIDs []string` field to `AgentProfileConfig` in `pkg/agent/config.go` and `AgentProfile` in `pkg/agent/profile.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Scope middleware and database migration that MUST be complete before user story work

**Warning**: No user story work can begin until this phase is complete

- [ ] T005 (antwort-e7kq.1) Create scope enforcement middleware in `pkg/auth/scope/middleware.go`: check effective scopes against hardcoded endpoint-to-scope map, return 403 with scope name on denial, no-op when nil/empty role config
- [ ] T006 (antwort-e7kq.2) Create hardcoded endpoint-to-scope map in `pkg/auth/scope/middleware.go` covering all 20 endpoints from data-model.md
- [ ] T007 (antwort-e7kq.3) Wire scope middleware into `cmd/server/main.go` after auth middleware, conditional on `RoleScopes` being configured
- [ ] T008 (antwort-e7kq.4) Inject effective scopes (JWT union role-expanded) into context in `pkg/auth/middleware.go`
- [ ] T009 (antwort-e7kq.5) Create PostgreSQL migration `pkg/storage/postgres/migrations/003_add_permissions.sql` adding `permissions TEXT NOT NULL DEFAULT 'rwd|---|---'` column to vector_stores and files tables

**Checkpoint**: Scope middleware and permissions infrastructure ready

---

## Phase 3: User Story 1 - Endpoint Authorization via Scopes (Priority: P1)

**Goal**: Users with configured roles can only access endpoints their scopes allow. 403 for denied requests. No-op when unconfigured.

**Independent Test**: Configure viewer role, authenticate as viewer, call POST /v1/responses (403), call GET /v1/responses (200).

### Tests for User Story 1

- [ ] T010 (antwort-8xc3.1) [P] [US1] Unit tests for role expansion: basic expansion, reference inheritance, cycle detection, undefined reference rejection in `pkg/auth/scope/scope_test.go`
- [ ] T011 (antwort-8xc3.2) [P] [US1] Unit tests for scope middleware: scope match, wildcard `*`, missing scope returns 403, no config returns no-op in `pkg/auth/scope/scope_test.go`
- [ ] T012 (antwort-8xc3.3) [P] [US1] Unit tests for Permissions type: parse valid strings, invalid strings, CanRead/CanWrite/CanDelete, String() round-trip in `pkg/authz/permissions_test.go`

### Implementation for User Story 1

- [ ] T013 (antwort-8xc3.4) [US1] Integration test: viewer role can GET but not POST responses in `tests/integration/scope_permissions_test.go`
- [ ] T014 (antwort-8xc3.5) [US1] Integration test: user role inherits viewer scopes plus create in `tests/integration/scope_permissions_test.go`
- [ ] T015 (antwort-8xc3.6) [US1] Integration test: admin wildcard `*` passes all endpoints in `tests/integration/scope_permissions_test.go`
- [ ] T016 (antwort-8xc3.7) [US1] Integration test: JWT scopes union with role scopes in `tests/integration/scope_permissions_test.go`
- [ ] T017 (antwort-8xc3.8) [US1] Integration test: no role_scopes config means no enforcement in `tests/integration/scope_permissions_test.go`

**Checkpoint**: Endpoint authorization fully functional

---

## Phase 4: User Story 2 - Shared Vector Stores (Priority: P1)

**Goal**: Vector stores with group/others permissions are visible and searchable by permitted users.

**Independent Test**: Alice creates store with group:r. Bob (same tenant) lists stores and sees it. Dave (different tenant) does not.

### Implementation for User Story 2

- [ ] T018 (antwort-u406.1) [P] [US2] Add `Permissions` field to `VectorStore` struct in `pkg/tools/builtins/filesearch/store.go`
- [ ] T019 (antwort-u406.2) [US2] Accept `permissions` JSON object on vector store create in `pkg/tools/builtins/filesearch/api.go`: parse `{"group": "r"}` into compact string, enforce owner level always `rwd`
- [ ] T020 (antwort-u406.3) [US2] Accept `permissions` JSON object on vector store update (if update endpoint exists) in `pkg/tools/builtins/filesearch/api.go`
- [ ] T021 (antwort-u406.4) [US2] Include `permissions` compact string in vector store GET/list responses in `pkg/tools/builtins/filesearch/api.go`
- [ ] T022 (antwort-u406.5) [US2] Extend `handleListStores` in `pkg/tools/builtins/filesearch/api.go`: include stores where caller has group or others read permission (not just owner/admin)
- [ ] T023 (antwort-u406.6) [US2] Extend `handleGetStore` in `pkg/tools/builtins/filesearch/api.go`: allow access based on group/others read permission
- [ ] T024 (antwort-u406.7) [US2] Add permission check to file_search execution in `pkg/tools/builtins/filesearch/provider.go` (lines 249-268): after tenant and owner checks, check group/others read permission. Skip inaccessible stores silently.
- [ ] T025 (antwort-u406.8) [US2] Integration test: default private store not visible to group members in `tests/integration/scope_permissions_test.go`
- [ ] T026 (antwort-u406.9) [P] [US2] Integration test: group-read store visible to same-tenant user in `tests/integration/scope_permissions_test.go`
- [ ] T027 (antwort-u406.10) [P] [US2] Integration test: group-read store not visible to different-tenant user in `tests/integration/scope_permissions_test.go`
- [ ] T028 (antwort-u406.11) [P] [US2] Integration test: others-read store visible to different-tenant user in `tests/integration/scope_permissions_test.go`
- [ ] T029 (antwort-u406.12) [US2] Integration test: permission revocation (update group to `---`) hides store from group in `tests/integration/scope_permissions_test.go`
- [ ] T030 (antwort-u406.13) [US2] Integration test: file_search silently skips inaccessible store in `tests/integration/scope_permissions_test.go`

**Checkpoint**: Shared vector stores functional with permission enforcement

---

## Phase 5: User Story 3 - Shared Files (Priority: P2)

**Goal**: Files with group/others permissions are accessible to permitted users. Citations work across permission boundaries.

**Independent Test**: Alice uploads file with group:r. Bob (same tenant) retrieves it. Dave (different tenant) gets 404.

### Implementation for User Story 3

- [ ] T031 (antwort-2x6v.1) [P] [US3] Add `Permissions` field to `File` struct in `pkg/files/types.go`
- [ ] T032 (antwort-2x6v.2) [US3] Accept `permissions` JSON object on file upload in file create handler: parse into compact string, enforce owner level always `rwd`
- [ ] T033 (antwort-2x6v.3) [US3] Include `permissions` compact string in file GET/list responses
- [ ] T034 (antwort-2x6v.4) [US3] Extend file Get/List methods in `pkg/files/metadata.go`: allow access based on group/others read permission (not just owner)
- [ ] T035 (antwort-2x6v.5) [US3] Integration test: group-read file accessible to same-tenant user in `tests/integration/scope_permissions_test.go`
- [ ] T036 (antwort-2x6v.6) [P] [US3] Integration test: group-read file not accessible to different-tenant user in `tests/integration/scope_permissions_test.go`
- [ ] T037 (antwort-2x6v.7) [US3] Integration test: file_citation annotation works for shared file in `tests/integration/scope_permissions_test.go`

**Checkpoint**: Shared files functional with working citations

---

## Phase 6: User Story 4 - Vector Store Union Merge (Priority: P2)

**Goal**: Agent profile vector_store_ids merge with request vector_store_ids via union.

**Independent Test**: Profile has vs_company. Request adds vs_mine. file_search uses both.

### Implementation for User Story 4

- [ ] T038 (antwort-j10r.1) [US4] Add `VectorStoreIDs` union merge logic to `pkg/agent/merge.go`: combine profile and request store IDs, deduplicate
- [ ] T039 (antwort-j10r.2) [US4] Wire VectorStoreIDs from profile config to AgentProfile in `pkg/agent/config.go` profile builder
- [ ] T040 (antwort-j10r.3) [US4] Ensure file_search tool receives merged vector_store_ids from the engine/profile resolution layer
- [ ] T041 (antwort-j10r.4) [US4] Integration test: profile stores merged with request stores (union) in `tests/integration/scope_permissions_test.go`
- [ ] T042 (antwort-j10r.5) [P] [US4] Integration test: duplicate store IDs deduplicated in `tests/integration/scope_permissions_test.go`
- [ ] T043 (antwort-j10r.6) [US4] Integration test: inaccessible store in merged list silently skipped in `tests/integration/scope_permissions_test.go`

**Checkpoint**: Agent profile union merge functional

---

## Phase 7: User Story 5 - Backward Compatibility (Priority: P1)

**Goal**: No-config deployments work unchanged. No scope enforcement. Default private permissions.

**Independent Test**: Start with no auth config. All endpoints work. No 403 errors.

### Implementation for User Story 5

- [ ] T044 (antwort-jiyp.1) [US5] Verify scope middleware is no-op when role_scopes is empty/nil across all code paths
- [ ] T045 (antwort-jiyp.2) [US5] Verify default permissions `rwd|---|---` on new resources when no permissions specified
- [ ] T046 (antwort-jiyp.3) [US5] Integration test: no role_scopes config, all endpoints accessible in `tests/integration/scope_permissions_test.go`
- [ ] T047 (antwort-jiyp.4) [P] [US5] Integration test: existing resources without permissions column accessible to owners after migration in `tests/integration/scope_permissions_test.go`

**Checkpoint**: Backward compatibility verified

---

## Phase 8: Polish and Cross-Cutting Concerns

**Purpose**: Documentation and final validation

- [ ] T048 (antwort-673v.1) [P] Update config reference with `auth.authorization.role_scopes` in `docs/modules/reference/pages/config-reference.adoc`
- [ ] T049 (antwort-673v.2) [P] Add authorization and permissions tutorial in `docs/modules/tutorial/pages/authorization.adoc`
- [ ] T050 (antwort-673v.3) [P] Update operations security guide with scope enforcement and permissions model in `docs/modules/operations/pages/security.adoc`
- [ ] T051 (antwort-673v.4) Run existing conformance and SDK tests to verify no regressions

---

## Dependencies and Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately. All tasks are [P].
- **Foundational (Phase 2)**: Depends on Setup. BLOCKS all user stories.
- **US1 (Phase 3)**: Depends on Foundational. Scope enforcement MVP.
- **US2 (Phase 4)**: Depends on Foundational (uses Permissions type). Can run in parallel with US1.
- **US3 (Phase 5)**: Depends on US2 (same permission pattern, file_citation depends on shared vector store).
- **US4 (Phase 6)**: Depends on US2 (needs shared vector stores for merge testing).
- **US5 (Phase 7)**: Depends on US1 + US2 (verifies backward compat of both features).
- **Polish (Phase 8)**: Depends on all user stories.

### User Story Dependencies

- **US1 (Scope Enforcement)**: Foundation only. **MVP for scopes.**
- **US2 (Shared Vector Stores)**: Foundation only. **MVP for permissions.** Can run parallel with US1.
- **US3 (Shared Files)**: Depends on US2 (same permission pattern).
- **US4 (Union Merge)**: Depends on US2 (needs shared stores).
- **US5 (Backward Compat)**: Depends on US1 + US2 (validates both).

### Parallel Opportunities

- T001-T004 can all run in parallel (Setup, different files)
- T010-T012 unit tests can run in parallel
- US1 and US2 can run in parallel after Foundational (different concerns)
- T026-T028 integration tests can run in parallel
- T048-T050 documentation tasks can run in parallel

---

## Implementation Strategy

### MVP First (US1 + US2)

1. Complete Phase 1: Setup (T001-T004)
2. Complete Phase 2: Foundational (T005-T009)
3. Complete Phase 3: US1 - Scope Enforcement (T010-T017)
4. Complete Phase 4: US2 - Shared Vector Stores (T018-T030)
5. **STOP and VALIDATE**: Scopes and sharing independently testable

### Incremental Delivery

1. Setup + Foundational -> Infrastructure ready
2. US1 (Scopes) + US2 (Shared Stores) in parallel -> Core authorization
3. US3 (Shared Files) -> RAG pipeline sharing complete
4. US4 (Union Merge) -> Agent profile integration
5. US5 (Backward Compat) -> Safe upgrade verified
6. Polish -> Documentation

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story
- Scope enforcement and permissions are independent features that share Foundational infrastructure
- Owner level is always `rwd` (immutable) per FR-010
- Permission input is JSON object, output is compact string
- Scope middleware returns 403 (not 404) per FR-005
- Role references resolved at startup, not per-request

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
