// Package mcp provides the MCP (Model Context Protocol) client integration
// for the antwort agentic loop. It connects to external MCP servers,
// discovers their tools, and executes tool calls as part of the engine's
// tool execution pipeline.
//
// The package wraps the official MCP Go SDK (github.com/modelcontextprotocol/go-sdk)
// and implements the tools.ToolExecutor interface, allowing MCP server tools
// to be used seamlessly alongside function tools and sandbox tools.
//
// Configuration is provided via ServerConfig structs, which specify the
// server name, transport type (SSE or streamable-http), URL, and optional
// authentication headers.
package mcp
