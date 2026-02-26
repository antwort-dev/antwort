# Research: Code Interpreter Tool

## R1: agent-sandbox CRD API Types

**Decision**: Use controller-runtime typed client with agent-sandbox Go module types.

**Rationale**: The agent-sandbox project provides Go API types but no generated client-go clientset. controller-runtime's typed client gives type safety and is what the project itself uses internally. The alternative (dynamic client with `unstructured.Unstructured`) would add boilerplate without reducing the dependency footprint.

**Key findings**:
- Module path: `sigs.k8s.io/agent-sandbox` (latest: v0.1.1)
- SandboxClaim types: `sigs.k8s.io/agent-sandbox/extensions/api/v1alpha1` (group `extensions.agents.x-k8s.io`)
- Sandbox types: `sigs.k8s.io/agent-sandbox/api/v1alpha1` (group `agents.x-k8s.io`)
- Scheme registration required: `sandboxv1alpha1.AddToScheme()` and `extensionsv1alpha1.AddToScheme()`

**SandboxClaim flow**:
1. Create SandboxClaim with `spec.sandboxTemplateRef.name` = template name
2. Controller creates a Sandbox resource with the same name as the claim
3. Watch the Sandbox for `Ready` condition becoming `True`
4. Read `status.serviceFQDN` from the Sandbox for the pod network address
5. Delete the SandboxClaim when done (cascades to Sandbox via owner reference)

**Alternatives considered**:
- client-go dynamic client: lighter dependency but loses type safety, more boilerplate for marshal/unmarshal
- Raw HTTP calls to Kubernetes API: too low-level, would reimplement what controller-runtime provides

## R2: Testing Strategy

**Decision**: Real sandbox-server binary for integration tests, controller-runtime fake client for Kubernetes API boundary.

**Rationale**: The constitution (v1.2.0) mandates real components over fakes. The sandbox-server is a Go binary in this repo that executes Python. GitHub Actions Ubuntu runners have Python pre-installed, so the real binary works without containers.

**Key findings**:
- GitHub Actions free-tier: Ubuntu latest, 2 cores, 7GB RAM, Go + Python pre-installed
- sandbox-server can run as a subprocess without gVisor (gVisor is a deployment isolation concern)
- controller-runtime provides `fake.NewClientBuilder().WithScheme().WithObjects().Build()` for typed fake K8s clients
- The fake client supports Create, Get, List, Delete, and Watch operations

**Alternatives considered**:
- httptest.Server mock: rejected for integration tests (constitution says use real components)
- Real Kubernetes cluster in CI: rejected (not available on free-tier GitHub Actions)
- testcontainers/podman: rejected (unnecessary, sandbox-server runs natively)

## R3: SandboxAcquirer Interface Fit

**Decision**: The existing `SandboxAcquirer` interface is sufficient for the SandboxClaim adapter.

**Rationale**: The interface `Acquire(ctx) -> (sandboxURL, release, error)` maps cleanly to the SandboxClaim lifecycle: `Acquire` creates the claim + watches for Ready + returns serviceFQDN, and the `release` function deletes the claim.

**Key findings**:
- `staticURLAcquirer` already implements the interface for development mode
- The SandboxClaim adapter implements the same interface with real Kubernetes operations
- The provider code (`provider.go`) needs zero changes to support SandboxClaim mode
- Only `New()` in provider.go needs to instantiate the correct acquirer based on config

**Interface**:
```go
type SandboxAcquirer interface {
    Acquire(ctx context.Context) (sandboxURL string, release func(), err error)
}
```
