# Spec 11: Kubernetes Sandbox Execution

**Branch**: `spec/11-sandbox`
**Dependencies**: Spec 04 (Agentic Loop, for ToolExecutor interface), Spec 07 (Deployment)
**Package**: `pkg/tools/sandbox`, `pkg/sandbox` (controller)

## Purpose

Implement a Kubernetes-native sandbox execution system for running arbitrary tool code in isolated pods. Antwort acts as a controller that manages a pool of sandbox pods backed by Deployments, delegates tool execution to these pods via a secured REST interface, and scales the pool based on demand.

No custom or potentially blocking code ever executes within the antwort process. All tool execution, code interpretation, file processing, and external system interaction is delegated to sandbox pods.

## Core Principles

1. **Antwort never executes tool code.** All execution is delegated to sandbox pods.
2. **Antwort is a Kubernetes controller.** It creates, manages, and scales sandbox pod pools using native Kubernetes primitives (Deployments, ReplicaSets, Services).
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

## Pod Pool Architecture

```
┌──────────────────────────────────────────────┐
│                Antwort Controller             │
│                                              │
│  ┌──────────────┐  ┌──────────────────────┐  │
│  │ Pool Manager │  │ Pod Router           │  │
│  │              │  │ (select pod by       │  │
│  │ - Scale up   │  │  environment match)  │  │
│  │ - Scale down │  │                      │  │
│  │ - Health     │  └──────────────────────┘  │
│  └──────────────┘                            │
└───────────┬──────────────────────────────────┘
            │ mTLS (SPIFFE)
            │
    ┌───────▼────────┐
    │ Sandbox Pool   │
    │                │
    │ ┌────┐ ┌────┐  │  General-purpose
    │ │Pod1│ │Pod2│  │  Python sandboxes
    │ └────┘ └────┘  │
    │                │
    │ ┌────┐         │  Specialized
    │ │Pod3│         │  (file search)
    │ └────┘         │
    │                │
    │ ┌────┐         │  Specialized
    │ │Pod4│         │  (web search)
    │ └────┘         │
    └────────────────┘
```

### Pool Manager

The pool manager is a Kubernetes controller embedded in antwort that:

- Creates Deployments for each sandbox type (general-purpose, specialized)
- Scales replicas based on demand (queue depth, concurrent executions)
- Monitors pod health via the sandbox REST API health endpoint
- Tracks pod state: idle, busy, warming up
- Recycles pods after configurable lifetime or execution count

### Pod Router

When a tool call arrives, the pod router selects the best pod:

1. **Match by type**: General-purpose or specialized (based on tool definition)
2. **Match by environment**: For general-purpose pods, prefer pods that already have the required virtual environment cached
3. **Availability**: Select an idle pod from the matched set
4. **Fallback**: If no pre-warmed pod matches, select any idle general-purpose pod (it will install dependencies on demand)

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

```go
type SandboxConfig struct {
    // General-purpose pool configuration.
    GeneralPool PoolConfig

    // Specialized pools (keyed by tool type name).
    SpecializedPools map[string]SpecializedPoolConfig

    // Python package index configuration.
    PythonIndex PythonIndexConfig

    // SPIFFE trust domain.
    TrustDomain string

    // Environment caching configuration.
    EnvCache EnvCacheConfig
}

type PoolConfig struct {
    MinReplicas     int           // Minimum warm pods
    MaxReplicas     int           // Maximum pods under load
    Image           string        // Container image for sandbox pods
    Resources       ResourceSpec  // CPU/memory limits per pod
    MaxLifetime     time.Duration // Max pod lifetime before recycling
    MaxExecutions   int           // Max executions before recycling
    IdleTimeout     time.Duration // Idle time before scale-down
}

type SpecializedPoolConfig struct {
    PoolConfig
    ToolName    string // Tool this pool serves
    Endpoints   []string // Additional REST endpoints beyond base interface
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

## Scaling Strategy

- **Scale up**: When pending tool executions exceed available idle pod capacity, the controller increases Deployment replicas
- **Scale down**: When pods are idle beyond `IdleTimeout`, the controller scales down (respecting `MinReplicas`)
- **HPA integration**: The controller can optionally expose custom metrics for Kubernetes HPA to manage scaling externally
- **Queue depth**: The controller tracks pending executions per pool and uses this as the primary scaling signal

## Open Questions

- Should the sandbox REST server be a standard component shipped as a container image by the antwort project, or should users bring their own?
- How should large file transfers between antwort and sandbox pods be handled (inline base64 in REST, or via shared PVC/object storage)?
- Should specialized pods support the same environment caching as general-purpose pods, or are they always pre-configured?
- What observability signals should sandbox pods expose (OpenTelemetry traces, Prometheus metrics)?
- How should sandbox pod failures during execution be reported to the agentic loop (retry, fail the tool call, fail the response)?
- For file search: which vector DB backend(s) should be supported (FAISS local, Milvus, pgvector, Elasticsearch)?
