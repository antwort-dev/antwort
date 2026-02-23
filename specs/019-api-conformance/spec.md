# Feature Specification: API Conformance & Integration Testing

**Feature Branch**: `019-api-conformance`
**Created**: 2026-02-23
**Status**: Draft

## Overview

This specification establishes a comprehensive API conformance testing pipeline for antwort. It introduces antwort's own OpenAPI specification (covering both OpenResponses endpoints and side-APIs), validates it against the upstream OpenResponses spec using oasdiff, runs the official Zod compliance suite for runtime validation, and adds Go-based integration tests covering every endpoint with a mock LLM backend.

All tests run in containers for GitHub Actions compatibility. A single `make api-test` command executes the full pipeline.

## Clarifications

### Session 2026-02-23

- Q: Should we write our own OpenAPI spec? -> A: Yes. We need one for side-APIs (vector stores, health, metrics) anyway. Starting now gives us a single source of truth. The OpenResponses portion is validated against upstream via oasdiff.
- Q: Container-based or native? -> A: Container-based. Easier for GitHub Actions (no host dependencies). Test container builds antwort + mock backend, runs all validations.
- Q: oasdiff vs Zod? -> A: Both. oasdiff validates schema structure (static). Zod validates runtime behavior (dynamic). Complementary.

## User Scenarios & Testing

### User Story 1 - OpenAPI Spec for Antwort (Priority: P1)

A developer consults antwort's OpenAPI specification to understand the full API surface, including OpenResponses endpoints and side-APIs (vector stores, health, metrics). The spec is the single source of truth for all HTTP endpoints.

**Acceptance Scenarios**:

1. **Given** the OpenAPI spec, **When** a developer reads it, **Then** all antwort endpoints are documented (responses, vector stores, health, metrics)
2. **Given** a new endpoint is added, **When** the spec is not updated, **Then** the conformance pipeline detects the gap

---

### User Story 2 - Schema Alignment with OpenResponses (Priority: P1)

An operator runs oasdiff to verify that antwort's OpenResponses endpoints align with the upstream specification. Breaking changes or missing fields are reported.

**Acceptance Scenarios**:

1. **Given** antwort's spec and the upstream OpenResponses spec, **When** oasdiff runs, **Then** it reports any breaking changes or missing endpoints
2. **Given** a compliant spec, **When** oasdiff runs, **Then** it reports no breaking changes

---

### User Story 3 - Integration Tests with Mock LLM (Priority: P1)

A developer runs the integration test suite against a running antwort instance with a mock LLM backend. Every endpoint is tested with valid and invalid inputs. No real LLM is needed.

**Acceptance Scenarios**:

1. **Given** the test suite running against antwort + mock backend, **When** all tests execute, **Then** every endpoint returns the expected status codes and response shapes
2. **Given** a streaming request, **When** the test validates SSE events, **Then** the event sequence matches the spec
3. **Given** an invalid request, **When** sent to antwort, **Then** the error response matches the documented error format

---

### User Story 4 - CI Pipeline (Priority: P1)

A maintainer merges a PR. The GitHub Actions workflow runs the full conformance pipeline (oasdiff + Zod + integration tests) and reports the result. Failures block the merge.

**Acceptance Scenarios**:

1. **Given** a PR, **When** the pipeline runs, **Then** oasdiff, Zod, and integration tests all execute
2. **Given** a passing pipeline, **When** the PR is reviewed, **Then** the conformance status is visible in the PR checks
3. **Given** a failing test, **When** the pipeline reports, **Then** the failure identifies the specific endpoint and expected vs actual behavior

---

### Edge Cases

- What happens when the upstream OpenResponses spec changes? oasdiff detects new breaking changes. The team decides whether to adopt the change or document the intentional divergence.
- What happens when a side-API endpoint is not in the OpenResponses spec? oasdiff only compares the OpenResponses portion. Side-APIs are validated by integration tests only.
- What happens when the mock backend doesn't support a feature? The test documents the limitation and skips the scenario.

## Requirements

### Functional Requirements

**OpenAPI Specification**

- **FR-001**: The project MUST maintain an OpenAPI specification file documenting all HTTP endpoints (OpenResponses + side-APIs)
- **FR-002**: The spec MUST cover: POST/GET/DELETE /v1/responses, /v1/vector_stores CRUD, /healthz, /metrics, streaming event schema
- **FR-003**: The spec MUST be kept in sync with code changes (validated by the conformance pipeline)

**oasdiff Validation**

- **FR-004**: The pipeline MUST download the upstream OpenResponses spec and compare the OpenResponses portion of antwort's spec using oasdiff
- **FR-005**: Breaking changes MUST be reported with field-level detail
- **FR-006**: The pipeline MUST fail if breaking changes are detected (unless explicitly marked as intentional divergences)

**Integration Tests**

- **FR-007**: The project MUST provide Go-based integration tests that run against a live antwort instance with a mock LLM backend
- **FR-008**: Integration tests MUST cover every documented endpoint with valid requests and verify response status codes and shapes
- **FR-009**: Integration tests MUST cover error cases (invalid input, missing auth, not found)
- **FR-010**: Integration tests MUST validate streaming SSE event sequences

**Zod Compliance Suite**

- **FR-011**: The pipeline MUST run the official OpenResponses Zod compliance suite (existing Spec 006 infrastructure)

**CI Pipeline**

- **FR-012**: All tests MUST run in containers (no host dependencies beyond a container runtime)
- **FR-013**: A GitHub Actions workflow MUST execute the full pipeline on every PR
- **FR-014**: A Makefile target `make api-test` MUST run the full pipeline locally
- **FR-015**: The pipeline MUST report a combined pass/fail result with per-test details

### Key Entities

- **OpenAPI Spec**: The canonical API surface definition for antwort.
- **Conformance Pipeline**: The combined oasdiff + Zod + integration test execution.
- **Integration Test Suite**: Go tests exercising every endpoint against a live instance.

## Success Criteria

- **SC-001**: The OpenAPI spec documents all antwort endpoints (responses, vector stores, health, metrics)
- **SC-002**: oasdiff reports zero breaking changes against the upstream OpenResponses spec
- **SC-003**: All integration tests pass with the mock LLM backend
- **SC-004**: The full pipeline runs in under 5 minutes in CI
- **SC-005**: A `make api-test` command runs the complete conformance pipeline locally

## Assumptions

- The upstream OpenResponses spec is available at a stable URL for downloading.
- The mock LLM backend (cmd/mock-backend) provides deterministic responses for all test scenarios.
- oasdiff is installed in the test container (available as a Go binary or pre-built container).
- The Zod compliance suite infrastructure from Spec 006 is reused.
- Side-APIs (vector stores, etc.) may not be in the upstream spec and are validated by integration tests only.

## Dependencies

- **Spec 006 (Conformance)**: Existing Zod compliance suite and mock backend.
- **All API-surface specs**: The OpenAPI spec documents endpoints from specs 002, 016, 017, 018.

## Scope Boundaries

### In Scope

- OpenAPI specification file (api/openapi.yaml)
- oasdiff validation against upstream OpenResponses spec
- Go integration tests for all endpoints
- Container-based test runner
- GitHub Actions workflow
- Makefile target (make api-test)
- Streaming SSE event validation

### Out of Scope

- Auto-generation of OpenAPI spec from Go code
- Performance/load testing
- Security scanning
- API documentation website (Swagger UI, Redoc)
