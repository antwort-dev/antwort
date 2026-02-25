# Tasks: Category-Based Debug Logging

## Phase 1: Debug Package

**Purpose**: Core debug package with category enable/disable and logging helpers.

- [ ] T001 (antwort-5t6.1) Create `pkg/debug/debug.go`: `Init(categories string, level string)` parses ANTWORT_DEBUG and ANTWORT_LOG_LEVEL, populates the enabled categories map. `Enabled(category string) bool` checks if a category is active. `Log(category, msg string, args ...any)` emits at DEBUG level via slog if category enabled. Define TRACE level constant (FR-001, FR-002, FR-004, FR-010, FR-012, FR-013)
- [ ] T002 (antwort-5t6.2) Create `pkg/debug/debug_test.go`: unit tests for Init, Enabled, Log. Table-driven tests: single category, multiple categories, "all", empty, invalid category, level parsing, TRACE level detection (FR-001 through FR-006, FR-012, FR-013)

**Checkpoint**: Debug package compiles and tests pass.

---

## Phase 2: Configuration Integration

**Goal**: Wire debug configuration from config file and environment.

- [ ] T003 (antwort-zvu.1) Add `Logging` section to config types in `pkg/config/types.go`: level (string, default "INFO"), debug (string, comma-separated categories), format (string, default "text") (FR-003, FR-007, FR-008, FR-009)
- [ ] T004 (antwort-zvu.2) Initialize debug system in `cmd/server/main.go`: call `debug.Init()` with values from config and env overrides. ANTWORT_DEBUG env takes precedence over config file (FR-002, FR-003, FR-007, FR-008)
- [ ] T005 (antwort-zvu.3) Configure slog handler level based on ANTWORT_LOG_LEVEL in `cmd/server/main.go`: set the default slog level to match the configured level (FR-006, FR-009)

**Checkpoint**: Debug categories configurable via config.yaml and env vars.

---

## Phase 3: US1 - Provider Instrumentation (P1)

**Goal**: "providers" category shows LLM backend communication.

- [ ] T006 (antwort-csp.1) [US1] Add debug logging in the openaicompat HTTP client: log outbound request (method, URL, model, message count, tool count) before sending, log response (status, timing, usage summary) after receiving. At TRACE level, log full request and response bodies (FR-014, FR-015, FR-011)
- [ ] T007 (antwort-csp.2) [US1] Add debug logging for streaming provider calls: log stream initiation and completion with timing (FR-014, FR-015)

**Checkpoint**: `ANTWORT_DEBUG=providers ANTWORT_LOG_LEVEL=DEBUG` shows provider communication.

---

## Phase 4: US2 - Engine Instrumentation (P1)

**Goal**: "engine" category shows agentic loop decisions.

- [ ] T008 (antwort-dam.1) [US2] Add debug logging in `pkg/engine/loop.go`: log each agentic loop turn (turn number, max turns), log tool calls received (names, count), log tool dispatch (executor type, tool name), log tool results (success/failure, timing) (FR-016)
- [ ] T009 (antwort-dam.2) [US2] Add debug logging in `pkg/engine/engine.go`: log request handling mode (streaming/non-streaming, agentic/direct), log model used, log response status (FR-016)

**Checkpoint**: `ANTWORT_DEBUG=engine ANTWORT_LOG_LEVEL=DEBUG` shows agentic loop decisions.

---

## Phase 5: Polish & Validation

- [ ] T010 (antwort-91e.1) Run full test suite (`go test ./pkg/... ./test/integration/`) and verify zero regressions (FR-012)
- [ ] T011 (antwort-91e.2) Run `make api-test` to verify no conformance regressions

**Checkpoint**: All tests green. Debug logging ready for deployment.

---

## Dependencies

- Phase 1: No dependencies
- Phase 2: Depends on Phase 1
- Phase 3: Depends on Phase 2
- Phase 4: Depends on Phase 2, independent of Phase 3
- Phase 5: Depends on Phase 3 and 4
