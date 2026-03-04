# Feature Specification: E2E Testing with LLM Recording/Replay

**Feature Branch**: `043-e2e-testing`
**Created**: 2026-03-04
**Status**: Draft
**Input**: Brainstorm 35: E2E testing with LLM recording/replay for comprehensive feature coverage in CI

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Verify Core API Through Real Deployment (Priority: P1)

A developer pushes a change to antwort. The CI pipeline deploys antwort to a temporary cluster and runs E2E tests that create, retrieve, stream, and delete responses using a standard client SDK. The tests exercise the full deployed stack (real networking, real configuration, real middleware chain) with realistic LLM responses replayed from pre-recorded sessions. The developer sees pass/fail results within the CI timeout and can trust that the core API works correctly in a real deployment.

**Why this priority**: The core response lifecycle (create, get, list, delete, streaming) is the foundation of the entire API. If this doesn't work in a real deployment, nothing else matters.

**Independent Test**: Deploy antwort to a temporary cluster with a replay backend, run SDK-based tests that create and consume responses, verify correct behavior.

**Acceptance Scenarios**:

1. **Given** antwort is deployed to a cluster with a replay backend loaded with pre-recorded LLM responses, **When** a test creates a non-streaming response using a standard client SDK, **Then** the response contains a valid ID, model, output text, and usage data matching what the recorded LLM would produce.
2. **Given** antwort is deployed with streaming support, **When** a test creates a streaming response, **Then** the test receives the complete SSE event sequence (response.created, output_item.added, text deltas, response.completed) with correct content.
3. **Given** a response has been created, **When** a test retrieves it by ID, **Then** the stored response matches the originally created response.
4. **Given** a response exists, **When** a test deletes it and then tries to retrieve it, **Then** the deletion succeeds and subsequent retrieval returns not found.

---

### User Story 2 - Verify Multi-User Authentication and Isolation (Priority: P1)

A developer changes authentication or ownership logic. The E2E tests verify that API key authentication works correctly in a deployed environment, that users can only see their own resources, and that unauthenticated requests are rejected. The developer has confidence that multi-user isolation works end-to-end before merging.

**Why this priority**: Authentication and ownership isolation are security-critical. Bugs here leak data between users. E2E validation in a real deployment catches configuration errors that unit tests miss.

**Independent Test**: Deploy antwort with API key authentication configured, run tests as different users, verify isolation.

**Acceptance Scenarios**:

1. **Given** antwort is deployed with API key authentication, **When** a test sends a request with a valid API key, **Then** the request succeeds and the response is associated with the authenticated user.
2. **Given** antwort requires authentication, **When** a test sends a request without credentials or with an invalid key, **Then** the request is rejected with an appropriate error.
3. **Given** two users each create resources, **When** one user tries to access the other's resource, **Then** the resource is not visible (as if it doesn't exist).

---

### User Story 3 - Verify Agentic Loop with Tool Calls (Priority: P1)

A developer modifies the agentic loop or tool dispatch logic. The E2E tests verify that multi-turn tool calling works in a deployed environment: the LLM requests a tool call, antwort executes it, sends the result back, and the LLM produces a final response. All of this uses replayed LLM responses so no real inference is needed.

**Why this priority**: The agentic loop is the most complex part of the system, involving multi-turn conversations with tool execution between turns. E2E validation ensures the full cycle works with realistic LLM behavior.

**Independent Test**: Deploy antwort with a tool executor and replay backend loaded with multi-turn recordings, run a test that triggers a tool call scenario, verify the complete cycle.

**Acceptance Scenarios**:

1. **Given** antwort is deployed with a tool executor and the replay backend has recordings for a tool-calling conversation, **When** a test sends a request that triggers tool use, **Then** the final response includes both the tool call and the tool result in the conversation output.
2. **Given** the same setup with streaming enabled, **When** a test sends a streaming request that triggers tool use, **Then** the test receives tool lifecycle SSE events (function_call_arguments.delta, function_call_arguments.done) followed by the final text response.

---

### User Story 4 - Verify Audit Trail in Deployment (Priority: P2)

An operator enables audit logging in the deployment configuration. The E2E tests verify that audit events are actually emitted in a deployed environment by checking the audit output after performing various operations (authentication, resource creation, deletion).

**Why this priority**: Audit logging was just implemented (Spec 042) and is critical for compliance. Verifying it works in a real deployment (not just in-process tests) catches configuration and wiring issues.

**Independent Test**: Deploy antwort with audit logging enabled (writing to a file), perform operations, read the audit file and verify events.

**Acceptance Scenarios**:

1. **Given** antwort is deployed with audit logging enabled to a file, **When** a test authenticates and creates a response, **Then** the audit file contains auth.success and resource.created events with correct user identity and resource details.
2. **Given** antwort is deployed with audit logging enabled, **When** a test sends a request with invalid credentials, **Then** the audit file contains an auth.failure event.

---

### User Story 5 - Replay Backend with Pre-Recorded LLM Responses (Priority: P1)

A developer needs to add new E2E test scenarios. They can record real LLM interactions once (against a real backend like vLLM or Ollama), store the recordings as versioned files, and replay them deterministically in CI without needing GPU access or a real LLM. The replay backend matches incoming requests to stored recordings using content-based hashing, supporting both non-streaming and streaming (SSE) responses.

**Why this priority**: The replay backend is the foundation that makes all other E2E tests possible without requiring real LLM infrastructure in CI.

**Independent Test**: Start the replay backend with a set of recordings, send matching requests, verify correct responses are returned.

**Acceptance Scenarios**:

1. **Given** the replay backend is loaded with a non-streaming recording, **When** a matching request arrives, **Then** the backend returns the recorded response with correct status code, headers, and body.
2. **Given** the replay backend is loaded with a streaming recording, **When** a matching request arrives, **Then** the backend streams the recorded SSE chunks in order with correct formatting.
3. **Given** the replay backend receives a request with no matching recording, **When** the request is processed, **Then** the backend returns an error response with diagnostic information (the request hash and available recordings) to help the developer create the missing recording.
4. **Given** the replay backend is started without a recordings directory, **When** requests arrive, **Then** the backend falls back to the existing deterministic mock responses (backward compatibility).

---

### User Story 6 - Recording New LLM Interactions (Priority: P3)

A developer needs to create recordings for a new test scenario. They configure the mock backend in recording mode, point it at a real LLM backend, run the scenario, and the request/response pairs are automatically saved as JSON files. These files can then be committed to the repository for replay in CI.

**Why this priority**: Recording is needed to bootstrap the initial set of recordings and to add new scenarios over time, but it's not needed for day-to-day CI runs.

**Independent Test**: Start the mock backend in recording mode pointing at a test LLM, make requests, verify recording files are created with correct format.

**Acceptance Scenarios**:

1. **Given** the mock backend is started in recording mode with a target backend URL, **When** a request arrives, **Then** the request is forwarded to the real backend, the response is returned to the caller, and both are saved as a JSON recording file.
2. **Given** recordings already exist for some requests, **When** the mock backend is in record-if-missing mode and a new request arrives, **Then** only requests without existing recordings are forwarded and saved.

---

### Edge Cases

- What happens when a recording file is corrupted or contains invalid JSON? The replay backend should return an error with the file path for debugging, not silently fail.
- What happens when multiple requests produce the same hash? This would indicate a non-deterministic request. The system should detect and warn about hash collisions.
- What happens when streaming recordings are incomplete (missing `[DONE]` sentinel)? The replay backend should stream what it has and close the connection gracefully.
- What happens when the E2E test cluster fails to start within the CI timeout? The test job should report clear diagnostics (Pod status, events, logs) before failing.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST provide a replay backend that serves pre-recorded LLM responses, matching incoming requests to recordings using content-based hashing.
- **FR-002**: The replay backend MUST support both non-streaming (JSON) and streaming (SSE) response replay.
- **FR-003**: The replay backend MUST support both the Chat Completions protocol (`/v1/chat/completions`) and the Responses API protocol (`/v1/responses`).
- **FR-004**: The replay backend MUST fall back to existing deterministic mock responses when no recordings directory is configured (backward compatibility with current tests).
- **FR-005**: The system MUST provide a recording mode that captures request/response pairs from a real LLM backend and saves them as replayable JSON files.
- **FR-006**: Recordings MUST be stored as individual JSON files, one per request/response pair, in a version-controlled directory within the repository.
- **FR-007**: The E2E test suite MUST use a standard client SDK to call the antwort API, validating that the API is compatible with standard clients.
- **FR-008**: The E2E test suite MUST verify core API operations: response creation (streaming and non-streaming), retrieval, listing, and deletion.
- **FR-009**: The E2E test suite MUST verify multi-user authentication and per-user resource isolation in a deployed environment.
- **FR-010**: The E2E test suite MUST verify the agentic loop with tool calling using replayed multi-turn LLM conversations.
- **FR-011**: The E2E test suite MUST verify audit event emission in a deployed environment when audit logging is enabled.
- **FR-012**: The E2E test suite MUST run in CI against a temporary cluster, completing within the existing CI timeout constraints.
- **FR-013**: The replay backend MUST return diagnostic information (request hash, available recordings) when no matching recording is found, to help developers create missing recordings.
- **FR-014**: The system MUST provide a mechanism to convert existing recordings from other projects (e.g., Llama Stack format) into the antwort recording format.

### Key Entities

- **Recording**: A JSON file containing a captured request/response pair from an LLM backend interaction. Contains: request (method, path, body), response (status, headers, body or SSE chunks), streaming flag, and metadata (recording timestamp, source, test ID).
- **Replay Backend**: A server component that matches incoming requests to stored recordings and returns the corresponding responses. Supports both protocols (Chat Completions and Responses API) and both modes (streaming and non-streaming).
- **E2E Test Suite**: A collection of tests that exercise the full deployed antwort stack through a standard client SDK, verifying feature correctness in a real deployment environment.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: The E2E test suite covers at least 15 distinct test scenarios across core API, authentication, agentic loop, and audit features.
- **SC-002**: All E2E tests pass deterministically in CI with 100% reproducibility (no flaky tests due to timing, ordering, or infrastructure variance).
- **SC-003**: The complete E2E test run (cluster setup, deployment, tests, teardown) completes within 10 minutes on GitHub Actions free-tier runners.
- **SC-004**: Adding a new E2E test scenario requires only: one recording file and one test function. No changes to infrastructure or deployment manifests.
- **SC-005**: The replay backend serves recorded responses with less than 10ms latency per request, ensuring tests are not bottlenecked by the mock.
- **SC-006**: Developers can run the same E2E tests locally against a local antwort instance with the replay backend, without requiring a cluster.

## Assumptions

- The existing `cmd/mock-backend` binary is evolved to support replay mode rather than creating a new binary. This maintains backward compatibility with existing CI jobs and tests.
- Recordings are small enough to commit to the repository (estimated 10-50KB per recording, 15-30 recordings for Phase 1, total under 1MB).
- The existing `kubernetes` CI job in `.github/workflows/ci.yml` is extended rather than replaced. The current functionality (healthz, SDK tests) is preserved.
- Initial recordings for the hybrid approach will be generated by running antwort against a real LLM backend (Ollama or vLLM) in a one-time setup. Converted llama-stack recordings supplement these where protocol-compatible.
- The E2E test suite is separate from the existing integration tests (`test/integration/`). Integration tests use in-process httptest servers; E2E tests use real deployed services.

## Dependencies

- **Spec 007 (Authentication)**: E2E tests verify API key authentication in a deployed environment.
- **Spec 040 (Resource Ownership)**: E2E tests verify per-user resource isolation.
- **Spec 042 (Audit Logging)**: E2E tests verify audit event emission in deployment.
- **Spec 033 (CI Pipeline)**: The E2E tests extend the existing CI pipeline with a new or enhanced job.
