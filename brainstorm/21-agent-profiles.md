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

### What is an Agent Profile?

An Agent Profile is a named configuration that bundles:
- **Model**: Which model to use
- **Instructions**: System prompt defining the agent's behavior
- **Tools**: Which tools are available (MCP servers, built-in tools, function definitions)
- **Constraints**: Max tokens, max tool calls, temperature, reasoning effort
- **Auth**: Which API keys or service accounts the agent can use

A client sends a request with `agent: "my-devops-agent"` instead of manually specifying model, instructions, and tools. The profile fills in everything the request doesn't specify.

### Why This Is Different from Kagent

| Aspect | Kagent | Antwort Agent Profiles |
|---|---|---|
| Deployment unit | One pod per agent | All agents share one gateway |
| Runtime | ADK/Python per agent | Single Go process, agentic loop |
| Tool connection | MCP sidecars per agent | Shared MCP connections |
| State management | Per-agent, in-process | Centralized (PostgreSQL) |
| API surface | A2A protocol | OpenResponses API |
| Scaling | Scale individual agent pods | Scale the gateway (stateless) |
| Configuration | CRDs per agent | Config file or CRD (one gateway) |
| Resource cost | N pods for N agents | 1 pod serves N agents |

The key insight: **Kagent agents are deployment-heavy (one pod each) while Antwort agents are configuration-light (one profile per agent, shared runtime)**. For organizations with 10-100 agents, the Kagent model means 10-100 pods. The Antwort model means 1 gateway pod with 10-100 profiles.

### How Agent Profiles Would Work

```yaml
# config.yaml
agents:
  devops-helper:
    model: "qwen-2.5-72b"
    instructions: |
      You are a DevOps assistant. Help users with Kubernetes operations.
      Always explain what you're doing before executing commands.
    tools:
      - type: mcp
        server: kubernetes-tools
      - type: builtin
        name: web_search
    max_tool_calls: 5
    temperature: 0.3

  code-reviewer:
    model: "deepseek-r1"
    instructions: |
      You are a code reviewer. Analyze code changes and provide feedback.
      Use reasoning to think through complex logic.
    reasoning:
      effort: high
    tools:
      - type: mcp
        server: github-tools

  rag-assistant:
    model: "qwen-2.5-7b"
    instructions: |
      You are a documentation assistant. Search the knowledge base
      before answering questions.
    tools:
      - type: builtin
        name: file_search
        vector_store: docs-store
```

Client usage:
```
POST /v1/responses
{
  "agent": "devops-helper",
  "input": [{"type": "message", "role": "user", "content": [...]}]
}
```

The `agent` field selects the profile. The profile's model, instructions, tools, and constraints are merged with the request. Request-level values override profile defaults.

### What This Enables

1. **Declarative agent definitions** without deploying pods
2. **Multi-agent on one gateway** (resource-efficient)
3. **Hot-reload** agent configurations (ConfigMap watching, already in Spec 012)
4. **Profile-level auth** (different API keys per agent)
5. **Profile-level metrics** (track usage per agent via Prometheus labels)
6. **Agent marketplace** via shared profiles (YAML files or CRDs)

### CRD Option (Kubernetes-Native)

For Kubernetes-native environments, Agent Profiles could also be CRDs:

```yaml
apiVersion: antwort.dev/v1alpha1
kind: AgentProfile
metadata:
  name: devops-helper
spec:
  model: qwen-2.5-72b
  instructions: |
    You are a DevOps assistant...
  tools:
    - type: mcp
      serverRef: kubernetes-tools
  constraints:
    maxToolCalls: 5
    temperature: 0.3
```

The Antwort controller watches AgentProfile CRDs and updates its configuration. This is similar to how GIE watches InferenceModel CRDs, and would be a natural extension of the ConfigMap watching in Spec 012.

## The Sandbox Connection

Agent Profiles define **what** the agent does. Agent-Sandbox defines **where** code executes safely. These are complementary:

- An Agent Profile might include a `code_interpreter` tool
- When the model calls `code_interpreter`, Antwort creates a `SandboxClaim`
- Agent-Sandbox provisions a gVisor-isolated pod from a warm pool
- The code executes in the sandbox, results flow back through the agentic loop
- The SSE events (from Spec 023) show progress

This is the exact flow described in the constitution (Principle IX: Kubernetes-Native Execution). Agent Profiles make it configurable; Agent-Sandbox makes it safe.

## What NOT to Do

1. **Don't build an agent runtime**: Antwort's agentic loop IS the runtime. Don't deploy separate agent pods.
2. **Don't build a tool framework**: MCP and FunctionProvider are the tool interfaces. Don't create a new one.
3. **Don't build A2A protocol**: Agent-to-agent communication is a different layer. Stay focused on client-to-agent via OpenResponses.
4. **Don't compete with Kagent on DevOps agents**: Kagent ships pre-built K8s/Istio/Helm agents. Antwort ships the platform that runs custom agents.

## The Pitch

**Kagent** is "pre-built agents as pods."
**Antwort** is "your agents as configuration."

Kagent is great if you want to deploy Solo.io's Kubernetes agent. Antwort is great if you want to deploy YOUR agents, with YOUR tools, on YOUR infrastructure, using the standard OpenResponses API.

## Open Questions

1. Should the `agent` field be part of the OpenResponses spec (extension), or a custom header?
2. Should Agent Profiles be config-file only, CRD-only, or both?
3. Should Agent Profiles support multi-agent orchestration (profile A calls profile B)?
4. How does profile-level auth interact with request-level auth? (Profile sets a ceiling, request operates within it?)
5. Should profiles be versioned? (Rolling updates, canary deployments of agent behavior?)

## Phasing

1. **Config-file Agent Profiles** (simplest): YAML config, agent field on request, profile merge logic
2. **CRD Agent Profiles**: Kubernetes-native, controller watches CRDs
3. **Sandbox integration**: Code interpreter tool backed by agent-sandbox
4. **Profile metrics**: Per-agent Prometheus labels
5. **Profile versioning**: Canary agent behavior deployments
