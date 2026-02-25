# Brainstorm 22: Sandbox Code Interpreter Provider

## Context

Brainstorm 11 defines the full sandbox architecture (agent-sandbox CRDs, REST API, SPIFFE, warm pools). This brainstorm focuses on the first concrete integration: a `code_interpreter` FunctionProvider that executes Python code in agent-sandbox pods.

This is a prerequisite for the Agent feature (brainstorm 21), where agent definitions can include `code_interpreter` as a tool type.

## What Gets Built

A new FunctionProvider (`pkg/tools/builtins/codeinterpreter/`) that:

1. Registers a `code_interpreter` tool the model can call
2. When called, creates a `SandboxClaim` to acquire a sandbox pod
3. Sends the code to the sandbox pod's REST API
4. Returns stdout/stderr/files as the tool result
5. Emits `code_interpreter_call.*` SSE events during execution
6. Releases the sandbox when done

## Simplified First Version

The full brainstorm 11 has environment caching, pod routing, SPIFFE mTLS, and specialized sandbox types. For the first version, simplify:

- **No environment caching**: Every execution gets a fresh sandbox from the warm pool
- **No pod routing**: Always claim a new sandbox, no reuse optimization
- **No SPIFFE initially**: Use Kubernetes ServiceAccount tokens and NetworkPolicy for security (SPIFFE can be added later)
- **No specialized pods**: Only the general-purpose Python sandbox

### Minimal Flow

```
1. Model calls code_interpreter(code="import pandas...")
2. Antwort creates SandboxClaim referencing SandboxTemplate "antwort-python"
3. Wait for SandboxClaim status to become Ready (pod assigned from warm pool)
4. POST sandbox-pod:8080/execute {"code": "...", "timeout_seconds": 30}
5. Receive result {stdout, stderr, exit_code}
6. Delete SandboxClaim (pod returns to warm pool)
7. Return tool result to agentic loop
```

### SSE Events During Execution

```
response.code_interpreter_call.in_progress  → SandboxClaim created
response.code_interpreter_call.interpreting → code executing
response.code_interpreter_call.completed    → result received
```

Or on failure:
```
response.code_interpreter_call.in_progress
response.code_interpreter_call.failed       → execution error
```

## Sandbox Pod Container Image

An antwort project deliverable. Minimal Python container:

```dockerfile
FROM python:3.12-slim
RUN pip install uv
COPY sandbox-server /usr/local/bin/sandbox-server
EXPOSE 8080
USER nobody
ENTRYPOINT ["sandbox-server"]
```

The `sandbox-server` binary is a simple HTTP server (could be Go or Python) that:
- Accepts `POST /execute` with code + requirements
- Creates a virtualenv (if requirements specified), installs packages via `uv`
- Executes the code in a subprocess with timeout
- Returns stdout, stderr, exit_code
- Exposes `GET /health` for readiness probes

## Configuration

```yaml
# In antwort config.yaml
providers:
  code_interpreter:
    enabled: true
    settings:
      sandbox_template: antwort-python
      sandbox_namespace: antwort-sandbox
      claim_timeout: 30s
      execution_timeout: 60s
```

## Dependencies

- agent-sandbox controller installed on the cluster
- SandboxTemplate and SandboxWarmPool resources created
- Antwort has permissions to create/delete SandboxClaim resources
- Sandbox pod container image built and available

## Phasing

1. **Sandbox server binary**: The HTTP server that runs inside sandbox pods
2. **Container image**: Dockerfile, build, push to registry
3. **SandboxClaim client**: Go code to create/watch/delete SandboxClaim CRDs
4. **CodeInterpreter FunctionProvider**: Register tool, dispatch to sandbox, return results
5. **SSE events**: Emit code_interpreter lifecycle events
6. **Kustomize manifests**: SandboxTemplate, SandboxWarmPool for deployment
7. **Integration tests**: Mock sandbox server, test the full flow

## Open Questions (from brainstorm 11, narrowed)

- Should the sandbox server be Go or Python? Go is consistent with the rest of the project, but Python makes virtualenv management simpler.
- How to handle large file outputs from code execution? Inline in the REST response for now, object storage later.
- What's the maximum execution timeout? 60 seconds default, configurable per agent profile.
