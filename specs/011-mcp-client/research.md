# Research: MCP Client Integration

**Feature**: 011-mcp-client
**Date**: 2026-02-19

## R1: Go MCP SDK Choice

**Decision**: Evaluate `github.com/mark3labs/mcp-go` as the primary candidate. If it doesn't support HTTP transports well, consider `github.com/modelcontextprotocol/go-sdk` (official, if available).

**Rationale**: Building MCP protocol handling from scratch (JSON-RPC 2.0, handshake, tool discovery) would be 500+ lines of boilerplate. An SDK provides tested protocol compliance.

**Key requirements from the SDK**:
- HTTP+SSE transport support
- Streamable HTTP transport support
- `tools/list` and `tools/call` client methods
- Context propagation for cancellation

## R2: Tool Merging Strategy

**Decision**: The engine's `CreateResponse` merges MCP tools into the request before calling `translateRequest`. The MCPExecutor exposes a `DiscoveredTools()` method that returns cached tools. The engine calls this during request processing.

**Rationale**: Tool merging is an engine concern, not an executor concern. The executor just knows how to execute. The engine knows the full tool set.

## R3: MCP Auth Provider Design

**Decision**: Interface with `GetHeaders(ctx) (map[string]string, error)`. Implementations: `StaticKeyAuth` (returns configured headers), `OAuthClientCredentialsAuth` (token endpoint + caching).

**Rationale**: Headers are the universal auth mechanism for HTTP. The provider returns headers, the client attaches them. Simple, composable.

## R4: Tool Name Namespacing

**Decision**: MCP tools are NOT namespaced. They use their native names from the MCP server. If two servers provide a tool with the same name, the first configured server wins.

**Rationale**: Namespacing (e.g., `server_name.tool_name`) would require the model to know about MCP server names, breaking the abstraction. The model should just see tool names.
