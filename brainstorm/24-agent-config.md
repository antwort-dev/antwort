# Brainstorm 24: Agent Config Loading

## Problem

Antwort currently requires clients to specify model, instructions, and tools on every request. For custom agents, this means the client carries the full agent definition. The first step toward the Agent platform is accepting an `agent` field on the request and resolving it against server-side agent definitions stored in YAML config.

This is Phase 2, Step 1: config-file agents. No Kubernetes CRDs yet. Just YAML configuration loaded from `config.yaml` (the same file Antwort already reads for providers, auth, MCP servers).

## What Gets Built

### 1. `agent` Field on CreateResponseRequest

A new optional string field:

```json
{
  "agent": "devops-helper",
  "input": [{"type": "message", "role": "user", "content": [...]}]
}
```

This is an Antwort extension to the OpenResponses API. Non-Antwort servers will ignore it (Go's JSON unmarshaling silently drops unknown fields).

### 2. Agent Definitions in config.yaml

```yaml
# In config.yaml
agents:
  devops-helper:
    model: qwen-2.5-72b
    instructions: |
      You are a DevOps assistant for Kubernetes clusters.
      When investigating issues, gather data first, then
      analyze it programmatically.
    tools:
      - type: mcp
        server: kubernetes-tools
      - type: builtin
        name: web_search
    constraints:
      max_tool_calls: 15
      max_output_tokens: 4096
      temperature: 0.3
    reasoning:
      effort: medium

  code-reviewer:
    model: deepseek-r1
    instructions: |
      You are a code reviewer. Analyze code changes and
      provide feedback. Use reasoning for complex logic.
    reasoning:
      effort: high
```

### 3. Profile Merge Logic

When a request includes `agent`, the engine resolves the agent definition and merges it with the request:

**Resolution order** (later wins):
1. Server defaults (from engine config)
2. Agent definition (from config.yaml)
3. Request fields (explicit values in the request)

**Constraint enforcement**:
- Agent defines a ceiling for `max_tool_calls`, `max_output_tokens`, `temperature`
- Request can set lower values but not exceed the agent's ceiling
- If the request exceeds the ceiling, the agent's value is used (capped, not rejected)

**Field merging**:

| Field | Behavior |
|---|---|
| `model` | Agent provides default, request overrides |
| `instructions` | Agent provides default, request overrides entirely (no concatenation) |
| `tools` | Agent provides the full tool list. Request `allowed_tools` can restrict but not add |
| `temperature`, `top_p`, etc. | Agent provides default, request overrides within ceiling |
| `max_tool_calls` | Agent defines ceiling, request can lower |
| `max_output_tokens` | Agent defines ceiling, request can lower |
| `metadata` | Request metadata merged with agent defaults |
| `reasoning` | Agent provides default, request overrides |
| `stream`, `store`, etc. | Request controls (no agent default) |

### 4. Config Loading Integration

The `config.Load()` function (Spec 012) already reads YAML and supports env overrides and ConfigMap watching. Agent definitions are a new section in the same config:

```go
type Config struct {
    // ... existing fields ...
    Agents map[string]AgentConfig `yaml:"agents"`
}

type AgentConfig struct {
    Model        string            `yaml:"model"`
    Instructions string            `yaml:"instructions"`
    Tools        []AgentToolConfig `yaml:"tools"`
    Constraints  AgentConstraints  `yaml:"constraints"`
    Reasoning    *ReasoningConfig  `yaml:"reasoning"`
}
```

ConfigMap watching (already implemented) means agent definitions hot-reload when the ConfigMap changes. No restart needed.

## What Happens at Request Time

```
1. Request arrives: {"agent": "devops-helper", "input": [...]}
2. Engine looks up "devops-helper" in the agent registry
3. If not found: return 400 error "unknown agent: devops-helper"
4. Merge agent config with request:
   - model = agent.model (request didn't specify)
   - instructions = agent.instructions (request didn't specify)
   - tools = resolve agent.tools (MCP servers, builtins)
   - constraints applied as ceilings
5. Proceed with normal request processing (agentic loop, etc.)
```

## What Does NOT Change

- The OpenResponses API surface (no new endpoints, just a new request field)
- The engine's request processing (after merge, it's a normal CreateResponseRequest)
- The provider layer (doesn't know about agents)
- Auth (agents are resolved after auth, the caller's identity is preserved)
- Storage (responses are stored normally, the agent field is echoed)

## Tool Resolution

Agent tool configs need to be resolved to actual tool definitions:

```yaml
tools:
  - type: mcp
    server: kubernetes-tools  # References an MCP server from config.mcp.servers
  - type: builtin
    name: web_search          # References a registered FunctionProvider
  - type: sandbox
    name: code_interpreter
    sandboxTemplate: python-datascience  # For future sandbox integration
  - type: function
    name: get_weather         # Client-side function tool
    parameters:
      type: object
      properties:
        location:
          type: string
```

MCP tool references are resolved against configured MCP servers. Builtin tool references match registered FunctionProviders. Function tools are passed through as-is (client executes them).

## Error Cases

- `agent` field set but agent not found: 400 "unknown agent"
- `agent` field set and `model` also set: request's model wins (override)
- `agent` field set but agent has no model and request has no model and no default: 400 "model is required"
- Agent references an MCP server that doesn't exist: log warning, exclude that tool

## Phasing

1. Add `agent` field to `CreateResponseRequest`
2. Add `AgentConfig` to `config.Config`
3. Implement merge logic in the engine (before `translateRequest`)
4. Add agent tool resolution (MCP refs, builtin refs)
5. Echo `agent` field in the response (extension field)
6. Integration tests with mock agents in test config

## Open Questions

- Should the `agent` field be echoed in the response? Likely yes, as an extension field.
- Should agent definitions support `version`? Not in the config-file phase (no rollout mechanism). Versioning comes with the CRD phase.
- Should agent names be validated (e.g., DNS-compatible)? Yes, for forward compatibility with CRD names.
