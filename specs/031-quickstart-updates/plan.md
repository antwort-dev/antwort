# Implementation Plan: Quickstart Updates

**Branch**: `031-quickstart-updates` | **Date**: 2026-02-28 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/031-quickstart-updates/spec.md`

## Summary

Add two new quickstarts (05-code-interpreter, 06-responses-proxy) and refresh existing quickstarts (01-04) with structured output and reasoning examples. All quickstarts are Kustomize-based Kubernetes manifests with READMEs containing copy-pasteable test commands.

## Technical Context

**Language/Version**: YAML (Kubernetes manifests), Markdown (READMEs), Bash (test commands)
**Primary Dependencies**: Kustomize, kubectl/oc CLI, existing antwort container images
**Storage**: N/A (no new persistence)
**Testing**: Manual deployment + curl commands documented in READMEs
**Target Platform**: Kubernetes 1.26+, OpenShift 4.14+ (ROSA)
**Project Type**: Infrastructure/documentation
**Performance Goals**: N/A (quickstarts, not production workloads)
**Constraints**: Each quickstart must be self-contained and deployable in under 10 minutes
**Scale/Scope**: 2 new quickstart directories, 4 README updates

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Interface-First Design | N/A | No new Go interfaces |
| II. Zero External Dependencies | PASS | No new code dependencies |
| III. Nil-Safe Composition | N/A | No new Go code |
| IV. Typed Error Domain | N/A | No new error types |
| V. Validate Early, Fail Fast | N/A | No new validation logic |

No violations. This is a documentation/infrastructure feature.

## Project Structure

### Documentation (this feature)

```text
specs/031-quickstart-updates/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0 output
├── quickstart.md        # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
quickstarts/
├── 01-minimal/
│   └── README.md                    # UPDATE: add structured output, reasoning, next steps
├── 02-production/
│   └── README.md                    # UPDATE: add structured output, reasoning, next steps
├── 03-multi-user/
│   └── README.md                    # UPDATE: add structured output, reasoning, next steps
├── 04-mcp-tools/
│   └── README.md                    # UPDATE: add structured output, reasoning, next steps
├── 05-code-interpreter/             # NEW
│   ├── base/
│   │   ├── kustomization.yaml
│   │   ├── antwort-configmap.yaml
│   │   ├── antwort-deployment.yaml
│   │   ├── antwort-service.yaml
│   │   ├── antwort-serviceaccount.yaml
│   │   ├── sandbox-deployment.yaml
│   │   ├── sandbox-service.yaml
│   │   └── sandbox-configmap.yaml
│   ├── openshift/
│   │   ├── kustomization.yaml
│   │   └── antwort-route.yaml
│   └── README.md
└── 06-responses-proxy/              # NEW
    ├── base/
    │   ├── kustomization.yaml
    │   ├── backend-configmap.yaml
    │   ├── backend-deployment.yaml
    │   ├── backend-service.yaml
    │   ├── backend-serviceaccount.yaml
    │   ├── frontend-configmap.yaml
    │   ├── frontend-deployment.yaml
    │   ├── frontend-service.yaml
    │   └── frontend-serviceaccount.yaml
    ├── openshift/
    │   ├── kustomization.yaml
    │   └── frontend-route.yaml
    └── README.md
```

**Structure Decision**: Follows established quickstart directory pattern from 01-04. The 05-code-interpreter uses the 04-mcp-tools pattern (multiple deployments in base/). The 06-responses-proxy introduces backend/frontend naming to distinguish the two antwort instances.

## Phase 1: 05-code-interpreter Quickstart (US1, P1)

### Manifests

**sandbox-deployment.yaml**: Single-replica Deployment for the sandbox-server container.
- Image: `quay.io/rhuss/antwort-sandbox:latest` (built from `Containerfile.sandbox`)
- Port: 8080
- Env: `SANDBOX_MODE=python`, `SANDBOX_MAX_CONCURRENT=3`
- Health probes: HTTP GET `/health`
- Resource limits: 256Mi request, 512Mi limit
- Security context: non-root

**sandbox-service.yaml**: ClusterIP Service exposing port 8080.

**sandbox-configmap.yaml**: Optional, for sandbox configuration overrides.

**antwort-configmap.yaml**: Extends 01-minimal config with:
```yaml
providers:
  code_interpreter:
    enabled: true
    settings:
      sandbox_url: "http://sandbox-server:8080"
      execution_timeout: 60
```

**antwort-deployment.yaml**: Standard antwort deployment (same as 01-minimal).

**kustomization.yaml**: References all resources with `app.kubernetes.io/part-of: antwort-code-interpreter`.

**OpenShift overlay**: Adds Route for antwort (not for sandbox, which stays cluster-internal).

### README Content

1. Title + description (sandbox code execution)
2. Prerequisites (shared LLM backend)
3. Deploy section (kubectl apply -k, rollout status for both deployments)
4. Test section with three examples:
   - Basic computation (Fibonacci)
   - Package installation (numpy std deviation)
   - Data analysis prompt
5. Advanced: SandboxClaim section (documentation only)
6. What's Deployed table
7. Configuration reference
8. Next Steps (link to 06-responses-proxy)
9. Cleanup

## Phase 2: 06-responses-proxy Quickstart (US2, P2)

### Manifests

**Backend**: Standard antwort deployment with vllm provider, named `antwort-backend`.
- ConfigMap: `engine.provider: vllm`, `engine.backend_url: http://llm-predictor...`
- Service: `antwort-backend:8080`

**Frontend**: Second antwort deployment with responses provider, named `antwort-frontend`.
- ConfigMap: `engine.provider: vllm-responses`, `engine.backend_url: http://antwort-backend:8080`
- Service: `antwort-frontend:8080`

**kustomization.yaml**: References all backend + frontend resources with `app.kubernetes.io/part-of: antwort-responses-proxy`.

**OpenShift overlay**: Route for frontend only (backend stays cluster-internal).

### README Content

1. Title + description (proxy/gateway architecture)
2. Prerequisites (shared LLM backend)
3. Architecture diagram (ASCII: User -> Frontend -> Backend -> LLM)
4. Deploy section
5. Test section:
   - Non-streaming request through proxy
   - Streaming request through proxy
   - Verify backend is directly accessible (optional)
6. What's Deployed table
7. Configuration reference
8. Next Steps
9. Cleanup

## Phase 3: Refresh Existing READMEs (US3, P2)

For each quickstart (01-04), add three sections before "Cleanup":

### Structured Output Test Section

```bash
curl -s -X POST "$URL/v1/responses" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "/mnt/models",
    "input": [{"type": "message", "role": "user",
      "content": [{"type": "input_text", "text": "List 3 programming languages with their year of creation"}]}],
    "text": {
      "format": {
        "type": "json_schema",
        "name": "languages",
        "schema": {
          "type": "object",
          "properties": {
            "languages": {
              "type": "array",
              "items": {
                "type": "object",
                "properties": {
                  "name": {"type": "string"},
                  "year": {"type": "integer"}
                },
                "required": ["name", "year"]
              }
            }
          },
          "required": ["languages"]
        }
      }
    }
  }' | jq '.output[] | select(.type == "message") | .content[0].text' -r | jq .
```

### Reasoning Test Section

```bash
curl -s -X POST "$URL/v1/responses" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "/mnt/models",
    "input": [{"type": "message", "role": "user",
      "content": [{"type": "input_text", "text": "What is 15% of 240?"}]}],
    "reasoning": {"effort": "medium"}
  }' | jq '{output_types: [.output[].type], reasoning: [.output[] | select(.type == "reasoning") | .summary[0].text], answer: [.output[] | select(.type == "message") | .content[0].text]}'
```

### Next Steps Section

Each quickstart links to the next in the progression:
- 01-minimal -> 02-production
- 02-production -> 03-multi-user
- 03-multi-user -> 04-mcp-tools
- 04-mcp-tools -> 05-code-interpreter

## Implementation Order

1. **Phase 1** (US1, P1): 05-code-interpreter manifests + README
2. **Phase 2** (US2, P2): 06-responses-proxy manifests + README
3. **Phase 3** (US3, P2): Refresh 01-04 READMEs (parallel with Phase 2)

Phases 2 and 3 are independent and can be done in parallel.

## Complexity Tracking

No constitution violations to justify.
