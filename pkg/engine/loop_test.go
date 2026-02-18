package engine

import (
	"context"
	"testing"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/provider"
	"github.com/rhuss/antwort/pkg/tools"
)

// turnAwareProvider is a mock provider that returns different responses
// depending on which turn of the agentic loop we're on.
type turnAwareProvider struct {
	turn      int
	responses []*provider.ProviderResponse
	streamFns []func(context.Context, *provider.ProviderRequest) (<-chan provider.ProviderEvent, error)
	caps      provider.ProviderCapabilities
}

func (p *turnAwareProvider) Name() string                                 { return "turn-aware" }
func (p *turnAwareProvider) Capabilities() provider.ProviderCapabilities  { return p.caps }
func (p *turnAwareProvider) ListModels(_ context.Context) ([]provider.ModelInfo, error) { return nil, nil }
func (p *turnAwareProvider) Close() error                                 { return nil }

func (p *turnAwareProvider) Complete(_ context.Context, _ *provider.ProviderRequest) (*provider.ProviderResponse, error) {
	if p.turn < len(p.responses) {
		resp := p.responses[p.turn]
		p.turn++
		return resp, nil
	}
	// Default: return empty completed response.
	return &provider.ProviderResponse{Status: api.ResponseStatusCompleted}, nil
}

func (p *turnAwareProvider) Stream(ctx context.Context, req *provider.ProviderRequest) (<-chan provider.ProviderEvent, error) {
	if p.turn < len(p.streamFns) {
		fn := p.streamFns[p.turn]
		p.turn++
		return fn(ctx, req)
	}
	ch := make(chan provider.ProviderEvent, 1)
	close(ch)
	return ch, nil
}

// alwaysExecutor is a mock executor that handles all tools.
type alwaysExecutor struct {
	results map[string]string // tool name -> output
}

func (e *alwaysExecutor) Kind() tools.ToolKind          { return tools.ToolKindMCP }
func (e *alwaysExecutor) CanExecute(name string) bool   { return true }
func (e *alwaysExecutor) Execute(_ context.Context, call tools.ToolCall) (*tools.ToolResult, error) {
	output := "default result"
	if o, ok := e.results[call.Name]; ok {
		output = o
	}
	return &tools.ToolResult{CallID: call.ID, Output: output}, nil
}

// TestAgenticLoop_TwoTurns tests a 2-turn non-streaming agentic loop:
// Turn 1: model calls a tool -> executor returns result
// Turn 2: model returns final text answer
func TestAgenticLoop_TwoTurns(t *testing.T) {
	prov := &turnAwareProvider{
		caps: provider.ProviderCapabilities{Streaming: true, ToolCalling: true},
		responses: []*provider.ProviderResponse{
			// Turn 1: tool call
			{
				Status: api.ResponseStatusCompleted,
				Items: []api.Item{
					{Type: api.ItemTypeFunctionCall, Status: api.ItemStatusCompleted,
						FunctionCall: &api.FunctionCallData{Name: "get_weather", CallID: "c1", Arguments: `{"city":"Berlin"}`}},
				},
				Usage: api.Usage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15},
			},
			// Turn 2: final answer
			{
				Status: api.ResponseStatusCompleted,
				Items: []api.Item{
					{Type: api.ItemTypeMessage, Status: api.ItemStatusCompleted,
						Message: &api.MessageData{Role: api.RoleAssistant, Output: []api.OutputContentPart{{Type: "output_text", Text: "It's 22C in Berlin."}}}},
				},
				Usage: api.Usage{InputTokens: 20, OutputTokens: 10, TotalTokens: 30},
			},
		},
	}

	exec := &alwaysExecutor{results: map[string]string{"get_weather": `{"temp":22}`}}

	eng, err := New(prov, nil, Config{
		Executors: []tools.ToolExecutor{exec},
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	req := &api.CreateResponseRequest{
		Model: "m",
		Input: []api.Item{{Type: api.ItemTypeMessage, Message: &api.MessageData{Role: api.RoleUser, Content: []api.ContentPart{{Type: "input_text", Text: "Weather?"}}}}},
		Tools: []api.ToolDefinition{{Type: "function", Name: "get_weather"}},
	}

	w := &mockResponseWriter{}
	if err := eng.CreateResponse(context.Background(), req, w); err != nil {
		t.Fatalf("CreateResponse failed: %v", err)
	}

	if w.response == nil {
		t.Fatal("expected response")
	}

	// Should have: function_call + function_call_output + message = 3 output items.
	if len(w.response.Output) != 3 {
		t.Fatalf("expected 3 output items, got %d", len(w.response.Output))
	}

	// Verify order: function_call, function_call_output, message.
	if w.response.Output[0].Type != api.ItemTypeFunctionCall {
		t.Errorf("output[0] type = %q, want function_call", w.response.Output[0].Type)
	}
	if w.response.Output[1].Type != api.ItemTypeFunctionCallOutput {
		t.Errorf("output[1] type = %q, want function_call_output", w.response.Output[1].Type)
	}
	if w.response.Output[2].Type != api.ItemTypeMessage {
		t.Errorf("output[2] type = %q, want message", w.response.Output[2].Type)
	}

	// Verify cumulative usage.
	if w.response.Usage.InputTokens != 30 {
		t.Errorf("usage input_tokens = %d, want 30", w.response.Usage.InputTokens)
	}
	if w.response.Usage.OutputTokens != 15 {
		t.Errorf("usage output_tokens = %d, want 15", w.response.Usage.OutputTokens)
	}

	// Verify status.
	if w.response.Status != api.ResponseStatusCompleted {
		t.Errorf("status = %q, want completed", w.response.Status)
	}
}

// TestAgenticLoop_ConcurrentToolCalls tests multiple tool calls in one turn.
func TestAgenticLoop_ConcurrentToolCalls(t *testing.T) {
	prov := &turnAwareProvider{
		caps: provider.ProviderCapabilities{ToolCalling: true},
		responses: []*provider.ProviderResponse{
			// Turn 1: two tool calls.
			{
				Status: api.ResponseStatusCompleted,
				Items: []api.Item{
					{Type: api.ItemTypeFunctionCall, Status: api.ItemStatusCompleted,
						FunctionCall: &api.FunctionCallData{Name: "get_weather", CallID: "c1", Arguments: `{"city":"Berlin"}`}},
					{Type: api.ItemTypeFunctionCall, Status: api.ItemStatusCompleted,
						FunctionCall: &api.FunctionCallData{Name: "get_time", CallID: "c2", Arguments: `{"tz":"CET"}`}},
				},
				Usage: api.Usage{InputTokens: 10, OutputTokens: 10, TotalTokens: 20},
			},
			// Turn 2: final answer.
			{
				Status: api.ResponseStatusCompleted,
				Items: []api.Item{
					{Type: api.ItemTypeMessage, Status: api.ItemStatusCompleted,
						Message: &api.MessageData{Role: api.RoleAssistant, Output: []api.OutputContentPart{{Type: "output_text", Text: "Done"}}}},
				},
				Usage: api.Usage{InputTokens: 5, OutputTokens: 5, TotalTokens: 10},
			},
		},
	}

	exec := &alwaysExecutor{results: map[string]string{
		"get_weather": `{"temp":22}`,
		"get_time":    `{"time":"14:00"}`,
	}}

	eng, err := New(prov, nil, Config{Executors: []tools.ToolExecutor{exec}})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	req := &api.CreateResponseRequest{
		Model: "m",
		Input: []api.Item{{Type: api.ItemTypeMessage, Message: &api.MessageData{Role: api.RoleUser, Content: []api.ContentPart{{Type: "input_text", Text: "Info"}}}}},
		Tools: []api.ToolDefinition{{Type: "function", Name: "get_weather"}, {Type: "function", Name: "get_time"}},
	}

	w := &mockResponseWriter{}
	if err := eng.CreateResponse(context.Background(), req, w); err != nil {
		t.Fatalf("CreateResponse failed: %v", err)
	}

	// Should have: 2 function_calls + 2 function_call_outputs + 1 message = 5 items.
	if len(w.response.Output) != 5 {
		t.Fatalf("expected 5 output items, got %d", len(w.response.Output))
	}
}

// TestAgenticLoop_MaxTurns tests that the loop terminates at max turns.
func TestAgenticLoop_MaxTurns(t *testing.T) {
	// Provider always returns tool calls.
	prov := &turnAwareProvider{
		caps: provider.ProviderCapabilities{ToolCalling: true},
		responses: []*provider.ProviderResponse{
			{Status: api.ResponseStatusCompleted, Items: []api.Item{{Type: api.ItemTypeFunctionCall, Status: api.ItemStatusCompleted, FunctionCall: &api.FunctionCallData{Name: "fn", CallID: "c1", Arguments: "{}"}}}},
			{Status: api.ResponseStatusCompleted, Items: []api.Item{{Type: api.ItemTypeFunctionCall, Status: api.ItemStatusCompleted, FunctionCall: &api.FunctionCallData{Name: "fn", CallID: "c2", Arguments: "{}"}}}},
			{Status: api.ResponseStatusCompleted, Items: []api.Item{{Type: api.ItemTypeFunctionCall, Status: api.ItemStatusCompleted, FunctionCall: &api.FunctionCallData{Name: "fn", CallID: "c3", Arguments: "{}"}}}},
		},
	}

	exec := &alwaysExecutor{results: map[string]string{"fn": "ok"}}

	eng, err := New(prov, nil, Config{
		MaxAgenticTurns: 2,
		Executors:       []tools.ToolExecutor{exec},
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	req := &api.CreateResponseRequest{
		Model: "m",
		Input: []api.Item{{Type: api.ItemTypeMessage, Message: &api.MessageData{Role: api.RoleUser, Content: []api.ContentPart{{Type: "input_text", Text: "Go"}}}}},
		Tools: []api.ToolDefinition{{Type: "function", Name: "fn"}},
	}

	w := &mockResponseWriter{}
	if err := eng.CreateResponse(context.Background(), req, w); err != nil {
		t.Fatalf("CreateResponse failed: %v", err)
	}

	if w.response.Status != api.ResponseStatusIncomplete {
		t.Errorf("status = %q, want incomplete", w.response.Status)
	}
}

// TestAgenticLoop_ToolError tests that tool errors are fed back to the model.
func TestAgenticLoop_ToolError(t *testing.T) {
	prov := &turnAwareProvider{
		caps: provider.ProviderCapabilities{ToolCalling: true},
		responses: []*provider.ProviderResponse{
			{Status: api.ResponseStatusCompleted, Items: []api.Item{
				{Type: api.ItemTypeFunctionCall, Status: api.ItemStatusCompleted,
					FunctionCall: &api.FunctionCallData{Name: "failing_tool", CallID: "c1", Arguments: "{}"}},
			}},
			// Model gets the error and produces final answer.
			{Status: api.ResponseStatusCompleted, Items: []api.Item{
				{Type: api.ItemTypeMessage, Status: api.ItemStatusCompleted,
					Message: &api.MessageData{Role: api.RoleAssistant, Output: []api.OutputContentPart{{Type: "output_text", Text: "Tool failed, sorry"}}}},
			}},
		},
	}

	// Executor that always fails.
	failExec := &mockExecutorForEngine{
		canExec: func(name string) bool { return true },
		execFn: func(_ context.Context, call tools.ToolCall) (*tools.ToolResult, error) {
			return nil, api.NewServerError("connection refused")
		},
	}

	eng, err := New(prov, nil, Config{Executors: []tools.ToolExecutor{failExec}})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	req := &api.CreateResponseRequest{
		Model: "m",
		Input: []api.Item{{Type: api.ItemTypeMessage, Message: &api.MessageData{Role: api.RoleUser, Content: []api.ContentPart{{Type: "input_text", Text: "Do"}}}}},
		Tools: []api.ToolDefinition{{Type: "function", Name: "failing_tool"}},
	}

	w := &mockResponseWriter{}
	if err := eng.CreateResponse(context.Background(), req, w); err != nil {
		t.Fatalf("CreateResponse failed: %v", err)
	}

	// Should have completed (model recovered from error).
	if w.response.Status != api.ResponseStatusCompleted {
		t.Errorf("status = %q, want completed", w.response.Status)
	}

	// The function_call_output should have is_error (we check output contains error text).
	var foundErrorOutput bool
	for _, item := range w.response.Output {
		if item.Type == api.ItemTypeFunctionCallOutput && item.FunctionCallOutput != nil {
			if item.FunctionCallOutput.Output != "" {
				foundErrorOutput = true
			}
		}
	}
	if !foundErrorOutput {
		t.Error("expected function_call_output with error message")
	}
}

// TestAgenticLoop_RequiresAction tests that unhandled tools return requires_action.
func TestAgenticLoop_RequiresAction(t *testing.T) {
	prov := &turnAwareProvider{
		caps: provider.ProviderCapabilities{ToolCalling: true},
		responses: []*provider.ProviderResponse{
			{Status: api.ResponseStatusCompleted, Items: []api.Item{
				{Type: api.ItemTypeFunctionCall, Status: api.ItemStatusCompleted,
					FunctionCall: &api.FunctionCallData{Name: "client_tool", CallID: "c1", Arguments: "{}"}},
			}},
		},
	}

	// Executor that only handles "server_tool", not "client_tool".
	selectiveExec := &mockExecutorForEngine{
		canExec: func(name string) bool { return name == "server_tool" },
		execFn: func(_ context.Context, call tools.ToolCall) (*tools.ToolResult, error) {
			return &tools.ToolResult{CallID: call.ID, Output: "ok"}, nil
		},
	}

	eng, err := New(prov, nil, Config{Executors: []tools.ToolExecutor{selectiveExec}})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	req := &api.CreateResponseRequest{
		Model: "m",
		Input: []api.Item{{Type: api.ItemTypeMessage, Message: &api.MessageData{Role: api.RoleUser, Content: []api.ContentPart{{Type: "input_text", Text: "Do"}}}}},
		Tools: []api.ToolDefinition{{Type: "function", Name: "client_tool"}},
	}

	w := &mockResponseWriter{}
	if err := eng.CreateResponse(context.Background(), req, w); err != nil {
		t.Fatalf("CreateResponse failed: %v", err)
	}

	if w.response.Status != api.ResponseStatusRequiresAction {
		t.Errorf("status = %q, want requires_action", w.response.Status)
	}
}

// TestAgenticLoop_AllowedToolsRejection tests that non-allowed tools are rejected.
func TestAgenticLoop_AllowedToolsRejection(t *testing.T) {
	prov := &turnAwareProvider{
		caps: provider.ProviderCapabilities{ToolCalling: true},
		responses: []*provider.ProviderResponse{
			// Model calls "dangerous_tool" which is not allowed.
			{Status: api.ResponseStatusCompleted, Items: []api.Item{
				{Type: api.ItemTypeFunctionCall, Status: api.ItemStatusCompleted,
					FunctionCall: &api.FunctionCallData{Name: "dangerous_tool", CallID: "c1", Arguments: "{}"}},
			}},
			// Model gets error and produces final answer.
			{Status: api.ResponseStatusCompleted, Items: []api.Item{
				{Type: api.ItemTypeMessage, Status: api.ItemStatusCompleted,
					Message: &api.MessageData{Role: api.RoleAssistant, Output: []api.OutputContentPart{{Type: "output_text", Text: "Cannot use that tool."}}}},
			}},
		},
	}

	exec := &alwaysExecutor{results: map[string]string{}}

	eng, err := New(prov, nil, Config{Executors: []tools.ToolExecutor{exec}})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	req := &api.CreateResponseRequest{
		Model:        "m",
		Input:        []api.Item{{Type: api.ItemTypeMessage, Message: &api.MessageData{Role: api.RoleUser, Content: []api.ContentPart{{Type: "input_text", Text: "Delete"}}}}},
		Tools:        []api.ToolDefinition{{Type: "function", Name: "safe_tool"}, {Type: "function", Name: "dangerous_tool"}},
		AllowedTools: []string{"safe_tool"},
	}

	w := &mockResponseWriter{}
	if err := eng.CreateResponse(context.Background(), req, w); err != nil {
		t.Fatalf("CreateResponse failed: %v", err)
	}

	// Should have completed (model recovered from rejection).
	if w.response.Status != api.ResponseStatusCompleted {
		t.Errorf("status = %q, want completed", w.response.Status)
	}

	// Verify the rejection error was fed back.
	var foundRejection bool
	for _, item := range w.response.Output {
		if item.Type == api.ItemTypeFunctionCallOutput && item.FunctionCallOutput != nil {
			if item.FunctionCallOutput.Output != "" {
				foundRejection = true
			}
		}
	}
	if !foundRejection {
		t.Error("expected rejection error in output")
	}
}

// TestAgenticLoop_NoExecutors_SingleShot tests backward compatibility.
func TestAgenticLoop_NoExecutors_SingleShot(t *testing.T) {
	prov := &turnAwareProvider{
		caps: provider.ProviderCapabilities{ToolCalling: true},
		responses: []*provider.ProviderResponse{
			{Status: api.ResponseStatusCompleted, Items: []api.Item{
				{Type: api.ItemTypeFunctionCall, Status: api.ItemStatusCompleted,
					FunctionCall: &api.FunctionCallData{Name: "fn", CallID: "c1", Arguments: "{}"}},
			}},
		},
	}

	// No executors.
	eng, err := New(prov, nil, Config{})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	req := &api.CreateResponseRequest{
		Model: "m",
		Input: []api.Item{{Type: api.ItemTypeMessage, Message: &api.MessageData{Role: api.RoleUser, Content: []api.ContentPart{{Type: "input_text", Text: "Go"}}}}},
		Tools: []api.ToolDefinition{{Type: "function", Name: "fn"}},
	}

	w := &mockResponseWriter{}
	if err := eng.CreateResponse(context.Background(), req, w); err != nil {
		t.Fatalf("CreateResponse failed: %v", err)
	}

	// Should return completed with tool call items (single-shot behavior).
	if w.response.Status != api.ResponseStatusCompleted {
		t.Errorf("status = %q, want completed", w.response.Status)
	}
	if len(w.response.Output) != 1 || w.response.Output[0].Type != api.ItemTypeFunctionCall {
		t.Error("expected single function_call item in output (single-shot)")
	}
}

// TestAgenticLoop_ToolChoiceNone tests that tool_choice "none" skips the loop.
func TestAgenticLoop_ToolChoiceNone(t *testing.T) {
	prov := &turnAwareProvider{
		caps: provider.ProviderCapabilities{ToolCalling: true},
		responses: []*provider.ProviderResponse{
			{Status: api.ResponseStatusCompleted, Items: []api.Item{
				{Type: api.ItemTypeFunctionCall, Status: api.ItemStatusCompleted,
					FunctionCall: &api.FunctionCallData{Name: "fn", CallID: "c1", Arguments: "{}"}},
			}},
		},
	}

	exec := &alwaysExecutor{results: map[string]string{}}

	eng, err := New(prov, nil, Config{Executors: []tools.ToolExecutor{exec}})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	req := &api.CreateResponseRequest{
		Model:      "m",
		Input:      []api.Item{{Type: api.ItemTypeMessage, Message: &api.MessageData{Role: api.RoleUser, Content: []api.ContentPart{{Type: "input_text", Text: "Go"}}}}},
		Tools:      []api.ToolDefinition{{Type: "function", Name: "fn"}},
		ToolChoice: &api.ToolChoice{String: "none"},
	}

	w := &mockResponseWriter{}
	if err := eng.CreateResponse(context.Background(), req, w); err != nil {
		t.Fatalf("CreateResponse failed: %v", err)
	}

	// tool_choice "none" should skip the loop even with executors.
	// Tool calls are returned as-is with completed status.
	if w.response.Status != api.ResponseStatusCompleted {
		t.Errorf("status = %q, want completed", w.response.Status)
	}
}

// mockExecutorForEngine wraps function pointers for flexible test mocking.
type mockExecutorForEngine struct {
	canExec func(string) bool
	execFn  func(context.Context, tools.ToolCall) (*tools.ToolResult, error)
}

func (m *mockExecutorForEngine) Kind() tools.ToolKind          { return tools.ToolKindMCP }
func (m *mockExecutorForEngine) CanExecute(name string) bool   { return m.canExec(name) }
func (m *mockExecutorForEngine) Execute(ctx context.Context, call tools.ToolCall) (*tools.ToolResult, error) {
	return m.execFn(ctx, call)
}
