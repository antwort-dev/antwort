# Tasks: CI/CD Pipeline

**Input**: Design documents from `/specs/033-ci-pipeline/`
**Prerequisites**: plan.md (required), spec.md (required), research.md

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each CI job.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3, US4)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create the unified workflow skeleton and shared test infrastructure

- [ ] T001 (antwort-dnm.1) Create unified CI workflow skeleton with 4 empty jobs in `.github/workflows/ci.yml`
- [ ] T002 (antwort-dnm.2) Delete the old workflow file `.github/workflows/api-conformance.yml`

---

## Phase 2: Foundational (Mock Backend Container)

**Purpose**: Build the mock-backend container image used by the K8s job

**CRITICAL**: The kubernetes job (US4) depends on this Containerfile

- [ ] T003 (antwort-nri.1) Create mock-backend container image at `Containerfile.mock` following the pattern in `Containerfile`

**Checkpoint**: Mock-backend can be built as a container image

---

## Phase 3: User Story 1 - Fast Feedback on Code Quality (Priority: P1)

**Goal**: Contributors get lint + unit test results within 3 minutes on every PR

**Independent Test**: Push a commit with a deliberate compile error and verify the lint-test job fails with a clear message

### Implementation for User Story 1

- [ ] T004 (antwort-5hr.1) [US1] Implement the `lint-test` job in `.github/workflows/ci.yml` with Go 1.25 setup, `go vet ./...`, and `go test ./pkg/... -timeout 60s -count=1`
- [ ] T005 (antwort-5hr.2) [US1] Add job timeout (5 min) and Go module caching to the lint-test job in `.github/workflows/ci.yml`

**Checkpoint**: Pushing to a PR branch triggers lint + unit tests and reports pass/fail as a GitHub check

---

## Phase 4: User Story 2 - OpenResponses Spec Compliance (Priority: P1)

**Goal**: Every PR validates API schema compliance and passes the official OpenResponses conformance suite

**Independent Test**: Modify an API response field and verify the conformance job detects the breakage

### Implementation for User Story 2

- [ ] T006 (antwort-o1h.1) [US2] Implement the `conformance` job in `.github/workflows/ci.yml` with oasdiff validation step calling `./api/validate-oasdiff.sh`
- [ ] T007 (antwort-o1h.2) [US2] Add Go integration tests step (`go test ./test/integration/ -timeout 120s -v -count=1`) to the conformance job in `.github/workflows/ci.yml`
- [ ] T008 (antwort-o1h.3) [US2] Add server startup steps to the conformance job in `.github/workflows/ci.yml`: build binaries, start mock-backend (port 9090), start antwort server (port 8080), wait for health endpoints
- [ ] T009 (antwort-o1h.4) [US2] Add OpenResponses compliance suite steps to the conformance job in `.github/workflows/ci.yml`: setup bun, clone repo, install deps, run tests with JSON output
- [ ] T010 (antwort-o1h.5) [US2] Add step summary generation and artifact upload to the conformance job in `.github/workflows/ci.yml`
- [ ] T011 (antwort-o1h.6) [US2] Add failure diagnostics step (server logs on failure) to the conformance job in `.github/workflows/ci.yml`

**Checkpoint**: Conformance job runs oasdiff + integration tests + compliance suite and produces a summary table

---

## Phase 5: User Story 3 - SDK Client Compatibility (Priority: P1)

**Goal**: Python and TypeScript OpenAI SDK test scripts validate that real client code works against antwort

**Independent Test**: Run the Python test script locally against a running antwort instance and verify all 6 test cases pass

### Implementation for User Story 3

- [ ] T012 (antwort-ioc.1) [P] [US3] Create Python SDK test file at `test/sdk/python/test_antwort.py` with 6 test cases: basic response, streaming, tool calling, conversation chaining, structured output, model listing
- [ ] T013 (antwort-ioc.2) [P] [US3] Create Python requirements file at `test/sdk/python/requirements.txt` with `openai` and `pytest` dependencies
- [ ] T014 (antwort-ioc.3) [P] [US3] Create TypeScript SDK test file at `test/sdk/typescript/test_antwort.test.ts` with 6 test cases matching the Python tests
- [ ] T015 (antwort-ioc.4) [P] [US3] Create TypeScript package file at `test/sdk/typescript/package.json` with `openai` dependency and bun test config
- [ ] T016 (antwort-ioc.5) [US3] Implement the `sdk-clients` job in `.github/workflows/ci.yml`: build + start servers, setup Python 3.12, install requirements, run pytest, setup bun, install TS deps, run bun test
- [ ] T017 (antwort-ioc.6) [US3] Add job timeout (10 min) and failure diagnostics to the sdk-clients job in `.github/workflows/ci.yml`

**Checkpoint**: SDK test job runs both Python and TypeScript tests against the live server and reports pass/fail

---

## Phase 6: User Story 4 - Kubernetes Deployment Validation (Priority: P2)

**Goal**: kind cluster validates that container images build, Pods start, health endpoints work, and requests succeed

**Independent Test**: Run the kind deployment locally with `kind create cluster`, load images, apply manifests, and send a curl request

### Implementation for User Story 4

- [ ] T018 (antwort-lbv.1) [P] [US4] Create CI kustomization overlay at `quickstarts/01-minimal/ci/kustomization.yaml` that patches image names, adds mock-backend deployment, sets imagePullPolicy to Never
- [ ] T019 (antwort-lbv.2) [P] [US4] Create mock-backend K8s manifests at `quickstarts/01-minimal/ci/mock-backend.yaml` with Deployment + Service for the mock-backend image
- [ ] T020 (antwort-lbv.3) [US4] Implement the `kubernetes` job in `.github/workflows/ci.yml`: docker build both images, install kind, create cluster, load images
- [ ] T021 (antwort-lbv.4) [US4] Add deployment steps to the kubernetes job in `.github/workflows/ci.yml`: create namespace, kustomize build + apply from `quickstarts/01-minimal/ci/`, wait for Pod readiness (60s timeout)
- [ ] T022 (antwort-lbv.5) [US4] Add smoke test steps to the kubernetes job in `.github/workflows/ci.yml`: port-forward, curl /healthz and /readyz, send test POST request to /v1/responses
- [ ] T023 (antwort-lbv.6) [US4] Add cleanup and failure diagnostics to the kubernetes job in `.github/workflows/ci.yml`: kind delete cluster, kubectl logs on failure

**Checkpoint**: K8s job builds images, deploys to kind, and verifies a request round-trip through Kubernetes services

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Final cleanup and validation across all jobs

- [ ] T024 (antwort-6li.1) Add Go module caching (`actions/cache`) to all Go-based jobs (conformance, sdk-clients, kubernetes) in `.github/workflows/ci.yml`
- [ ] T025 (antwort-6li.2) Verify all 4 jobs appear as separate GitHub status checks by pushing a test PR
- [ ] T026 (antwort-6li.3) Add a `Makefile` target `ci-sdk-test` that runs SDK tests locally (start servers + pytest + bun test) in `Makefile`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: No dependencies, can run in parallel with Phase 1
- **US1 lint-test (Phase 3)**: Depends on Phase 1 (workflow skeleton exists)
- **US2 conformance (Phase 4)**: Depends on Phase 1
- **US3 sdk-clients (Phase 5)**: Depends on Phase 1
- **US4 kubernetes (Phase 6)**: Depends on Phase 1 + Phase 2 (needs Containerfile.mock)
- **Polish (Phase 7)**: Depends on all user story phases

### User Story Dependencies

- **US1 (P1)**: Independent, start after Phase 1
- **US2 (P1)**: Independent, start after Phase 1
- **US3 (P1)**: Independent, start after Phase 1 (test scripts are standalone files)
- **US4 (P2)**: Depends on T003 (Containerfile.mock) from Phase 2

### Parallel Opportunities

- T001 and T003 can run in parallel (different files)
- T012, T013, T014, T015 can all run in parallel (different files in different directories)
- T018 and T019 can run in parallel (different K8s manifests)
- US1, US2, US3 can all be implemented in parallel (independent CI jobs)

---

## Parallel Example: User Story 3 (SDK Tests)

```bash
# Launch all test file creation in parallel:
Task: "Create Python SDK test file at test/sdk/python/test_antwort.py"
Task: "Create Python requirements at test/sdk/python/requirements.txt"
Task: "Create TypeScript SDK test file at test/sdk/typescript/test_antwort.test.ts"
Task: "Create TypeScript package at test/sdk/typescript/package.json"

# Then wire into workflow (sequential):
Task: "Implement sdk-clients job in .github/workflows/ci.yml"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Workflow skeleton
2. Complete Phase 3: lint-test job
3. **STOP and VALIDATE**: Push a PR, verify lint-test runs
4. This alone provides value: automated code quality checks

### Incremental Delivery

1. Setup + US1 (lint-test) → First green CI check
2. Add US2 (conformance) → Spec compliance automated
3. Add US3 (SDK tests) → Client compatibility validated
4. Add US4 (K8s) → Full deployment pipeline
5. Each job adds protection without breaking previous ones

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story
- Each job is independently testable by pushing to a PR branch
- The workflow file `.github/workflows/ci.yml` is modified across multiple tasks; commit after each phase
- SDK test scripts should be runnable locally: `cd test/sdk/python && pytest` (with servers running)

## Beads Task Management

This project uses beads (`bd`) for persistent task tracking across sessions:
- Run `/sdd:beads-task-sync` to create bd issues from this file
- `bd ready --json` returns unblocked tasks (dependencies resolved)
- `bd close <id>` marks a task complete (use `-r "reason"` for close reason, NOT `--comment`)
- `bd comments add <id> "text"` adds a detailed comment to an issue
- `bd sync` persists state to git
- `bd create "DISCOVERED: [short title]" --labels discovered` tracks new work
  - Keep titles crisp (under 80 chars); add details via `bd comments add <id> "details"`
- Run `/sdd:beads-task-sync --reverse` to update checkboxes from bd state
- **Always use `jq` to parse bd JSON output, NEVER inline Python one-liners**
