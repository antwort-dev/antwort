# Brainstorm 12: Function Provider Registry

**Dependencies**: Spec 004 (Agentic Loop, ToolExecutor interface)
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
└── Close() error

FunctionRegistry:
├── Register(provider)
├── Implements ToolExecutor          (plugs into engine)
├── HTTPHandler() http.Handler       (combined management APIs)
└── DiscoveredTools() []ToolDefinition
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

## Configuration

```yaml
builtins:
  web_search:
    enabled: true
    search_engine_url: http://searxng:8080
  file_search:
    enabled: true
    vector_db: pgvector  # or qdrant
    embedding_endpoint: http://embedding-service:8080
  code_interpreter:
    enabled: false  # requires sandbox (Spec 11)
```

## Package Structure

```
pkg/tools/
├── registry/
│   ├── registry.go       # FunctionRegistry + FunctionProvider interface
│   └── registry_test.go
├── builtins/
│   ├── websearch/        # WebSearchProvider
│   └── filesearch/       # FileSearchProvider
```

## Open Questions

- Should the registry be a separate ToolExecutor or embedded in the existing executor dispatch?
  -> Separate. It implements ToolExecutor, registered alongside MCPExecutor.
- Should built-in tools be auto-added to every request or only when enabled?
  -> Only when enabled in config. The engine merges them like MCP tools.

## Deliverables

- [ ] FunctionProvider interface
- [ ] FunctionRegistry implementing ToolExecutor
- [ ] HTTP handler merging for management APIs
- [ ] Config integration (enable/disable per provider)
