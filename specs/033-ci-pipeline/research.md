# Research: CI/CD Pipeline

## Decision 1: Workflow Structure

**Decision**: Single workflow file (`.github/workflows/ci.yml`) with 4 parallel jobs, replacing the existing `api-conformance.yml`.

**Rationale**: The existing workflow already has 2 parallel jobs. Extending to 4 parallel jobs in a single file keeps all CI logic co-located. GitHub Actions reports each job as a separate status check, so branch protection works per-job.

**Alternatives considered**:
- Multiple workflow files: More modular, but harder to reason about trigger conditions and creates multiple entries in the Actions tab.
- Single sequential workflow: Simpler but slower (total time = sum vs max of jobs).

## Decision 2: SDK Client Test Runner

**Decision**: Python tests use `pytest` with the `openai` package. TypeScript tests use `bun test` with the `openai` package.

**Rationale**: pytest is the de facto Python test runner. Bun is already used in the conformance job (via `oven-sh/setup-bun@v2`), so no new tooling is needed for TypeScript. The `openai` package is the official SDK we want to validate compatibility with.

**Alternatives considered**:
- unittest (Python): More verbose, less community adoption.
- vitest/jest (TypeScript): Adds Node.js dependency; bun's built-in test runner is simpler and already available.

## Decision 3: Kubernetes Testing Approach

**Decision**: Use `kind` (Kubernetes IN Docker) on GitHub Actions runners. Build images with `docker build`, load into kind with `kind load docker-image`.

**Rationale**: kind is well-supported on GitHub Actions ubuntu-latest runners (Docker is pre-installed). It creates a real K8s cluster in ~90 seconds. Loading images directly avoids registry costs. The quickstart 01-minimal manifests test the exact same deployment users follow.

**Alternatives considered**:
- k3s: Lighter but less standard for CI. kind has better GitHub Actions integration.
- minikube: Heavier, slower startup, overkill for smoke testing.
- Manifest validation only (kubeconform): Doesn't catch runtime issues (image build failures, port mismatches, health probe failures).

## Decision 4: Mock Backend in kind Cluster

**Decision**: Build a separate mock-backend container image, deploy it alongside antwort in the kind cluster as a Kubernetes Deployment + Service. The antwort server points to the mock-backend service via ANTWORT_BACKEND_URL.

**Rationale**: The quickstart 01-minimal manifests expect a backend at a configurable URL. Rather than including the full vLLM stack (which requires GPU and model weights), we deploy the mock-backend as a lightweight substitute. This tests the full Kubernetes networking path (Pod → Service → Pod) without any external dependencies.

**Alternatives considered**:
- Run mock-backend on localhost and use port-forward: Doesn't test K8s service discovery.
- Use the mock-backend as an init container: Wrong pattern, it's a long-running service.

## Decision 5: SDK Test Server Lifecycle

**Decision**: Build and start mock-backend + antwort server as background processes in the SDK test job, using the same pattern as the existing conformance job.

**Rationale**: The conformance job already has a proven pattern: build binaries, start services in background, wait for health endpoints, run tests. Reusing this pattern for SDK tests keeps the workflow consistent.

**Alternatives considered**:
- Docker Compose: Adds complexity, not needed for two processes.
- GitHub Actions services: Would require pre-built images, can't test current code.

## Decision 6: kind Version and K8s Version

**Decision**: Install kind via `go install sigs.k8s.io/kind@latest` and use its default K8s version (currently 1.32.x). Pin only if flakiness occurs.

**Rationale**: kind's default version tracks stable Kubernetes releases. Pinning creates maintenance burden without clear benefit for smoke testing.

## Decision 7: Mock-Backend Containerfile

**Decision**: Create a new `Containerfile.mock` for the mock-backend, following the same multi-stage pattern as the main `Containerfile`.

**Rationale**: The kind cluster needs a container image for the mock-backend. The existing `Containerfile` builds only the main server binary. A separate file keeps concerns separated.

**Alternatives considered**:
- Multi-target Containerfile with build args: More complex, harder to read.
- Embedding mock-backend in the main image: Wrong separation of concerns.
