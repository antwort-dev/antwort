# Spec 10: MCP Client Integration

**Branch**: `spec/10-mcp-client`
**Dependencies**: Spec 04 (Agentic Loop, for ToolExecutor interface)
**Package**: `pkg/tools/mcp`

## Purpose

Implement a Model Context Protocol (MCP) client that connects to MCP servers, discovers available tools, and executes tool calls within the agentic loop. The MCP executor implements the `ToolExecutor` interface from Spec 04, plugging into the agentic loop as a server-side tool execution backend.

## Scope

### In Scope

- MCP client connecting to MCP servers
- Transport abstraction (stdio, HTTP+SSE, streamable HTTP)
- Tool discovery from MCP servers (`tools/list`)
- Tool execution (`tools/call`)
- MCP tool executor implementing `ToolExecutor` from Spec 04
- Server lifecycle management (connection pooling, reconnection, health checks)
- Configuration: static server list from config

### Out of Scope

- MCP server implementation (antwort is a client only)
- Dynamic MCP server discovery (future work)
- MCP resource and prompt capabilities (tools only for now)
- Sandbox execution (Spec 11)

## MCP Client Architecture

```go
// MCPExecutor implements tools.ToolExecutor for MCP-connected tools.
type MCPExecutor struct {
    clients map[string]*MCPClient // server name -> client
}

// MCPClient wraps a connection to a single MCP server.
type MCPClient struct {
    name      string
    transport MCPTransport
    tools     []ToolDefinition // discovered tools, cached
}

// MCPTransport abstracts the MCP connection protocol.
type MCPTransport interface {
    // Initialize performs the MCP handshake.
    Initialize(ctx context.Context) error

    // Call sends a JSON-RPC request and returns the response.
    Call(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error)

    // Close terminates the connection.
    Close() error
}
```

## Transport Implementations

Three MCP transport types:

1. **Stdio**: Subprocess communication via stdin/stdout (for local MCP servers)
2. **HTTP+SSE**: Legacy HTTP transport with SSE for server-to-client messages
3. **Streamable HTTP**: Modern MCP transport using HTTP with streaming

Each transport handles JSON-RPC 2.0 message framing.

## Tool Discovery

On startup (or on-demand), the MCP client calls `tools/list` on each configured server and caches the available tools. These tools are merged into the request's tool list when the user references an MCP server.

```go
// DiscoverTools queries the MCP server for available tools
// and returns them as ToolDefinitions with Kind=ToolKindMCP.
func (c *MCPClient) DiscoverTools(ctx context.Context) ([]ToolDefinition, error)
```

## Configuration

```go
type MCPConfig struct {
    Servers []MCPServerConfig
}

type MCPServerConfig struct {
    Name      string            // Logical name for this server
    Transport string            // "stdio", "sse", "streamable-http"
    Command   string            // For stdio: command to run
    Args      []string          // For stdio: command arguments
    URL       string            // For HTTP transports: server URL
    Headers   map[string]string // For HTTP transports: additional headers
    Env       map[string]string // For stdio: environment variables
}
```

## Security Considerations

- Antwort NEVER executes external code itself. MCP servers are remote HTTP services, not local subprocesses. Stdio transport is explicitly deferred (desktop concern, not server-side).
- HTTP-based MCP servers should use TLS. Authentication is handled via pluggable MCPAuthProvider (see brainstorm/10-mcp-security.md).
- Tool results from MCP servers are untrusted input. The engine should sanitize or validate results before feeding them back to the model.
- MCP security (OAuth, token exchange) is a first-class concern. See brainstorm/10-mcp-security.md for the full design.

## Decisions (Session 2026-02-19)

- **No stdio transport**: Antwort is a server-side framework. Stdio MCP servers are designed for desktop usage and require subprocess execution, which violates "antwort never executes external code." HTTP transports (SSE, streamable HTTP) are the priority.
- **Use Go MCP SDK**: Use `github.com/mark3labs/mcp-go` (or equivalent) as an external dependency in the adapter package (`pkg/tools/mcp/`). Permitted by Constitution Principle II.
- **Lazy tool discovery**: Discover tools on first use, cache results. Resilient to MCP server downtime at startup.
- **Disconnection handling**: Return error result for the tool call. The agentic loop feeds it back to the model per Spec 04 FR-027.
- **No health endpoint coupling**: MCP server status does not affect antwort's readiness probe.
- **Auth via MCPAuthProvider**: Pluggable per-server authentication. P1: API key headers. P2+: OAuth client_credentials, token exchange.

## Open Questions (Resolved)

All open questions resolved in the decisions above.
