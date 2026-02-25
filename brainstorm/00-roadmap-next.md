# Antwort Roadmap: Sandbox + Agents

## Revised Implementation Order

The sandbox is a prerequisite for the full Agent story. Code execution
in isolated pods is the capability that differentiates Antwort from
every other OpenResponses implementation. Agents without sandbox are
just prompts with MCP tools (which we already support).

## Phase 1: Sandbox Foundation

| Brainstorm | Spec | What | Depends on |
|---|---|---|---|
| 11 (sandbox) | 024 | Sandbox server binary + container image | Nothing |
| 22 (code-interpreter) | 025 | SandboxClaim client + CodeInterpreter FunctionProvider | 024 |

**Deliverable**: `code_interpreter` tool works in the agentic loop. Model can write Python, it executes in a gVisor pod, results come back. SSE events show progress.

## Phase 2: Agent Definitions

| Brainstorm | Spec | What | Depends on |
|---|---|---|---|
| 21 (agents) | 026 | `agent` field on request, YAML config-based agent definitions, merge logic | Nothing (just config) |
| NEW (agent-crd) | 027 | `Agent` CRD, controller, watch/reconcile | 026 |

**Deliverable**: `kubectl apply -f my-agent.yaml` defines an agent. Clients use `"agent": "my-agent"` in requests.

## Phase 3: Agent Capabilities

| Brainstorm | Spec | What | Depends on |
|---|---|---|---|
| NEW (agent-auth) | 028 | Profile access control, tool credentials | 026, 027 |
| NEW (multi-agent) | 029 | Profile references as tools, nested loops | 026, 027 |
| NEW (versioning) | 030 | spec.version, canary rollout, status | 027 |

**Deliverable**: Agents with access control, multi-agent orchestration, and versioned rollouts.

## Other Work (Independent)

| Item | What | Status |
|---|---|---|
| vLLM Responses API provider | Provider adapter for vLLM's native Responses API | Brainstorm needed |
| Container image rebuild | Deploy specs 019-023 to ROSA | Ready to do |
| Brainstorm 19 (annotations) | 3 more SSE events | Low priority |

## Why Sandbox First

1. The Agent walkthrough demo (Google Doc) shows `code_interpreter` as the key differentiator
2. Without sandbox, Antwort's tool story is "MCP + HTTP built-ins" (same as everyone else)
3. With sandbox, Antwort is "the only OpenResponses gateway with secure code execution"
4. The sandbox server and container image are standalone deliverables useful even without Agents
5. agent-sandbox v0.1.0 is released and ready for integration
