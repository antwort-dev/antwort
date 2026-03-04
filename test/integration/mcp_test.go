package integration

import (
	"context"
	"encoding/json"
	"testing"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rhuss/antwort/pkg/tools"
	"github.com/rhuss/antwort/pkg/tools/mcp"
)

// setupMCPTestServer creates an in-memory MCP server with the given tools
// and returns a connected MCPClient. This avoids starting an external process.
func setupMCPTestServer(t *testing.T, serverTools map[string]gomcp.ToolHandler) *mcp.MCPClient {
	t.Helper()

	server := gomcp.NewServer(
		&gomcp.Implementation{Name: "integration-test-server", Version: "1.0.0"},
		nil,
	)

	for name, handler := range serverTools {
		server.AddTool(
			&gomcp.Tool{
				Name:        name,
				Description: "Test tool: " + name,
				InputSchema: map[string]any{"type": "object"},
			},
			handler,
		)
	}

	serverTransport, clientTransport := gomcp.NewInMemoryTransports()

	ctx := context.Background()
	go func() {
		_ = server.Run(ctx, serverTransport)
	}()

	client := mcp.NewMCPClient(mcp.ServerConfig{Name: "integration-test"})
	if err := client.ConnectWithTransport(ctx, clientTransport); err != nil {
		t.Fatalf("connecting MCP client: %v", err)
	}

	t.Cleanup(func() {
		_ = client.Close()
	})

	return client
}

// TestMCPToolDiscovery verifies that tools are discovered from a connected
// MCP server and made available through the executor.
func TestMCPToolDiscovery(t *testing.T) {
	client := setupMCPTestServer(t, map[string]gomcp.ToolHandler{
		"get_time": func(ctx context.Context, req *gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
			return &gomcp.CallToolResult{
				Content: []gomcp.Content{&gomcp.TextContent{Text: "12:00 UTC"}},
			}, nil
		},
		"echo": func(ctx context.Context, req *gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
			return &gomcp.CallToolResult{
				Content: []gomcp.Content{&gomcp.TextContent{Text: "echo result"}},
			}, nil
		},
	})

	executor := mcp.NewMCPExecutor(map[string]*mcp.MCPClient{
		"integration-test": client,
	})
	defer executor.Close()

	discovered := executor.DiscoveredTools()
	if len(discovered) != 2 {
		t.Fatalf("expected 2 discovered tools, got %d", len(discovered))
	}

	names := map[string]bool{}
	for _, td := range discovered {
		names[td.Name] = true
		if td.Type != "function" {
			t.Errorf("tool %q has type %q, want 'function'", td.Name, td.Type)
		}
	}

	if !names["get_time"] {
		t.Error("expected 'get_time' tool to be discovered")
	}
	if !names["echo"] {
		t.Error("expected 'echo' tool to be discovered")
	}
}

// TestMCPToolExecution verifies that a discovered tool can be executed
// and returns the expected result.
func TestMCPToolExecution(t *testing.T) {
	client := setupMCPTestServer(t, map[string]gomcp.ToolHandler{
		"echo": func(ctx context.Context, req *gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
			msg := "default"
			var args map[string]any
			if err := json.Unmarshal(req.Params.Arguments, &args); err == nil {
				if m, ok := args["message"]; ok {
					if s, ok := m.(string); ok {
						msg = s
					}
				}
			}
			return &gomcp.CallToolResult{
				Content: []gomcp.Content{&gomcp.TextContent{Text: "Echo: " + msg}},
			}, nil
		},
	})

	executor := mcp.NewMCPExecutor(map[string]*mcp.MCPClient{
		"integration-test": client,
	})
	defer executor.Close()

	result, err := executor.Execute(context.Background(), tools.ToolCall{
		ID:        "call_echo_1",
		Name:      "echo",
		Arguments: `{"message":"hello world"}`,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.CallID != "call_echo_1" {
		t.Errorf("call ID = %q, want 'call_echo_1'", result.CallID)
	}
	if result.Output != "Echo: hello world" {
		t.Errorf("output = %q, want 'Echo: hello world'", result.Output)
	}
	if result.IsError {
		t.Error("expected IsError=false")
	}
}

// TestMCPCanExecute verifies CanExecute returns the correct result
// for known and unknown tools.
func TestMCPCanExecute(t *testing.T) {
	client := setupMCPTestServer(t, map[string]gomcp.ToolHandler{
		"known_tool": func(ctx context.Context, req *gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
			return &gomcp.CallToolResult{
				Content: []gomcp.Content{&gomcp.TextContent{Text: "ok"}},
			}, nil
		},
	})

	executor := mcp.NewMCPExecutor(map[string]*mcp.MCPClient{
		"integration-test": client,
	})
	defer executor.Close()

	if !executor.CanExecute("known_tool") {
		t.Error("expected CanExecute('known_tool') to return true")
	}
	if executor.CanExecute("nonexistent_tool") {
		t.Error("expected CanExecute('nonexistent_tool') to return false")
	}
}

// TestMCPUnknownToolExecution verifies that executing an unknown tool
// returns an error result (not an error return value).
func TestMCPUnknownToolExecution(t *testing.T) {
	client := setupMCPTestServer(t, map[string]gomcp.ToolHandler{
		"real_tool": func(ctx context.Context, req *gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
			return &gomcp.CallToolResult{
				Content: []gomcp.Content{&gomcp.TextContent{Text: "ok"}},
			}, nil
		},
	})

	executor := mcp.NewMCPExecutor(map[string]*mcp.MCPClient{
		"integration-test": client,
	})
	defer executor.Close()

	result, err := executor.Execute(context.Background(), tools.ToolCall{
		ID:   "call_unknown",
		Name: "does_not_exist",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for unknown tool")
	}
}

// TestMCPExecutorKind verifies the executor reports the correct kind.
func TestMCPExecutorKind(t *testing.T) {
	executor := mcp.NewMCPExecutor(nil)
	if executor.Kind() != tools.ToolKindMCP {
		t.Errorf("Kind() = %v, want ToolKindMCP", executor.Kind())
	}
}
