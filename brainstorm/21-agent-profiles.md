# Brainstorm 21: Agent Profiles and Antwort as an Agent Deployment Platform

## The Question

How can Antwort expand from "OpenResponses gateway" to "agent deployment platform" without losing focus? Where's the sweet spot between a focused inference proxy and a full agent framework?

## Competitive Landscape

### Kagent (CNCF Sandbox)

[Kagent](https://kagent.dev/) is a Kubernetes-native agent framework built on Google's ADK. Agents are CRDs (`Agent`, `ModelConfig`, `RemoteMCPServer`). The controller creates Deployments, Services, and ConfigMaps. Tools are MCP servers running as sidecars. Agents expose A2A protocol endpoints.

**What Kagent is**: A framework for deploying pre-built DevOps agents (Kubernetes, Istio, Helm, Prometheus agents ship by default). Each agent is a separate pod with its own runtime.

**What Kagent is NOT**: An inference gateway. It doesn't translate APIs, manage streaming, handle multi-turn state, or execute agentic loops. It delegates to LLM providers and uses ADK for orchestration.

### Kagenti (github.com/kagenti)

A separate project building "A2A agents as a control plane." Less mature, focused on agent-to-agent communication via CRDs.

### Agent-Sandbox (kubernetes-sigs)

The Kubernetes SIG project providing isolated execution environments. Not an agent framework, but infrastructure: Sandbox, SandboxTemplate, SandboxClaim CRDs. gVisor/Kata isolation. Warm pools for fast startup.

## Where Antwort Sits Today

Antwort is an **inference gateway** with agentic capabilities:

```
Client (OpenResponses SDK)
  |
  v
Antwort (Responses API, agentic loop, streaming, state, auth, MCP, tools)
  |
  v
LLM Backend (vLLM, LiteLLM)
```

It's NOT an agent framework. It doesn't deploy agents as separate pods. It runs the agentic loop in-process, executing tools on behalf of the model. The agent behavior is defined by the **request** (model, instructions, tools), not by a CRD.

## The Sweet Spot: Agent Profiles

The gap between "gateway" and "agent framework" can be bridged with **Agent Profiles**: server-side configurations that predefine agent behaviors without deploying separate pods.

**Kagent** = "pre-built agents as pods" (N agents = N pods)
**Antwort** = "your agents as configuration" (N agents = 1 gateway + N profiles)

| Aspect | Kagent | Antwort Agent Profiles |
|---|---|---|
| Deployment unit | One pod per agent | All agents share one gateway |
| Runtime | ADK/Python per agent | Single Go process, agentic loop |
| Tool connection | MCP sidecars per agent | Shared MCP connections |
| State management | Per-agent, in-process | Centralized (PostgreSQL) |
| API surface | A2A protocol | OpenResponses API |
| Scaling | Scale individual agent pods | Scale the gateway (stateless) |
| Configuration | CRDs per agent | One CRD = one agent |
| Resource cost | N pods for N agents | 1 pod serves N agents |

## Decision: Agent Profiles as CRDs

Agent Profiles MUST be Kubernetes CRDs for type-safe, declarative management. The prompt (instructions) belongs IN the CR, not referenced externally.

**Rationale**: One resource = one agent. `kubectl apply -f my-agent.yaml` deploys a complete agent. GitOps, code review for prompt changes, and `kubectl diff` all work naturally.

**Prompt size**: Kubernetes objects have a ~1.5MB etcd limit. System prompts rarely exceed a few KB. For the rare case of very large prompts, support `instructionsFrom.configMapRef` as an escape hatch.

### CRD Design

```yaml
apiVersion: antwort.dev/v1alpha1
kind: AgentProfile
metadata:
  name: devops-helper
  labels:
    antwort.dev/category: operations
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
      sandboxTemplate: python-analyzer
    - type: builtin
      name: web_search
  constraints:
    maxToolCalls: 15
    maxOutputTokens: 4096
    temperature: 0.3
  reasoning:
    effort: medium
```

## API Extension: The `agent` Field

The OpenResponses spec has no `agent` field. This is an Antwort extension at the top level of the request:

```json
{
  "agent": "devops-helper",
  "input": [{"type": "message", "role": "user", "content": [...]}]
}
```

Go's JSON unmarshaling silently ignores unknown fields, so non-Antwort servers would ignore it. The profile resolves to model, instructions, tools, and constraints, which merge with the request (request-level values override within profile constraints).

If the `agent` field is set, the `model` field becomes optional (the profile provides it). If neither is set, the gateway's default model is used as before.

## Auth Model: Intersection with Elevation

Two identities in play:

**The caller**: authenticated via API key or JWT. Determines tenant isolation, rate limits, audit trail.

**The agent profile**: defines capabilities (tools, models) and may carry service account credentials for tool access.

The model:
- The caller must be authenticated (identity for audit and tenancy)
- The profile defines a **ceiling** (max tokens, max tool calls, allowed tools)
- The caller can restrict further but cannot exceed the profile's limits
- Profile tool credentials are separate from caller credentials (the profile may grant MCP access the caller doesn't have directly)
- Profile access is controlled via an `access` field listing allowed users/groups

```yaml
spec:
  access:
    - group: platform-team
    - user: ci-bot
  tools:
    - type: mcp
      serverRef: kubernetes-tools
      auth:
        serviceAccount: devops-agent-sa
  constraints:
    maxToolCalls: 10
    maxOutputTokens: 4096
```

If the caller requests `maxOutputTokens: 8192` but the profile ceiling is 4096, the result is 4096.

## Multi-Agent Orchestration

Profiles reference other profiles as callable agents:

```yaml
spec:
  instructions: |
    You are a lead engineer. Delegate to specialists:
    - Use @code-reviewer for code analysis
    - Use @devops-helper for cluster operations
  agents:
    - ref: code-reviewer
      description: "Analyzes code changes and provides feedback"
    - ref: devops-helper
      description: "Executes Kubernetes operations"
```

The engine exposes referenced profiles as tools. When the model calls `@code-reviewer`, the engine runs a nested agentic loop with that profile's configuration. The outer agent sees the result as a tool output. No network hop, no separate process.

## Versioning and Rollout

```yaml
spec:
  version: "2.0.0"
  rollout:
    strategy: canary
    canaryPercent: 10
status:
  activeVersion: "2.0.0"
  previousVersion: "1.1.0"
  observedGeneration: 5
```

- New requests route to the new version at the canary percentage
- In-flight requests complete on the old version
- Rollback is `kubectl apply` with the previous version
- Status subresource tracks active version

## Sandbox Integration

Agent Profiles define WHAT the agent does. Agent-Sandbox defines WHERE code executes safely.

Flow when an agent profile includes `code_interpreter`:
1. Model calls `code_interpreter` with Python code
2. Antwort creates a `SandboxClaim` CR
3. Agent-sandbox controller assigns a gVisor pod from the warm pool (sub-second)
4. Code executes in isolation (no network, no host filesystem)
5. Results flow back through the agentic loop
6. Sandbox pod returns to the warm pool
7. SSE lifecycle events show progress to the client

See the walkthrough document for the complete 53-event SSE trace.

## What NOT to Do

1. **Don't build an agent runtime**: Antwort's agentic loop IS the runtime. Don't deploy separate agent pods.
2. **Don't build a tool framework**: MCP and FunctionProvider are the tool interfaces. Don't create a new one.
3. **Don't build A2A protocol**: Agent-to-agent communication is a different layer. Stay focused on client-to-agent via OpenResponses.
4. **Don't compete with Kagent on DevOps agents**: Kagent ships pre-built K8s/Istio/Helm agents. Antwort ships the platform that runs custom agents.

## Phasing

1. **Config-file Agent Profiles**: YAML config, `agent` field on request, profile merge logic. No CRDs yet. Validates the concept.
2. **CRD Agent Profiles**: `AgentProfile` CRD, controller watches CRDs, type-safe.
3. **Profile auth**: Access control, profile-level service accounts for tools.
4. **Sandbox integration**: `code_interpreter` tool backed by agent-sandbox `SandboxClaim`.
5. **Multi-agent**: Profile references as callable tools, nested agentic loops.
6. **Profile versioning**: Canary rollouts, version tracking in status subresource.
7. **Profile metrics**: Per-agent Prometheus labels, usage tracking.
