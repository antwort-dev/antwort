# Feature Specification: Quickstart Series

**Feature Branch**: `015-quickstarts`
**Created**: 2026-02-20
**Status**: Draft

## Overview

This specification defines a progressive quickstart series that showcases antwort's capabilities from minimal to full RAG. Each quickstart builds on the previous, creating a learning path from "deploy in 5 minutes" to "enterprise-grade agentic gateway with authenticated tool execution and document retrieval."

All quickstarts are Kubernetes-native, self-contained with their own Kustomize manifests, and share a common LLM backend. A developer can pick any quickstart matching their use case and deploy it independently (with its prerequisites).

## Quickstart Inventory

### Shared: LLM Backend

**Status**: Done

Deploys vLLM with Qwen 2.5 7B Instruct as the inference backend for all quickstarts. Includes model download job, Deployment, Service, and PVC.

**Prerequisites**: GPU node (A10G 24GB or better), NVIDIA GPU Operator

---

### 01-Minimal: Hello Antwort

**Status**: Done

The simplest deployment: antwort with in-memory storage, no authentication, connecting to the shared LLM backend. Validates basic text completion, streaming, and metrics.

**Prerequisites**: Shared LLM backend
**Demonstrates**: Basic OpenResponses API, streaming, health checks, Prometheus metrics

---

### 02-Production: Persistent Storage + Monitoring

**Status**: Not started

Adds PostgreSQL for persistent response storage and Prometheus + Grafana for monitoring. Includes a pre-configured Grafana dashboard with LLM-specific panels (request latency, token throughput, streaming connections, OTel GenAI metrics).

**Prerequisites**: 01-Minimal
**Demonstrates**: Response persistence (save/retrieve/delete), conversation chaining via `previous_response_id`, Prometheus metrics scraping, Grafana dashboards
**New pods**: PostgreSQL (StatefulSet), Prometheus, Grafana

---

### 03-Multi-User: Authentication + Tenant Isolation

**Status**: Not started

Adds Keycloak as an identity provider with JWT authentication. Two test users (alice and bob) in separate tenants demonstrate that each user can only access their own responses.

**Prerequisites**: 02-Production
**Demonstrates**: JWT authentication via Keycloak, tenant isolation (alice cannot see bob's responses), rate limiting by service tier
**New pods**: Keycloak (Deployment + PostgreSQL)

---

### 04-MCP Tools: Agentic Tool Calling

**Status**: Not started

Adds an MCP server with tools (get_time, echo, web_search). The model calls tools, antwort executes them via MCP, feeds results back, and the model produces a final answer. The full agentic loop in action.

**Prerequisites**: 01-Minimal or 03-Multi-User
**Demonstrates**: MCP tool discovery, agentic loop (multi-turn inference + tool execution), concurrent tool calls
**New pods**: MCP test server

---

### 05-MCP Secured: OAuth for Tool Servers

**Status**: Not started

Configures OAuth client_credentials authentication for the MCP server. Antwort obtains tokens from Keycloak and passes them to the MCP server. Demonstrates secure tool execution in enterprise environments.

**Prerequisites**: 03-Multi-User + 04-MCP Tools
**Demonstrates**: OAuth client_credentials for MCP, token caching and refresh, Keycloak as OAuth provider for tool servers
**New pods**: None (configuration only)

---

### 06-RAG: Document Retrieval

**Status**: Not started

Adds MinIO for file storage and Qdrant for vector search. A RAG MCP server handles document upload, embedding, and retrieval. The model queries documents to answer questions with source citations.

**Prerequisites**: 03-Multi-User or 04-MCP Tools
**Demonstrates**: File upload to object storage, document embedding and vector indexing, retrieval-augmented generation via MCP tools
**New pods**: MinIO, Qdrant, RAG MCP server

## User Scenarios & Testing

### User Story 1 - Deploy Minimal Quickstart (Priority: P1)

A developer deploys quickstart 01 on a Kubernetes cluster with an LLM backend. Within 5 minutes, they send their first request through antwort and receive a response.

**Acceptance Scenarios**:

1. **Given** the shared LLM backend is running, **When** the developer applies 01-minimal manifests, **Then** antwort starts and responds to requests within 5 minutes
2. **Given** 01-minimal is deployed, **When** the developer follows the README test examples, **Then** all examples (text completion, streaming, health, metrics) work as documented

---

### User Story 2 - Deploy Production Quickstart (Priority: P1)

A developer deploys quickstart 02 with PostgreSQL and monitoring. Responses are persisted and visible in Grafana dashboards.

**Acceptance Scenarios**:

1. **Given** 02-production is deployed, **When** a response is created, **Then** it can be retrieved by ID after a pod restart (persistence verified)
2. **Given** 02-production is deployed, **When** Grafana is accessed, **Then** the antwort dashboard shows request rate, latency, and token throughput

---

### User Story 3 - Deploy Multi-User Quickstart (Priority: P2)

A developer deploys quickstart 03 with Keycloak. Two users authenticate with JWT tokens and their responses are isolated.

**Acceptance Scenarios**:

1. **Given** 03-multi-user is deployed, **When** alice creates a response, **Then** bob cannot retrieve it (tenant isolation)
2. **Given** Keycloak is running, **When** a user obtains a JWT and sends it to antwort, **Then** the request is authenticated and the tenant is scoped

---

### User Story 4 - Deploy MCP Tools Quickstart (Priority: P2)

A developer deploys quickstart 04 with an MCP server. The model calls tools and produces answers using tool results.

**Acceptance Scenarios**:

1. **Given** 04-mcp-tools is deployed, **When** the user asks "what time is it?", **Then** the model calls get_time via MCP and answers with the current time
2. **Given** the MCP server provides multiple tools, **When** tools are discovered, **Then** they appear in the response's tools array

---

### User Story 5 - Deploy MCP Secured Quickstart (Priority: P3)

A developer deploys quickstart 05 with OAuth-secured MCP. Tool calls are authenticated via Keycloak tokens.

**Acceptance Scenarios**:

1. **Given** 05-mcp-secured is deployed, **When** a tool call is made, **Then** antwort includes a valid OAuth Bearer token in the MCP request

---

### User Story 6 - Deploy RAG Quickstart (Priority: P3)

A developer deploys quickstart 06 with document retrieval. Files are uploaded, embedded, and searchable via the model.

**Acceptance Scenarios**:

1. **Given** 06-rag is deployed, **When** a document is uploaded and the user asks a question about it, **Then** the model retrieves relevant passages and answers with citations

## Requirements

### Functional Requirements

**Structure**

- **FR-001**: All quickstarts MUST be stored in a top-level `quickstarts/` directory
- **FR-002**: Each quickstart MUST be self-contained with its own Kustomize manifests, ConfigMap, and README
- **FR-003**: Each quickstart MUST document its prerequisites (which other quickstarts must be deployed first)
- **FR-004**: The shared LLM backend MUST be a separate reusable component in `quickstarts/shared/llm-backend/`

**Documentation**

- **FR-005**: Each quickstart MUST include a README with: purpose, prerequisites, deploy instructions, test commands with expected output, what's deployed, cleanup instructions
- **FR-006**: Deploy instructions MUST work with a single `kubectl apply -k` command
- **FR-007**: Test commands MUST be copy-pasteable and produce verifiable output

**Kubernetes**

- **FR-008**: All quickstarts MUST use Kustomize (no Helm dependency)
- **FR-009**: Each quickstart MUST include an OpenShift overlay with Route for external access
- **FR-010**: All pods MUST run as non-root with restricted security context where possible
- **FR-011**: Sensitive values (passwords, API keys) MUST be in Kubernetes Secrets, not ConfigMaps

**Progressive Complexity**

- **FR-012**: Quickstarts MUST build progressively: each adds one major capability to the previous
- **FR-013**: A developer MUST be able to start at any quickstart (given prerequisites are met)

### Key Entities

- **Quickstart**: A self-contained, deployable showcase of one or more antwort capabilities.
- **Shared Component**: A reusable infrastructure component (LLM backend) deployed once and shared across quickstarts.

## Success Criteria

- **SC-001**: Each quickstart deploys successfully with a single `kubectl apply -k` command (after prerequisites)
- **SC-002**: Each quickstart's README test commands produce the documented expected output
- **SC-003**: The full series (01 through 06) can be deployed incrementally on a single cluster
- **SC-004**: A developer new to antwort can deploy quickstart 01 and send their first request within 10 minutes

## Assumptions

- A Kubernetes cluster with a GPU node is available (for the shared LLM backend).
- The antwort container image is available at `quay.io/rhuss/antwort:latest`.
- Quickstarts 05 and 06 may require antwort features not yet implemented (token exchange, RAG MCP server). These quickstarts will be implemented after the underlying features are ready.
- The RAG MCP server (quickstart 06) is a separate project, not part of antwort's Go codebase.

## Dependencies

- **Spec 005 (Storage)**: PostgreSQL adapter for quickstart 02.
- **Spec 007 (Auth)**: JWT authentication for quickstart 03.
- **Spec 011 (MCP Client)**: MCP tool execution for quickstart 04.
- **Spec 014 (MCP OAuth)**: OAuth for MCP for quickstart 05.

## Scope Boundaries

### In Scope

- Shared LLM backend manifests (Done)
- 01-Minimal quickstart (Done)
- 02-Production quickstart (PostgreSQL + Prometheus + Grafana)
- 03-Multi-User quickstart (Keycloak + JWT)
- 04-MCP Tools quickstart (MCP test server)
- 05-MCP Secured quickstart (OAuth for MCP)
- 06-RAG quickstart (MinIO + Qdrant + RAG MCP server)
- README for each quickstart
- OpenShift overlays for each quickstart

### Out of Scope

- Helm chart versions of quickstarts
- CI/CD pipeline for quickstart testing
- Production-grade infrastructure (HA, backup, monitoring alerting)
- Cloud-specific quickstarts (AWS, GCP, Azure)
