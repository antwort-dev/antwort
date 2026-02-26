# Tasks: Code Interpreter Tool

## Phase 1: Types and Interfaces

**Purpose**: Add the code_interpreter item types and sandbox client interface.

- [x] T001 (antwort-x9x.1) (antwort-21e.1) Add `CodeInterpreterCallData` and `CodeInterpreterOutput` types to `pkg/api/types.go`. Add `ItemTypeCodeInterpreterCall` constant. Support `logs` and `image` output types (FR-007, FR-008, FR-009)
- [x] T002 (antwort-x9x.2) (antwort-21e.2) [P] Add `code_interpreter_call` SSE event type constants (`EventCodeInterpreterInProgress`, `EventCodeInterpreterInterpreting`, `EventCodeInterpreterCompleted`) to `pkg/api/events.go` with MarshalJSON cases (FR-014)
- [x] T003 (antwort-x9x.3) (antwort-21e.3) [P] Update `classifyToolType()` in `pkg/engine/loop.go` to recognize `code_interpreter` and return the correct lifecycle event types (FR-014)

**Checkpoint**: Types compile. SSE event classification works for code_interpreter.

---

## Phase 2: Sandbox HTTP Client

**Goal**: HTTP client that calls the sandbox server's /execute endpoint.

- [x] T004 (antwort-6lv.1) (antwort-at5.1) Create `pkg/tools/builtins/codeinterpreter/types.go`: define sandbox request/response types matching the Spec 024 REST API (code, timeout_seconds, requirements, files, stdout, stderr, exit_code, files_produced) (FR-003, FR-006)
- [x] T005 (antwort-6lv.2) (antwort-at5.2) Create `pkg/tools/builtins/codeinterpreter/client.go`: HTTP client with `Execute(ctx, sandboxURL, request) -> (response, error)`. Handles JSON encoding, timeout via context, error mapping (FR-003, FR-004, FR-005, FR-006)
- [x] T005a Create `pkg/tools/builtins/codeinterpreter/client_test.go`: unit tests for the sandbox HTTP client. Table-driven tests covering: successful execution, HTTP timeout, invalid JSON response, non-200 status codes, empty stdout, sandbox server unreachable (FR-003, FR-005, FR-006)

**Checkpoint**: Sandbox HTTP client can call a sandbox server and return results. Unit tests cover error paths.

---

## Phase 3: SandboxClaim Adapter (controller-runtime)

**Goal**: Kubernetes adapter implementing `SandboxAcquirer` via SandboxClaim CRDs.

- [x] T007 (antwort-ym5.1) Add `sigs.k8s.io/agent-sandbox` (v0.1.1) and `sigs.k8s.io/controller-runtime` dependencies to `go.mod`. Only imported by the adapter package `kubernetes/` (constitution Principle II) (FR-010)
- [x] T006 (antwort-ym5.2) Create `pkg/tools/builtins/codeinterpreter/kubernetes/acquirer.go`: `claimAcquirer` struct implementing `SandboxAcquirer` interface. `Acquire(ctx)` creates a SandboxClaim CR with `spec.sandboxTemplateRef.name`, watches the Sandbox resource (same name) for `Ready` condition, returns `status.serviceFQDN` as sandbox URL. The returned `release` function deletes the SandboxClaim. Scheme registration for agent-sandbox types. Configurable claim timeout (FR-010, FR-011, FR-012, NFR-001)
- [x] T006a Create `pkg/tools/builtins/codeinterpreter/kubernetes/acquirer_test.go`: tests using controller-runtime `fake.NewClientBuilder()`. Table-driven tests: claim created and Sandbox becomes ready (returns serviceFQDN), claim timeout (Sandbox stays pending), claim deleted after release, claim deleted on acquire error (no leak), concurrent acquisitions (NFR-002) (FR-010, FR-011, FR-012)
- [x] T006b Update `New()` in `pkg/tools/builtins/codeinterpreter/provider.go`: when `sandbox_template` is configured, create a controller-runtime client and instantiate `claimAcquirer`. Remove the "not yet implemented" error (FR-010, FR-016)

**Checkpoint**: SandboxClaim adapter acquires and releases sandbox pods. Tests verify no claim leaks on error paths. Provider works in both static URL and SandboxClaim modes.

---

## Phase 4: CodeInterpreter FunctionProvider

**Goal**: Full FunctionProvider that wires sandbox client, SandboxClaim lifecycle, and output formatting.

- [x] T008 (antwort-3t4.1) (antwort-6q6.1) Create `pkg/tools/builtins/codeinterpreter/provider.go`: implements FunctionProvider interface. `Name()` returns "code_interpreter". `Tools()` returns the tool definition with code and requirements parameters. `Execute()` acquires sandbox (via SandboxClaim or static URL), calls sandbox HTTP client, formats result as code_interpreter_call output, releases sandbox (FR-001, FR-002, FR-003, FR-007, FR-008, FR-009)
- [x] T009 (antwort-3t4.2) (antwort-6q6.2) Implement static URL mode in provider: when `sandbox_url` is configured, skip SandboxClaim and call the URL directly (FR-013, FR-016)
- [x] T010 (antwort-3t4.3) (antwort-6q6.3) Implement file output handling: decode base64 files from sandbox response, format as CodeInterpreterOutput entries with type "image" for image files and type "logs" for text (FR-009)
- [x] T011 (antwort-3t4.4) (antwort-6q6.4) Add configuration support: add `code_interpreter` to the providers section in config loader, validate mutual exclusion of sandbox_url and sandbox_template (FR-015, FR-016)

**Checkpoint**: CodeInterpreter provider registered and functional with static URL.

---

## Phase 5: Server Wiring and Integration

**Goal**: Wire the provider into the server and test end-to-end.

- [x] T012 (antwort-7cn.1) Wire code_interpreter provider in `cmd/server/main.go`: register it in the function registry when enabled in config (FR-001)
- [x] T013 (antwort-3qw.2) Create `pkg/tools/builtins/codeinterpreter/integration_test.go`: integration test using real sandbox-server binary as subprocess. Tests verify: code execution returns stdout, requirements install, error handling, invalid arguments (FR-001 through FR-009, FR-013)
- [x] T014 (antwort-3qw.3) Run full test suite (`go test ./pkg/... ./test/integration/`) and verify zero regressions

**Checkpoint**: Code interpreter works end-to-end with real sandbox-server in integration tests.

---

## Phase 6: Polish

- [x] T015 (antwort-3qw.4) Run `go vet ./pkg/... ./cmd/...` and verify clean
- [x] T016 (antwort-vw5.1) Run `make api-test` to verify no conformance regressions

**Checkpoint**: All tests green. Code interpreter ready for deployment.

---

## Dependencies

- Phase 1: Done
- Phase 2: Done
- Phase 3: T007 first (deps), then T006 + T006a (parallel), then T006b
- Phase 4: Done
- Phase 5: Depends on Phase 3 (T006b wires the acquirer into provider)
- Phase 6: Depends on Phase 5
