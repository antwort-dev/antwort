# Tasks: Unified Configuration System

**Input**: Design documents from `/specs/012-configuration/`

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Config Struct + Loading (P1)

**Goal**: Config struct, YAML loading, env override, `_file` resolution.

- [x] T001 [US1] Create `pkg/config/config.go`: Config struct with sections (Server, Engine, Storage, Auth, MCP, Observability). Include compiled defaults. Add `_file` fields for all sensitive values (engine.api_key_file, storage.postgres.password_file, etc.) (FR-001, FR-006).
- [x] T002 [US1] [US2] Create `pkg/config/loader.go`: Load() function that discovers config file (FR-004, FR-005), parses YAML, applies env var overrides with ANTWORT_ prefix (FR-002, FR-003), resolves `_file` references (FR-006, FR-007), and returns the merged Config. Include backward compatibility mapping for existing env var names (FR-019).
- [x] T003 [US1] [US3] [US5] Write config loading tests in `pkg/config/config_test.go`: YAML loading, env override precedence, `_file` resolution, file discovery order, backward compatibility with old env vars, missing file error.

**Checkpoint**: Config loads from file + env + secrets.

---

## Phase 2: Validation (P1)

**Goal**: Startup validation with clear error messages.

- [x] T004 [US1] Create `pkg/config/validate.go`: Validate() method on Config that checks required fields, type consistency, and cross-field dependencies. Return errors with field path (FR-009, FR-010, FR-011).
- [x] T005 [US1] Write validation tests: missing backend URL, invalid port, empty _file reference, type mismatch.

**Checkpoint**: Invalid config fails fast with clear errors.

---

## Phase 3: Server Refactoring (P1)

**Goal**: Replace os.Getenv calls with config.Load().

- [x] T006 [US1] [US2] Refactor `cmd/server/main.go` to use config.Load() with --config flag (FR-017, FR-018). Replace all os.Getenv calls. Wire config sections to provider, store, auth, MCP executor creation. Verify conformance tests still pass.

**Checkpoint**: Server uses unified config. No more raw env vars.

---

## Phase 4: File Watching (P2)

**Goal**: Hot reload on config and secret file changes.

- [ ] T007 [US4] Create `pkg/config/watcher.go`: ConfigWatcher that watches config file and all `_file` paths. On change: re-read, validate, call reload callback with new Config. Keep old config on validation failure (FR-012, FR-013, FR-016).
- [ ] T008 [US4] Define hot-reloadable vs restart-required settings. Implement reload callback in cmd/server that applies hot-reloadable changes (log level, MCP connections, auth credentials) (FR-014, FR-015).
- [ ] T009 [US4] Write watcher tests: file change detected, secret rotation detected, invalid config keeps old, deleted file keeps old.

**Checkpoint**: Hot reload works for config and secrets.

---

## Phase 5: Polish

- [x] T010 [P] Run `go vet ./...` and `go test ./...` across all packages.
- [x] T011 [P] Run `make conformance` to verify no regressions.
- [x] T012 Create example `config.yaml` with all settings documented.

---

## Dependencies

- **Phase 1**: No dependencies.
- **Phase 2**: Depends on Phase 1 (config struct).
- **Phase 3**: Depends on Phase 2 (validation).
- **Phase 4**: Depends on Phase 3 (server uses config).
- **Phase 5**: Depends on all.

## Implementation Strategy

### MVP: Phases 1-3

1. Config struct + loading + validation
2. Server refactoring
3. **STOP**: Unified config works, env vars replaced

### Full: + Phase 4-5

4. File watching + hot reload
5. Polish + example config
