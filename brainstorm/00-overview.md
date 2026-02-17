# Antwort: Multi-Spec Project Overview

## Vision

Antwort is a Go library and server implementing the [OpenResponses](https://www.openresponses.org/) specification as a production-grade protocol translation engine. It translates between the OpenResponses API and LLM inference backends (vLLM, LiteLLM, and future providers).

Designed interface-first, antwort can be deployed as a standalone API server or integrated into routing infrastructure.

## Key Decisions

| Decision | Choice |
|---|---|
| Language | Go |
| Module | `github.com/rhuss/antwort` |
| Purpose | Production proxy |
| Storage | PostgreSQL |
| Deployment | Kubernetes / OpenShift |
| Architecture | Interface-first |
| Auth | Pluggable (API key, OAuth, mTLS) |
| Tools | Full spec compliance, stateless/stateful tiers |
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

## Spec Inventory

| # | Spec | Branch | Description |
|---|---|---|---|
| 01 | Core Protocol & Data Model | `spec/01-core-protocol` | Items, content, state machines, errors, extensions |
| 02 | Transport Layer | `spec/02-transport` | HTTP/SSE, gRPC adapters |
| 03 | Provider Abstraction (vLLM) | `spec/03-provider-vllm` | Provider interface + vLLM adapter |
| 04 | Tool System | `spec/04-tools` | Function calling, MCP, internal tools, agentic loop |
| 05 | State Management & Storage | `spec/05-storage` | Storage interface + PostgreSQL adapter |
| 06 | Authentication & Authorization | `spec/06-auth` | Auth interface + adapters |
| 07 | Deployment & Operations | `spec/07-deployment` | Container, k8s, observability |
| 08 | Provider: LiteLLM | `spec/08-provider-litellm` | LiteLLM adapter implementation |

## Implementation Order

Specs are numbered in dependency order. Each spec is developed on its own branch and merged sequentially.

```
01-core-protocol
  └─> 02-transport
  └─> 03-provider-vllm
        └─> 05-storage
        └─> 04-tools
              └─> 06-auth
                    └─> 07-deployment
                    └─> 08-provider-litellm
```

## Architecture

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
          │              │             │
     ┌────┴────┐              ┌────┴──────┐
     │  cmd/   │              │ pkg/      │
     │ server  │              │ embed     │
     └─────────┘              └───────────┘
```
