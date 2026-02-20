# Feature Specification: MCP Client Integration

**Feature Branch**: `011-mcp-client`
**Created**: 2026-02-19
**Status**: Draft

## Overview

This specification adds Model Context Protocol (MCP) client support to antwort, enabling the agentic loop to connect to remote MCP servers, discover their tools, and execute tool calls server-side. The MCP executor implements the `ToolExecutor` interface from Spec 004, plugging directly into the existing agentic loop infrastructure.

MCP transforms antwort from an inference proxy into a true agentic gateway: the model decides which tools to call, antwort connects to MCP servers and executes the tools, feeds results back to the model, and loops until a final answer is produced. All tool execution happens on remote servers; antwort never executes external code itself.

Authentication to MCP servers is pluggable per server, supporting API keys (P1) and OAuth client credentials (P2).

## Clarifications

### Session 2026-02-19

- Q: Stdio or HTTP transports? -> A: HTTP only. Antwort is a server-side framework. Stdio MCP servers are designed for desktop usage and require subprocess execution, which violates "antwort never executes external code." HTTP transports (SSE, streamable HTTP) are the priority.
- Q: Build or use MCP SDK? -> A: Use a Go MCP SDK as an external dependency in the adapter package. Permitted by Constitution Principle II.
- Q: Eager or lazy tool discovery? -> A: Lazy (on first use, then cached). Resilient to MCP server downtime at startup.
- Q: MCP server disconnection during agentic loop? -> A: Return error result for the tool call. The agentic loop feeds it back to the model (Spec 04 FR-027).
- Q: MCP server health in antwort's readiness probe? -> A: No. MCP servers are optional. A downed MCP server doesn't make antwort unready.
- Q: MCP server authentication? -> A: Pluggable per server via MCPAuthProvider. P1: static API key headers. P2: OAuth client credentials. P3: token exchange (future).

## User Scenarios & Testing

### User Story 1 - Connect to MCP Server and Discover Tools (Priority: P1)

An operator configures antwort with one or more remote MCP servers. On the first request that could use tools, antwort connects to each configured server, performs the MCP handshake, and discovers available tools via `tools/list`. The discovered tools are cached and made available to the model alongside any explicitly defined tools in the request.

**Why this priority**: Without tool discovery, MCP tools can't be offered to the model.

**Independent Test**: Configure an MCP server URL, send a request, verify that the server's tools appear in the model's tool list.

**Acceptance Scenarios**:

1. **Given** an MCP server configured with URL and name, **When** the first request arrives, **Then** antwort connects, handshakes, and discovers tools
2. **Given** discovered tools from an MCP server, **When** a request is sent without explicit tools, **Then** the MCP tools are included in the provider request
3. **Given** a configured MCP server that is unreachable, **When** tool discovery is attempted, **Then** an error is logged and the request proceeds without MCP tools (graceful degradation)

---

### User Story 2 - Execute MCP Tool Calls in the Agentic Loop (Priority: P1)

A developer sends a request that triggers the model to call an MCP-provided tool. The MCP executor handles the tool call by sending a `tools/call` request to the appropriate MCP server, collects the result, and feeds it back to the model via the agentic loop. The model uses the result to produce a final answer.

**Why this priority**: This is the core MCP value proposition. Without execution, tool discovery is useless.

**Independent Test**: Configure an MCP server with a "get_weather" tool. Send a request asking about the weather. Verify the model calls the tool, antwort executes it via MCP, and the model produces an answer using the result.

**Acceptance Scenarios**:

1. **Given** an MCP server with a tool, **When** the model calls that tool, **Then** the MCP executor sends `tools/call` to the server and returns the result
2. **Given** multiple MCP servers, **When** the model calls tools from different servers, **Then** each call is routed to the correct server
3. **Given** an MCP tool call that returns an error, **When** the result is fed back, **Then** the model receives the error and can decide how to proceed
4. **Given** multiple tool calls in one turn, **When** some are MCP and some are function (client-executed), **Then** the turn pauses with `requires_action` (per Spec 004 mixed tool kind rule)

---

### User Story 3 - MCP Server Authentication with API Keys (Priority: P1)

An operator configures an MCP server with an API key for authentication. All requests from antwort to that MCP server include the API key in the configured header. Credentials are stored in Kubernetes Secrets, not in plain-text configuration.

**Why this priority**: Most production MCP servers require authentication.

**Acceptance Scenarios**:

1. **Given** an MCP server configured with an API key header, **When** antwort connects, **Then** the API key is included in all requests to that server
2. **Given** an MCP server with invalid credentials, **When** antwort attempts to connect, **Then** the connection fails with a clear error

---

### User Story 4 - Multiple MCP Servers (Priority: P2)

An operator configures multiple MCP servers, each providing different tools. Tools from all servers are merged and presented to the model. When the model calls a tool, antwort routes the call to the correct server based on which server provides that tool.

**Acceptance Scenarios**:

1. **Given** two MCP servers (A with tool "search", B with tool "calculate"), **When** the model calls "search", **Then** the call goes to server A
2. **Given** two servers with the same tool name, **When** the model calls it, **Then** the first configured server's tool takes precedence (deterministic)

---

### User Story 5 - OAuth Client Credentials for MCP Servers (Priority: P2)

An operator configures an MCP server with OAuth client credentials authentication. Antwort obtains an access token from the OAuth token endpoint, caches it, and refreshes before expiry. The access token is sent as a Bearer token in MCP requests.

**Acceptance Scenarios**:

1. **Given** OAuth client credentials configured for an MCP server, **When** antwort connects, **Then** it obtains and caches an access token
2. **Given** a cached token near expiry, **When** the next MCP request is made, **Then** the token is refreshed before the request

---

### Edge Cases

- What happens when an MCP server disconnects mid-tool-call? The executor returns an error result. The agentic loop feeds it back to the model.
- What happens when tool discovery returns no tools? The server is logged as having no tools. No error.
- What happens when the MCP server returns malformed JSON in a tool result? The executor wraps it as an error result with the raw response for debugging.
- What happens when the model calls a tool that was discovered but the MCP server has since removed it? The server returns an error, which is fed back to the model.

## Requirements

### Functional Requirements

**MCP Client**

- **FR-001**: The system MUST provide an MCP client that connects to remote MCP servers via HTTP-based transports (SSE, streamable HTTP)
- **FR-002**: The MCP client MUST perform the MCP protocol handshake (initialize) on first connection
- **FR-003**: The MCP client MUST discover tools via the `tools/list` method and cache the results
- **FR-004**: The MCP client MUST execute tool calls via the `tools/call` method and return structured results

**MCP Executor (ToolExecutor)**

- **FR-005**: The system MUST provide an MCP executor that implements the `ToolExecutor` interface from Spec 004
- **FR-006**: The MCP executor MUST return `ToolKindMCP` from `Kind()`
- **FR-007**: The MCP executor MUST return `true` from `CanExecute()` for tools discovered from configured MCP servers
- **FR-008**: The MCP executor MUST route `Execute()` calls to the correct MCP server based on which server provides the requested tool

**Tool Merging**

- **FR-009**: When MCP servers are configured, their discovered tools MUST be merged with any tools explicitly defined in the request before sending to the provider
- **FR-010**: If a tool name exists in both the request and an MCP server, the request's definition takes precedence (explicit overrides discovered)

**Authentication**

- **FR-011**: Each MCP server MUST support independent authentication configuration
- **FR-012**: The system MUST support static API key authentication (key in header) for MCP servers
- **FR-013**: The system MUST support OAuth 2.0 client credentials authentication for MCP servers
- **FR-014**: OAuth tokens MUST be cached and refreshed before expiry

**Configuration**

- **FR-015**: MCP servers MUST be configurable via the server binary (environment variables or config)
- **FR-016**: Each server configuration MUST include: name, transport type, URL, and optional authentication

**Error Handling**

- **FR-017**: MCP connection failures MUST NOT prevent antwort from starting or serving requests (graceful degradation)
- **FR-018**: MCP tool execution errors MUST be returned as `ToolResult` with `IsError: true`, fed back to the model via the agentic loop
- **FR-019**: The MCP executor MUST log connection events, tool discovery, and execution errors with structured fields

**Security**

- **FR-020**: Antwort MUST NOT execute any external code itself. All tool execution is delegated to remote MCP servers via HTTP.
- **FR-021**: MCP server credentials MUST be stored securely (Kubernetes Secrets), not in plain-text configuration

### Key Entities

- **MCPExecutor**: A `ToolExecutor` implementation that routes tool calls to MCP servers. Manages multiple server connections.
- **MCPClient**: A connection to a single MCP server. Handles handshake, tool discovery, and tool execution.
- **MCPAuthProvider**: Pluggable per-server authentication. Provides credentials (headers) for MCP requests.

## Success Criteria

### Measurable Outcomes

- **SC-001**: An agentic loop with MCP tools completes a multi-turn conversation: model calls MCP tool, antwort executes via MCP server, result fed back, model produces final answer
- **SC-002**: Tools from multiple MCP servers are correctly merged and routed
- **SC-003**: MCP server authentication (API key) works end-to-end
- **SC-004**: MCP server failures degrade gracefully without affecting non-MCP requests
- **SC-005**: The MCP executor integrates with the existing agentic loop without modifications to the loop itself (clean ToolExecutor interface)

## Assumptions

- A Go MCP SDK is used as an external dependency in the adapter package (`pkg/tools/mcp/`). Permitted by Constitution Principle II.
- Only HTTP-based MCP transports are supported. Stdio is explicitly out of scope (desktop concern, not server-side).
- Tool discovery is lazy (on first use) with caching. No eager startup discovery.
- The MCP protocol version supported matches the current MCP specification (2024-11-05 or later).
- OAuth client credentials (P2) can be deferred to a follow-up iteration if the SDK integration is complex.

## Dependencies

- **Spec 004 (Agentic Loop)**: The `ToolExecutor` interface that the MCP executor implements.
- **Spec 007 (Auth)**: The auth framework patterns (pluggable providers, credential management).
- **Go MCP SDK**: External dependency for protocol handling.

## Scope Boundaries

### In Scope

- MCP client with HTTP transports (SSE, streamable HTTP)
- Tool discovery (`tools/list`) with caching
- Tool execution (`tools/call`) via ToolExecutor interface
- Multi-server support with tool routing
- Per-server authentication (API key, OAuth client credentials)
- Server binary integration (MCP server configuration)
- Structured error handling and logging

### Out of Scope

- Stdio MCP transport (desktop concern, not server-side)
- MCP server implementation (antwort is client only)
- Dynamic MCP server discovery at runtime
- MCP resource and prompt capabilities (tools only)
- Token exchange / user context propagation (P3, future)
- Sandbox execution (Spec 11)
