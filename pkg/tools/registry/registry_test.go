package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/tools"
)

// mockProvider implements FunctionProvider for testing.
type mockProvider struct {
	name       string
	toolDefs   []api.ToolDefinition
	execFn     func(context.Context, tools.ToolCall) (*tools.ToolResult, error)
	routes     []Route
	collectors []prometheus.Collector
	closed     bool
}

func (m *mockProvider) Name() string                    { return m.name }
func (m *mockProvider) Tools() []api.ToolDefinition     { return m.toolDefs }
func (m *mockProvider) Collectors() []prometheus.Collector { return m.collectors }
func (m *mockProvider) Routes() []Route                 { return m.routes }

func (m *mockProvider) CanExecute(name string) bool {
	for _, td := range m.toolDefs {
		if td.Name == name {
			return true
		}
	}
	return false
}

func (m *mockProvider) Execute(ctx context.Context, call tools.ToolCall) (*tools.ToolResult, error) {
	if m.execFn != nil {
		return m.execFn(ctx, call)
	}
	return &tools.ToolResult{CallID: call.ID, Output: "default"}, nil
}

func (m *mockProvider) Close() error {
	m.closed = true
	return nil
}

// Verify mockProvider implements FunctionProvider.
var _ FunctionProvider = (*mockProvider)(nil)

func TestRegistry_DiscoverTools(t *testing.T) {
	reg := New()

	p := &mockProvider{
		name: "test-provider",
		toolDefs: []api.ToolDefinition{
			{Type: "function", Name: "tool_a", Description: "Tool A"},
			{Type: "function", Name: "tool_b", Description: "Tool B"},
		},
	}
	reg.Register(p)

	discovered := reg.DiscoveredTools()
	if len(discovered) != 2 {
		t.Fatalf("DiscoveredTools() returned %d tools, want 2", len(discovered))
	}

	names := make(map[string]bool)
	for _, td := range discovered {
		names[td.Name] = true
	}
	if !names["tool_a"] || !names["tool_b"] {
		t.Errorf("expected tool_a and tool_b, got %v", names)
	}
}

func TestRegistry_CanExecute(t *testing.T) {
	reg := New()

	p := &mockProvider{
		name: "test-provider",
		toolDefs: []api.ToolDefinition{
			{Type: "function", Name: "known_tool"},
		},
	}
	reg.Register(p)

	if !reg.CanExecute("known_tool") {
		t.Error("expected CanExecute(known_tool) = true")
	}
	if reg.CanExecute("unknown_tool") {
		t.Error("expected CanExecute(unknown_tool) = false")
	}
}

func TestRegistry_Execute(t *testing.T) {
	reg := New()

	p := &mockProvider{
		name: "calc",
		toolDefs: []api.ToolDefinition{
			{Type: "function", Name: "add"},
		},
		execFn: func(_ context.Context, call tools.ToolCall) (*tools.ToolResult, error) {
			var args struct {
				A, B int
			}
			json.Unmarshal([]byte(call.Arguments), &args)
			return &tools.ToolResult{
				CallID: call.ID,
				Output: fmt.Sprintf("%d", args.A+args.B),
			}, nil
		},
	}
	reg.Register(p)

	result, err := reg.Execute(context.Background(), tools.ToolCall{
		ID:        "call_1",
		Name:      "add",
		Arguments: `{"A":3,"B":4}`,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.CallID != "call_1" {
		t.Errorf("CallID = %q, want %q", result.CallID, "call_1")
	}
	if result.Output != "7" {
		t.Errorf("Output = %q, want %q", result.Output, "7")
	}
	if result.IsError {
		t.Error("expected IsError = false")
	}
}

func TestRegistry_Execute_UnknownTool(t *testing.T) {
	reg := New()

	result, err := reg.Execute(context.Background(), tools.ToolCall{
		ID:   "call_1",
		Name: "nonexistent",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError = true for unknown tool")
	}
	if result.CallID != "call_1" {
		t.Errorf("CallID = %q, want %q", result.CallID, "call_1")
	}
}

func TestRegistry_ToolNameConflict(t *testing.T) {
	reg := New()

	p1 := &mockProvider{
		name: "provider-1",
		toolDefs: []api.ToolDefinition{
			{Type: "function", Name: "shared_tool"},
		},
		execFn: func(_ context.Context, call tools.ToolCall) (*tools.ToolResult, error) {
			return &tools.ToolResult{CallID: call.ID, Output: "from-p1"}, nil
		},
	}
	p2 := &mockProvider{
		name: "provider-2",
		toolDefs: []api.ToolDefinition{
			{Type: "function", Name: "shared_tool"},
		},
		execFn: func(_ context.Context, call tools.ToolCall) (*tools.ToolResult, error) {
			return &tools.ToolResult{CallID: call.ID, Output: "from-p2"}, nil
		},
	}

	reg.Register(p1)
	reg.Register(p2)

	// First provider wins.
	result, err := reg.Execute(context.Background(), tools.ToolCall{
		ID:   "call_1",
		Name: "shared_tool",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.Output != "from-p1" {
		t.Errorf("Output = %q, want %q (first provider should win)", result.Output, "from-p1")
	}

	// DiscoveredTools still includes both providers' tools (all definitions).
	discovered := reg.DiscoveredTools()
	if len(discovered) != 2 {
		t.Errorf("DiscoveredTools() returned %d tools, want 2 (both providers contribute)", len(discovered))
	}
}

func TestRegistry_PanicRecovery(t *testing.T) {
	reg := New()

	p := &mockProvider{
		name: "panicky",
		toolDefs: []api.ToolDefinition{
			{Type: "function", Name: "crash_tool"},
		},
		execFn: func(_ context.Context, call tools.ToolCall) (*tools.ToolResult, error) {
			panic("something went terribly wrong")
		},
	}
	reg.Register(p)

	result, err := reg.Execute(context.Background(), tools.ToolCall{
		ID:   "call_panic",
		Name: "crash_tool",
	})
	if err != nil {
		t.Fatalf("expected nil error after panic recovery, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result after panic recovery")
	}
	if !result.IsError {
		t.Error("expected IsError = true after panic")
	}
	if result.CallID != "call_panic" {
		t.Errorf("CallID = %q, want %q", result.CallID, "call_panic")
	}
}

func TestRegistry_HTTPHandler(t *testing.T) {
	reg := New()

	p := &mockProvider{
		name: "web-provider",
		toolDefs: []api.ToolDefinition{
			{Type: "function", Name: "web_tool"},
		},
		routes: []Route{
			{
				Method:  "GET",
				Pattern: "/providers/web/status",
				Handler: func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"status":"ok"}`))
				},
			},
			{
				Method:  "POST",
				Pattern: "/providers/web/callback",
				Handler: func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusAccepted)
					w.Write([]byte("accepted"))
				},
			},
		},
	}
	reg.Register(p)

	handler := reg.HTTPHandler()

	// Test GET route.
	req := httptest.NewRequest("GET", "/providers/web/status", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /providers/web/status status = %d, want %d", rec.Code, http.StatusOK)
	}
	body, _ := io.ReadAll(rec.Body)
	if string(body) != `{"status":"ok"}` {
		t.Errorf("GET response body = %q, want %q", string(body), `{"status":"ok"}`)
	}

	// Test POST route.
	req = httptest.NewRequest("POST", "/providers/web/callback", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("POST /providers/web/callback status = %d, want %d", rec.Code, http.StatusAccepted)
	}
}

func TestRegistry_EmptyRegistry(t *testing.T) {
	reg := New()

	// No tools discovered.
	discovered := reg.DiscoveredTools()
	if len(discovered) != 0 {
		t.Errorf("DiscoveredTools() returned %d tools, want 0", len(discovered))
	}

	// CanExecute returns false.
	if reg.CanExecute("any_tool") {
		t.Error("expected CanExecute = false for empty registry")
	}

	// Execute returns error result.
	result, err := reg.Execute(context.Background(), tools.ToolCall{
		ID:   "call_1",
		Name: "any_tool",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError = true for empty registry")
	}

	// HasProviders returns false.
	if reg.HasProviders() {
		t.Error("expected HasProviders() = false for empty registry")
	}

	// HTTPHandler returns a valid (empty) handler.
	handler := reg.HTTPHandler()
	if handler == nil {
		t.Fatal("expected non-nil handler from empty registry")
	}

	// Close on empty registry should not error.
	if err := reg.Close(); err != nil {
		t.Errorf("Close() on empty registry failed: %v", err)
	}
}

func TestRegistry_Kind(t *testing.T) {
	reg := New()
	if reg.Kind() != tools.ToolKindBuiltin {
		t.Errorf("Kind() = %d, want ToolKindBuiltin (%d)", reg.Kind(), tools.ToolKindBuiltin)
	}
}

func TestRegistry_Close(t *testing.T) {
	reg := New()

	p1 := &mockProvider{name: "p1", toolDefs: []api.ToolDefinition{{Type: "function", Name: "t1"}}}
	p2 := &mockProvider{name: "p2", toolDefs: []api.ToolDefinition{{Type: "function", Name: "t2"}}}

	reg.Register(p1)
	reg.Register(p2)

	if err := reg.Close(); err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	if !p1.closed {
		t.Error("provider p1 was not closed")
	}
	if !p2.closed {
		t.Error("provider p2 was not closed")
	}
}

func TestRegistry_ExecuteError(t *testing.T) {
	reg := New()

	p := &mockProvider{
		name: "error-provider",
		toolDefs: []api.ToolDefinition{
			{Type: "function", Name: "fail_tool"},
		},
		execFn: func(_ context.Context, call tools.ToolCall) (*tools.ToolResult, error) {
			return nil, fmt.Errorf("provider internal error")
		},
	}
	reg.Register(p)

	_, err := reg.Execute(context.Background(), tools.ToolCall{
		ID:   "call_err",
		Name: "fail_tool",
	})
	if err == nil {
		t.Fatal("expected error from Execute")
	}
}

func TestRegistry_ExecuteToolError(t *testing.T) {
	reg := New()

	p := &mockProvider{
		name: "tool-err-provider",
		toolDefs: []api.ToolDefinition{
			{Type: "function", Name: "tool_err"},
		},
		execFn: func(_ context.Context, call tools.ToolCall) (*tools.ToolResult, error) {
			return &tools.ToolResult{
				CallID:  call.ID,
				Output:  "tool-level error",
				IsError: true,
			}, nil
		},
	}
	reg.Register(p)

	result, err := reg.Execute(context.Background(), tools.ToolCall{
		ID:   "call_te",
		Name: "tool_err",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError = true for tool error")
	}
}

func TestRegistry_MultipleProviders(t *testing.T) {
	reg := New()

	p1 := &mockProvider{
		name: "math",
		toolDefs: []api.ToolDefinition{
			{Type: "function", Name: "add"},
			{Type: "function", Name: "multiply"},
		},
		execFn: func(_ context.Context, call tools.ToolCall) (*tools.ToolResult, error) {
			return &tools.ToolResult{CallID: call.ID, Output: "math:" + call.Name}, nil
		},
	}
	p2 := &mockProvider{
		name: "string",
		toolDefs: []api.ToolDefinition{
			{Type: "function", Name: "concat"},
		},
		execFn: func(_ context.Context, call tools.ToolCall) (*tools.ToolResult, error) {
			return &tools.ToolResult{CallID: call.ID, Output: "string:" + call.Name}, nil
		},
	}

	reg.Register(p1)
	reg.Register(p2)

	// Verify routing to correct provider.
	result, err := reg.Execute(context.Background(), tools.ToolCall{ID: "c1", Name: "add"})
	if err != nil {
		t.Fatalf("Execute(add) failed: %v", err)
	}
	if result.Output != "math:add" {
		t.Errorf("add output = %q, want %q", result.Output, "math:add")
	}

	result, err = reg.Execute(context.Background(), tools.ToolCall{ID: "c2", Name: "concat"})
	if err != nil {
		t.Fatalf("Execute(concat) failed: %v", err)
	}
	if result.Output != "string:concat" {
		t.Errorf("concat output = %q, want %q", result.Output, "string:concat")
	}

	// All 3 tools should be discovered.
	if len(reg.DiscoveredTools()) != 3 {
		t.Errorf("DiscoveredTools() = %d, want 3", len(reg.DiscoveredTools()))
	}

	if !reg.HasProviders() {
		t.Error("expected HasProviders() = true")
	}
}
