# Brainstorm 37: Autonomous Agents and Antwort's Scope

**Date**: 2026-03-05
**Participants**: Roland Huss
**Goal**: Clarify Antwort's role in the autonomous agent landscape and identify what to build to make it the best inference backend for autonomous agents.

## The Scope Question

Three possible models for how autonomous agents (OpenClaw, LangGraph, CrewAI, custom) relate to Antwort:

### Model A: Agents as Clients of Antwort (CHOSEN)

Autonomous agents run as separate K8s workloads (Pods, Jobs). They call Antwort's Responses API for LLM reasoning. Antwort is their inference backend, not their runtime.

```
┌─────────────────────────────────────────────────┐
│  Kubernetes Cluster                             │
│                                                 │
│  ┌──────────────┐     ┌───────────────────────┐ │
│  │ OpenClaw Pod │────>│    Antwort Gateway    │ │
│  │ (agent)      │     │  /v1/responses        │──>  LLM
│  └──────────────┘     │  + tools, storage,    │ │
│  ┌──────────────┐     │    auth, audit         │ │
│  │ Custom Agent │────>└───────────────────────┘ │
│  │ Pod          │                               │
│  └──────────────┘                               │
└─────────────────────────────────────────────────┘
```

**Why this model**: Clean separation of concerns. Antwort does inference well. Agents use any framework they want. K8s handles lifecycle. No scope creep.

### Model B: Antwort as Agent Orchestrator (REJECTED)

Antwort spawns and manages agent processes. Rejected because it duplicates K8s primitives, massively expands scope, and requires supporting arbitrary container runtimes.

### Model C: Agents as Tools (REJECTED for long-running agents)

Agent registered as a tool in the agentic loop. Rejected for autonomous agents because of timeout issues and the fundamental mismatch between synchronous tool execution and long-running agent processes. Could work for short delegations (< 30s) but that's just a regular tool, not an autonomous agent.

## Why Model A is Right

1. **Antwort is the API that agents call for reasoning.** It would be confusing for Antwort to also be the agent. That's like a database trying to also be the application.

2. **Agent frameworks evolve fast.** OpenClaw, LangGraph, CrewAI, AutoGen, custom, new ones weekly. Antwort shouldn't couple to any of them. By being protocol-compatible (OpenAI SDK), it works with all of them.

3. **K8s already manages workloads.** Agent lifecycle (start, stop, restart, scale, schedule) is what K8s does. Building an agent orchestrator inside Antwort would be building a worse K8s.

4. **Separation enables independent scaling.** Agents scale based on demand, LLM inference scales based on GPU capacity. These have different resource profiles and shouldn't be coupled.

## What Antwort Should Build to Support Autonomous Agents Better

With Model A chosen, the question becomes: what should Antwort improve to be the best inference backend for autonomous agents?

### Gap 1: Long-Running Conversation Memory

Autonomous agents have conversations that span hours or days. Current conversation chaining (`previous_response_id`) works but is linear. Agents need:

- **Conversation branching**: Try multiple approaches, backtrack
- **Conversation summarization**: Compress long histories to fit context windows
- **Persistent memory across sessions**: Agent restarts shouldn't lose context

**Potential spec**: Enhanced Conversations API with branching, summarization hooks, and durable storage.

### Gap 2: Async Response API

Some agent requests take minutes (complex reasoning, large code generation). Current API is synchronous (request blocks until response). Agents need:

- `POST /v1/responses` with `background: true` returns immediately with response ID
- `GET /v1/responses/{id}` polls for completion
- Webhook callback when response is ready

**Note**: The `background` field already exists in the API spec but isn't implemented. This is the natural extension point.

### Gap 3: Agent Identity and Quota Management

Multiple autonomous agents sharing an Antwort instance need:

- Per-agent API keys (already supported via auth)
- Per-agent rate limits (partially supported via service tiers)
- Per-agent usage tracking and quotas
- Agent-to-agent isolation (already supported via ownership)

**Most of this already exists.** Agent Profiles + Auth + Ownership cover 80% of the need.

### Gap 4: Structured Agent Feedback

Agents often need to report progress, errors, or intermediate results back through the API. Current API returns a response and that's it. Agents need:

- Status updates during long-running agentic loops
- Intermediate tool results visible to the caller
- Error details that help the agent retry intelligently

**Potential**: Enhance streaming events to include agent-level status.

### Gap 5: Tool Discovery and Registration

Autonomous agents running in K8s may want to dynamically discover and use tools:

- MCP servers already provide dynamic tool discovery
- Agent Profiles define per-agent tool configurations
- What's missing: runtime tool registration (agent registers its own tools)

**Potential spec**: Tool Registry API for dynamic tool management.

## What NOT to Build

- **Agent orchestration**: K8s Jobs, Argo Workflows, Tekton handle this
- **Agent scheduling/triggers**: K8s CronJobs, event-driven architectures
- **Agent framework integration**: let frameworks use the standard OpenAI SDK
- **Agent state management**: agents manage their own state, Antwort provides conversation storage

## How OpenClaw Would Use Antwort Today

```python
from openai import OpenAI

# OpenClaw agent connects to Antwort as its LLM backend
client = OpenAI(
    base_url="http://antwort:8080/v1",
    api_key="openclaw-agent-key"  # per-agent API key
)

# Agent profile configures system prompt, tools, model
response = client.responses.create(
    model="llama-3.2-instruct",  # or use agent profile default
    agent="coder",  # resolves agent profile with instructions + tools
    input=[{"role": "user", "content": "Fix the bug in auth.py"}],
    tools=[{"type": "code_interpreter"}, {"type": "web_search"}],
)

# Antwort handles: LLM inference, tool execution (sandboxed),
# conversation storage, audit logging, rate limiting
```

OpenClaw's own control loop (planning, error recovery, multi-step execution) runs in the agent Pod. Antwort just provides the inference and tool execution.

## Relationship to Sally's K8s Demo

Without seeing the Slack thread details, I'd expect Sally's OpenClaw K8s demo shows:
- OpenClaw running as a K8s Deployment/Job
- Connected to an LLM backend (vLLM, Ollama, or OpenAI)
- Possibly with sandboxed execution for code changes

**Antwort's value add for this scenario**:
- Replace the direct LLM connection with Antwort for: multi-model routing, tool execution, conversation persistence, auth, audit
- Sandboxed code execution via agent-sandbox CRDs (already built)
- Multi-user isolation if multiple agent instances share the cluster

## Next Steps

1. **No new spec needed for Model A.** Antwort already works as an inference backend for agents.
2. **Consider spec for async responses** (`background: true`) if there's demand from agent frameworks.
3. **Consider spec for conversation branching** if agents need non-linear conversation history.
4. **Document the "Agents as Clients" pattern** in the tutorial module with working examples.

## Open Questions

1. Are there specific features from the Slack thread that agents need from the inference layer? (paste thread details to refine this brainstorm)
2. Should Antwort provide a reference agent deployment (Helm chart / kustomize overlay) for running OpenClaw against Antwort on K8s?
3. Is there interest in an "agent quickstart" showing OpenClaw + Antwort + vLLM in a single kind cluster?
