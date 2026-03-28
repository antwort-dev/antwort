# Tasks: Audit Logging

**Input**: Design documents from `/specs/042-audit-logging/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md

**Tests**: Included. The spec requires nil-safe tests and error path coverage per constitution testing standards.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create the audit logging package and configuration

- [x] T001 Create `pkg/audit/audit.go` with nil-safe Logger type, Log method, and helper methods for event-specific field assembly. Logger wraps `*slog.Logger`, all methods are no-ops when receiver is nil. Include `New(cfg AuditConfig) (*Logger, error)` constructor that validates config and creates the slog.Handler (JSON or text, stdout or file).
- [x] T002 Add `AuditConfig` struct to `pkg/config/config.go` with fields: Enabled (bool), Format (string, default "json"), Output (string, default "stdout"), File (string). Add `Audit AuditConfig` field to top-level `Config` struct. Add YAML tags and env var override support (`ANTWORT_AUDIT_ENABLED`, `ANTWORT_AUDIT_FORMAT`, `ANTWORT_AUDIT_OUTPUT`, `ANTWORT_AUDIT_FILE`).
- [x] T003 [P] Write table-driven tests in `pkg/audit/audit_test.go`: nil-safe no-op test (nil Logger emits nothing), JSON format output test, text format output test, identity extraction from context test, missing identity test (anonymous), config validation tests (invalid format, invalid output, file output without path, non-writable file path).

**Checkpoint**: Audit package exists, is tested, and can be instantiated from config.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Wire audit logger into server startup and emit the startup event

**CRITICAL**: No user story work can begin until this phase is complete

- [x] T004 Wire audit logger creation in `cmd/server/main.go`: after config load, call `audit.New(cfg.Audit)` to create the logger. If creation fails (config validation error), log error and exit. Pass the logger to middleware and components in subsequent wiring steps.
- [x] T005 Emit `config.startup` audit event in `cmd/server/main.go` after all components are initialized, with fields: `auth_enabled`, `audit_enabled`, `role_count`, `scope_enforcement`. This implements FR-006.

**Checkpoint**: Foundation ready. Server starts with audit logger wired (or nil when disabled). Startup event emitted when enabled.

---

## Phase 3: User Story 1 - Debug Authorization Issues (Priority: P1) MVP

**Goal**: Operators can see authorization denial events and admin override events in the audit log to debug "why can't user X access resource Y?" questions.

**Independent Test**: Enable audit logging, make requests as different users with different permissions, verify audit log contains correct authorization events.

### Implementation for User Story 1

- [x] T006 [US1] Add audit logger parameter to `auth.Middleware()` in `pkg/auth/middleware.go`. Update the function signature to accept `*audit.Logger` as an optional parameter. Emit `auth.success` event on successful authentication (line ~55) with fields: `auth_method`, `remote_addr`. Emit `auth.failure` event on failed authentication (line ~34) with fields: `auth_method`, `remote_addr`, `error`. Emit `auth.rate_limited` event on rate limit exceeded (line ~64) with fields: `tier`, `remote_addr`. Update caller in `cmd/server/main.go`.
- [x] T007 [US1] Add audit logger parameter to `scope.Middleware()` in `pkg/auth/scope/middleware.go`. Emit `authz.scope_denied` event on 403 response (line ~88) with fields: `endpoint`, `required_scope`, `effective_scopes`. Extract identity from context for subject/tenant fields. Update caller in `cmd/server/main.go`.
- [x] T008 [US1] Add `SetAuditLogger(*audit.Logger)` setter to the memory store in `pkg/storage/memory/memory.go`. In `ownerAllowed()` function: emit `authz.ownership_denied` event when ownership check fails (line ~40) with fields: `resource_type`, `resource_id`, `operation`. Emit `authz.admin_override` event when admin bypass is used (line ~30) with fields: `resource_type`, `resource_id`, `resource_owner`, `operation`. Pass resource_type from callers (GetResponse, DeleteResponse, ListResponses, etc.).
- [x] T009 [US1] Write tests in `pkg/auth/middleware_test.go` (or extend existing): verify auth.success, auth.failure, auth.rate_limited events are emitted with correct fields. Verify nil audit logger produces no events. Use a `bytes.Buffer` as slog handler output to capture and parse audit events.
- [x] T010 [P] [US1] Write tests in `pkg/auth/scope/scope_test.go` (or extend existing): verify authz.scope_denied event is emitted with correct fields including effective_scopes. Verify nil audit logger produces no events.
- [x] T011 [P] [US1] Write tests in `pkg/storage/memory/ownership_test.go` (or extend existing): verify authz.ownership_denied and authz.admin_override events are emitted with correct resource_type, resource_id, and operation fields. Verify nil audit logger produces no events.

**Checkpoint**: User Story 1 fully functional. All authorization events visible in audit log. Operators can filter by user and time to debug access issues.

---

## Phase 4: User Story 5 - Opt-In with Zero Overhead (Priority: P1)

**Goal**: Verify that audit logging adds zero overhead when disabled and works correctly when enabled.

**Independent Test**: Start server without audit config (verify no audit output), start with audit enabled (verify events appear with correct structure).

### Implementation for User Story 5

- [x] T012 [US5] Add nil-safe integration test: create a full request flow with nil audit logger, verify no panics and no audit output. Test in `pkg/audit/audit_test.go`.
- [x] T013 [US5] Add enabled integration test: create a full request flow with audit logger writing to a buffer, verify structured events appear with timestamp, event name, and severity. Test in `pkg/audit/audit_test.go`.
- [x] T014 [US5] Add config validation edge case tests in `pkg/audit/audit_test.go`: audit disabled ignores all other fields, file output with writable path succeeds, file output with non-writable path returns error at construction time.

**Checkpoint**: Zero-overhead guarantee validated. Opt-in behavior confirmed.

---

## Phase 5: User Story 2 - Track Resource Mutations (Priority: P2)

**Goal**: Operators can see who created, deleted, or changed permissions on resources.

**Independent Test**: Enable audit logging, create and delete resources as different users, verify audit log records each mutation.

### Implementation for User Story 2

- [x] T015 [P] [US2] Add `SetAuditLogger(*audit.Logger)` setter to `transporthttp.Adapter` in `pkg/transport/http/adapter.go`. In `handleCreateResponse()` (line ~159): emit `resource.created` event after successful response creation with fields: `resource_type` ("response"), `resource_id`. In `handleDeleteResponse()` (line ~268): emit `resource.deleted` event after successful deletion with fields: `resource_type` ("response"), `resource_id`.
- [x] T016 [P] [US2] In `pkg/transport/http/conversations.go`: emit `resource.created` event in `handleCreateConversation()` after successful save with fields: `resource_type` ("conversation"), `resource_id`. Emit `resource.deleted` event in conversation delete handler after successful deletion. Use the audit logger from the Adapter.
- [x] T017 [P] [US2] Add `SetAuditLogger(*audit.Logger)` setter to FilesAPI in `pkg/files/api.go`. In `handleUpload()` (line ~107): emit `resource.created` event after successful file storage with fields: `resource_type` ("file"), `resource_id`. In `handleDeleteFile()` (line ~196): emit `resource.deleted` event after successful deletion.
- [x] T018 [P] [US2] Add `SetAuditLogger(*audit.Logger)` setter to filesearch Provider in `pkg/tools/builtins/filesearch/api.go`. In `handleCreateStore()` (line ~120): emit `resource.created` event after successful creation with fields: `resource_type` ("vector_store"), `resource_id`. In `handleDeleteStore()` (line ~276): emit `resource.deleted` event. In permissions update handler: emit `resource.permissions_changed` event with `old_permissions` and `new_permissions` fields.
- [x] T019 [US2] Wire audit logger to all resource handlers in `cmd/server/main.go`: call `adapter.SetAuditLogger(auditLogger)`, `filesAPI.SetAuditLogger(auditLogger)`, `filesearchProvider.SetAuditLogger(auditLogger)`.
- [x] T020 [US2] Write tests for resource mutation events: verify resource.created and resource.deleted events for each resource type (response, conversation, file, vector_store). Verify resource.permissions_changed event includes old and new values. Tests in handler test files or a new `pkg/audit/integration_test.go`.

**Checkpoint**: All resource mutations are audited. Operators can trace who created or deleted any resource.

---

## Phase 6: User Story 3 - Monitor Authentication Activity (Priority: P2)

**Goal**: Operators can see authentication success/failure patterns across the deployment.

**Independent Test**: Enable audit logging, send requests with valid and invalid credentials, verify audit log records each authentication outcome.

### Implementation for User Story 3

Note: The authentication audit events (auth.success, auth.failure, auth.rate_limited) were already implemented in US1 (T006) as part of authorization debugging. This story validates that those events contain the right fields for security monitoring use cases.

- [x] T021 [US3] Write authentication monitoring validation tests in `pkg/auth/middleware_test.go`: verify auth.success includes auth_method (jwt vs apikey), verify auth.failure includes error reason, verify auth.rate_limited includes tier and remote_addr. These tests focus on the monitoring perspective (pattern detection) vs the debugging perspective tested in T009.
- [x] T022 [US3] Verify remote_addr is correctly populated in auth events across different request scenarios (proxied requests with X-Forwarded-For, direct connections). Add test cases if gaps found.

**Checkpoint**: Authentication events contain sufficient detail for security monitoring and pattern detection.

---

## Phase 7: User Story 4 - Track Tool Execution (Priority: P3)

**Goal**: Operators can see which tools are being invoked and whether any are failing.

**Independent Test**: Enable audit logging, trigger tool calls through the agentic loop, verify audit log records each tool dispatch and any failures.

### Implementation for User Story 4

- [x] T023 [US4] Add `AuditLogger *audit.Logger` field to `engine.Config` in `pkg/engine/engine.go`. Store it in the Engine struct.
- [x] T024 [US4] In `pkg/engine/loop.go`, emit audit events in `executeToolsConcurrently()` and `executeToolsSequentially()`: emit `tool.executed` event after successful tool execution with fields: `tool_type` (from `classifyToolType()`), `tool_name`, `response_id` (from context or args). Emit `tool.failed` event on tool execution error with additional `error` field. Response ID should be extracted from the request context if available.
- [x] T025 [US4] Wire audit logger to engine in `cmd/server/main.go`: pass `auditLogger` in `engine.Config{AuditLogger: auditLogger}`.
- [x] T026 [US4] Write tests for tool audit events in `pkg/engine/loop_test.go` (or extend existing): verify tool.executed emitted for successful tool calls with correct tool_type and tool_name, verify tool.failed emitted on errors with error details. Verify nil audit logger produces no events. Use table-driven tests covering MCP, builtin, and function tool types.

**Checkpoint**: Tool execution is fully auditable. Operators can track tool usage patterns and failures.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Documentation and final validation

- [x] T027 [P] Update `docs/modules/reference/pages/config-reference.adoc` with audit configuration section: `audit.enabled`, `audit.format`, `audit.output`, `audit.file` with defaults and descriptions. Follow semantic line breaks (one sentence per line).
- [x] T028 [P] Update `docs/modules/reference/pages/environment-variables.adoc` with `ANTWORT_AUDIT_ENABLED`, `ANTWORT_AUDIT_FORMAT`, `ANTWORT_AUDIT_OUTPUT`, `ANTWORT_AUDIT_FILE` environment variables.
- [x] T029 [P] Update `docs/modules/operations/pages/security.adoc` with audit logging section: overview, configuration, event catalog (all 12 events with fields), example output, integration with log aggregation tools, and troubleshooting tips.
- [x] T030 Run full test suite to verify no regressions from audit logger injection into existing middleware and handlers. Verify all existing tests pass with nil audit logger (backward compatibility).

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (T001, T002)
- **US1 (Phase 3)**: Depends on Phase 2 (T004, T005)
- **US5 (Phase 4)**: Depends on Phase 1 (T001, T003), can run parallel with US1
- **US2 (Phase 5)**: Depends on Phase 2 (T004), can run parallel with US1
- **US3 (Phase 6)**: Depends on US1 (T006, T009), validates existing auth events
- **US4 (Phase 7)**: Depends on Phase 2 (T004), can run parallel with US1/US2
- **Polish (Phase 8)**: Depends on all user stories being complete

### User Story Dependencies

- **US1 (P1)**: After Foundational. No dependencies on other stories.
- **US5 (P1)**: After Setup. Tests audit package directly, independent of integration.
- **US2 (P2)**: After Foundational. Independent of US1 (different files).
- **US3 (P2)**: After US1 (validates auth events from T006). Sequential.
- **US4 (P3)**: After Foundational. Independent of US1/US2 (engine vs middleware/handlers).

### Within Each User Story

- Config/setup before event emission code
- Event emission before tests (tests validate emissions)
- Wiring in main.go before integration tests

### Parallel Opportunities

- T003 (audit tests) parallel with T002 (config)
- T010, T011 parallel with each other (scope tests, ownership tests)
- T015, T016, T017, T018 all parallel (different handler files)
- T027, T028, T029 all parallel (different doc files)
- US1, US2, US4 can proceed in parallel after Foundational phase

---

## Parallel Example: User Story 2

```bash
# Launch all resource handler tasks in parallel (different files):
Task: "T015 - Add audit to response handlers in pkg/transport/http/adapter.go"
Task: "T016 - Add audit to conversation handlers in pkg/transport/http/conversations.go"
Task: "T017 - Add audit to file handlers in pkg/files/api.go"
Task: "T018 - Add audit to vector store handlers in pkg/tools/builtins/filesearch/api.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 + 5 Only)

1. Complete Phase 1: Setup (T001-T003)
2. Complete Phase 2: Foundational (T004-T005)
3. Complete Phase 3: US1 - Authorization debugging (T006-T011)
4. Complete Phase 4: US5 - Zero overhead validation (T012-T014)
5. **STOP and VALIDATE**: Authorization denial events visible, zero overhead confirmed
6. Deploy/demo if ready

### Incremental Delivery

1. Setup + Foundational -> Audit package ready
2. US1 + US5 -> Authorization debugging MVP
3. US2 -> Resource mutation tracking
4. US3 -> Authentication monitoring validation
5. US4 -> Tool execution visibility
6. Polish -> Documentation complete

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- All audit logger injections use nil-safe pattern (nil = no audit)
- Existing function signatures change (auth.Middleware, scope.Middleware gain audit param), update all callers
- Tests capture audit output via bytes.Buffer wrapped in slog.Handler
- 30 total tasks across 8 phases

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
