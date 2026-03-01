# Feature Specification: CI/CD Pipeline

**Feature Branch**: `033-ci-pipeline`
**Created**: 2026-03-01
**Status**: Draft
**Input**: CI/CD pipeline with 4 parallel jobs: lint+unit tests, OpenResponses conformance, SDK client compatibility (Python/TypeScript), and Kubernetes deployment validation via kind. Zero-cost GitHub Actions only.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Fast Feedback on Code Quality (Priority: P1)

A contributor opens a pull request with a code change. Within minutes, they see whether their change compiles, passes lint checks, and all unit tests pass. This is the fastest feedback loop and the first gate every change must pass.

**Why this priority**: Fast feedback is the most important CI capability. Without it, contributors waste time on broken changes and reviewers waste time reviewing code that does not compile.

**Independent Test**: A contributor pushes a commit to a PR branch and receives lint + unit test results within 3 minutes.

**Acceptance Scenarios**:

1. **Given** a contributor pushes to a PR branch, **When** the pipeline runs, **Then** lint and unit test results appear as a GitHub check within 3 minutes.
2. **Given** a change introduces a compile error, **When** the pipeline runs, **Then** the lint-test job fails with a clear error message identifying the broken file and line.
3. **Given** a change breaks a unit test, **When** the pipeline runs, **Then** the test failure is reported with the test name, expected vs actual output, and file location.

---

### User Story 2 - OpenResponses Spec Compliance (Priority: P1)

A contributor changes the API surface or engine behavior. The pipeline validates that the change remains compliant with the OpenResponses specification by running the official compliance suite and checking for API schema drift.

**Why this priority**: Spec compliance is the project's core value proposition. Any regression in compliance breaks the promise that standard OpenAI SDKs work without modification.

**Independent Test**: A change that alters an API response field triggers a compliance test failure identifying the exact deviation from the spec.

**Acceptance Scenarios**:

1. **Given** a PR is opened, **When** the pipeline runs, **Then** the API schema is validated against the upstream OpenResponses specification and any breaking changes are flagged.
2. **Given** a change modifies the response format, **When** the compliance suite runs, **Then** each test case reports pass/fail with a summary table in the GitHub Actions step summary.
3. **Given** all compliance tests pass, **When** the contributor checks the PR, **Then** they see a green conformance check with the number of tests passed.

---

### User Story 3 - SDK Client Compatibility (Priority: P1)

A contributor changes streaming behavior, response structure, or tool calling. The pipeline runs test scripts using the official Python and TypeScript OpenAI SDKs to verify that real client code still works correctly against the changed server.

**Why this priority**: SDK compatibility cannot be inferred from Go unit tests alone. The official SDKs have their own parsing, validation, and type mapping. A change that passes Go tests can still break Python or TypeScript clients.

**Independent Test**: A test script using the Python `openai` package sends a streaming request, parses the response, and verifies the output matches expectations.

**Acceptance Scenarios**:

1. **Given** a PR is opened, **When** the SDK test job runs, **Then** the Python OpenAI SDK successfully creates a response, streams it, chains a conversation, calls a tool, and validates structured output.
2. **Given** a PR is opened, **When** the SDK test job runs, **Then** the TypeScript OpenAI SDK passes the same test cases as Python.
3. **Given** a change breaks SSE event formatting, **When** the SDK tests run, **Then** the streaming test fails, identifying that the SDK could not parse the event stream.
4. **Given** a change breaks tool call response structure, **When** the SDK tests run, **Then** the tool calling test fails, identifying the parsing error in the SDK.

---

### User Story 4 - Kubernetes Deployment Validation (Priority: P2)

A contributor modifies container images, Kustomize manifests, or server startup code. The pipeline creates a real Kubernetes cluster (via kind), deploys the built images, and verifies that Pods start, health endpoints respond, and requests are handled correctly.

**Why this priority**: Kubernetes is the only supported deployment target. Manifests and container images can break silently (wrong ports, missing env vars, image build failures) without end-to-end validation. However, this is P2 because the other three jobs catch most code-level regressions.

**Independent Test**: The pipeline deploys antwort into a kind cluster using the quickstart 01-minimal manifests and successfully sends a test request through the Kubernetes service.

**Acceptance Scenarios**:

1. **Given** a PR is opened, **When** the Kubernetes job runs, **Then** container images build successfully from the current code.
2. **Given** images are built, **When** they are deployed to a kind cluster via kustomize, **Then** all Pods reach the Ready state within 60 seconds.
3. **Given** Pods are running, **When** the health endpoints are checked, **Then** both `/healthz` and `/readyz` return 200.
4. **Given** services are healthy, **When** a test request is sent via port-forward, **Then** a valid response is returned.
5. **Given** the Kustomize manifests are modified with invalid YAML, **When** the pipeline runs, **Then** the deployment step fails with a clear error.

---

### Edge Cases

- What happens when GitHub Actions runners are slow? Each job has a timeout (lint: 5min, conformance: 10min, SDK: 10min, K8s: 10min) to prevent runaway builds.
- What happens when the kind cluster fails to start? The job reports the kind error output and fails clearly rather than timing out on health checks.
- What happens when the OpenResponses compliance repo changes? The suite is cloned at HEAD each run, so upstream changes are detected immediately. If this causes flaky failures, a pinned commit hash can be used.
- What happens when an SDK releases a breaking version? The tests use the latest SDK version by default. If a breakage is from the SDK side (not antwort), the contributor can pin the SDK version in the test requirements file.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The pipeline MUST run on every push to the main branch and on every pull request targeting main.
- **FR-002**: The pipeline MUST execute four jobs in parallel: lint-test, conformance, sdk-clients, and kubernetes.
- **FR-003**: The lint-test job MUST run Go vet and all unit tests, completing within 5 minutes.
- **FR-004**: The conformance job MUST validate the API schema against the upstream OpenResponses specification using oasdiff.
- **FR-005**: The conformance job MUST run the Go integration tests and the official OpenResponses TypeScript compliance suite.
- **FR-006**: The conformance job MUST produce a GitHub Actions step summary table showing each compliance test with its pass/fail status.
- **FR-007**: The sdk-clients job MUST test the Python OpenAI SDK with at least 6 test cases: basic response, streaming, tool calling, conversation chaining, structured output, and model listing.
- **FR-008**: The sdk-clients job MUST test the TypeScript OpenAI SDK with the same test cases as Python.
- **FR-009**: The sdk-clients job MUST use the project's mock backend (not a real LLM) to provide deterministic test responses.
- **FR-010**: The kubernetes job MUST build container images from the current source code without pushing to any registry.
- **FR-011**: The kubernetes job MUST create a kind cluster, load the built images, and deploy using the quickstart 01-minimal Kustomize manifests plus a mock backend.
- **FR-012**: The kubernetes job MUST verify Pod readiness, health endpoint responses, and a successful test request through the Kubernetes service.
- **FR-013**: All jobs MUST run on GitHub Actions free-tier runners (ubuntu-latest) without requiring any paid services, GPUs, or external infrastructure.
- **FR-014**: The pipeline MUST report each job as a separate GitHub status check so that branch protection rules can require all four to pass.
- **FR-015**: SDK test scripts MUST reside in the project repository under `test/sdk/python/` and `test/sdk/typescript/`.
- **FR-016**: The pipeline MUST replace the existing `api-conformance.yml` workflow, consolidating all CI into a single workflow file.

### Key Entities

- **Job**: A parallel unit of work in the pipeline. Each job runs on its own runner and produces an independent pass/fail status.
- **Mock Backend**: A deterministic LLM simulator that responds based on request classification (tool calls, streaming, system prompts) without requiring GPU or model weights.
- **Compliance Suite**: The official OpenResponses TypeScript test suite that validates API conformance against the specification.
- **SDK Test Script**: A test file in Python or TypeScript that uses the official OpenAI SDK to exercise the antwort server and verify correct behavior.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Every pull request receives automated feedback from all four jobs within 10 minutes of the push.
- **SC-002**: The pipeline catches 100% of API schema breaking changes (validated by intentionally introducing a breaking change and confirming detection).
- **SC-003**: The Python and TypeScript SDK test suites each cover at least 6 distinct interaction patterns (response creation, streaming, tool calling, conversation chaining, structured output, model listing).
- **SC-004**: The Kubernetes job successfully deploys, health-checks, and sends a test request through the kind cluster on every run.
- **SC-005**: The entire pipeline runs within GitHub Actions free-tier limits with zero external cost.
- **SC-006**: A contributor can understand a test failure from the GitHub Actions log without needing to reproduce locally (clear error messages, relevant log output on failure).

## Assumptions

- GitHub Actions free-tier provides sufficient compute for all four parallel jobs (ubuntu-latest runners with 2 cores, 7GB RAM).
- The `kind` tool can create a functional Kubernetes cluster on a GitHub Actions runner within 2 minutes.
- The official OpenAI Python and TypeScript SDKs are stable enough that test failures indicate antwort regressions, not SDK bugs.
- The existing mock backend (`cmd/mock-backend/`) provides sufficient response variety for SDK test cases (basic text, streaming, tool calls, multi-turn).
- Docker is available on GitHub Actions runners for building container images and running kind.

## Dependencies

- Spec 006 (Conformance): provides the OpenResponses compliance suite integration
- Spec 010 (Kustomize Deploy): provides the Kubernetes deployment manifests
- Spec 015 (Quickstarts): provides the quickstart 01-minimal manifests used in the K8s job
- Existing `.github/workflows/api-conformance.yml`: will be replaced by the new unified workflow

## Out of Scope

- Container image publishing to a registry (handled by a separate release workflow)
- Performance or load testing
- Multi-architecture image builds (arm64, etc.)
- Integration with external LLM backends (all tests use the mock backend)
- Notifications (Slack, email) on pipeline failure
- Code coverage reporting
