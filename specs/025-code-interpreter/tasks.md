# Tasks: Code Interpreter Tool

## Phase 1: Types and Interfaces

**Purpose**: Add the code_interpreter item types and sandbox client interface.

- [ ] T001 (antwort-21e.1) Add `CodeInterpreterCallData` and `CodeInterpreterOutput` types to `pkg/api/types.go`. Add `ItemTypeCodeInterpreterCall` constant. Support `logs` and `image` output types (FR-007, FR-008, FR-009)
- [ ] T002 (antwort-21e.2) [P] Add `code_interpreter_call` SSE event type constants (`EventCodeInterpreterInProgress`, `EventCodeInterpreterInterpreting`, `EventCodeInterpreterCompleted`) to `pkg/api/events.go` with MarshalJSON cases (FR-014)
- [ ] T003 (antwort-21e.3) [P] Update `classifyToolType()` in `pkg/engine/loop.go` to recognize `code_interpreter` and return the correct lifecycle event types (FR-014)

**Checkpoint**: Types compile. SSE event classification works for code_interpreter.

---

## Phase 2: Sandbox HTTP Client

**Goal**: HTTP client that calls the sandbox server's /execute endpoint.

- [ ] T004 (antwort-at5.1) Create `pkg/tools/builtins/codeinterpreter/types.go`: define sandbox request/response types matching the Spec 024 REST API (code, timeout_seconds, requirements, files, stdout, stderr, exit_code, files_produced) (FR-003, FR-006)
- [ ] T005 (antwort-at5.2) Create `pkg/tools/builtins/codeinterpreter/client.go`: HTTP client with `Execute(ctx, sandboxURL, request) -> (response, error)`. Handles JSON encoding, timeout via context, error mapping (FR-003, FR-004, FR-005, FR-006)

**Checkpoint**: Sandbox HTTP client can call a sandbox server and return results.

---

## Phase 3: SandboxClaim Client (Adapter)

**Goal**: Kubernetes adapter that creates/watches/deletes SandboxClaim CRDs.

- [ ] T006 (antwort-iqr.1) Create `pkg/tools/builtins/codeinterpreter/kubernetes/sandbox.go`: SandboxClaim client using client-go. `AcquireSandbox(ctx, template, namespace, timeout) -> (podAddress, claimName, error)` creates a SandboxClaim, watches until Ready, returns pod address. `ReleaseSandbox(ctx, claimName, namespace) -> error` deletes the claim (FR-010, FR-011, FR-012)
- [ ] T007 (antwort-iqr.2) Add `k8s.io/client-go` dependency to `go.mod`. Only imported by the adapter package (constitution Principle II)

**Checkpoint**: SandboxClaim client can acquire and release sandbox pods on a cluster with agent-sandbox.

---

## Phase 4: CodeInterpreter FunctionProvider

**Goal**: Full FunctionProvider that wires sandbox client, SandboxClaim lifecycle, and output formatting.

- [ ] T008 (antwort-6q6.1) Create `pkg/tools/builtins/codeinterpreter/provider.go`: implements FunctionProvider interface. `Name()` returns "code_interpreter". `Tools()` returns the tool definition with code and requirements parameters. `Execute()` acquires sandbox (via SandboxClaim or static URL), calls sandbox HTTP client, formats result as code_interpreter_call output, releases sandbox (FR-001, FR-002, FR-003, FR-007, FR-008, FR-009)
- [ ] T009 (antwort-6q6.2) Implement static URL mode in provider: when `sandbox_url` is configured, skip SandboxClaim and call the URL directly (FR-013, FR-016)
- [ ] T010 (antwort-6q6.3) Implement file output handling: decode base64 files from sandbox response, format as CodeInterpreterOutput entries with type "image" for image files and type "logs" for text (FR-009)
- [ ] T011 (antwort-6q6.4) Add configuration support: add `code_interpreter` to the providers section in config loader, validate mutual exclusion of sandbox_url and sandbox_template (FR-015, FR-016)

**Checkpoint**: CodeInterpreter provider registered and functional with static URL.

---

## Phase 5: Server Wiring and Integration

**Goal**: Wire the provider into the server and test end-to-end.

- [ ] T012 (antwort-3qw.1) Wire code_interpreter provider in `cmd/server/main.go`: register it in the function registry when enabled in config (FR-001)
- [ ] T013 (antwort-3qw.2) Add mock sandbox server to integration test helpers in `test/integration/helpers_test.go`: respond to /execute with deterministic results based on code content
- [ ] T014 (antwort-3qw.3) Create `test/integration/codeinterpreter_test.go`: test code_interpreter tool in the agentic loop using static URL mode. Verify code execution, output format, SSE events (FR-001 through FR-009, FR-013, FR-014)
- [ ] T015 (antwort-3qw.4) Run full test suite (`go test ./pkg/... ./test/integration/`) and verify zero regressions

**Checkpoint**: Code interpreter works end-to-end with mock sandbox in integration tests.

---

## Phase 6: Polish

- [ ] T016 (antwort-x89.1) Run `go vet ./pkg/... ./cmd/...` and verify clean
- [ ] T017 (antwort-x89.2) Run `make api-test` to verify no conformance regressions

**Checkpoint**: All tests green. Code interpreter ready for deployment.

---

## Dependencies

- Phase 1: No dependencies
- Phase 2: No dependencies (standalone HTTP client)
- Phase 3: No dependencies (standalone K8s adapter)
- Phase 4: Depends on Phase 2 and 3
- Phase 5: Depends on Phase 4
- Phase 6: Depends on Phase 5
