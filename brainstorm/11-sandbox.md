# Spec 11: Kubernetes Sandbox Execution

**Branch**: `spec/11-sandbox`
**Dependencies**: Spec 04 (Agentic Loop, for ToolExecutor interface), Spec 07 (Deployment)
**Package**: `pkg/tools/sandbox`

## Purpose

Implement a Kubernetes-native sandbox execution system for running arbitrary tool code in isolated pods. Antwort delegates tool execution to sandbox pods via a secured REST interface.

No custom or potentially blocking code ever executes within the antwort process. All tool execution, code interpretation, file processing, and external system interaction is delegated to sandbox pods.

## Foundation: kubernetes-sigs/agent-sandbox

Sandbox pod lifecycle is managed by the [agent-sandbox](https://github.com/kubernetes-sigs/agent-sandbox) project from Kubernetes SIGs. This provides:

- **`Sandbox` CRD**: Isolated, stateful, singleton container with stable hostname and persistent storage
- **`SandboxTemplate`**: Reusable templates for creating similar sandboxes (general-purpose Python, specialized tools)
- **`SandboxWarmPool`**: Pre-warmed sandbox pools for fast allocation, eliminating cold-start latency
- **`SandboxClaim`**: Abstraction layer for claiming sandboxes from templates on demand

Antwort is a **CRD consumer**, not a controller. The agent-sandbox controller handles pod lifecycle, warm pool management, and stable identity. Antwort creates `SandboxClaim` resources to acquire sandboxes from warm pools and communicates with them via their stable FQDN.

### Responsibility Split

| Concern | Owner |
|---------|-------|
| Pod lifecycle (create, pause, resume, delete) | agent-sandbox controller |
| Warm pool sizing and pre-warming | agent-sandbox `SandboxWarmPool` |
| Stable hostname and network identity | agent-sandbox `Sandbox` |
| Persistent storage for cached environments | agent-sandbox `Sandbox` + PVC |
| Container image (Python, uv, REST server) | antwort project |
| REST API contract for code execution | antwort project |
| Acquiring and releasing sandboxes | antwort via `SandboxClaim` |
| Routing requests to sandboxes by environment | antwort `pkg/tools/sandbox` |
| SPIFFE/SPIRE identity infrastructure | cluster operator |

## Core Principles

1. **Antwort never executes tool code.** All execution is delegated to sandbox pods.
2. **Antwort consumes CRDs, not manages pods.** Pod lifecycle is handled by the agent-sandbox controller. Antwort creates `SandboxClaim` and `SandboxTemplate` resources.
3. **Kubernetes-only.** Antwort is designed exclusively for Kubernetes. There is no local/standalone execution mode.
4. **Workload identity via SPIFFE/SPIRE.** All communication between antwort and sandbox pods uses mutual TLS with SPIFFE identities. No shared secrets.
5. **Pod reuse with environment caching.** Sandbox pods advertise their installed environments so antwort can route requests to pods that already have the required dependencies.

## Sandbox Pod Types

### General-Purpose Python Sandbox

The default sandbox type. A Python-enabled pod with `uv` for fast package management.

**Base image contents:**
- Python 3.12+
- `uv` package manager
- SPIFFE/SPIRE workload agent (sidecar or init container)
- Sandbox REST API server (lightweight HTTP server accepting execution requests)
- Minimal base OS (distroless or similar)

**Capabilities:**
- Execute arbitrary Python code
- Install packages from configurable Python indices (public PyPI, private registries, air-gapped mirrors)
- Manage virtual environments (create, cache, reuse, cleanup)
- Return structured results via REST API

### Specialized/Bespoke Sandbox Pods

Pre-configured pods for specific tool implementations. These use custom container images with pre-installed dependencies and specialized interfaces.

Examples:
- **File search pod**: Pre-built with vector embedding model and search index (backend TBD: could be local FAISS, or client to external vector DB)
- **Web search pod**: Pre-built with web scraping/search API client
- **Code interpreter pod**: Enhanced Python sandbox with data science libraries pre-installed
- **Custom tool pods**: User-defined images registered via configuration

Specialized pods implement the same REST interface as general-purpose pods but may have additional endpoints or capabilities.

## Architecture with agent-sandbox

```
┌──────────────────────────────────────────────────┐
│                Antwort                            │
│                                                  │
│  ┌──────────────────┐  ┌──────────────────────┐  │
│  │ Sandbox Client   │  │ Pod Router           │  │
│  │                  │  │ (select sandbox by   │  │
│  │ - SandboxClaim   │  │  environment match)  │  │
│  │   creation       │  │                      │  │
│  │ - REST calls     │  └──────────────────────┘  │
│  └──────────────────┘                            │
└───────────┬──────────────────────────────────────┘
            │ mTLS (SPIFFE)
            │
┌───────────▼──────────────────────────────────────┐
│          agent-sandbox controller                 │
│                                                  │
│  ┌────────────────┐  ┌─────────────────────────┐ │
│  │ SandboxWarmPool│  │ SandboxTemplate         │ │
│  │ (general-      │  │ (specialized:           │ │
│  │  purpose)      │  │  file-search,           │ │
│  │                │  │  web-search, etc.)      │ │
│  └───────┬────────┘  └────────────┬────────────┘ │
│          │                        │              │
│    ┌─────▼─────┐  ┌─────┐  ┌─────▼─────┐        │
│    │ Sandbox   │  │ ... │  │ Sandbox   │        │
│    │ (Python)  │  │     │  │ (search)  │        │
│    └───────────┘  └─────┘  └───────────┘        │
└──────────────────────────────────────────────────┘
```

### Sandbox Acquisition Flow

1. Tool call arrives at antwort
2. Pod router checks if a claimed sandbox with the right environment exists
3. If yes: route request to that sandbox's stable FQDN
4. If no: create a `SandboxClaim` referencing the appropriate `SandboxTemplate`
5. agent-sandbox allocates a sandbox from the warm pool (or creates one)
6. Antwort waits for the `SandboxClaim` to be bound (status becomes ready)
7. Execute tool via the sandbox's REST API
8. Release or retain the sandbox based on caching policy

### Pod Router

When a tool call arrives, the pod router selects the best sandbox:

1. **Match by template**: General-purpose or specialized (based on tool definition)
2. **Match by environment**: For general-purpose sandboxes, prefer ones that already have the required virtual environment cached
3. **Availability**: Select an idle claimed sandbox from the matched set
4. **Fallback**: Claim a new sandbox from the warm pool (it will install dependencies on demand)

## Pod Environment Caching

General-purpose sandbox pods advertise their cached environments via metadata:

```go
// PodEnvironment describes a cached virtual environment on a sandbox pod.
type PodEnvironment struct {
    Name         string   // e.g., "data-analysis", "web-scraping"
    Packages     []string // e.g., ["pandas==2.2.0", "numpy>=1.26"]
    PythonVersion string  // e.g., "3.12"
    CreatedAt    time.Time
    LastUsedAt   time.Time
}

// PodStatus is reported by the sandbox pod via its REST API.
type PodStatus struct {
    State        string            // "idle", "busy", "warming"
    Environments []PodEnvironment  // cached venvs available
    Capacity     int               // max concurrent executions
    CurrentLoad  int               // current executions in progress
}
```

Antwort periodically polls pod status (or receives updates via watch) to maintain a routing table of available environments.

**Environment lifecycle:**
1. First request needing `pandas`: pod creates venv, installs via `uv pip install`, caches it
2. Subsequent requests needing `pandas`: routed to the same pod, venv already warm
3. Cleanup: environments evicted by LRU policy after configurable TTL or when pod disk usage exceeds threshold
4. Pod recycle: when a pod is recycled, all cached environments are lost (new pod starts clean)

## Sandbox REST Interface

Every sandbox pod (general-purpose and specialized) exposes the same base REST interface:

```
POST /execute     Execute code or tool invocation
GET  /health      Health check and status reporting
GET  /envs        List cached virtual environments
POST /envs        Create/warm a virtual environment
DELETE /envs/{name}  Remove a cached virtual environment
```

### POST /execute

```json
Request:
{
  "tool_name": "python_execute",
  "code": "import pandas as pd\ndf = pd.read_csv('data.csv')\nprint(df.describe())",
  "requirements": ["pandas>=2.0", "numpy"],
  "env_name": "data-analysis",
  "timeout_seconds": 30,
  "files": {
    "data.csv": "<base64-encoded content>"
  },
  "python_index": "https://pypi.org/simple/"
}

Response:
{
  "status": "success",
  "output": "       col1    col2\ncount  100.0  100.0\n...",
  "stderr": "",
  "exit_code": 0,
  "execution_time_ms": 1234,
  "files_produced": {
    "result.json": "<base64-encoded content>"
  }
}
```

### GET /health

```json
Response:
{
  "status": "healthy",
  "state": "idle",
  "environments": [
    {
      "name": "data-analysis",
      "packages": ["pandas==2.2.0", "numpy==1.26.4"],
      "python_version": "3.12",
      "last_used_at": "2026-02-17T15:00:00Z"
    }
  ],
  "capacity": 3,
  "current_load": 0,
  "uptime_seconds": 3600
}
```

## Security: SPIFFE/SPIRE Workload Identity

All communication between antwort and sandbox pods uses mutual TLS with SPIFFE identities.

### Identity Model

- **Antwort SPIFFE ID**: `spiffe://cluster.local/ns/<namespace>/sa/antwort-controller`
- **Sandbox pod SPIFFE ID**: `spiffe://cluster.local/ns/<namespace>/sa/antwort-sandbox-<type>`

### Trust Domain

- SPIRE server runs as a Deployment in the cluster
- SPIRE agents run as a DaemonSet on each node
- Both antwort and sandbox pods obtain SVIDs (SPIFFE Verifiable Identity Documents) from the local SPIRE agent
- mTLS is established using these SVIDs; no shared secrets, no pre-distributed certificates

### Authorization

- Antwort only accepts connections from sandbox pods with matching SPIFFE IDs
- Sandbox pods only accept connections from the antwort controller SPIFFE ID
- Network policies restrict traffic to only antwort <-> sandbox communication

### Sandbox Isolation

- Sandbox pods run with restricted security contexts (non-root, read-only root filesystem except for /tmp and venv paths)
- Resource limits enforced (CPU, memory, ephemeral storage)
- Network policies restrict outbound traffic (only configurable Python indices by default)
- No access to Kubernetes API from sandbox pods
- No access to other namespaces or cluster services unless explicitly configured

## Configuration

Antwort's sandbox configuration focuses on what antwort controls (REST client settings, environment routing, Python indices). Pool sizing and pod resources are configured via agent-sandbox CRDs (`SandboxWarmPool`, `SandboxTemplate`).

```go
type SandboxConfig struct {
    // Kubernetes namespace where sandbox resources are created.
    Namespace string

    // Templates maps tool types to SandboxTemplate names.
    // The "default" key maps to the general-purpose Python sandbox.
    Templates map[string]string // tool type -> SandboxTemplate name

    // Python package index configuration (passed to sandbox pods).
    PythonIndex PythonIndexConfig

    // SPIFFE trust domain for mTLS.
    TrustDomain string

    // Environment caching configuration.
    EnvCache EnvCacheConfig

    // ClaimTimeout is how long to wait for a SandboxClaim to be bound.
    ClaimTimeout time.Duration
}

type PythonIndexConfig struct {
    DefaultIndex    string   // Default PyPI index URL
    ExtraIndices    []string // Additional index URLs
    TrustedHosts    []string // Hosts to skip TLS verification for
    AllowedPackages []string // Allowlist (empty = all allowed)
}

type EnvCacheConfig struct {
    MaxEnvsPerPod   int           // Max cached environments per pod
    EnvTTL          time.Duration // Time before unused envs are evicted
    MaxDiskUsage    string        // e.g., "5Gi"
}
```

### agent-sandbox CRD Configuration (managed by cluster operator)

```yaml
# SandboxTemplate for general-purpose Python sandbox
apiVersion: sandbox.k8s.io/v1alpha1
kind: SandboxTemplate
metadata:
  name: antwort-python
spec:
  podTemplate:
    spec:
      containers:
      - name: sandbox
        image: ghcr.io/rhuss/antwort-sandbox:latest
        ports:
        - containerPort: 8080
        resources:
          limits:
            cpu: "2"
            memory: 4Gi
            ephemeral-storage: 10Gi
  volumeClaimTemplates:
  - metadata:
      name: venvs
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 5Gi
---
# SandboxWarmPool for pre-warming
apiVersion: sandbox.k8s.io/v1alpha1
kind: SandboxWarmPool
metadata:
  name: antwort-python-pool
spec:
  templateRef:
    name: antwort-python
  replicas: 3  # Number of pre-warmed sandboxes
```

## Scaling Strategy

Scaling is managed by the agent-sandbox `SandboxWarmPool`, not by antwort:

- **Warm pool sizing**: Configure `replicas` in `SandboxWarmPool` to control how many pre-warmed sandboxes are available
- **On-demand creation**: If the warm pool is exhausted, `SandboxClaim` triggers creation of a new sandbox (with cold-start latency)
- **Release policy**: Antwort releases `SandboxClaim` resources when a sandbox is no longer needed, returning it to the pool
- **Specialized pools**: Separate `SandboxWarmPool` resources per tool type (file search, web search, etc.)

## Open Questions

- How should large file transfers between antwort and sandbox pods be handled (inline base64 in REST, or via shared PVC/object storage)?
- Should specialized pods support the same environment caching as general-purpose pods, or are they always pre-configured?
- What observability signals should sandbox pods expose (OpenTelemetry traces, Prometheus metrics)?
- How should sandbox pod failures during execution be reported to the agentic loop (retry, fail the tool call, fail the response)?
- For file search: which vector DB backend(s) should be supported (FAISS local, Milvus, pgvector, Elasticsearch)?
- How does agent-sandbox's `SandboxWarmPool` interact with SPIFFE/SPIRE identity provisioning for pre-warmed pods?
- Should antwort use the agent-sandbox Python SDK or interact directly with CRDs via the Kubernetes API?
