// Package tools defines the tool executor interface and types for the
// antwort agentic loop. It provides the ToolExecutor contract that
// pluggable tool backends implement: function tools (client-executed),
// MCP server tools, and Kubernetes sandbox tools.
//
// The package also provides tool filtering logic for allowed_tools
// enforcement and ToolCall/ToolResult types for executor communication.
//
// This package depends only on pkg/api and has no external dependencies.
package tools
