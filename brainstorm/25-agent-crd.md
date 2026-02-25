# Brainstorm 25: Agent CRD

## Problem

Config-file agents (brainstorm 24) validate the concept, but YAML config isn't the Kubernetes-native experience. Operators want to manage agents as CRDs: type-safe, kubectl-native, watchable, with status subresources.

This is Phase 2, Step 2: the `Agent` CRD that replaces (or supplements) config-file agent definitions.

## What Gets Built

### 1. Agent CRD Definition

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: agents.antwort.dev
spec:
  group: antwort.dev
  versions:
    - name: v1alpha1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
                version:
                  type: string
                model:
                  type: string
                instructions:
                  type: string
                tools:
                  type: array
                  items:
                    type: object
                constraints:
                  type: object
                reasoning:
                  type: object
                agents:
                  type: array
                  items:
                    type: object
            status:
              type: object
              properties:
                activeVersion:
                  type: string
                ready:
                  type: boolean
                toolCount:
                  type: integer
                lastUpdated:
                  type: string
      subresources:
        status: {}
      additionalPrinterColumns:
        - name: Model
          type: string
          jsonPath: .spec.model
        - name: Version
          type: string
          jsonPath: .spec.version
        - name: Tools
          type: integer
          jsonPath: .status.toolCount
        - name: Ready
          type: boolean
          jsonPath: .status.ready
  scope: Namespaced
  names:
    plural: agents
    singular: agent
    kind: Agent
    shortNames:
      - ag
```

Usage:

```
$ kubectl get agents
NAME             MODEL           VERSION   TOOLS   READY
devops-helper    qwen-2.5-72b   1.0.0     3       true
code-reviewer    deepseek-r1    1.2.0     1       true
data-analyst     qwen-2.5-72b   2.0.0     2       true
```

### 2. Agent Resource Example

```yaml
apiVersion: antwort.dev/v1alpha1
kind: Agent
metadata:
  name: devops-helper
  namespace: antwort
spec:
  version: "1.0.0"
  model: qwen-2.5-72b
  instructions: |
    You are a DevOps assistant for Kubernetes clusters.
    When investigating issues, gather data first, then
    analyze it programmatically.
  tools:
    - type: mcp
      serverRef: kubernetes-tools
    - type: sandbox
      name: code_interpreter
      sandboxTemplate: python-datascience
    - type: builtin
      name: web_search
  constraints:
    maxToolCalls: 15
    maxOutputTokens: 4096
    temperature: 0.3
  reasoning:
    effort: medium
```

### 3. Controller

A lightweight Go controller that:
- Watches `Agent` resources in the antwort namespace
- Validates the spec (model exists in providers, MCP servers exist, sandbox templates exist)
- Updates status (ready, tool count, last updated)
- Feeds the agent registry in the gateway process

**Not** a separate controller binary. The controller runs inside the antwort gateway process using `client-go` informers. This keeps the deployment footprint at one pod.

### 4. CRD + Config Coexistence

Both CRD agents and config-file agents work simultaneously. Resolution order:
1. CRD agents (watched via informer)
2. Config-file agents (from config.yaml)
3. If both define the same name, CRD takes precedence

This allows migration: start with config-file agents, gradually move to CRDs, delete config entries.

## Dependencies

- Brainstorm 24 (agent config loading): the merge logic, tool resolution, and `agent` field on the request must exist first
- `client-go` dependency: this is the first external dependency in the core gateway (currently stdlib only). It goes into an adapter package per constitution Principle II.

## Architecture Concern: client-go in the Gateway

The constitution says core packages use stdlib only. `client-go` is a large dependency. Options:

**A: client-go in an adapter package** (recommended)
```
pkg/agents/
├── agent.go          # AgentConfig type, registry interface (stdlib only)
├── config.go         # Config-file loader (stdlib only)
└── kubernetes/
    └── controller.go # CRD watcher using client-go (adapter package)
```

The core agent registry is an interface. The Kubernetes adapter implements it using client-go. The gateway main.go wires the adapter. This follows the same pattern as `pkg/storage/postgres` (pgx dependency in adapter only).

**B: Separate sidecar controller**
A separate binary that watches CRDs and writes to a shared config file or API. Avoids client-go in the gateway. Adds operational complexity (two binaries).

**Recommendation: A.** client-go in an adapter package is consistent with how we handle other external deps. The gateway binary gets larger but remains a single deployment.

## Status Subresource

The controller updates status after reconciliation:

```yaml
status:
  activeVersion: "1.0.0"
  ready: true
  toolCount: 3
  resolvedTools:
    - name: kubectl (mcp: kubernetes-tools)
    - name: code_interpreter (sandbox: python-datascience)
    - name: web_search (builtin)
  lastUpdated: "2026-02-25T12:00:00Z"
  conditions:
    - type: ToolsResolved
      status: "True"
      lastTransitionTime: "2026-02-25T12:00:00Z"
    - type: ModelAvailable
      status: "True"
      lastTransitionTime: "2026-02-25T12:00:00Z"
```

## Multi-Agent References

Agent CRDs can reference other agents:

```yaml
spec:
  agents:
    - ref: code-reviewer
      description: "Analyzes code changes"
    - ref: devops-helper
      description: "Executes K8s operations"
```

The controller validates that referenced agents exist. The status tracks resolution:

```yaml
status:
  conditions:
    - type: AgentRefsResolved
      status: "True"
```

## instructionsFrom Escape Hatch

For very large prompts that exceed comfortable inline size:

```yaml
spec:
  instructionsFrom:
    configMapRef:
      name: devops-helper-prompt
      key: system-prompt
```

The controller resolves the ConfigMap reference and populates the agent's instructions. Changes to the ConfigMap trigger re-reconciliation.

## Phasing

1. CRD definition + validation webhook
2. In-process controller with client-go informers
3. Status subresource updates
4. CRD + config coexistence (priority ordering)
5. Multi-agent reference validation
6. instructionsFrom ConfigMap resolution

## RBAC Requirements

The gateway's ServiceAccount needs:

```yaml
- apiGroups: ["antwort.dev"]
  resources: ["agents", "agents/status"]
  verbs: ["get", "list", "watch", "update"]
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "watch"]  # For instructionsFrom
```

## Open Questions

- Should the CRD be cluster-scoped or namespace-scoped? Namespace-scoped (recommended). Agents belong to a namespace, matching the deployment model.
- Should the controller validate that the model exists in the provider? Yes, surface it as a condition. But don't block readiness (the provider might not expose a model list).
- Should there be a `kubectl antwort` plugin for better UX? Deferred. `kubectl get agents` and `kubectl apply` are sufficient initially.
