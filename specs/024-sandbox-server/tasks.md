# Tasks: Sandbox Server for Code Execution

## Phase 1: Server Binary

**Purpose**: Build the sandbox server HTTP binary.

- [x] T001 Create `cmd/sandbox-server/main.go`: HTTP server listening on :8080 with routes for `POST /execute`, `GET /health`. Include graceful shutdown, configurable port via `SANDBOX_PORT` env, and request body size limit (FR-001, FR-010, FR-011)
- [x] T002 Implement `POST /execute` handler: parse request (code, timeout_seconds, requirements, files), validate required fields, create temporary working directory, write input files, execute code in subprocess with timeout, collect stdout/stderr/exit_code, collect output files, cleanup temp dir, return JSON response (FR-001 through FR-007, FR-016, FR-017)
- [x] T003 Implement package installation: when `requirements` is non-empty, run `uv pip install` in the temp directory before executing code. Support configurable index URL via `SANDBOX_PYTHON_INDEX` env. Return error if installation fails (FR-008, FR-009)
- [x] T004 Implement `GET /health` handler: return JSON with status, capacity (configurable max concurrent executions), current load (atomic counter), and uptime (FR-010, FR-011)
- [x] T005 Add concurrent execution tracking: atomic counter incremented before execution, decremented after. Reject requests when at capacity with 429 status (FR-010)

**Checkpoint**: `go run cmd/sandbox-server/main.go` starts and handles requests.

---

## Phase 2: Container Image

**Purpose**: Build the container image packaging the server with Python and uv.

- [x] T006 Create `Containerfile.sandbox`: multi-stage build. Stage 1 builds the Go binary. Stage 2 uses `python:3.12-slim` base, installs `uv`, copies the binary, sets non-root user, exposes port 8080 (FR-012, FR-013, FR-014, FR-015)
- [x] T007 Add `sandbox-build` and `sandbox-push` targets to `Makefile` for building and pushing the sandbox image (FR-012)

**Checkpoint**: `make sandbox-build` produces a working container image.

---

## Phase 3: Testing

**Purpose**: Integration tests for the sandbox server.

- [x] T008 Create `test/sandbox/sandbox_test.go`: TestMain starts the sandbox server binary, tests include: basic code execution, timeout enforcement, package installation (if Python available), file I/O, health endpoint, concurrent requests (FR-001 through FR-011)
- [x] T009 Add `test-sandbox` target to `Makefile` for running sandbox tests

**Checkpoint**: `make test-sandbox` passes all tests.

---

## Phase 4: Polish

- [x] T010 Add the sandbox server to the build matrix: update `make build` to also build `cmd/sandbox-server/`
- [x] T011 Run `go vet ./cmd/sandbox-server/...` and verify clean output

**Checkpoint**: Sandbox server builds cleanly as part of the project.

---

## Dependencies

- Phase 1: No dependencies
- Phase 2: Depends on Phase 1 (needs the binary)
- Phase 3: Depends on Phase 1 (tests run the binary)
- Phase 4: Depends on Phase 1
