# Feature Specification: Function Provider Registry

**Feature Branch**: `016-function-registry`
**Created**: 2026-02-20
**Status**: Draft

## Overview

This specification defines a pluggable framework for built-in hosted tools in antwort. A `FunctionProvider` registers tools the model can call, executes them in-process, and optionally exposes HTTP management endpoints (e.g., `/v1/vector_stores` for file search). A `FunctionRegistry` manages all providers, implements `ToolExecutor` for seamless integration with the agentic loop, and wraps provider routes with existing auth and metrics infrastructure.

Built-in providers are different from MCP tools (external servers) and function tools (client-executed). They run in-process, benefit from antwort's auth, metrics, and tenant scoping automatically, and can expose management APIs on the same HTTP server.

This is the framework only. Concrete providers (web search, file search, code interpreter) are separate specs that implement the `FunctionProvider` interface.

## Clarifications

### Session 2026-02-20

- Q: How do providers get auth protection? -> A: The registry wraps provider management API routes with the existing auth middleware automatically. Providers don't implement auth.
- Q: How do providers export metrics? -> A: Two levels. The registry records automatic execution metrics (tool count, duration) for every provider. Providers can also register custom Prometheus collectors via `Collectors()` that appear at `/metrics`.
- Q: How does tenant scoping work? -> A: The auth middleware injects tenant into context. Providers that store data read `storage.GetTenant(ctx)` and scope accordingly. The registry delivers the tenant, providers respect it.
- Q: Where do provider routes go? -> A: The registry merges all provider routes into a single `http.Handler` that the server mounts. Auth + metrics middleware wraps each route.
- Q: How is provider configuration structured? -> A: A top-level `providers` section in config.yaml with a map keyed by provider type. Each value is provider-specific (schema defined by the provider). The registry reads the map, finds the factory for each type, passes the config to the factory. Unknown types produce a startup error.

### Session 2026-02-26

- Q: How do OpenResponses built-in tool types work with Chat Completions backends? -> A: The OpenResponses API allows `{"type": "code_interpreter"}` in the tools array, but Chat Completions backends only understand `{"type": "function"}`. The engine's `expandBuiltinTools()` replaces these stubs with the full function definition from the matching FunctionProvider before translating the request. Added as FR-007a.

## User Scenarios & Testing

### User Story 1 - Register a Built-In Provider (Priority: P1)

A developer creates a FunctionProvider implementation (e.g., web search) and registers it with the FunctionRegistry. The provider's tools are automatically discovered and offered to the model. Tool execution flows through the agentic loop.

**Acceptance Scenarios**:

1. **Given** a provider registered with the registry, **When** a request arrives, **Then** the provider's tools appear in the model's tool list
2. **Given** the model calls a built-in tool, **When** the registry dispatches the call, **Then** the correct provider executes it and returns a result
3. **Given** a disabled provider (config), **When** a request arrives, **Then** the provider's tools do not appear

---

### User Story 2 - Management API with Auth and Metrics (Priority: P1)

A developer creates a provider with management API routes (e.g., `/v1/vector_stores`). The registry mounts these routes on the server with auth and metrics middleware applied automatically.

**Acceptance Scenarios**:

1. **Given** a provider with routes, **When** a request hits a provider endpoint, **Then** it passes through auth middleware (401 without valid credentials)
2. **Given** a provider endpoint call, **When** metrics are scraped, **Then** `antwort_builtin_api_requests_total` records the call with provider, method, path, and status labels
3. **Given** a multi-tenant setup, **When** a provider endpoint is called, **Then** the provider receives the tenant from context

---

### User Story 3 - Custom Provider Metrics (Priority: P2)

A provider registers custom Prometheus metrics (e.g., web search query count, vector search latency). These appear at `/metrics` alongside antwort's core metrics.

**Acceptance Scenarios**:

1. **Given** a provider with custom collectors, **When** registered, **Then** its metrics appear at `/metrics`
2. **Given** the provider records custom metrics during execution, **When** Prometheus scrapes, **Then** the custom metrics have correct values

---

### Edge Cases

- What happens when two providers register the same tool name? The first registered provider wins (deterministic). A warning is logged.
- What happens when a provider's `Execute` panics? The registry recovers the panic and returns an error result to the model.
- What happens when a provider is disabled after registration? Its tools are removed from the discovered set. Ongoing calls complete. No new calls are dispatched.

## Requirements

### Functional Requirements

**FunctionProvider Interface**

- **FR-001**: The system MUST define a `FunctionProvider` interface with methods for: name, tool definitions, executability check, execution, management routes, custom metrics collectors, and close
- **FR-002**: Providers MUST be able to return zero or more tool definitions
- **FR-003**: Providers MUST be able to return zero or more HTTP routes for management APIs
- **FR-004**: Providers MUST be able to return zero or more custom metric collectors

**FunctionRegistry**

- **FR-005**: The registry MUST implement the `ToolExecutor` interface from Spec 004
- **FR-006**: The registry MUST merge tools from all registered providers into a unified tool set
- **FR-007**: The registry MUST route tool execution calls to the correct provider based on `CanExecute`
- **FR-007a**: The engine MUST expand OpenResponses built-in tool type stubs (e.g., `{"type": "code_interpreter"}`, `{"type": "file_search"}`, `{"type": "web_search_preview"}`) to full function definitions from the matching registered provider before translating to the provider request. Chat Completions backends only understand `{"type": "function"}` tools.
- **FR-008**: The registry MUST provide an `http.Handler` that serves all provider management API routes

**Infrastructure Integration (Automatic)**

- **FR-009**: The registry MUST wrap provider management API routes with the existing auth middleware (Spec 007)
- **FR-010**: The registry MUST record automatic metrics for every tool execution: `antwort_builtin_tool_executions_total{provider, tool_name, status}` and `antwort_builtin_tool_duration_seconds{provider, tool_name}`
- **FR-011**: The registry MUST record automatic metrics for every management API call: `antwort_builtin_api_requests_total{provider, method, path, status}` and `antwort_builtin_api_duration_seconds{provider, method, path}`
- **FR-012**: The registry MUST register all provider custom metric collectors with the metrics system so they appear at `/metrics`
- **FR-013**: The registry MUST propagate tenant identity from auth context to provider route handlers

**Configuration**

- **FR-014**: The config system MUST include a top-level `providers` section containing a map keyed by provider type name (e.g., `web_search`, `file_search`). Each key maps to a provider-specific configuration schema.
- **FR-015**: Each provider entry MUST support an `enabled` field (default: false). Disabled providers MUST NOT have their tools offered to the model or their routes mounted.
- **FR-016**: The provider-specific config schema MUST be defined by the provider itself. The registry passes the raw config (as a generic map or typed struct) to the provider's constructor.
- **FR-017**: The registry MUST validate that each configured provider type has a registered factory. Unknown provider types MUST produce a startup error.

Example config structure:
```yaml
providers:
  web_search:
    enabled: true
    backend: searxng
    url: http://searxng:8080
    max_results: 5
  file_search:
    enabled: true
    vector_db: pgvector
    embedding_url: http://llm:8080/v1/embeddings
    chunk_size: 512
  code_interpreter:
    enabled: false
```

**Error Handling**

- **FR-018**: The registry MUST recover from provider execution panics and return an error result
- **FR-019**: Tool name conflicts across providers MUST be resolved deterministically (first registered wins) with a warning logged

### Key Entities

- **FunctionProvider**: A pluggable component that registers tools, executes them, and optionally exposes management APIs.
- **FunctionRegistry**: Manages providers, implements ToolExecutor, merges routes, handles infrastructure integration.
- **Route**: An HTTP method + pattern + handler exposed by a provider.

## Success Criteria

- **SC-001**: A registered provider's tools are automatically offered to the model and executable via the agentic loop
- **SC-002**: Provider management API routes are protected by auth and recorded in metrics without provider-side code
- **SC-003**: Custom provider metrics appear at `/metrics` alongside core antwort metrics
- **SC-004**: The registry integrates as a standard ToolExecutor (the engine doesn't know about built-in vs MCP)

## Assumptions

- The FunctionProvider interface is framework-only. No concrete providers are shipped in this spec.
- Providers are registered at startup. Dynamic provider loading at runtime is a future enhancement.
- The registry is a single ToolExecutor instance (not one per provider). It multiplexes internally.
- Provider management APIs share the same HTTP server and port as the OpenResponses API.

## Dependencies

- **Spec 004 (Agentic Loop)**: ToolExecutor interface that the registry implements.
- **Spec 007 (Auth)**: Auth middleware for wrapping provider routes.
- **Spec 013 (Observability)**: Prometheus metrics for automatic and custom metrics.
- **Spec 012 (Configuration)**: Provider enable/disable config.

## Scope Boundaries

### In Scope

- FunctionProvider interface definition
- FunctionRegistry implementing ToolExecutor
- Auth middleware wrapping for provider routes
- Automatic execution and API metrics
- Custom metric collector registration
- Tenant context propagation
- Configuration integration
- Tool name conflict resolution
- Panic recovery

### Out of Scope

- Concrete provider implementations (web search, file search, code interpreter)
- Dynamic provider loading
- Provider-to-provider communication
- Provider sandboxing (providers run in-process)
