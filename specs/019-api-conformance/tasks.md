# Tasks: API Conformance & Integration Testing

## Phase 1: OpenAPI Spec (P1)

- [ ] T001 [US1] Create `api/openapi.yaml`: Full OpenAPI 3.1 spec covering POST/GET/DELETE /v1/responses (with request/response schemas), streaming SSE events, /v1/vector_stores CRUD, /healthz, /metrics. Include all schema types (Response, Item, Usage, StreamEvent, etc.) matching our actual Go types (FR-001, FR-002).
- [ ] T002 [US1] Validate the spec with an OpenAPI linter (e.g., `openapi-generator-cli validate` or `spectral lint`).

**Checkpoint**: OpenAPI spec documents all endpoints.

---

## Phase 2: oasdiff Validation (P1)

- [ ] T003 [US2] Create `api/validate-oasdiff.sh`: Downloads upstream OpenResponses spec from GitHub, runs oasdiff to compare the /v1/responses endpoints against our spec. Reports breaking changes. Fails on unexpected divergences (FR-004, FR-005, FR-006).
- [ ] T004 [US2] Document intentional divergences in `api/DIVERGENCES.md` (if any). oasdiff can be configured to ignore known differences.

**Checkpoint**: oasdiff passes against upstream.

---

## Phase 3: Go Integration Tests (P1)

- [ ] T005 [US3] Create `test/integration/helpers_test.go`: TestMain that starts mock backend + antwort server (in-process using httptest), provides helper functions for making requests and asserting responses.
- [ ] T006 [US3] Create `test/integration/responses_test.go`: POST /v1/responses (non-streaming), GET /v1/responses/{id}, DELETE /v1/responses/{id}, response field validation (FR-007, FR-008).
- [ ] T007 [US3] Create `test/integration/streaming_test.go`: POST /v1/responses with stream=true, validate SSE event sequence (response.created, output_item.added, content deltas, response.completed) (FR-010).
- [ ] T008 [US3] Create `test/integration/errors_test.go`: Invalid JSON (400), missing model (400), invalid response ID (404), auth required without token (401) (FR-009).
- [ ] T009 [US3] [P] Create `test/integration/health_test.go`: GET /healthz, GET /metrics (verify Prometheus format).
- [ ] T010 [US3] [P] Create `test/integration/vectorstores_test.go`: POST/GET/DELETE /v1/vector_stores (if file_search provider is enabled in test config).

**Checkpoint**: Integration tests cover all endpoints.

---

## Phase 4: CI Pipeline (P1)

- [ ] T011 [US4] Create `test/Containerfile`: Builds antwort + mock backend, installs oasdiff, includes Zod compliance suite runner. Single container runs the full pipeline.
- [ ] T012 [US4] Create `test/run.sh`: Pipeline script that runs oasdiff, Go integration tests, and Zod compliance suite. Reports combined results (FR-012, FR-015).
- [ ] T013 [US4] Create `.github/workflows/api-conformance.yml`: GitHub Actions workflow triggered on PR. Builds test container, runs pipeline, reports status (FR-013).
- [ ] T014 [US4] Add `make api-test` target to Makefile (FR-014).

**Checkpoint**: `make api-test` runs locally. GitHub Actions workflow ready.

---

## Dependencies

- Phase 1: No dependencies.
- Phase 2: Depends on Phase 1 (needs the OpenAPI spec).
- Phase 3: Depends on Phase 1 (tests validate against spec).
- Phase 4: Depends on Phase 2 + 3.
