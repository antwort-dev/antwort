# Brainstorm: Quickstart Series

## Purpose

Progressive quickstarts showcasing antwort's capabilities, from minimal to full RAG. Each builds on the previous, all Kubernetes-native, self-contained with their own Kustomize manifests.

## Directory Structure

```
quickstarts/
├── shared/
│   └── llm-backend/           # vLLM + Qwen 2.5 7B (prerequisite for all)
│
├── 01-minimal/                 # antwort + vLLM, in-memory, no auth
├── 02-production/              # + PostgreSQL + Prometheus + Grafana
├── 03-multi-user/              # + Keycloak + JWT auth
├── 04-mcp-tools/               # + MCP server (no auth)
├── 05-mcp-secured/             # + OAuth token exchange via Keycloak
└── 06-rag/                     # + MinIO + Qdrant + RAG MCP server
```

## Model Choice

**Qwen 2.5 7B Instruct** (FP16, 14GB). Verified working with:
- vLLM `--enable-auto-tool-choice --tool-call-parser hermes`
- Tool calling in agentic loop
- Fits on single A10G (24GB)

## Quickstart Dependency Chain

```
shared/llm-backend
    │
    ├── 01-minimal
    │       │
    │       ├── 02-production (adds PostgreSQL, Prometheus, Grafana)
    │       │       │
    │       │       └── 03-multi-user (adds Keycloak, JWT)
    │       │               │
    │       │               ├── 04-mcp-tools (adds MCP server)
    │       │               │       │
    │       │               │       └── 05-mcp-secured (adds OAuth for MCP)
    │       │               │
    │       │               └── 06-rag (adds MinIO, Qdrant, RAG MCP)
```

## Quickstart Details

### shared/llm-backend
- vLLM ServingRuntime with `--enable-auto-tool-choice --tool-call-parser hermes`
- InferenceService for Qwen 2.5 7B
- Model download Job
- Prerequisites: GPU node (A10G or better)

### 01-minimal
- Single antwort pod, in-memory storage, no auth
- ConfigMap pointing at vLLM
- Route for external access
- Test: curl a completion, verify response
- **Time to deploy**: 5 minutes (after LLM is running)

### 02-production
- Adds PostgreSQL (StatefulSet + PVC)
- Adds Prometheus (community chart or operator)
- Adds Grafana with pre-configured dashboard
- Antwort config: storage=postgres, metrics enabled
- Test: send requests, verify persistence (GET by ID), check Grafana dashboard

### 03-multi-user
- Adds Keycloak (Deployment + PostgreSQL)
- Keycloak realm with two users (alice, bob)
- Antwort config: auth=jwt, JWKS from Keycloak
- Test: get JWT from Keycloak, use it with antwort, verify tenant isolation

### 04-mcp-tools
- Adds MCP test server (get_time, echo tools)
- Antwort config: MCP server pointing at MCP test service
- Test: ask model to use tool, verify agentic loop
- Can start from 01-minimal OR 03-multi-user

### 05-mcp-secured
- Configures OAuth client_credentials for MCP server
- MCP server validates tokens against Keycloak
- Antwort config: MCP auth type=oauth_client_credentials
- Test: agentic loop with authenticated MCP calls
- Requires: 03-multi-user + 04-mcp-tools

### 06-rag
- Adds MinIO (object storage for files)
- Adds Qdrant (vector database)
- Adds RAG MCP server (Python/FastMCP, embeds + searches)
- Test: upload document, ask question about it, verify retrieval

## RAG Implementation Options

### OpenAI file_search Compatibility
OpenAI's Responses API has `file_search` as a built-in hosted tool with a Vector Store API. Full compatibility would require implementing `/v1/vector_stores`, `/v1/files`, and a built-in file_search tool. This is a major spec.

### MCP-based RAG (quickstart approach)
A separate MCP server handles file upload, embedding, and vector search. The model calls it like any MCP tool. Not OpenAI-compatible but demonstrates antwort's extensibility via MCP.

### Decision
Quickstart 06 uses MCP-based RAG. Full file_search API compatibility is a separate future spec.

## Open Questions

- Should the RAG MCP server be part of the antwort project or a separate repo?
  -> Separate project. Antwort is the gateway, not the tool implementation.
- Should quickstarts use Helm or Kustomize?
  -> Kustomize. Self-contained, no Helm dependency.
- Should quickstarts work on vanilla Kubernetes or OpenShift only?
  -> Both. Base manifests for vanilla k8s, OpenShift overlay for Route.
