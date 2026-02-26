package responses

import (
	"encoding/json"
	"testing"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/provider"
)

func TestTranslateRequest_StoreAlwaysFalse(t *testing.T) {
	req := &provider.ProviderRequest{
		Model: "test-model",
		Messages: []provider.ProviderMessage{
			{Role: "user", Content: "hello"},
		},
	}

	rr, err := translateRequest(req)
	if err != nil {
		t.Fatalf("translateRequest: %v", err)
	}
	if rr.Store != false {
		t.Error("store should always be false")
	}
}

func TestTranslateRequest_BuiltinToolsExpanded(t *testing.T) {
	req := &provider.ProviderRequest{
		Model: "test-model",
		Messages: []provider.ProviderMessage{
			{Role: "user", Content: "run code"},
		},
		Tools: []provider.ProviderTool{
			{Type: "code_interpreter"}, // stub, no function name
		},
		BuiltinToolDefs: []provider.ProviderTool{
			{
				Type: "function",
				Function: provider.ProviderFunctionDef{
					Name:        "code_interpreter",
					Description: "Execute Python code",
					Parameters:  json.RawMessage(`{"type":"object"}`),
				},
			},
		},
	}

	rr, err := translateRequest(req)
	if err != nil {
		t.Fatalf("translateRequest: %v", err)
	}

	if len(rr.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(rr.Tools))
	}
	if rr.Tools[0].Name != "code_interpreter" {
		t.Errorf("tool name = %q, want %q", rr.Tools[0].Name, "code_interpreter")
	}
	if rr.Tools[0].Type != "function" {
		t.Errorf("tool type = %q, want %q", rr.Tools[0].Type, "function")
	}
}

func TestTranslateRequest_FunctionToolsPassThrough(t *testing.T) {
	req := &provider.ProviderRequest{
		Model: "test-model",
		Messages: []provider.ProviderMessage{
			{Role: "user", Content: "hello"},
		},
		Tools: []provider.ProviderTool{
			{
				Type: "function",
				Function: provider.ProviderFunctionDef{
					Name:        "get_weather",
					Description: "Get weather",
					Parameters:  json.RawMessage(`{"type":"object"}`),
				},
			},
		},
	}

	rr, err := translateRequest(req)
	if err != nil {
		t.Fatalf("translateRequest: %v", err)
	}

	if len(rr.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(rr.Tools))
	}
	if rr.Tools[0].Name != "get_weather" {
		t.Errorf("tool name = %q, want %q", rr.Tools[0].Name, "get_weather")
	}
}

func TestTranslateRequest_Messages(t *testing.T) {
	req := &provider.ProviderRequest{
		Model: "test-model",
		Messages: []provider.ProviderMessage{
			{Role: "system", Content: "You are helpful"},
			{Role: "user", Content: "What is 2+2?"},
			{
				Role:    "assistant",
				Content: nil,
				ToolCalls: []provider.ProviderToolCall{
					{
						ID: "call_123",
						Function: provider.ProviderFunctionCall{
							Name:      "calculator",
							Arguments: `{"expr":"2+2"}`,
						},
					},
				},
			},
			{Role: "tool", Content: "4", ToolCallID: "call_123"},
		},
	}

	rr, err := translateRequest(req)
	if err != nil {
		t.Fatalf("translateRequest: %v", err)
	}

	// Verify input is a valid JSON array.
	var items []json.RawMessage
	if err := json.Unmarshal(rr.Input, &items); err != nil {
		t.Fatalf("input is not a JSON array: %v", err)
	}

	// Should have: system message, user message, function_call, function_call_output
	if len(items) != 4 {
		t.Errorf("expected 4 input items, got %d", len(items))
	}
}

func TestTranslateResponse_Message(t *testing.T) {
	resp := &responsesResponse{
		Model:  "test-model",
		Status: "completed",
		Output: []responsesItem{
			{
				ID:   "item_001",
				Type: "message",
				Role: "assistant",
				Content: json.RawMessage(`[{"type":"output_text","text":"Hello world"}]`),
			},
		},
		Usage: &responsesUsage{
			InputTokens:  10,
			OutputTokens: 5,
			TotalTokens:  15,
		},
	}

	pr, err := translateResponse(resp)
	if err != nil {
		t.Fatalf("translateResponse: %v", err)
	}

	if pr.Model != "test-model" {
		t.Errorf("model = %q, want %q", pr.Model, "test-model")
	}
	if pr.Status != api.ResponseStatusCompleted {
		t.Errorf("status = %q, want %q", pr.Status, api.ResponseStatusCompleted)
	}
	if len(pr.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(pr.Items))
	}
	if pr.Items[0].Type != api.ItemTypeMessage {
		t.Errorf("item type = %q, want %q", pr.Items[0].Type, api.ItemTypeMessage)
	}
	if pr.Items[0].Message == nil {
		t.Fatal("message data is nil")
	}
	if len(pr.Items[0].Message.Output) != 1 || pr.Items[0].Message.Output[0].Text != "Hello world" {
		t.Errorf("message text = %v, want 'Hello world'", pr.Items[0].Message.Output)
	}
	if pr.Usage.InputTokens != 10 || pr.Usage.OutputTokens != 5 {
		t.Errorf("usage = %+v, want input=10 output=5", pr.Usage)
	}
}

func TestTranslateResponse_ToolCall(t *testing.T) {
	resp := &responsesResponse{
		Model:  "test-model",
		Status: "completed",
		Output: []responsesItem{
			{
				ID:        "item_002",
				Type:      "function_call",
				CallID:    "call_abc",
				Name:      "get_weather",
				Arguments: `{"city":"Berlin"}`,
			},
		},
	}

	pr, err := translateResponse(resp)
	if err != nil {
		t.Fatalf("translateResponse: %v", err)
	}

	if len(pr.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(pr.Items))
	}
	item := pr.Items[0]
	if item.Type != api.ItemTypeFunctionCall {
		t.Errorf("item type = %q, want %q", item.Type, api.ItemTypeFunctionCall)
	}
	if item.FunctionCall == nil {
		t.Fatal("function call data is nil")
	}
	if item.FunctionCall.Name != "get_weather" {
		t.Errorf("name = %q, want %q", item.FunctionCall.Name, "get_weather")
	}
	if item.FunctionCall.CallID != "call_abc" {
		t.Errorf("call_id = %q, want %q", item.FunctionCall.CallID, "call_abc")
	}
}
