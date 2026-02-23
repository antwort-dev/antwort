# Antwort

**The server-side agentic framework.**

A production-grade [OpenResponses](https://www.openresponses.org/) API implementation written in Go. Antwort runs AI agents server-side on Kubernetes with sandboxed code execution, multi-tenant isolation, and full API compatibility with any OpenAI SDK.

**Website:** [antwort-dev.github.io](https://antwort-dev.github.io) | **Docs:** [antwort-dev.github.io/docs](https://antwort-dev.github.io/docs/)

## What It Does

Antwort is the most complete open-source server-side implementation of the OpenResponses standard. It translates between the Responses API and any `/v1/chat/completions` backend (vLLM, Ollama, LiteLLM, or any OpenAI-compatible server), adding agentic capabilities, tool execution, multi-user isolation, and production operations on top.

Any existing OpenAI SDK (Python, Node, Go, Rust) works without modification. Point your client at Antwort, and the Responses API works as expected.

## Status

Antwort started as a proof-of-concept for [Specification-Driven Development (SDD)](https://github.com/rhuss/cc-sdd-plugin), exploring how an AI-assisted, spec-first methodology works for building a non-trivial system from scratch. The project has since grown beyond that original scope into a full agentic AI platform targeting production Kubernetes environments.

| Spec | Status | Description |
|------|--------|-------------|
| 001 Core Protocol & Data Model | Implemented | Items, content types, state machines, validation, errors, extensions |
| 002 Transport Layer | Implemented | HTTP/SSE adapter, middleware chain, graceful shutdown, in-flight registry |
| 003 Core Engine & Provider | Implemented | Protocol-agnostic Provider interface, vLLM adapter, streaming, conversation chaining |
| 004 Agentic Loop | Implemented | Multi-turn reasoning, concurrent tool execution, tool call routing |
| 005 Storage | Implemented | Storage interface, PostgreSQL adapter, in-memory store |
| 006 Conformance | Implemented | Compliance test suite for the OpenResponses API |
| 007 Authentication | Implemented | JWT/OIDC, API key auth, multi-user isolation |
| 008 Provider: LiteLLM | Implemented | LiteLLM adapter for multi-provider access |
| 009 MCP Tools | Implemented | Model Context Protocol integration with OAuth token exchange |
| 010 Web Search | Implemented | SearXNG integration as built-in function provider |
| 011 Observability | Implemented | Prometheus metrics, OpenTelemetry GenAI conventions |
| 012-017 Various | Implemented | Configuration, deployment overlays, file search provider |
| 018 Landing Page | Implemented | [antwort-dev.github.io](https://antwort-dev.github.io), Antora documentation |

### Platform Vision (Next Phases)

Antwort is evolving into a server-native agent platform that brings the best ideas from client-side agent frameworks (like OpenClaw) to the server, with Kubernetes-native security as a first principle. See the [platform vision document](specs/vision-agent-platform.md) and the [blog post](blog-server-native-agent-platform.md) for the full story.

| Phase | Capability | Status |
|-------|-----------|--------|
| 1 | Kubernetes Sandbox Executor (code_interpreter via agent-sandbox CRDs) | Planned |
| 2 | Agent Profiles (server-side SOUL.md, `/v1/agents` API) | Planned |
| 3 | Memory & Knowledge (pluggable vector stores, file_search) | Planned |
| 4 | Ambient Agents (webhooks, cron triggers, completion hooks) | Planned |
| 5 | Delivery Channels (Slack, Teams, email, webhooks) | Planned |
| 6 | Tool Registry (curated, audited, per-tenant permissions) | Planned |
| 7 | Kubernetes Operator (declarative CRDs for lifecycle management) | Planned |

## Architecture

Antwort is designed interface-first. Every major subsystem is defined as a Go interface with pluggable implementations. The core depends only on the Go standard library.

```
┌──────────────────────────────────────────────────────┐
│                    Antwort Gateway                    │
│                                                      │
│  ┌──────────┐  ┌──────────┐  ┌────────────────────┐ │
│  │Transport │  │  Auth    │  │   Observability    │ │
│  │HTTP/SSE  │  │JWT/OIDC  │  │Prometheus/OTel     │ │
│  └────┬─────┘  │API Key   │  └────────────────────┘ │
│       │        └──────────┘                          │
│  ┌────▼──────────────────────────────────┐           │
│  │  Engine (Agentic Loop)                │           │
│  │  Multi-turn reasoning, tool routing   │           │
│  └────┬──────────────┬───────────────────┘           │
│       │              │                               │
│  ┌────▼─────┐  ┌─────▼──────────────────────┐       │
│  │ Provider │  │    Tool Executors           │       │
│  │ vLLM     │  │  ┌─────┐ ┌─────┐ ┌──────┐  │       │
│  │ LiteLLM  │  │  │ MCP │ │Web  │ │Sand- │  │       │
│  │ Ollama   │  │  │     │ │Srch │ │box   │  │       │
│  └──────────┘  │  └─────┘ └─────┘ └──────┘  │       │
│                └─────────────────────────────┘       │
│  ┌──────────────────────────┐                        │
│  │  Storage                 │                        │
│  │  PostgreSQL / In-memory  │                        │
│  └──────────────────────────┘                        │
└──────────────────────────────────────────────────────┘
```

### Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Language | Go 1.22+ | High concurrency, single binary, interface-first |
| Core dependencies | Zero (stdlib only) | Maximum portability, minimal attack surface |
| Provider protocol | Protocol-agnostic | Any backend via adapter pattern |
| API modes | Stateless + stateful per-request | `store: false` for fire-and-forget, `store: true` for conversation chaining |
| Storage | Pluggable (PostgreSQL default) | Clean interface for custom backends |
| Deployment | Kubernetes-exclusive | No standalone mode. HPA, Prometheus, Kustomize overlays |

## Methodology: Specification-Driven Development

Antwort is built entirely with [Specification-Driven Development (SDD)](https://github.com/rhuss/cc-sdd-plugin), a methodology where every feature starts as a formal specification before any code is written. The project was originally created as a proof-of-concept for this approach, demonstrating how AI-assisted, spec-first development works for building a non-trivial production system from scratch.

The SDD workflow for each feature:

1. **Brainstorm** - Explore the problem space, identify design decisions
2. **Specify** - Formal spec with functional requirements, success criteria, user stories
3. **Plan** - Phased implementation plan with dependency graphs
4. **Tasks** - Dependency-ordered, file-level task breakdown
5. **Review** - Coverage matrix, red flag scanning, quality validation
6. **Implement** - Execute tasks in order with spec verification
7. **Verify** - Tests, spec compliance, functional requirement coverage

Each spec produces a complete set of artifacts:

```
specs/001-core-protocol/
├── spec.md              # Functional requirements, success criteria
├── plan.md              # Phased implementation plan
├── tasks.md             # Dependency-ordered task breakdown
├── data-model.md        # Type definitions and relationships
├── research.md          # Design decisions with rationale
├── review-summary.md    # Coverage matrix, review results
└── quickstart.md        # Getting started guide
```

The SDD plugin for Claude Code is available at [github.com/rhuss/cc-sdd-plugin](https://github.com/rhuss/cc-sdd-plugin).

## Quick Start

### Prerequisites

- Kubernetes cluster
- An LLM backend (vLLM, LiteLLM, Ollama, or any OpenAI-compatible endpoint)

### Deploy

```bash
# Configure your LLM provider
export ANTWORT_PROVIDER_URL=http://your-llm-backend:8000

# Deploy to Kubernetes
kubectl apply -k deploy/overlays/dev
```

### Send a Request

```bash
curl -X POST http://antwort:8080/v1/responses \
  -H "Content-Type: application/json" \
  -d '{
    "model": "your-model",
    "tools": [{"type": "web_search"}],
    "input": [{
      "role": "user",
      "content": "What are the latest AI news?"
    }]
  }'
```

Any OpenAI SDK works too:

```python
from openai import OpenAI

client = OpenAI(base_url="http://antwort:8080/v1", api_key="your-key")
response = client.responses.create(
    model="your-model",
    input="What are the latest AI news?",
    tools=[{"type": "web_search"}],
)
print(response.output_text)
```

## Related Work

- [openresponses-gw](https://github.com/leseb/openresponses-gw) - A Go gateway adding statefulness, MCP tools, and file search to Responses API backends
- [Llama Stack](https://github.com/llamastack/llama-stack) - LLM framework with Responses API support and safety features (Llama Guard)
- [vLLM](https://github.com/vllm-project/vllm) - Inference engine with experimental `/v1/responses` endpoint
- [Semantic Router](https://github.com/vllm-project/semantic-router) - Envoy filter providing Responses API translation

## License

Apache 2.0
