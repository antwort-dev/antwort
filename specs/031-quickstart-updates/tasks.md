# Tasks: Quickstart Updates

**Input**: Design documents from `/specs/031-quickstart-updates/`
**Prerequisites**: plan.md, spec.md, research.md, quickstart.md

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup

**Purpose**: Create directory structure for new quickstarts.

- [ ] T001 (antwort-cy9.1) [P] Create directory structure `quickstarts/05-code-interpreter/base/` and `quickstarts/05-code-interpreter/openshift/`
- [ ] T002 (antwort-cy9.2) [P] Create directory structure `quickstarts/06-responses-proxy/base/` and `quickstarts/06-responses-proxy/openshift/`

**Checkpoint**: Directory structure ready.

---

## Phase 2: User Story 1 - Code Interpreter Quickstart (Priority: P1)

**Goal**: Deploy antwort with a Python sandbox for LLM code execution.

**Independent Test**: Deploy `quickstarts/05-code-interpreter/base/`, send a Fibonacci prompt, verify the response contains a `code_interpreter_call` output item.

- [ ] T003 (antwort-jn6.1) [P] [US1] Create `quickstarts/05-code-interpreter/base/sandbox-deployment.yaml`: single-replica Deployment for `quay.io/rhuss/antwort-sandbox:latest`, port 8080, env `SANDBOX_MODE=python` and `SANDBOX_MAX_CONCURRENT=3`, HTTP health probe on `/health`, resource limits 256Mi/512Mi, non-root security context (FR-001, FR-013)
- [ ] T004 (antwort-jn6.2) [P] [US1] Create `quickstarts/05-code-interpreter/base/sandbox-service.yaml`: ClusterIP Service named `sandbox-server` exposing port 8080 (FR-001)
- [ ] T005 (antwort-jn6.3) [P] [US1] Create `quickstarts/05-code-interpreter/base/antwort-serviceaccount.yaml`: ServiceAccount for antwort (FR-009)
- [ ] T006 (antwort-jn6.4) [P] [US1] Create `quickstarts/05-code-interpreter/base/antwort-deployment.yaml`: single-replica Deployment for `quay.io/rhuss/antwort:latest`, port 8080, volume mount for config, health probes, non-root security context (FR-009)
- [ ] T007 (antwort-csp.2) [P] [US1] Create `quickstarts/05-code-interpreter/base/antwort-service.yaml`: ClusterIP Service named `antwort` exposing port 8080 (FR-009)
- [ ] T008 (antwort-dam.1) [US1] Create `quickstarts/05-code-interpreter/base/antwort-configmap.yaml`: ConfigMap with `config.yaml` containing server, engine (provider: vllm, backend_url pointing to LLM backend), storage (memory), auth (none), observability (metrics enabled), and `providers.code_interpreter` with `enabled: true`, `sandbox_url: http://sandbox-server:8080`, `execution_timeout: 60` (FR-002)
- [ ] T009 (antwort-dam.2) [US1] Create `quickstarts/05-code-interpreter/base/kustomization.yaml`: reference all 6 resources, set `commonLabels` with `app.kubernetes.io/part-of: antwort-code-interpreter` (FR-009)
- [ ] T010 (antwort-91e.1) [P] [US1] Create `quickstarts/05-code-interpreter/openshift/antwort-route.yaml`: Route with edge TLS termination for antwort Service (FR-008)
- [ ] T011 (antwort-6mj.1) [US1] Create `quickstarts/05-code-interpreter/openshift/kustomization.yaml`: extend `../base`, add route resource (FR-008)
- [ ] T012 (antwort-jn6.5) [US1] Create `quickstarts/05-code-interpreter/README.md`: title, prerequisites (shared LLM backend), deploy section (kubectl apply -k, rollout status for sandbox-server and antwort), OpenShift section with Route, test section with three examples (basic computation, package installation with numpy, data analysis), advanced SandboxClaim documentation section, what's deployed table, configuration reference, next steps (link to 06-responses-proxy), cleanup section (FR-003, FR-004, FR-009)
- [ ] T013 (antwort-jn6.6) [US1] Verify `kustomize build quickstarts/05-code-interpreter/base/` produces valid YAML

**Checkpoint**: 05-code-interpreter quickstart is self-contained and deployable.

---

## Phase 3: User Story 2 - Responses API Proxy Quickstart (Priority: P2)

**Goal**: Deploy two antwort instances in a proxy chain (frontend -> backend -> LLM).

**Independent Test**: Deploy `quickstarts/06-responses-proxy/base/`, send a prompt to the frontend, verify the response comes from the backend's LLM.

- [ ] T014 (antwort-3xn.1) [P] [US2] Create `quickstarts/06-responses-proxy/base/backend-serviceaccount.yaml`: ServiceAccount for backend antwort (FR-006)
- [ ] T015 (antwort-3xn.2) [P] [US2] Create `quickstarts/06-responses-proxy/base/backend-deployment.yaml`: single-replica Deployment named `antwort-backend` for `quay.io/rhuss/antwort:latest`, port 8080, volume mount for config, health probes (FR-005, FR-006)
- [ ] T016 (antwort-3xn.3) [P] [US2] Create `quickstarts/06-responses-proxy/base/backend-service.yaml`: ClusterIP Service named `antwort-backend` exposing port 8080 (FR-006)
- [ ] T017 (antwort-3xn.4) [P] [US2] Create `quickstarts/06-responses-proxy/base/backend-configmap.yaml`: ConfigMap with `config.yaml` for backend: `engine.provider: vllm`, `engine.backend_url` pointing to LLM backend, storage memory, auth none (FR-005)
- [ ] T018 (antwort-3xn.5) [P] [US2] Create `quickstarts/06-responses-proxy/base/frontend-serviceaccount.yaml`: ServiceAccount for frontend antwort (FR-006)
- [ ] T019 (antwort-3xn.6) [P] [US2] Create `quickstarts/06-responses-proxy/base/frontend-deployment.yaml`: single-replica Deployment named `antwort-frontend` for `quay.io/rhuss/antwort:latest`, port 8080, volume mount for config, health probes (FR-005, FR-006)
- [ ] T020 (antwort-3xn.7) [P] [US2] Create `quickstarts/06-responses-proxy/base/frontend-service.yaml`: ClusterIP Service named `antwort-frontend` exposing port 8080 (FR-006)
- [ ] T021 (antwort-3xn.8) [US2] Create `quickstarts/06-responses-proxy/base/frontend-configmap.yaml`: ConfigMap with `config.yaml` for frontend: `engine.provider: vllm-responses`, `engine.backend_url: http://antwort-backend:8080`, storage memory, auth none (FR-005)
- [ ] T022 (antwort-3xn.9) [US2] Create `quickstarts/06-responses-proxy/base/kustomization.yaml`: reference all 8 resources, set `commonLabels` with `app.kubernetes.io/part-of: antwort-responses-proxy` (FR-009)
- [ ] T023 (antwort-3xn.10) [P] [US2] Create `quickstarts/06-responses-proxy/openshift/frontend-route.yaml`: Route with edge TLS for frontend Service only (FR-008)
- [ ] T024 (antwort-3xn.11) [US2] Create `quickstarts/06-responses-proxy/openshift/kustomization.yaml`: extend `../base`, add route resource (FR-008)
- [ ] T025 (antwort-3xn.12) [US2] Create `quickstarts/06-responses-proxy/README.md`: title, architecture diagram (ASCII: User -> Frontend -> Backend -> LLM), prerequisites, deploy section, test section with non-streaming and streaming examples through proxy chain, what's deployed table, configuration reference, next steps, cleanup (FR-007, FR-009)
- [ ] T026 (antwort-3xn.13) [US2] Verify `kustomize build quickstarts/06-responses-proxy/base/` produces valid YAML

**Checkpoint**: 06-responses-proxy quickstart is self-contained and deployable.

---

## Phase 4: User Story 3 - Refresh Existing Quickstart READMEs (Priority: P2)

**Goal**: Add structured output, reasoning, and next steps sections to existing quickstarts.

**Independent Test**: Deploy any existing quickstart (01-04), run the new structured output curl command, verify JSON output matching the schema.

- [ ] T027 (antwort-b0g.1) [P] [US3] Update `quickstarts/01-minimal/README.md`: add "Structured Output" test section with json_schema curl example, add "Reasoning" test section with reasoning effort curl example, add "Next Steps" section linking to 02-production (FR-010, FR-011, FR-012)
- [ ] T028 (antwort-b0g.2) [P] [US3] Update `quickstarts/02-production/README.md`: add "Structured Output" test section, add "Reasoning" test section, update "Next Steps" section linking to 03-multi-user (FR-010, FR-011, FR-012)
- [ ] T029 (antwort-b0g.3) [P] [US3] Update `quickstarts/03-multi-user/README.md`: add "Structured Output" test section (with auth header), add "Reasoning" test section (with auth header), update "Next Steps" section linking to 04-mcp-tools (FR-010, FR-011, FR-012)
- [ ] T030 (antwort-b0g.4) [P] [US3] Update `quickstarts/04-mcp-tools/README.md`: add "Structured Output" test section, add "Reasoning" test section, update "Next Steps" section linking to 05-code-interpreter (FR-010, FR-011, FR-012)

**Checkpoint**: All existing quickstarts show structured output and reasoning examples.

---

## Phase 5: Polish & Cross-Cutting Concerns

- [ ] T031 (antwort-xot.1) Verify all quickstart kustomize builds produce valid YAML: `kustomize build quickstarts/05-code-interpreter/base/` and `kustomize build quickstarts/06-responses-proxy/base/` and OpenShift overlays
- [ ] T032 (antwort-xot.2) Verify curl examples in quickstart.md match the README test sections

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies
- **US1 - Code Interpreter (Phase 2)**: Depends on Phase 1 (T001)
- **US2 - Responses Proxy (Phase 3)**: Depends on Phase 1 (T002), independent of Phase 2
- **US3 - README Refresh (Phase 4)**: No dependencies on Phases 2-3, can run in parallel
- **Polish (Phase 5)**: Depends on Phases 2-4

### User Story Dependencies

- **User Story 1 (P1)**: Independent. Requires only directory creation (T001)
- **User Story 2 (P2)**: Independent. Requires only directory creation (T002)
- **User Story 3 (P2)**: Fully independent. Updates existing files only

### Parallel Opportunities

- T001 and T002 in parallel (different directories)
- T003-T007, T010 in parallel (different files within 05-code-interpreter)
- T014-T020, T023 in parallel (different files within 06-responses-proxy)
- T027-T030 all in parallel (different quickstart READMEs)
- US1, US2, and US3 are fully independent and can run in parallel

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Phase 1: Create directories (T001)
2. Phase 2: Build 05-code-interpreter (T003-T013)
3. **STOP**: Deploy and validate code execution works end-to-end

### Incremental Delivery

4. Phase 3: Build 06-responses-proxy (T014-T026)
5. Phase 4: Refresh READMEs (T027-T030) (parallel with Phase 3)
6. Phase 5: Polish and verify all builds
