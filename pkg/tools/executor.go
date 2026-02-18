package tools

import (
	"context"
)

// ToolKind classifies how a tool is hosted and executed.
type ToolKind int

const (
	// ToolKindFunction is a client-executed tool. The model produces
	// function_call items, and execution is delegated back to the client
	// via the requires_action response status.
	ToolKindFunction ToolKind = iota

	// ToolKindMCP is a tool connected via the Model Context Protocol.
	// The engine connects to the MCP server and executes the tool
	// server-side within the agentic loop.
	ToolKindMCP

	// ToolKindSandbox is a tool executed in an isolated Kubernetes
	// sandbox pod. The engine delegates execution to a pod from the
	// sandbox pool and collects the result.
	ToolKindSandbox
)

// ToolExecutor executes tool calls. Implementations exist for each
// ToolKind: function (returns to client), MCP (calls MCP server),
// sandbox (delegates to k8s pod).
type ToolExecutor interface {
	// Kind returns the type of tools this executor handles.
	Kind() ToolKind

	// CanExecute checks if this executor can handle the given tool name.
	CanExecute(toolName string) bool

	// Execute runs the tool and returns the result.
	// For function tools, this returns ErrRequiresClientAction to signal
	// that execution must be delegated to the client.
	Execute(ctx context.Context, call ToolCall) (*ToolResult, error)
}

// ToolCall represents a model's request to invoke a tool.
type ToolCall struct {
	// ID is the unique call identifier (from the model, e.g., "call_abc123").
	ID string

	// Name is the tool function name.
	Name string

	// Arguments is the JSON-encoded arguments string.
	Arguments string
}

// ToolResult represents the output of a tool execution.
type ToolResult struct {
	// CallID matches the originating ToolCall.ID.
	CallID string

	// Output is the tool output content (text).
	Output string

	// IsError indicates that the output is an error message.
	IsError bool
}
