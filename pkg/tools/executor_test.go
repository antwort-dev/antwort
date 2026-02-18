package tools

import (
	"context"
	"testing"
)

// mockExecutor is a test executor that handles all tools.
type mockExecutor struct {
	kind      ToolKind
	canExec   func(string) bool
	execFn    func(context.Context, ToolCall) (*ToolResult, error)
}

func (m *mockExecutor) Kind() ToolKind                { return m.kind }
func (m *mockExecutor) CanExecute(name string) bool   { return m.canExec(name) }
func (m *mockExecutor) Execute(ctx context.Context, call ToolCall) (*ToolResult, error) {
	return m.execFn(ctx, call)
}

// Verify mockExecutor satisfies the interface.
var _ ToolExecutor = (*mockExecutor)(nil)

func TestToolExecutor_MockSatisfiesInterface(t *testing.T) {
	exec := &mockExecutor{
		kind:    ToolKindMCP,
		canExec: func(string) bool { return true },
		execFn: func(_ context.Context, call ToolCall) (*ToolResult, error) {
			return &ToolResult{CallID: call.ID, Output: "result"}, nil
		},
	}

	if exec.Kind() != ToolKindMCP {
		t.Errorf("Kind() = %d, want ToolKindMCP", exec.Kind())
	}

	if !exec.CanExecute("any_tool") {
		t.Error("expected CanExecute to return true")
	}

	result, err := exec.Execute(context.Background(), ToolCall{ID: "c1", Name: "test", Arguments: "{}"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.CallID != "c1" {
		t.Errorf("CallID = %q, want %q", result.CallID, "c1")
	}
	if result.Output != "result" {
		t.Errorf("Output = %q, want %q", result.Output, "result")
	}
}

func TestToolExecutor_SelectiveCanExecute(t *testing.T) {
	exec := &mockExecutor{
		kind:    ToolKindSandbox,
		canExec: func(name string) bool { return name == "python_execute" },
		execFn: func(_ context.Context, call ToolCall) (*ToolResult, error) {
			return &ToolResult{CallID: call.ID, Output: "ok"}, nil
		},
	}

	if !exec.CanExecute("python_execute") {
		t.Error("expected CanExecute(python_execute) = true")
	}
	if exec.CanExecute("get_weather") {
		t.Error("expected CanExecute(get_weather) = false")
	}
}

func TestToolResult_ErrorFlag(t *testing.T) {
	result := &ToolResult{
		CallID:  "c1",
		Output:  "connection refused",
		IsError: true,
	}

	if !result.IsError {
		t.Error("expected IsError to be true")
	}
}

func TestToolKind_Values(t *testing.T) {
	if ToolKindFunction != 0 {
		t.Errorf("ToolKindFunction = %d, want 0", ToolKindFunction)
	}
	if ToolKindMCP != 1 {
		t.Errorf("ToolKindMCP = %d, want 1", ToolKindMCP)
	}
	if ToolKindSandbox != 2 {
		t.Errorf("ToolKindSandbox = %d, want 2", ToolKindSandbox)
	}
}
