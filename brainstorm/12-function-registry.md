# Brainstorm 12: Function Provider Registry

**Dependencies**: Spec 004 (Agentic Loop, ToolExecutor interface), Spec 007 (Auth), Spec 013 (Observability)
**Package**: `pkg/tools/registry/`

## Purpose

Define a pluggable framework for built-in hosted tools. A `FunctionProvider` registers tools the model can call AND HTTP endpoints for management APIs. A `FunctionRegistry` manages all providers and implements `ToolExecutor` so it plugs directly into the agentic loop.

This is different from MCP tools (external servers) and function tools (client-executed). Built-in providers run in-process, have direct access to antwort's storage and config, and expose management APIs on the same HTTP server.

## Architecture

```
FunctionProvider interface:
├── Name() string
├── Tools() []ToolDefinition        (what the model sees)
├── CanExecute(name string) bool    (can handle this tool?)
├── Execute(ctx, ToolCall) *ToolResult (run the tool)
├── Routes() []Route                (management API endpoints)
├── Collectors() []prometheus.Collector (custom metrics)
└── Close() error

FunctionRegistry:
├── Register(provider)
├── Implements ToolExecutor          (plugs into engine)
├── HTTPHandler() http.Handler       (combined management APIs, wrapped with auth + metrics)
├── DiscoveredTools() []ToolDefinition
└── MetricsCollectors() []prometheus.Collector (aggregated from all providers)
```

## How it fits

```
Engine executors:
├── FunctionRegistry (built-in tools)
│   ├── WebSearchProvider
│   ├── FileSearchProvider
│   └── CodeInterpreterProvider (future, via sandbox)
├── MCPExecutor (external MCP servers)
└── (function tools → requires_action)
```

The engine doesn't know the difference between a built-in provider and an MCP tool. Both implement `ToolExecutor`. The registry just manages multiple providers under one executor.

## Infrastructure Integration

### Metrics (automatic + provider-custom)

Every provider gets automatic metrics from the registry for free:

```
# Automatic (recorded by the registry wrapper, not the provider)
antwort_builtin_tool_executions_total{provider, tool_name, status}
antwort_builtin_tool_duration_seconds{provider, tool_name}
antwort_builtin_api_requests_total{provider, method, path, status}
antwort_builtin_api_duration_seconds{provider, method, path}
```

Providers can also register their own custom metrics via `Collectors()`:

```
# WebSearchProvider custom metrics
antwort_websearch_queries_total{backend, status}
antwort_websearch_results_returned{backend}
antwort_websearch_backend_latency_seconds{backend}

# FileSearchProvider custom metrics
antwort_filesearch_documents_total{vector_store_id}
antwort_filesearch_chunks_total{vector_store_id}
antwort_filesearch_embedding_duration_seconds{}
antwort_filesearch_vector_search_duration_seconds{vector_store_id}
```

The registry collects all provider metrics and registers them with the Prometheus registry. They appear at `/metrics` alongside antwort's core metrics automatically.

### Auth (automatic)

The registry wraps each provider's management API routes with the existing auth middleware (Spec 007). Providers don't implement auth themselves. The registry does it:

```
Provider routes:
  POST /v1/vector_stores → FileSearchProvider.HandleCreateStore

Registry wraps as:
  POST /v1/vector_stores → AuthMiddleware → TenantInjection → MetricsMiddleware → FileSearchProvider.HandleCreateStore
```

This means:
- All management APIs are automatically protected by the configured auth (API key, JWT)
- Tenant isolation applies to provider data (vector stores scoped per tenant)
- Request metrics are recorded for all management API calls
- Providers receive a context with the authenticated identity and tenant

### Tenant Scoping

Providers that store data (FileSearchProvider) receive the tenant from context (via `storage.GetTenant(ctx)`) and scope their data accordingly. The registry doesn't enforce this, the provider must respect the tenant. But the infrastructure delivers the tenant transparently.

## Configuration

```yaml
builtins:
  web_search:
    enabled: true
    search_engine_url: http://searxng:8080
  file_search:
    enabled: true
    vector_db: pgvector
    embedding_endpoint: http://embedding-service:8080
  code_interpreter:
    enabled: false  # requires sandbox (Spec 11)
```

## Package Structure

```
pkg/tools/
├── registry/
│   ├── registry.go       # FunctionRegistry + FunctionProvider interface
│   ├── middleware.go      # Auth + metrics wrapping for provider routes
│   └── registry_test.go
├── builtins/
│   ├── websearch/        # WebSearchProvider
│   └── filesearch/       # FileSearchProvider
```

## Interface Detail

```go
type FunctionProvider interface {
    // Name returns the provider's identifier (e.g., "web_search", "file_search").
    Name() string

    // Tools returns the tool definitions this provider offers to the model.
    Tools() []api.ToolDefinition

    // CanExecute returns true if this provider handles the given tool name.
    CanExecute(name string) bool

    // Execute runs the tool and returns the result.
    Execute(ctx context.Context, call tools.ToolCall) (*tools.ToolResult, error)

    // Routes returns HTTP routes for management APIs.
    // These are wrapped with auth and metrics middleware by the registry.
    // Return nil if the provider has no management API.
    Routes() []Route

    // Collectors returns Prometheus metric collectors for this provider.
    // These are registered with the global Prometheus registry.
    // Return nil if the provider has no custom metrics.
    Collectors() []prometheus.Collector

    // Close releases resources.
    Close() error
}

type Route struct {
    Method  string           // "GET", "POST", "DELETE"
    Pattern string           // "/v1/vector_stores", "/v1/vector_stores/{id}"
    Handler http.HandlerFunc
}
```

## Open Questions (Resolved)

- Should the registry be a separate ToolExecutor or embedded in the existing executor dispatch?
  -> Separate. It implements ToolExecutor, registered alongside MCPExecutor.
- Should built-in tools be auto-added to every request or only when enabled?
  -> Only when enabled in config. The engine merges them like MCP tools.
- How do providers get auth and metrics?
  -> The registry wraps provider routes with auth + metrics middleware automatically. Providers also expose custom Prometheus collectors.

## Deliverables

- [ ] FunctionProvider interface (with Collectors() for custom metrics)
- [ ] FunctionRegistry implementing ToolExecutor
- [ ] Middleware wrapper for provider routes (auth + metrics + tenant injection)
- [ ] Automatic execution metrics (per-provider, per-tool)
- [ ] Custom metrics aggregation from providers
- [ ] HTTP handler merging for management APIs
- [ ] Config integration (enable/disable per provider)
