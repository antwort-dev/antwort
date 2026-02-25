# Tasks: Sandbox Multi-Runtime Modes

## Phase 1: Mode Infrastructure

**Purpose**: Add mode field, mode configuration, and auto-detection to the sandbox server.

- [ ] T001 (antwort-9hu.1) Add `mode` field to `sandboxServer` struct. Add `SANDBOX_MODE` env var parsing in `main()`. Add `modeConfig()` method that returns interpreter command ([]string), file extension (string), and extra env vars ([]string) based on the active mode (FR-001, FR-002, FR-005, FR-006, FR-007)
- [ ] T002 (antwort-9hu.2) Implement `detectMode()`: check for runtimes in PATH in priority order (python3, go, node, bash). Return the first found. Fail with error if none found (FR-004, FR-009, FR-010)
- [ ] T003 (antwort-9hu.3) Add startup validation: if SANDBOX_MODE is set but the runtime isn't in PATH, fail to start with a descriptive error

**Checkpoint**: Server starts with explicit mode or auto-detects. Fails cleanly on invalid mode.

---

## Phase 2: Mode Implementations

**Goal**: Each mode correctly selects interpreter, file extension, and package handling.

- [ ] T004 (antwort-31r.1) [US1] Refactor execute handler to use `modeConfig()` for interpreter, file extension, and env vars instead of hardcoded `python3` and `.py` (FR-005, FR-006, FR-008, FR-013)
- [ ] T005 (antwort-31r.2) [US2] Implement shell mode: interpreter=`bash`, extension=`.sh`, no package installation. Skip requirements silently (FR-014)
- [ ] T006 (antwort-31r.3) [US3] Implement Go mode: interpreter=`go run`, extension=`.go`, no package installation (FR-015)
- [ ] T007 (antwort-31r.4) [US4] Implement Node.js mode: interpreter=`node`, extension=`.js`, packages via `npm install` in temp dir when requirements specified (FR-016)
- [ ] T008 (antwort-31r.5) Refactor `installRequirements()` to be mode-aware: Python uses `uv pip install --target`, Node uses `npm install`, Go and shell skip (FR-007, FR-013, FR-014, FR-015, FR-016)

**Checkpoint**: All four modes execute code correctly.

---

## Phase 3: Health and Reporting

**Goal**: Health endpoint reports mode and runtime version.

- [ ] T009 (antwort-46k.1) [US6] Add `mode` and `runtime_version` fields to `healthResponse`. Detect runtime version at startup (e.g., `python3 --version`, `go version`, `node --version`, `bash --version`) and cache it (FR-011, FR-012)

**Checkpoint**: `/health` shows mode and version.

---

## Phase 4: Testing

**Goal**: Unit tests for mode selection and auto-detection.

- [ ] T010 (antwort-fn0.1) Create `cmd/sandbox-server/main_test.go`: table-driven tests for `modeConfig()` (verify interpreter, extension, env for each mode), test `detectMode()` with mocked PATH, test startup failure on invalid mode (FR-001 through FR-010)
- [ ] T011 (antwort-fn0.2) Run existing tests (`go test ./pkg/... ./test/integration/`) to verify no regressions

**Checkpoint**: All tests pass. Mode system validated.

---

## Phase 5: Polish

- [ ] T012 (antwort-7m8.1) Update sandbox server startup log to include the active mode and runtime version
- [ ] T013 (antwort-3qw.2) Run `go vet ./cmd/sandbox-server/...` and verify clean

**Checkpoint**: Clean build, informative startup logs.

---

## Dependencies

- Phase 1: No dependencies
- Phase 2: Depends on Phase 1
- Phase 3: Depends on Phase 1
- Phase 4: Depends on Phase 2 and 3
- Phase 5: Depends on Phase 4
