package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/tools"
)

// MCPExecutor implements tools.ToolExecutor for MCP server tools.
// It manages connections to multiple MCP servers, discovers their tools,
// and routes tool calls to the appropriate server.
type MCPExecutor struct {
	mu sync.RWMutex

	// clients maps server name to MCPClient.
	clients map[string]*MCPClient

	// toolToServer maps tool name to the server name that provides it.
	toolToServer map[string]string

	// discovered tracks whether tools have been discovered.
	discovered bool
}

// Ensure MCPExecutor implements tools.ToolExecutor at compile time.
var _ tools.ToolExecutor = (*MCPExecutor)(nil)

// NewMCPExecutor creates a new MCPExecutor with the given MCP clients.
func NewMCPExecutor(clients map[string]*MCPClient) *MCPExecutor {
	return &MCPExecutor{
		clients:      clients,
		toolToServer: make(map[string]string),
	}
}

// Kind returns ToolKindMCP.
func (e *MCPExecutor) Kind() tools.ToolKind {
	return tools.ToolKindMCP
}

// CanExecute returns true if any connected MCP server provides the named tool.
// On the first call, it triggers lazy tool discovery.
func (e *MCPExecutor) CanExecute(toolName string) bool {
	e.ensureDiscovered()

	e.mu.RLock()
	defer e.mu.RUnlock()
	_, ok := e.toolToServer[toolName]
	return ok
}

// Execute routes the tool call to the correct MCP server and returns the result.
func (e *MCPExecutor) Execute(ctx context.Context, call tools.ToolCall) (*tools.ToolResult, error) {
	e.ensureDiscovered()

	e.mu.RLock()
	serverName, ok := e.toolToServer[call.Name]
	if !ok {
		e.mu.RUnlock()
		return &tools.ToolResult{
			CallID:  call.ID,
			Output:  fmt.Sprintf("no MCP server provides tool %q", call.Name),
			IsError: true,
		}, nil
	}
	client := e.clients[serverName]
	e.mu.RUnlock()

	return client.CallTool(ctx, call)
}

// DiscoveredTools returns all tools discovered from connected MCP servers.
// This is useful for the engine to merge MCP tools into the request's
// tool definitions.
func (e *MCPExecutor) DiscoveredTools() []api.ToolDefinition {
	e.ensureDiscovered()

	e.mu.RLock()
	defer e.mu.RUnlock()

	var allTools []api.ToolDefinition
	for _, client := range e.clients {
		client.mu.Lock()
		allTools = append(allTools, client.cachedTools...)
		client.mu.Unlock()
	}
	return allTools
}

// Close closes all MCP client connections.
func (e *MCPExecutor) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	var lastErr error
	for name, client := range e.clients {
		if err := client.Close(); err != nil {
			slog.Warn("failed to close MCP client", "server", name, "error", err)
			lastErr = err
		}
	}
	return lastErr
}

// ensureDiscovered triggers tool discovery if it hasn't been done yet.
func (e *MCPExecutor) ensureDiscovered() {
	e.mu.RLock()
	if e.discovered {
		e.mu.RUnlock()
		return
	}
	e.mu.RUnlock()

	e.mu.Lock()
	defer e.mu.Unlock()

	// Double-check after acquiring write lock.
	if e.discovered {
		return
	}

	ctx := context.Background()
	for name, client := range e.clients {
		toolDefs, err := client.DiscoverTools(ctx)
		if err != nil {
			slog.Error("failed to discover tools from MCP server",
				"server", name,
				"error", err,
			)
			continue
		}

		for _, td := range toolDefs {
			if _, exists := e.toolToServer[td.Name]; exists {
				slog.Warn("duplicate MCP tool name, using first provider",
					"tool", td.Name,
					"server", name,
				)
				continue
			}
			e.toolToServer[td.Name] = name
		}

		slog.Info("discovered MCP tools",
			"server", name,
			"count", len(toolDefs),
		)
	}

	e.discovered = true
}
