# Feature Specification: OpenResponses Conformance Testing

**Feature Branch**: `006-conformance`
**Created**: 2026-02-18
**Status**: Draft
**Input**: User description: "OpenResponses conformance testing infrastructure: server wiring layer, mock backend, official compliance test suite integration, CI-ready test runner, and layered test profiles."

## Overview

This specification defines the conformance testing infrastructure for antwort. It creates the wiring layer needed to run antwort as a standalone server, provides a deterministic mock Chat Completions backend for reproducible testing, and integrates the official OpenResponses TypeScript compliance test suite from [openresponses.org](https://www.openresponses.org/compliance).

The testing approach is layered: test profiles correspond to implemented feature specs, allowing the conformance score to grow as more specs are completed. Each profile enables a subset of the 6 official compliance tests based on what antwort currently supports. As new specs are implemented (auth, MCP, deployment), the conformance profile expands to cover their test cases.

The goal is a CI-integrated conformance pipeline: on every PR, start antwort with the mock backend, run the official test suite, and report the conformance score. The score becomes a project health metric.

## Clarifications

### Session 2026-02-18

- Q: Should we reimplement the compliance tests in Go? -> A: No. Use the official TypeScript suite from the openresponses/openresponses repository. It validates against the canonical Zod schemas and stays synchronized with spec changes. Reimplementing risks drift.
- Q: How to run the TypeScript suite in CI? -> A: Run it in a container. The official suite uses `bun` as the runtime. A containerized runner (podman) executes the tests against a running antwort instance.
- Q: What about tests that need a real LLM (like image input)? -> A: The mock backend provides deterministic responses for all test scenarios. No real LLM is needed for conformance testing. The mock produces responses that satisfy the schema validators.
- Q: How to handle features not yet implemented? -> A: Test profiles. A "core" profile runs tests 1-4 and 6 (basic, streaming, system prompt, tool calling, multi-turn). Multi-turn (test 6) works without storage because the official test sends conversation history inline via the input array, not via previous_response_id. An "extended" profile adds test 5 (image input, requires vision-capable backend mock). Profiles grow as future specs add testable features.
- Q: How does profile filtering work with the official suite? -> A: The official suite runs all 6 tests. Profile filtering is post-hoc: all tests execute, but results are filtered by profile. Tests outside the profile are reported as "skipped" regardless of outcome. This preserves the "run as-is" principle without forking the suite.

## User Scenarios & Testing

### User Story 1 - Run Antwort as a Standalone Server (Priority: P1)

A developer starts antwort as a standalone HTTP server with minimal configuration: a backend URL, a model name, and a port. The server accepts OpenResponses API requests, proxies to the backend, and returns conformant responses. This is the wiring layer that connects all implemented specs into a runnable process.

**Why this priority**: Without a runnable server, no conformance testing is possible. This is the prerequisite for everything else.

**Independent Test**: Start the server, send a health check request, verify it responds. Send a simple inference request to a mock backend, verify the response is valid.

**Acceptance Scenarios**:

1. **Given** a configured backend URL and model, **When** the server starts, **Then** it listens on the configured port and accepts HTTP requests on `/v1/responses`
2. **Given** a running server with a mock backend, **When** a valid `POST /v1/responses` is sent, **Then** the server returns a valid OpenResponses response
3. **Given** a running server with storage configured, **When** a response is created, **Then** it can be retrieved via `GET /v1/responses/{id}` and deleted via `DELETE /v1/responses/{id}`
4. **Given** environment variables for configuration, **When** the server starts, **Then** it reads configuration from environment variables (backend URL, model, port, storage DSN)

---

### User Story 2 - Mock Chat Completions Backend (Priority: P1)

A developer runs a mock Chat Completions server that provides deterministic responses for each conformance test scenario. The mock understands the 6 compliance test prompts and returns appropriate responses (text completions, streaming chunks, tool calls, multimodal acknowledgments). No real LLM is needed.

**Why this priority**: Deterministic testing requires predictable backend behavior. A real LLM introduces non-determinism that makes conformance testing unreliable.

**Independent Test**: Start the mock, send a Chat Completions request, verify the response is valid and deterministic.

**Acceptance Scenarios**:

1. **Given** a mock backend, **When** a non-streaming Chat Completions request is sent with "Say hello in exactly 3 words", **Then** the mock returns a fixed 3-word response with valid usage statistics
2. **Given** a mock backend, **When** a streaming request is sent with "Count from 1 to 5", **Then** the mock returns SSE chunks with "1", "2", "3", "4", "5" as text deltas, followed by `[DONE]`
3. **Given** a mock backend, **When** a request with tool definitions and a weather question is sent, **Then** the mock returns a response with a `function_call` tool call for the weather function
4. **Given** a mock backend, **When** a request with an image content part is sent, **Then** the mock returns a response acknowledging the image
5. **Given** a mock backend, **When** a request with system instructions is sent, **Then** the mock returns a response consistent with the system prompt

---

### User Story 3 - Run Official Conformance Suite (Priority: P1)

A developer runs the official OpenResponses compliance test suite against a running antwort instance. The suite validates response schemas (via Zod), streaming event formats, and semantic correctness. Results are reported with a pass/fail status per test and an overall conformance score.

**Why this priority**: The official suite is the canonical validator. Running it ensures antwort is truly conformant, not just "looks right."

**Independent Test**: Start antwort with mock backend, run the compliance suite, verify all enabled tests pass.

**Acceptance Scenarios**:

1. **Given** a running antwort instance with a mock backend, **When** the official compliance suite runs, **Then** all tests in the active profile pass with schema and semantic validation
2. **Given** the compliance suite results, **When** the test runner finishes, **Then** it outputs a conformance score (e.g., 6/6 tests passed) and per-test details
3. **Given** a CI pipeline, **When** a PR is submitted, **Then** the conformance suite runs automatically and the score is reported

---

### User Story 4 - Layered Test Profiles (Priority: P2)

An operator selects a conformance test profile that matches the implemented feature set. The "core" profile tests basic text, streaming, system prompts, tool calling, and multi-turn conversation (tests 1-4, 6). Multi-turn works without storage because the official test sends history inline. The "extended" profile adds image input (test 5, requires vision-capable mock). As future specs add testable features, profiles expand.

**Why this priority**: Not all features are implemented at once. Profiles prevent false failures for features that are intentionally not yet available.

**Independent Test**: Run with the "core" profile, verify only tests 1-4 run. Switch to "full" profile, verify all 6 tests run.

**Acceptance Scenarios**:

1. **Given** the "core" profile, **When** the suite runs, **Then** results for basic text, streaming, system prompt, tool calling, and multi-turn (tests 1-4, 6) are reported; image input (test 5) is reported as skipped
2. **Given** the "extended" profile, **When** the suite runs, **Then** all 6 tests are reported (core + image input)
3. **Given** any profile, **When** all 6 tests execute, **Then** only tests in the active profile contribute to the pass/fail score; tests outside the profile are marked "skipped"
4. **Given** a future profile update, **When** new tests are added to the official suite, **Then** they default to "skipped" until explicitly included in a profile

---

### User Story 5 - Conformance Score Tracking (Priority: P3)

A project maintainer tracks the conformance score over time. Each test run records the profile used, tests passed/failed, and timestamp. The score is visible in CI output and optionally in a badge or report.

**Why this priority**: Score tracking provides visibility into conformance progress as more specs are implemented.

**Independent Test**: Run the suite twice with different results, verify the scores are recorded and can be compared.

**Acceptance Scenarios**:

1. **Given** a completed test run, **When** results are saved, **Then** the score (passed/total) and per-test status are available as structured output (JSON)
2. **Given** CI integration, **When** a PR is tested, **Then** the conformance score appears in the CI output

---

### Edge Cases

- What happens when the mock backend receives a request it doesn't recognize? It returns a generic text completion response with a warning in the output. This prevents the conformance test from failing due to mock limitations rather than antwort bugs.
- What happens when antwort is not running when the suite starts? The test runner waits for a configurable startup timeout (default 10 seconds) before failing with a clear error.
- What happens when the official test suite is updated with new tests? The profile system handles this gracefully. Unknown tests are skipped. When antwort adds support, the profile is updated to include them.
- What happens when antwort returns a response that passes schema validation but fails semantic checks? This is reported as a test failure with details about which semantic validator failed, helping identify the specific translation issue.

## Requirements

### Functional Requirements

**Server Wiring Layer**

- **FR-001**: The system MUST provide a server binary (`cmd/server`) that wires together the transport layer, engine, provider, and optional storage into a running HTTP server
- **FR-002**: The server MUST be configurable via environment variables: backend URL, model name, listen port, optional storage DSN, optional API key
- **FR-003**: The server MUST expose `/v1/responses` (POST, GET, DELETE) endpoints per Spec 002
- **FR-004**: The server MUST support graceful shutdown on SIGINT/SIGTERM
- **FR-005**: The server MUST optionally expose a health endpoint for readiness checks

**Mock Backend**

- **FR-006**: The system MUST provide a mock Chat Completions server (`cmd/mock-backend`) that returns deterministic responses
- **FR-007**: The mock MUST handle non-streaming requests and return valid `chatCompletionResponse` JSON
- **FR-008**: The mock MUST handle streaming requests and return valid SSE chunks followed by `[DONE]`
- **FR-009**: The mock MUST return tool call responses when tool definitions are present in the request
- **FR-010**: The mock MUST return valid responses for multimodal requests (image content parts)
- **FR-011**: The mock MUST return valid usage statistics in all responses
- **FR-012**: The mock MUST be deterministic: the same request always produces the same response

**Conformance Suite Integration**

- **FR-013**: The system MUST integrate the official OpenResponses compliance test suite from [openresponses/openresponses](https://github.com/openresponses/openresponses)
- **FR-014**: The test runner MUST execute the TypeScript compliance suite via a containerized runtime (using podman)
- **FR-015**: The test runner MUST pass the antwort server URL and configuration to the compliance suite
- **FR-016**: The test runner MUST capture test results (pass/fail per test, overall score) and output them in a structured format

**Test Profiles**

- **FR-017**: The system MUST support test profiles that select which compliance tests to run
- **FR-018**: The "core" profile MUST include: basic text response, streaming response, system prompt, tool calling, and multi-turn conversation (tests 1-4, 6)
- **FR-019**: The "extended" profile MUST include: all 6 official compliance tests (core + image input)
- **FR-020**: Profile filtering MUST be post-hoc: all official tests execute, but the conformance score only counts tests in the active profile. Tests outside the profile are reported as "skipped".
- **FR-021**: Tests outside the active profile MUST NOT affect the conformance score (skipped, not failed)
- **FR-022**: Profiles MUST be extensible: new tests from future specs (auth, MCP, etc.) can be added to profiles without changing the test runner

**CI Integration**

- **FR-023**: The conformance test pipeline MUST be runnable via a single command (e.g., `make conformance PROFILE=core`)
- **FR-024**: The pipeline MUST start the mock backend, start antwort, wait for readiness, run the suite, and report results
- **FR-025**: The pipeline MUST clean up all processes after completion (regardless of test outcome)
- **FR-026**: Test results MUST be output as structured JSON for CI consumption

### Key Entities

- **Server Binary**: The runnable antwort process (`cmd/server`) that wires all components together.
- **Mock Backend**: A deterministic Chat Completions server (`cmd/mock-backend`) for reproducible testing.
- **Test Profile**: A named configuration selecting which compliance tests to run, corresponding to the implemented feature set.
- **Conformance Score**: The ratio of passed tests to total tests in the active profile.

## Success Criteria

### Measurable Outcomes

- **SC-001**: The antwort server starts from a single binary with environment-based configuration and serves valid OpenResponses API responses
- **SC-002**: The mock backend produces deterministic responses that satisfy the official compliance suite's schema and semantic validators
- **SC-003**: The "core" profile passes all 5 included tests (basic text, streaming, system prompt, tool calling, multi-turn) with 100% score
- **SC-004**: The "extended" profile passes all 6 official compliance tests with 100% score
- **SC-005**: The conformance pipeline runs end-to-end in under 60 seconds (mock backend, no real LLM)
- **SC-006**: The conformance score is available as structured JSON output from the test runner

## Assumptions

- The official OpenResponses compliance suite at [github.com/openresponses/openresponses](https://github.com/openresponses/openresponses) is the canonical test source. We run it as-is, without modifications.
- The compliance suite uses `bun` as its JavaScript runtime. We run it via a container to avoid requiring bun on the host.
- The mock backend needs to understand the 6 test prompts well enough to produce responses that pass both schema validation and semantic checks (e.g., "Say hello in exactly 3 words" should produce a 3-word response).
- The server wiring layer is intentionally minimal. Full configuration (Spec 09) is a later spec. For conformance testing, environment variables suffice.
- Profiles are a project-side concept, not part of the official test suite. We implement profile filtering by selectively running test subsets.

## Dependencies

- **Spec 001-003**: Core types, transport, engine, and provider. Required for the server to function.
- **Spec 004**: Agentic loop. Required for tool calling conformance test.
- **Spec 005**: Storage. Required for multi-turn conversation test and GET/DELETE endpoints.
- **Official OpenResponses repo**: [openresponses/openresponses](https://github.com/openresponses/openresponses) for the compliance test suite source.

## Scope Boundaries

### In Scope

- Server wiring layer (`cmd/server/main.go`)
- Mock Chat Completions backend (`cmd/mock-backend/main.go`)
- Official compliance suite integration (containerized TypeScript runner)
- Test profiles (core, extended, full)
- CI-ready pipeline script (`make conformance`)
- Structured test result output (JSON)
- Conformance score reporting

### Out of Scope

- Full configuration system (Spec 09)
- Authentication (Spec 06)
- Deployment manifests (Spec 07)
- Performance/load testing
- Custom test cases beyond the official suite
- Real LLM backend integration testing (separate from conformance)
