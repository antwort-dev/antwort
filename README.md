# Antwort

A standalone, production-grade [OpenResponses](https://www.openresponses.org/) gateway written in Go.

Antwort translates between the OpenResponses API and any `/v1/chat/completions` backend (vLLM, Ollama, LiteLLM, TGI, or any OpenAI-compatible server). It is not a framework, not an inference engine, not a proxy filter. It is a dedicated process whose primary job is the Responses API.

## Why

The OpenResponses specification is gaining traction. Llama Stack, vLLM, Semantic Router, and multiple independent projects all implement some version of it. But each one embeds the API inside a larger system: a Python framework, an inference engine, or an Envoy filter. Teams that already run inference infrastructure shouldn't need to adopt an entire framework to get a production Responses API.

Antwort explores what happens when the Responses API is the primary concern.

## Status

**Early development.** The foundational specs are implemented. The server does not yet handle end-to-end requests.

| Spec | Status | Description |
|------|--------|-------------|
| 001 Core Protocol & Data Model | Implemented | Items, content types, state machines, validation, errors, extensions |
| 002 Transport Layer | Implemented | HTTP/SSE adapter, middleware chain, graceful shutdown, in-flight registry |
| 003 Provider Abstraction (vLLM) | Planned | Provider interface + vLLM Chat Completions adapter |
| 004 Tool System | Planned | Function calling, MCP, internal tools, agentic loop |
| 005 State Management & Storage | Planned | Storage interface + PostgreSQL adapter |
| 006 Authentication & Authorization | Planned | Auth interface + pluggable adapters (API key, JWT, mTLS) |
| 007 Deployment & Operations | Planned | Container images, Kubernetes/Helm, observability |
| 008 Provider: LiteLLM | Planned | LiteLLM adapter |
| 009 Configuration | Planned | Unified config model, env vars, validation, hot reload |

Specs 001 and 002 have full implementations with comprehensive test coverage. Each spec was developed through the SDD methodology described below.

## Architecture

Antwort is designed interface-first. Every major subsystem (transport, providers, storage, auth, tools) is defined as a Go interface with pluggable implementations.

```
┌──────────────────────────────────────────────────┐
│                  antwort/pkg                     │
│                                                  │
│  ┌────────────┐  ┌───────────┐  ┌─────────────┐ │
│  │ Transport  │  │ Provider  │  │   Storage   │ │
│  │ Interface  │  │ Interface │  │  Interface  │ │
│  └─────┬──────┘  └─────┬─────┘  └──────┬──────┘ │
│        │               │               │        │
│  ┌─────┴──────┐  ┌─────┴─────┐  ┌──────┴──────┐ │
│  │ HTTP/SSE   │  │  vLLM     │  │ PostgreSQL  │ │
│  │ gRPC       │  │  LiteLLM  │  │ In-memory   │ │
│  └────────────┘  └───────────┘  └─────────────┘ │
│                                                  │
│  ┌────────────┐  ┌───────────┐                   │
│  │    Auth    │  │   Tools   │                   │
│  │ Interface  │  │ Interface │                   │
│  └────────────┘  └───────────┘                   │
│                                                  │
│          ┌───────────────────┐                    │
│          │   Core Engine     │                    │
│          │ (orchestration,   │                    │
│          │  state machine,   │                    │
│          │  agentic loop)    │                    │
│          └───────────────────┘                    │
└──────────────────────────────────────────────────┘
```

### Two API tiers

**Stateless** (`store: false`): Single-shot inference, streaming or non-streaming. No persistence required.

**Stateful** (`store: true`, default): Full CRUD on responses, `previous_response_id` chaining, conversation state. Requires PostgreSQL.

### Backend protocol

Antwort translates outbound to `/v1/chat/completions`, the widely supported standard. This means it works with any Chat Completions backend rather than requiring backends to implement the Responses API themselves.

## Methodology

Antwort is built with Specification-Driven Development (SDD). Each feature starts as a formal specification with data models, OpenAPI contracts, and dependency-ordered task plans before any code is written.

The workflow for each spec:

1. **Brainstorm** - Explore the problem space, identify design decisions, document rationale
2. **Specify** - Write a formal spec with functional requirements, success criteria, user stories, and edge cases
3. **Plan** - Create a phased implementation plan with dependency graphs
4. **Tasks** - Break the plan into dependency-ordered, file-level tasks
5. **Review** - Validate the plan against the spec (coverage matrix, red flags, task quality)
6. **Implement** - Execute tasks in dependency order with verification against the spec
7. **Verify** - Run tests, check spec compliance, confirm all functional requirements are covered

Each spec produces a complete set of artifacts in `specs/<number>-<name>/`:

```
specs/001-core-protocol/
├── spec.md              # Functional requirements, success criteria, user stories
├── plan.md              # Phased implementation plan
├── tasks.md             # Dependency-ordered task breakdown
├── data-model.md        # Type definitions and relationships
├── research.md          # Design decisions with rationale
├── review-summary.md    # Coverage matrix, review results
└── quickstart.md        # Getting started guide
```

This approach trades speed for rigor. Spec 001 produced 40 functional requirements and 45+ tests for just the data model layer. The trade-off matters when other services start depending on the system.

## Project Structure

```
antwort/
├── cmd/demo/              # Demo executable
├── pkg/
│   ├── api/               # Spec 001: Core Protocol & Data Model
│   │   ├── types.go       # Item, Message, Request, Response types
│   │   ├── validation.go  # Request/response validation
│   │   ├── state.go       # State machine transitions
│   │   ├── events.go      # Streaming event types
│   │   ├── errors.go      # Structured API errors
│   │   └── id.go          # Prefixed ID generation
│   └── transport/         # Spec 002: Transport Layer
│       ├── handler.go     # ResponseCreator, ResponseStore interfaces
│       ├── middleware.go   # Middleware composition
│       ├── inflight.go    # In-flight registry for cancellation
│       └── http/          # HTTP/SSE adapter
│           ├── adapter.go # Request routing
│           ├── sse.go     # Server-Sent Events writer
│           └── server.go  # Server lifecycle, graceful shutdown
├── specs/                 # Specification documents
│   ├── 001-core-protocol/
│   └── 002-transport-layer/
├── brainstorm/            # Early design exploration
└── go.mod
```

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Language | Go | High concurrency, low memory per connection, single binary deployment, natural fit for interface-first design |
| Dependencies | Zero (stdlib only) | Maximum portability, minimal attack surface |
| Backend protocol | Chat Completions | Works with any backend, not just those that speak Responses API |
| Storage | PostgreSQL (planned) | Durable, shared across replicas, production standard |
| Deployment | Kubernetes/OpenShift | Health probes, Helm charts, CRDs for declarative configuration |

## Spec Dependency Graph

Specs are numbered in dependency order. Each builds on the ones before it.

```
001 Core Protocol
  └─> 002 Transport
  └─> 003 Provider (vLLM)
        └─> 005 Storage
        └─> 004 Tools
              └─> 006 Auth
                    └─> 007 Deployment
                    └─> 008 Provider (LiteLLM)
```

## What a Production Gateway Needs

Regardless of which project or combination moves forward, a production OpenResponses gateway needs capabilities that no single project provides today:

- **Chat Completions translation** so it works with any backend
- **Durable storage** shared across replicas
- **Multi-user isolation** at the data layer, with per-user scoping
- **Authentication** with pluggable backends
- **Agentic tool loop** with MCP support for server-side tool execution
- **Observability** (Prometheus metrics, OpenTelemetry tracing, structured logging)
- **Kubernetes deployment** with real health probes, Helm charts, and optionally CRDs

Antwort's roadmap addresses all of these through the spec sequence above.

## Related Work

- [openresponses-gw](https://github.com/leseb/openresponses-gw) - A Go gateway that sits in front of Responses API backends, adding statefulness, MCP tools, and file search
- [Llama Stack](https://github.com/meta-llama/llama-stack) - Meta's LLM framework with the most complete Responses API implementation
- [vLLM](https://github.com/vllm-project/vllm) - Inference engine with an experimental `/v1/responses` endpoint
- [Semantic Router](https://github.com/vllm-project/semantic-router) - Envoy filter providing Responses API translation

## License

TBD
