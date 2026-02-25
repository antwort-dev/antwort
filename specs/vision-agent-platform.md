# Vision: Antwort as a Server-Native Agent Platform

**Created**: 2026-02-22
**Status**: Draft
**Scope**: Cross-cutting vision document spanning multiple future specs

## Executive Summary

Antwort started as an OpenResponses API gateway. This document describes its evolution into a **server-native agent platform** that absorbs the best ideas from client-side agent frameworks (particularly OpenClaw) while fixing their fundamental security and scalability limitations.

The core thesis: OpenClaw proved that users want agentic AI that *acts*, not just *answers*. But OpenClaw's single-user, laptop-bound, sandbox-off-by-default architecture is incompatible with enterprise deployment. Antwort builds the same experience server-side, with Kubernetes-native security as a first principle.

The **Responses API remains the primary runtime interface** (data plane). Platform capabilities that don't fit naturally into the Responses API are exposed via side-APIs (control plane). A vanilla OpenAI client can use antwort without knowing about the side-APIs. An antwort-aware client unlocks the full platform.

## Context: What OpenClaw Proved

OpenClaw (formerly Clawdbot) is an open-source personal AI assistant created by Peter Steinberger. It reached 200,000 GitHub stars and became the fastest-growing open source project of 2026. It proves several things:

**What works:**
- Agents that execute code, browse the web, and manage files are far more useful than chatbots that only generate text
- Multi-channel delivery (WhatsApp, Telegram, Slack, Discord) meets users where they already are
- Proactive scheduling (cron/heartbeat) enables automation that works while users sleep
- Markdown-based personality (`SOUL.md`, `IDENTITY.md`) gives users complete control over agent behavior without model fine-tuning
- A skills/plugin ecosystem (ClawHub) lets the community extend capabilities rapidly

**What's broken:**
- Sandbox is off by default. CVE-2026-25253 (CVSS 8.8) led to 135,000+ exposed instances, with 50,000+ directly vulnerable to remote code execution
- Security researcher Maor Dayan found 42,665 exposed instances on Shodan, 93.4% with authentication bypasses
- 12% of ClawHub skills (341 of 2,857 audited) were found to be malicious, performing data exfiltration or prompt injection
- Single-user, single-machine trust boundary. No multi-tenancy, no centralized policy enforcement
- Config files store secrets in plaintext. Infostealers now specifically target OpenClaw configuration
- No audit trail for tool executions. No compliance story

These security failures are not bugs to be patched. They are architectural limitations of a client-side, local-first model. The server-side model eliminates them structurally.

## Design Principles

These principles extend the [constitution](constitution.md) for the platform layer.

### P1: Data Plane / Control Plane Separation

The Responses API is the **data plane**: the runtime path where requests flow, agents reason, tools execute, and responses stream back. It stays spec-compliant with the OpenResponses standard. Existing clients (OpenAI SDK, any HTTP client) work without modification.

Platform capabilities that don't map naturally to request/response semantics live in **control plane side-APIs**: agent profiles, tool registry management, memory ingestion, schedule definitions, delivery channel configuration.

The critical constraint: **the data plane must be fully functional without the control plane.** A developer who only knows the Responses API gets a working agentic experience, including sandbox code execution.

### P2: Secure by Default, Not Secure by Opt-In

OpenClaw's fatal flaw is that sandboxing is optional and off by default. In antwort, code execution *only* happens inside Kubernetes sandbox pods. There is no "unsandboxed" execution path. The question is not whether to sandbox, but which sandbox policy to apply.

### P3: Extensions, Not Dialect

When the Responses API needs platform-specific data, it flows through the existing `extensions` field (`map[string]json.RawMessage`) on Response objects, or as extra fields that OpenAI clients silently ignore. Antwort never introduces breaking changes to the Responses API schema.

The bridge mechanism: Responses API requests can reference control plane resources by ID (e.g., `agent_id` to load a stored agent profile). When these fields are absent, the request works standalone. When present, the platform enriches the request from stored configuration.

### P4: Agent-Sandbox as Infrastructure

Antwort does not implement its own pod management. It consumes the [agent-sandbox](https://github.com/kubernetes-sigs/agent-sandbox) CRDs (`Sandbox`, `SandboxClaim`, `SandboxWarmPool`, `SandboxTemplate`) from Kubernetes SIG Apps. The agent-sandbox controller handles pod lifecycle, warm pools, and stable identity. Antwort owns the container image and the REST API that runs inside the sandbox pod.

## API Strategy

### Data Plane: Responses API

The Responses API handles all runtime interactions. The following capabilities map naturally to it:

| Capability | Responses API Mapping |
|---|---|
| Code execution | `tools: [{type: "code_interpreter"}]`. Server executes in sandbox pod. Client receives standard `code_interpreter_call` output items. |
| Agent personality | `instructions` field (inline) or `agent_id` field (references stored profile). |
| Conversation memory | `previous_response_id` for conversation chaining. `store: true` for persistence. |
| Knowledge retrieval | `tools: [{type: "file_search"}]` as a built-in tool. Server queries vector store. |
| MCP tools | `tools: [{type: "mcp", ...}]`. Server discovers and executes MCP tools. |
| Built-in tools | `tools: [{type: "web_search"}]` and similar. Server-side execution. |

### Control Plane: Side-APIs

Side-APIs handle configuration, management, and capabilities that don't fit the request/response model:

| Side-API | Purpose | Used At |
|---|---|---|
| `/v1/agents` | CRUD for agent profiles (name, instructions, default tools, model, memory settings) | Setup time, referenced by `agent_id` at runtime |
| `/v1/sandboxes` | Sandbox policy management (resource limits, network rules, image allowlists, timeouts) | Setup time, policies applied automatically at runtime |
| `/v1/vector_stores` | Knowledge base management (document ingestion, index configuration) | Ingestion time, queried via `file_search` tool at runtime |
| `/v1/schedules` | Cron/trigger definitions that invoke agent profiles on a schedule | Setup time, executions create Responses API requests internally |
| `/v1/channels` | Delivery channel configuration (Slack, Teams, email, webhooks) | Setup time, delivery triggered by response completion |
| `/v1/tools` | Tool registry (browse, register, audit, per-tenant allowlists) | Setup time, tools available at runtime based on tenant permissions |

### The Bridge: How Data Plane References Control Plane

```json
// Minimal request (any OpenAI client):
{
  "model": "llama-4",
  "instructions": "You are a helpful assistant.",
  "input": [{"role": "user", "content": "Analyze this dataset"}],
  "tools": [{"type": "code_interpreter"}]
}

// Platform-aware request (antwort extensions):
{
  "model": "llama-4",
  "agent_id": "agent_data_analyst",
  "input": [{"role": "user", "content": "Analyze this dataset"}]
}
```

The first request works with any OpenAI SDK. The second request loads the agent profile from the control plane, which provides instructions, tools (including code_interpreter with specific sandbox policy), model preferences, and memory configuration. Both produce the same output format.

## Platform Capabilities

### 1. Kubernetes Sandbox Executor

**What it does:** Executes code generated by the LLM in isolated Kubernetes pods, using the agent-sandbox CRDs for pod lifecycle management.

**API surface:** Pure Responses API. The `code_interpreter` tool type triggers server-side execution. Clients see standard `code_interpreter_call` items in the response.

**Architecture:**
- Antwort creates `SandboxClaim` resources to request sandbox pods
- The agent-sandbox controller provisions pods from `SandboxWarmPool` (sub-second allocation) or creates new ones from `SandboxTemplate`
- Each sandbox pod runs an antwort-built container image with Python runtime, `uv` package manager, and a REST execution server
- Communication between antwort and sandbox pods uses mutual TLS with SPIFFE/SPIRE workload identities
- The execution server accepts code, runs it, captures stdout/stderr, and returns results (including generated files)

**Session model:** Session-scoped sandboxes by default. The sandbox pod persists across tool calls within a single conversation (installed packages, created files, variables carry over). The pod is released when the conversation ends or an idle timeout expires. Warm pools ensure fast allocation for new sessions.

**Language support:** Python initially. The architecture supports bring-your-own (BYO) images by configuring alternate `SandboxTemplate` resources with different base images (Node.js, Go, multi-language). The sandbox REST API is language-agnostic.

**Security layers:**

| Layer | Mechanism | Default |
|---|---|---|
| Kernel isolation | gVisor RuntimeClass via agent-sandbox | Enabled |
| Network isolation | Kubernetes NetworkPolicy | Deny all egress |
| Resource limits | Pod resource requests/limits | 500m CPU, 512Mi memory, 1Gi ephemeral storage |
| Process isolation | SecurityContext: non-root, read-only rootfs, no privilege escalation, drop all capabilities | Enabled |
| Communication | mTLS with SPIFFE/SPIRE between antwort and sandbox pods | Required |
| Execution timeout | Hard kill after configurable limit | 30 seconds |
| Image control | Only images from approved SandboxTemplate resources | Enforced |
| Audit | Every execution logged with tenant, code hash, output, duration, resource usage | Always on |

### 2. Agent Profiles

**What it does:** Stores reusable agent configurations that bundle instructions, tools, model preferences, and memory settings. This is the server-side equivalent of OpenClaw's `SOUL.md` + `IDENTITY.md`.

**API surface:** Control plane side-API (`/v1/agents`) for CRUD. Referenced via `agent_id` in Responses API requests.

**Agent profile fields:**
- `name`: Human-readable identifier
- `instructions`: System prompt (the "soul" of the agent)
- `model`: Default model for this agent
- `tools`: Default tool set (code_interpreter, file_search, MCP tools, functions)
- `sandbox_policy`: Which sandbox configuration to use for code execution
- `memory`: Memory configuration (vector store IDs, conversation retention policy)
- `metadata`: Arbitrary key-value pairs for organizational tagging

**Why this matters:** Without agent profiles, every Responses API request must include the full agent configuration inline. For enterprise deployments with many agents serving different roles (security analyst, data engineer, documentation writer), profiles eliminate repetition and enable centralized management.

### 3. Memory and Knowledge

**What it does:** Provides agents with persistent memory beyond conversation chaining. Two forms: vector-based knowledge retrieval (RAG) and conversation memory.

**API surface:**
- Control plane: `/v1/vector_stores` for document ingestion and index management
- Data plane: `file_search` tool in the Responses API for runtime retrieval

**Conversation memory:** Already partially implemented via `previous_response_id` chaining and the storage layer. The platform extension adds longer-term memory: summarization of past conversations, user preference tracking, and cross-conversation context.

**Knowledge retrieval:** Upload documents to a vector store via the side-API. At runtime, the `file_search` built-in tool queries the store and injects relevant chunks into the agent's context. This is functionally equivalent to OpenAI's file_search tool.

### 4. Proactive Scheduling

**What it does:** Triggers agent invocations on a schedule or in response to events, without requiring a user to send a request. This is the server-native equivalent of OpenClaw's cron/heartbeat.

**API surface:** Control plane side-API (`/v1/schedules`) for defining triggers. Each trigger references an agent profile and an input template. When the trigger fires, antwort creates a Responses API request internally and routes the output to configured delivery channels.

**Trigger types:**
- **Cron:** Time-based schedules (e.g., "every morning at 9am, summarize overnight alerts")
- **Webhook:** External events trigger agent invocations (e.g., "when a GitHub issue is created, triage it")
- **Completion hook:** An agent invocation triggers another agent (e.g., "after the security scan completes, file a Jira ticket if vulnerabilities found")

**Implementation:** Kubernetes CronJobs for time-based triggers. Webhook endpoints in the antwort HTTP server for event-driven triggers. Completion hooks as post-processing in the agentic loop.

### 5. Delivery Channels

**What it does:** Pushes agent responses to communication platforms beyond the API response. When a scheduled agent completes, or when a user configures delivery rules, results are sent to Slack, Microsoft Teams, email, or arbitrary webhooks.

**API surface:** Control plane side-API (`/v1/channels`) for configuring delivery targets. Delivery rules can be attached to agent profiles, schedules, or individual responses.

**Channel types:**
- **Webhook:** POST response content to a URL (most flexible)
- **Slack:** Post to a Slack channel or DM via Slack API
- **Microsoft Teams:** Post via Teams webhook or Graph API
- **Email:** Send via SMTP or email API
- **Custom:** Pluggable delivery adapters behind an interface

**Scope note:** This is the lowest-priority platform capability. The webhook channel alone covers most enterprise integration needs. Native Slack/Teams/email adapters are convenience features that can be added incrementally.

### 6. Tool Registry

**What it does:** Provides a curated, security-audited catalog of tools that agents can use. Replaces OpenClaw's ClawHub model (community-contributed, unvetted) with a managed registry that enforces per-tenant permissions.

**API surface:** Control plane side-API (`/v1/tools`) for browsing, registering, and auditing tools. Tools in the registry are available in the Responses API's `tools` array by reference.

**Registry model:**
- Tools are registered with schema, description, and execution metadata
- MCP servers can be registered as tool sources (automatic tool discovery)
- Per-tenant allowlists control which tools are available
- Audit logs track tool usage across tenants
- Optional: security scanning of tool schemas for injection patterns

**Relationship to existing MCP integration:** The current MCP client in antwort connects to MCP servers and discovers tools at runtime. The registry wraps this with management: which MCP servers are trusted, which tools from each server are allowed, and per-tenant access control.

## Phasing

The capabilities are ordered by dependency and value delivery:

### Phase 1: Sandbox Executor (Foundation)

Implement `ToolKindSandbox` using agent-sandbox CRDs. This is the "why use antwort" differentiator and the foundation for all other platform capabilities. Delivers: secure code execution via `code_interpreter` tool, working with any existing OpenAI client.

**Depends on:** Existing specs 001-017 (core protocol through web search).
**Deliverables:** Sandbox executor implementation, sandbox container image (Python + uv + REST server), SandboxTemplate and SandboxWarmPool default configurations, integration with the agentic loop.

### Phase 2: Agent Profiles

CRUD API for agent profiles. The `agent_id` bridge field in Responses API requests. Makes antwort useful for teams managing multiple agent roles.

**Depends on:** Phase 1 (agent profiles reference sandbox policies).
**Deliverables:** `/v1/agents` side-API, agent profile storage (PostgreSQL), `agent_id` resolution in the engine.

### Phase 3: Memory and Knowledge

Vector store integration for RAG. `file_search` as a built-in tool. Longer-term conversation memory.

**Depends on:** Phase 2 (agent profiles reference memory configuration).
**Deliverables:** `/v1/vector_stores` side-API, file_search tool executor, document ingestion pipeline.

### Phase 4: Proactive Scheduling

Cron triggers, webhook triggers, completion hooks. Makes agents autonomous.

**Depends on:** Phase 2 (schedules reference agent profiles).
**Deliverables:** `/v1/schedules` side-API, Kubernetes CronJob integration, webhook endpoint, completion hook in agentic loop.

### Phase 5: Delivery Channels

Push agent responses to Slack, Teams, email, webhooks. Completes the "agent meets you where you are" story.

**Depends on:** Phase 4 (scheduled agents need delivery targets).
**Deliverables:** `/v1/channels` side-API, webhook delivery adapter, Slack adapter, email adapter.

### Phase 6: Tool Registry

Managed tool catalog with per-tenant permissions. Replaces unvetted community tools with a curated registry.

**Depends on:** Phase 1 (tools execute in sandbox), existing MCP integration.
**Deliverables:** `/v1/tools` side-API, registry storage, permission enforcement in engine.

Phases 1-2 deliver a compelling product: secure code execution with managed agent profiles, accessible via the standard Responses API. Phases 3-6 build toward the complete OpenClaw superset.

## Non-Goals

- **Client-side agent runtime.** Antwort is a server. It does not replace OpenClaw, Claude Code, or Cursor for local development workflows. It complements them by providing a secure backend.
- **Model hosting.** Antwort proxies to model providers (vLLM, LiteLLM, cloud APIs). It does not serve models directly.
- **General-purpose Kubernetes operator.** Antwort consumes agent-sandbox CRDs. It does not implement its own pod management controllers.
- **Fine-tuning or model customization.** Agent personality comes from system prompts (instructions), not from model fine-tuning.
- **Replacing MCP.** MCP is the tool integration standard. Antwort uses MCP, wraps it with management (registry, permissions), but does not compete with or replace the protocol.

## Open Questions

1. **Vector store implementation:** Build a custom vector store adapter or integrate with an existing solution (pgvector, Milvus, Weaviate)? The interface-first principle suggests defining the interface first and shipping with pgvector as the default adapter.

2. **Sandbox REST API design:** What endpoints does the execution server inside the sandbox pod expose? Minimum: POST /execute (code in, results out). Extended: GET /files (list generated files), GET /files/{id} (download file), POST /install (install packages), GET /status (health check).

3. **Agent profile versioning:** Should agent profiles be versioned (like Kubernetes resources) so that changes don't affect running conversations? Or is latest-wins acceptable?

4. **Multi-agent orchestration:** OpenClaw's sessions allow agent-to-agent communication. Should antwort support a "supervisor" pattern where one agent invocation spawns sub-invocations? This could be modeled as nested Responses API calls, but the authorization and resource accounting implications need careful design.

5. **File handling in the Responses API:** The `code_interpreter_call` output can include file references. How are generated files stored and retrieved? Options: ephemeral (lost when sandbox pod is destroyed), persisted to object storage (S3/GCS), or persisted to the vector store for future retrieval.

6. **Delivery channel authentication:** Slack and Teams integrations require OAuth tokens. Where are these stored? Kubernetes Secrets with external secret operators seems aligned with the Kubernetes-native principle.

7. **Scheduling granularity:** Should schedules be defined at the agent level (agent runs on a schedule) or at the task level (specific tasks run on schedules with different agents)? The task-level model is more flexible but more complex.

---

**This document is a living vision.** Individual capabilities will be specified as formal specs (following the SDD workflow) before implementation. The phasing may adjust based on user feedback and implementation experience.
