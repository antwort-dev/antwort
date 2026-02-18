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

- MCP servers running via stdio execute as subprocesses of antwort. In a Kubernetes deployment, these run inside the antwort pod with the same security context.
- HTTP-based MCP servers should use TLS. Integration with the cluster's SPIFFE/SPIRE identity (from Spec 11) for mTLS is desirable.
- Tool results from MCP servers are untrusted input. The engine should sanitize or validate results before feeding them back to the model.

## Open Questions

- Should MCP tool discovery happen at startup (eager) or on first use (lazy)?
- How to handle MCP server disconnections during an agentic loop turn?
- Should antwort expose MCP server health status via its own health endpoint?
- How to handle MCP servers that require authentication (API keys, OAuth)?
