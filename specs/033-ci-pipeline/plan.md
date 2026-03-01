# Implementation Plan: CI/CD Pipeline

**Branch**: `033-ci-pipeline` | **Date**: 2026-03-01 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/033-ci-pipeline/spec.md`

## Summary

Replace the existing `api-conformance.yml` GitHub Actions workflow with a unified CI pipeline (`ci.yml`) that runs four parallel jobs: lint+unit tests, OpenResponses conformance, SDK client compatibility (Python + TypeScript), and Kubernetes deployment validation via kind. All jobs use free-tier GitHub Actions runners with the project's mock backend for deterministic testing.

## Technical Context

**Language/Version**: Go 1.25 (server, mock-backend), Python 3.x (SDK tests), TypeScript/Bun (SDK tests + conformance)
**Primary Dependencies**: `openai` Python/TypeScript SDK, `kind` (K8s in Docker), `oasdiff`, `bun`, `pytest`
**Storage**: N/A (CI pipeline, no persistent storage)
**Testing**: Go test (unit + integration), pytest (Python SDK), bun test (TypeScript SDK), OpenResponses compliance suite
**Target Platform**: GitHub Actions ubuntu-latest runners (2 cores, 7GB RAM)
**Project Type**: CI/CD pipeline (GitHub Actions workflow + test scripts)
**Performance Goals**: All 4 parallel jobs complete within 10 minutes total
**Constraints**: Zero external cost, free-tier only, no GPU, no external services
**Scale/Scope**: 4 CI jobs, ~12 SDK test cases, 1 kind cluster smoke test

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Interface-First | N/A | No new Go interfaces in this spec |
| II. Zero External Dependencies (Core) | PASS | No changes to core packages |
| III. Nil-Safe Composition | N/A | No new optional dependencies |
| IV. Typed Error Domain | N/A | No new error types |
| V. Validate Early | N/A | No new request validation |
| VI. Protocol-Agnostic Provider | N/A | No provider changes |
| VII. Streaming First-Class | PASS | SDK tests validate streaming works correctly |
| VIII. Context Carries Data | N/A | No context changes |
| IX. Kubernetes-Native | PASS | kind job validates K8s deployment |
| Testing: CI-friendly | PASS | All tests designed for free-tier runners |
| Testing: Real components | PASS | Real SDKs, real kind cluster, mock only at LLM boundary |
| Testing: 10-min completion | PASS | SC-001 requires 10-minute total |

No violations. All gates pass.

## Project Structure

### Documentation (this feature)

```text
specs/033-ci-pipeline/
├── spec.md
├── plan.md              # This file
├── research.md          # Technology decisions
├── tasks.md             # Task breakdown (generated next)
└── checklists/
    └── requirements.md  # Quality checklist
```

### Source Code (repository root)

```text
.github/workflows/
└── ci.yml                          # Unified CI workflow (replaces api-conformance.yml)

test/sdk/
├── python/
│   ├── test_antwort.py             # Python SDK test cases (6 tests)
│   └── requirements.txt            # openai, pytest
└── typescript/
    ├── test_antwort.test.ts        # TypeScript SDK test cases (6 tests)
    └── package.json                # openai, @types/bun

Containerfile.mock                   # Mock-backend container image

quickstarts/01-minimal/
└── ci/
    └── kustomization.yaml          # CI overlay: mock-backend + antwort with local images
```

**Structure Decision**: Test scripts live under `test/sdk/` alongside existing `test/integration/`. The CI workflow file replaces the existing `api-conformance.yml`. A Containerfile for the mock-backend is added at the root. A CI-specific kustomization overlay adapts quickstart 01-minimal for kind.

## Implementation Design

### Job 1: lint-test (~2 min)

Reuses existing `make vet` and `make test` targets.

```yaml
steps:
  - checkout
  - setup-go 1.25
  - go vet ./...
  - go test ./pkg/... -timeout 60s -count=1
```

### Job 2: conformance (~4 min)

Refactors the existing two jobs into one. Reuses the proven service startup pattern.

```yaml
steps:
  - checkout
  - setup-go 1.25
  - install oasdiff
  - run oasdiff validation (./api/validate-oasdiff.sh)
  - run integration tests (go test ./test/integration/)
  - build server + mock-backend binaries
  - start mock-backend (port 9090)
  - start antwort server (port 8080)
  - wait for health endpoints
  - setup-bun
  - clone + install OpenResponses compliance suite
  - run compliance tests
  - generate step summary
  - upload results artifact
```

### Job 3: sdk-clients (~3 min)

New job. Starts the same server pair, then runs Python and TypeScript SDK tests.

```yaml
steps:
  - checkout
  - setup-go 1.25
  - build + start mock-backend + antwort server
  - wait for health endpoints
  # Python
  - setup-python 3.12
  - pip install -r test/sdk/python/requirements.txt
  - pytest test/sdk/python/ -v
  # TypeScript
  - setup-bun
  - cd test/sdk/typescript && bun install && bun test
```

**Python test cases** (`test/sdk/python/test_antwort.py`):
1. `test_basic_response` - `client.responses.create()` returns text
2. `test_streaming` - iterate stream events, reconstruct text
3. `test_tool_calling` - function call in response output
4. `test_conversation_chaining` - `previous_response_id` round-trip
5. `test_structured_output` - JSON schema constraint
6. `test_model_listing` - `client.models.list()` returns models

**TypeScript test cases** (`test/sdk/typescript/test_antwort.test.ts`):
Same 6 test patterns, using bun's built-in test runner.

### Job 4: kubernetes (~5 min)

New job. Builds container images, creates kind cluster, deploys, and smoke-tests.

```yaml
steps:
  - checkout
  - setup-go 1.25
  # Build images
  - docker build -t antwort:ci -f Containerfile .
  - docker build -t mock-backend:ci -f Containerfile.mock .
  # Create kind cluster
  - go install sigs.k8s.io/kind@latest
  - kind create cluster --name ci
  # Load images
  - kind load docker-image antwort:ci --name ci
  - kind load docker-image mock-backend:ci --name ci
  # Deploy
  - kubectl create namespace antwort
  - kustomize build quickstarts/01-minimal/ci | kubectl apply -n antwort -f -
  # Wait for pods
  - kubectl wait --for=condition=ready pod -l app=antwort -n antwort --timeout=60s
  - kubectl wait --for=condition=ready pod -l app=mock-backend -n antwort --timeout=60s
  # Health checks
  - kubectl port-forward -n antwort svc/antwort 8080:8080 &
  - curl -sf http://localhost:8080/healthz
  - curl -sf http://localhost:8080/readyz
  # Smoke test
  - curl -sf -X POST http://localhost:8080/v1/responses -H 'Content-Type: application/json' -d '{"model":"mock-model","input":"hello"}'
  # Cleanup
  - kind delete cluster --name ci
```

### CI Kustomization Overlay

`quickstarts/01-minimal/ci/kustomization.yaml` patches quickstart 01-minimal to:
- Use `antwort:ci` image (locally built, no registry pull)
- Add a mock-backend Deployment + Service
- Set `ANTWORT_BACKEND_URL` to `http://mock-backend:9090`
- Set `ANTWORT_STORAGE` to `memory`
- Set `imagePullPolicy: Never` (images loaded via kind)

### Containerfile.mock

Minimal container for mock-backend, following the same pattern as the main Containerfile:

```dockerfile
FROM golang:1.25 AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o mock-backend ./cmd/mock-backend/

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /build/mock-backend /mock-backend
EXPOSE 9090
ENTRYPOINT ["/mock-backend"]
```

## Key Files to Create/Modify

| File | Action | Purpose |
|------|--------|---------|
| `.github/workflows/ci.yml` | Create | Unified CI workflow with 4 jobs |
| `.github/workflows/api-conformance.yml` | Delete | Replaced by ci.yml |
| `test/sdk/python/test_antwort.py` | Create | Python SDK test cases |
| `test/sdk/python/requirements.txt` | Create | Python dependencies |
| `test/sdk/typescript/test_antwort.test.ts` | Create | TypeScript SDK test cases |
| `test/sdk/typescript/package.json` | Create | TypeScript dependencies |
| `Containerfile.mock` | Create | Mock-backend container image |
| `quickstarts/01-minimal/ci/kustomization.yaml` | Create | CI-specific K8s overlay |
| `quickstarts/01-minimal/ci/mock-backend.yaml` | Create | Mock-backend K8s manifests |

## Complexity Tracking

No constitution violations to justify.
