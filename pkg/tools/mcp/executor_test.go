package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rhuss/antwort/pkg/tools"
)

// setupTestServer creates a test MCP server with tools and connects it
// to a client via in-memory transports. Returns the client ready for use.
func setupTestServer(t *testing.T, serverTools map[string]mcp.ToolHandler) *MCPClient {
	t.Helper()

	server := mcp.NewServer(
		&mcp.Implementation{Name: "test-server", Version: "1.0.0"},
		nil,
	)

	for name, handler := range serverTools {
		server.AddTool(
			&mcp.Tool{
				Name:        name,
				Description: "Test tool: " + name,
				InputSchema: map[string]any{"type": "object"},
			},
			handler,
		)
	}

	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	// Start the server in a background goroutine.
	ctx := context.Background()
	go func() {
		_ = server.Run(ctx, serverTransport)
	}()

	// Connect the client using the in-memory transport.
	client := &MCPClient{
		cfg: ServerConfig{Name: "test-server"},
	}
	if err := client.ConnectWithTransport(ctx, clientTransport); err != nil {
		t.Fatalf("ConnectWithTransport failed: %v", err)
	}

	t.Cleanup(func() {
		_ = client.Close()
	})

	return client
}

func TestMCPExecutor_DiscoverTools(t *testing.T) {
	client := setupTestServer(t, map[string]mcp.ToolHandler{
		"get_weather": func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "sunny"}},
			}, nil
		},
		"get_time": func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "12:00"}},
			}, nil
		},
	})

	executor := NewMCPExecutor(map[string]*MCPClient{
		"test-server": client,
	})
	defer executor.Close()

	discovered := executor.DiscoveredTools()
	if len(discovered) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(discovered))
	}

	// Verify tool names are present (order may vary).
	names := map[string]bool{}
	for _, td := range discovered {
		names[td.Name] = true
		if td.Type != "function" {
			t.Errorf("expected type 'function', got %q for tool %q", td.Type, td.Name)
		}
	}
	if !names["get_weather"] {
		t.Error("expected tool 'get_weather' not found")
	}
	if !names["get_time"] {
		t.Error("expected tool 'get_time' not found")
	}

	// Verify tools are cached: calling again returns the same results.
	discovered2 := executor.DiscoveredTools()
	if len(discovered2) != len(discovered) {
		t.Error("cached tools mismatch")
	}
}

func TestMCPExecutor_CanExecute(t *testing.T) {
	client := setupTestServer(t, map[string]mcp.ToolHandler{
		"available_tool": func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "ok"}},
			}, nil
		},
	})

	executor := NewMCPExecutor(map[string]*MCPClient{
		"test-server": client,
	})
	defer executor.Close()

	if !executor.CanExecute("available_tool") {
		t.Error("CanExecute should return true for discovered tool")
	}
	if executor.CanExecute("unknown_tool") {
		t.Error("CanExecute should return false for unknown tool")
	}
}

func TestMCPExecutor_CallTool(t *testing.T) {
	client := setupTestServer(t, map[string]mcp.ToolHandler{
		"greet": func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var args struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return nil, err
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "Hello, " + args.Name + "!"}},
			}, nil
		},
	})

	executor := NewMCPExecutor(map[string]*MCPClient{
		"test-server": client,
	})
	defer executor.Close()

	result, err := executor.Execute(context.Background(), tools.ToolCall{
		ID:        "call_123",
		Name:      "greet",
		Arguments: `{"name":"World"}`,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.CallID != "call_123" {
		t.Errorf("expected call ID 'call_123', got %q", result.CallID)
	}
	if result.Output != "Hello, World!" {
		t.Errorf("expected output 'Hello, World!', got %q", result.Output)
	}
	if result.IsError {
		t.Error("expected IsError=false, got true")
	}
}

func TestMCPExecutor_MultiServer(t *testing.T) {
	// Server A provides "tool_a".
	clientA := setupTestServer(t, map[string]mcp.ToolHandler{
		"tool_a": func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "from server A"}},
			}, nil
		},
	})

	// Server B provides "tool_b".
	clientB := setupTestServer(t, map[string]mcp.ToolHandler{
		"tool_b": func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "from server B"}},
			}, nil
		},
	})

	executor := NewMCPExecutor(map[string]*MCPClient{
		"server-a": clientA,
		"server-b": clientB,
	})
	defer executor.Close()

	// Both tools should be executable.
	if !executor.CanExecute("tool_a") {
		t.Error("CanExecute should return true for tool_a")
	}
	if !executor.CanExecute("tool_b") {
		t.Error("CanExecute should return true for tool_b")
	}

	// Call tool_a, verify it routes to server A.
	resultA, err := executor.Execute(context.Background(), tools.ToolCall{
		ID:   "call_a",
		Name: "tool_a",
	})
	if err != nil {
		t.Fatalf("Execute tool_a failed: %v", err)
	}
	if resultA.Output != "from server A" {
		t.Errorf("tool_a: expected 'from server A', got %q", resultA.Output)
	}

	// Call tool_b, verify it routes to server B.
	resultB, err := executor.Execute(context.Background(), tools.ToolCall{
		ID:   "call_b",
		Name: "tool_b",
	})
	if err != nil {
		t.Fatalf("Execute tool_b failed: %v", err)
	}
	if resultB.Output != "from server B" {
		t.Errorf("tool_b: expected 'from server B', got %q", resultB.Output)
	}
}

func TestMCPExecutor_ToolCallError(t *testing.T) {
	client := setupTestServer(t, map[string]mcp.ToolHandler{
		"failing_tool": func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "something went wrong"}},
				IsError: true,
			}, nil
		},
	})

	executor := NewMCPExecutor(map[string]*MCPClient{
		"test-server": client,
	})
	defer executor.Close()

	result, err := executor.Execute(context.Background(), tools.ToolCall{
		ID:   "call_err",
		Name: "failing_tool",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for error result")
	}
	if result.Output != "something went wrong" {
		t.Errorf("expected error output 'something went wrong', got %q", result.Output)
	}
}

func TestMCPExecutor_UnknownTool(t *testing.T) {
	client := setupTestServer(t, map[string]mcp.ToolHandler{
		"known_tool": func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "ok"}},
			}, nil
		},
	})

	executor := NewMCPExecutor(map[string]*MCPClient{
		"test-server": client,
	})
	defer executor.Close()

	result, err := executor.Execute(context.Background(), tools.ToolCall{
		ID:   "call_unknown",
		Name: "nonexistent_tool",
	})
	if err != nil {
		t.Fatalf("Execute failed with unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for unknown tool")
	}
}

func TestMCPExecutor_Kind(t *testing.T) {
	executor := NewMCPExecutor(nil)
	if executor.Kind() != tools.ToolKindMCP {
		t.Errorf("expected ToolKindMCP, got %v", executor.Kind())
	}
}
