# Implementation Plan: MCP Client Integration

**Branch**: `011-mcp-client` | **Date**: 2026-02-19 | **Spec**: [spec.md](spec.md)

## Summary

Implement an MCP client that connects to remote MCP servers via HTTP transports, discovers tools, and executes tool calls within the agentic loop. Uses a Go MCP SDK as an external dependency. The MCP executor implements `ToolExecutor` from Spec 004.

## Technical Context

**Language/Version**: Go 1.25+
**Primary Dependencies**: Go MCP SDK (e.g., `github.com/mark3labs/mcp-go` or similar). External dep in adapter package.
**Testing**: `go test` with mock MCP server (httptest). Integration test with a real MCP server (optional).
**Constraints**: HTTP transports only. No stdio. No external code execution.

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Interface-First | PASS | Implements ToolExecutor (3 methods) |
| II. Zero Dependencies | PASS | MCP SDK in `pkg/tools/mcp/` only |
| III. Nil-Safe | PASS | No MCP config = no MCP tools |
| IX. Kubernetes-Native | PASS | No subprocess execution, Secrets for credentials |

## Project Structure

```text
pkg/
└── tools/
    └── mcp/
        ├── executor.go        # MCPExecutor (implements ToolExecutor)
        ├── client.go          # MCPClient (single server connection)
        ├── auth.go            # MCPAuthProvider interface + API key impl
        ├── config.go          # MCPConfig, MCPServerConfig
        ├── executor_test.go   # Executor tests with mock MCP server
        └── auth_test.go       # Auth provider tests

cmd/
└── server/
    └── main.go               # MODIFIED: MCP server configuration wiring
```
