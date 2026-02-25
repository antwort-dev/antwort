# Brainstorm 23: Sandbox Flavors and Capability Routing

## Problem

The sandbox server currently only supports Python. Real-world agents need different execution environments: data science Python with heavy libraries pre-installed, Go with a compiler, Node.js, shell with CLI tools, or domain-specific setups. The model doesn't know which sandbox to use; Antwort needs to route to the right one.

## Decisions

### Routing: Explicit in Agent Profile

Each agent defines which sandbox template to use. No auto-detection or capability matching. Simple and predictable.

```yaml
apiVersion: antwort.dev/v1alpha1
kind: Agent
metadata:
  name: data-analyst
spec:
  tools:
    - type: sandbox
      name: code_interpreter
      sandboxTemplate: python-datascience
```

Future work (separate spec): add capability annotations to SandboxTemplates for auto-selection when the profile says `sandboxTemplate: auto`.

### Server Binary: One Binary, Mode Flag + Auto-Detect

One `sandbox-server` binary for all flavors. The `--mode` flag selects the runtime explicitly. If no mode is given, auto-detect based on what's installed in PATH.

```
sandbox-server --mode=python    # Use python3
sandbox-server --mode=golang    # Use go run
sandbox-server --mode=node      # Use node
sandbox-server --mode=shell     # Use bash -c
sandbox-server                  # Auto-detect: check python3, go, node, bash in PATH
```

The mode determines:
- Which interpreter to invoke (python3, go run, node, bash -c)
- How to handle package installation (uv pip install, go get, npm install, skip)
- The file extension for the code file (.py, .go, .js, .sh)

The REST API contract (`POST /execute`, `GET /health`) is identical across all modes.

## Container Image Strategy

The sandbox server binary is the same in every image. Only the base image and pre-installed tools differ:

### Base Images

```
antwort-sandbox:latest              Python 3.12 + uv (current)
antwort-sandbox-datascience:latest  Python 3.12 + uv + pandas/numpy/scipy/matplotlib/sklearn
antwort-sandbox-golang:latest       Go 1.25 + sandbox-server
antwort-sandbox-node:latest         Node 22 + sandbox-server
antwort-sandbox-shell:latest        Alpine + jq/yq/curl/git + sandbox-server
```

### Layered Containerfiles

Data science flavor layers on top of the base:

```dockerfile
FROM quay.io/rhuss/antwort-sandbox:latest
USER root
RUN uv pip install --system pandas numpy scipy matplotlib scikit-learn
USER sandbox
```

Go flavor uses a different base:

```dockerfile
FROM docker.io/library/golang:1.25-bookworm
COPY --from=builder /sandbox-server /usr/local/bin/sandbox-server
ENV HOME=/tmp
EXPOSE 8080
ENTRYPOINT ["sandbox-server", "--mode=golang"]
```

### Warm Pools Per Flavor

Each flavor gets its own SandboxWarmPool:

```yaml
apiVersion: sandbox.sigs.k8s.io/v1alpha1
kind: SandboxWarmPool
metadata:
  name: python-datascience-pool
spec:
  templateRef:
    name: python-datascience
  replicas: 2  # Keep 2 warm for data science tasks
---
apiVersion: sandbox.sigs.k8s.io/v1alpha1
kind: SandboxWarmPool
metadata:
  name: golang-pool
spec:
  templateRef:
    name: golang
  replicas: 1  # Keep 1 warm for Go tasks
```

## Implementation for the Sandbox Server

The mode flag changes the executor, not the HTTP server. The extension points:

### Code Executor Interface (internal to sandbox-server)

```
mode     | interpreter command | package installer      | file extension
---------|--------------------|-----------------------|---------------
python   | python3 script.py  | uv pip install --target| .py
golang   | go run script.go   | go get                 | .go
node     | node script.js     | npm install            | .js
shell    | bash script.sh     | (none)                 | .sh
```

### Auto-Detection Order

When `--mode` is not specified:
1. Check `python3 --version` -> python mode
2. Check `go version` -> golang mode
3. Check `node --version` -> node mode
4. Check `bash --version` -> shell mode
5. Fail with "no supported runtime found"

### Health Endpoint Reports Mode

```json
{
  "status": "healthy",
  "capacity": 3,
  "current_load": 0,
  "mode": "python",
  "runtime_version": "Python 3.12.12",
  "preinstalled_packages": ["pandas", "numpy", "scipy"]
}
```

This lets Antwort verify that the sandbox it's talking to matches the expected flavor.

## Phasing

1. **Current (Spec 024)**: Python-only sandbox server, `--mode` not yet implemented
2. **Next**: Add `--mode` flag with python as default, auto-detect fallback
3. **Then**: Add golang and shell modes
4. **Then**: Build data science and golang container images
5. **Then**: Agent profile `sandboxTemplate` field for routing

## What This Means for Agent Profiles

An agent that needs data analysis uses:

```yaml
tools:
  - type: sandbox
    name: code_interpreter
    sandboxTemplate: python-datascience
```

An agent that needs to compile and test Go code uses:

```yaml
tools:
  - type: sandbox
    name: code_interpreter
    sandboxTemplate: golang
```

An agent that needs shell access uses:

```yaml
tools:
  - type: sandbox
    name: code_interpreter
    sandboxTemplate: shell
```

The model writes code in whatever language the agent profile enables. The sandbox routes to the right flavor. The REST API is the same everywhere.
