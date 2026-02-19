# Antwort: Multi-Spec Project Overview

## Vision

Antwort is a cloud-native, Kubernetes-only Go server implementing the [OpenResponses](https://www.openresponses.org/) specification as a production-grade agentic inference gateway. It translates between the OpenResponses API and LLM inference backends (vLLM, LiteLLM, and future providers), orchestrates multi-turn agentic tool execution, and delegates all tool code execution to isolated Kubernetes sandbox pods.

Antwort is designed exclusively for Kubernetes. There is no standalone or local execution mode.

## Key Decisions

| Decision | Choice |
|---|---|
| Language | Go |
| Module | `github.com/rhuss/antwort` |
| Purpose | Agentic inference gateway |
| Runtime | Kubernetes only (no standalone mode) |
| Storage | PostgreSQL |
| Deployment | Kubernetes / OpenShift |
| Architecture | Interface-first |
| Auth | Pluggable (API key, OAuth, mTLS) |
| Tool Execution | Delegated to Kubernetes sandbox pods (never in-process) |
| Workload Identity | SPIFFE/SPIRE |
| Transport | HTTP/SSE + gRPC |
| Providers | vLLM (primary), LiteLLM (secondary) |

## API Tiers

**Stateless** (no persistence required):
- `POST /v1/responses` with `store: false`
- Single-shot inference, streaming or non-streaming
- Suitable for lightweight, stateless deployments

**Stateful** (PostgreSQL required):
- `POST /v1/responses` with `store: true` (default)
- `GET /v1/responses/{id}`, `DELETE /v1/responses/{id}`
- `previous_response_id` chaining, conversation state

## Multi-User vs Multi-Tenant

**Multi-user**: One antwort instance, multiple users. Isolation at the data layer (each user sees only their own responses). Handled via auth identity propagated to storage queries.

**Multi-tenant**: Separate antwort instances per tenant. Isolation at the deployment layer (separate databases, separate configurations). Handled via Helm releases or namespaces.

## Spec Inventory

| # | Spec | Branch | Description |
|---|---|---|---|
| 01 | Core Protocol & Data Model | `001-core-protocol` | Items, content, state machines, errors, extensions |
| 02 | Transport Layer | `002-transport-layer` | HTTP/SSE, gRPC adapters |
| 03 | Core Engine & Provider (vLLM) | `003-core-engine` | Engine, provider interface, vLLM adapter |
| 04 | Agentic Loop & Tool Orchestration | `004-agentic-loop` | Tool types, choice enforcement, agentic cycle |
| 05 | State Management & Storage | `005-storage` | Storage interface + PostgreSQL adapter |
| 06 | Authentication & Authorization | `006-auth` | Auth interface + adapters |
| 07a | Container Image | `007a-container-image` | Containerfile, multi-stage build |
| 07b | Kubernetes Deployment | `007b-kustomize` | Kustomize base + overlays |
| 07c | Helm Chart | `007c-helm` | Parameterized Helm deployment |
| 07d | Observability | `007d-observability` | Metrics, tracing, structured logging |
| 08 | Provider: LiteLLM | `008-provider-litellm` | LiteLLM adapter implementation |
| 09 | Configuration | `009-configuration` | Unified config model, env vars, validation, hot reload |
| 10 | MCP Client | `010-mcp-client` | Model Context Protocol client integration |
| 11 | Sandbox Execution | `011-sandbox` | Kubernetes sandbox pods, pod pool, SPIFFE/SPIRE identity |

## Implementation Order

Specs are numbered in dependency order. Each spec is developed on its own branch and merged sequentially.

```
01 Core Protocol
 ├── 02 Transport Layer
 └── 03 Core Engine & Provider
      ├── 04 Agentic Loop
      │    ├── 10 MCP Client
      │    └── 11 Sandbox Execution ──> 07 Deployment
      ├── 05 Storage
      └── 06 Auth
           ├── 07 Deployment
           ├── 08 Provider: LiteLLM
           └── 09 Configuration
```

Note: Specs 04, 10, 11 form the tool execution subsystem. Spec 04 defines the orchestration and interfaces. Specs 10 and 11 provide pluggable executors (MCP and sandbox respectively) that are independent of each other.

## Architecture

```
┌──────────────────────────────────────────────────────┐
│                    antwort (Go)                       │
│                                                      │
│  ┌────────────┐  ┌───────────┐  ┌─────────────┐     │
│  │ Transport  │  │ Provider  │  │   Storage   │     │
│  │ Interface  │  │ Interface │  │  Interface  │     │
│  └─────┬──────┘  └─────┬─────┘  └──────┬──────┘     │
│        │               │               │            │
│  ┌─────┴──────┐  ┌─────┴─────┐  ┌──────┴──────┐     │
│  │ HTTP/SSE   │  │  vLLM     │  │ PostgreSQL  │     │
│  │ gRPC       │  │  LiteLLM  │  │ In-memory   │     │
│  └────────────┘  └───────────┘  └─────────────┘     │
│                                                      │
│  ┌────────────┐  ┌──────────────────────────────┐    │
│  │    Auth    │  │   Tool Orchestration         │    │
│  │ Interface  │  │   (ToolExecutor interface,   │    │
│  └────────────┘  │    agentic loop)             │    │
│                  └──────┬──────┬────────────────┘    │
│                         │      │                     │
│          ┌──────────────┘      └──────────┐          │
│          │                                │          │
│  ┌───────▼───────┐              ┌─────────▼────────┐ │
│  │  MCP Client   │              │  Sandbox Client  │ │
│  │  (in-process) │              │  (REST over mTLS)│ │
│  └───────────────┘              └─────────┬────────┘ │
│                                           │          │
│          ┌───────────────────┐            │          │
│          │   Core Engine     │            │          │
│          │ (orchestration,   │            │          │
│          │  agentic loop)    │            │          │
│          └───────────────────┘            │          │
└───────────────────────────────────────────┼──────────┘
                                            │ mTLS
                                            │ (SPIFFE/SPIRE)
                                            │
              ┌─────────────────────────────▼──────────┐
              │         Kubernetes Cluster              │
              │                                        │
              │  ┌──────────┐  ┌──────────┐            │
              │  │ Python   │  │ Python   │  General   │
              │  │ Sandbox  │  │ Sandbox  │  purpose   │
              │  └──────────┘  └──────────┘  pool      │
              │                                        │
              │  ┌──────────┐  ┌──────────┐            │
              │  │ File     │  │ Web      │ Specialized│
              │  │ Search   │  │ Search   │ pools      │
              │  └──────────┘  └──────────┘            │
              │                                        │
              │  ┌──────────┐                          │
              │  │ SPIRE    │  Identity infrastructure │
              │  │ Server   │                          │
              │  └──────────┘                          │
              └────────────────────────────────────────┘
```
