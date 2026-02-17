package vllm

import (
	"testing"

	"github.com/rhuss/antwort/pkg/api"
)

func TestTranslateResponse_TextContent(t *testing.T) {
	resp := &chatCompletionResponse{
		ID:    "chatcmpl-123",
		Model: "test-model",
		Choices: []chatChoice{
			{
				Index: 0,
				Message: chatMessage{
					Role:    "assistant",
					Content: "Hello, how can I help?",
				},
				FinishReason: "stop",
			},
		},
		Usage: &chatUsage{
			PromptTokens:     10,
			CompletionTokens: 8,
			TotalTokens:      18,
		},
	}

	pr := translateResponse(resp)

	if pr.Model != "test-model" {
		t.Errorf("expected model %q, got %q", "test-model", pr.Model)
	}
	if pr.Status != api.ResponseStatusCompleted {
		t.Errorf("expected status %q, got %q", api.ResponseStatusCompleted, pr.Status)
	}
	if pr.Usage.InputTokens != 10 {
		t.Errorf("expected input_tokens 10, got %d", pr.Usage.InputTokens)
	}
	if pr.Usage.OutputTokens != 8 {
		t.Errorf("expected output_tokens 8, got %d", pr.Usage.OutputTokens)
	}
	if pr.Usage.TotalTokens != 18 {
		t.Errorf("expected total_tokens 18, got %d", pr.Usage.TotalTokens)
	}
	if len(pr.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(pr.Items))
	}

	item := pr.Items[0]
	if item.Type != api.ItemTypeMessage {
		t.Errorf("expected item type %q, got %q", api.ItemTypeMessage, item.Type)
	}
	if item.Status != api.ItemStatusCompleted {
		t.Errorf("expected item status %q, got %q", api.ItemStatusCompleted, item.Status)
	}
	if !api.ValidateItemID(item.ID) {
		t.Errorf("expected valid item ID, got %q", item.ID)
	}
	if item.Message == nil {
		t.Fatal("expected message data to be set")
	}
	if item.Message.Role != api.RoleAssistant {
		t.Errorf("expected role %q, got %q", api.RoleAssistant, item.Message.Role)
	}
	if len(item.Message.Output) != 1 {
		t.Fatalf("expected 1 output part, got %d", len(item.Message.Output))
	}
	if item.Message.Output[0].Type != "output_text" {
		t.Errorf("expected output type %q, got %q", "output_text", item.Message.Output[0].Type)
	}
	if item.Message.Output[0].Text != "Hello, how can I help?" {
		t.Errorf("expected text %q, got %q", "Hello, how can I help?", item.Message.Output[0].Text)
	}
}

func TestTranslateResponse_ToolCalls(t *testing.T) {
	resp := &chatCompletionResponse{
		ID:    "chatcmpl-456",
		Model: "test-model",
		Choices: []chatChoice{
			{
				Index: 0,
				Message: chatMessage{
					Role:    "assistant",
					Content: nil,
					ToolCalls: []chatToolCall{
						{
							ID:   "call_abc",
							Type: "function",
							Function: chatFunctionCall{
								Name:      "get_weather",
								Arguments: `{"city":"Berlin"}`,
							},
						},
						{
							ID:   "call_def",
							Type: "function",
							Function: chatFunctionCall{
								Name:      "get_time",
								Arguments: `{"tz":"CET"}`,
							},
						},
					},
				},
				FinishReason: "tool_calls",
			},
		},
		Usage: &chatUsage{
			PromptTokens:     15,
			CompletionTokens: 20,
			TotalTokens:      35,
		},
	}

	pr := translateResponse(resp)

	if pr.Status != api.ResponseStatusCompleted {
		t.Errorf("expected status %q, got %q", api.ResponseStatusCompleted, pr.Status)
	}

	// No text content, so only tool call items.
	if len(pr.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(pr.Items))
	}

	// First tool call.
	item0 := pr.Items[0]
	if item0.Type != api.ItemTypeFunctionCall {
		t.Errorf("expected item type %q, got %q", api.ItemTypeFunctionCall, item0.Type)
	}
	if item0.FunctionCall == nil {
		t.Fatal("expected FunctionCall data to be set")
	}
	if item0.FunctionCall.Name != "get_weather" {
		t.Errorf("expected name %q, got %q", "get_weather", item0.FunctionCall.Name)
	}
	if item0.FunctionCall.CallID != "call_abc" {
		t.Errorf("expected call_id %q, got %q", "call_abc", item0.FunctionCall.CallID)
	}
	if item0.FunctionCall.Arguments != `{"city":"Berlin"}` {
		t.Errorf("expected arguments %q, got %q", `{"city":"Berlin"}`, item0.FunctionCall.Arguments)
	}
	if !api.ValidateItemID(item0.ID) {
		t.Errorf("expected valid item ID, got %q", item0.ID)
	}

	// Second tool call.
	item1 := pr.Items[1]
	if item1.FunctionCall.Name != "get_time" {
		t.Errorf("expected name %q, got %q", "get_time", item1.FunctionCall.Name)
	}
}

func TestTranslateResponse_TextAndToolCalls(t *testing.T) {
	resp := &chatCompletionResponse{
		ID:    "chatcmpl-789",
		Model: "m",
		Choices: []chatChoice{
			{
				Index: 0,
				Message: chatMessage{
					Role:    "assistant",
					Content: "Let me check that for you.",
					ToolCalls: []chatToolCall{
						{
							ID:   "call_1",
							Type: "function",
							Function: chatFunctionCall{
								Name:      "lookup",
								Arguments: "{}",
							},
						},
					},
				},
				FinishReason: "tool_calls",
			},
		},
		Usage: &chatUsage{PromptTokens: 5, CompletionTokens: 10, TotalTokens: 15},
	}

	pr := translateResponse(resp)

	// Should have 2 items: one message, one function_call.
	if len(pr.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(pr.Items))
	}
	if pr.Items[0].Type != api.ItemTypeMessage {
		t.Errorf("expected first item type %q, got %q", api.ItemTypeMessage, pr.Items[0].Type)
	}
	if pr.Items[1].Type != api.ItemTypeFunctionCall {
		t.Errorf("expected second item type %q, got %q", api.ItemTypeFunctionCall, pr.Items[1].Type)
	}
}

func TestTranslateResponse_FinishReasonMapping(t *testing.T) {
	tests := []struct {
		finishReason string
		expected     api.ResponseStatus
	}{
		{"stop", api.ResponseStatusCompleted},
		{"length", api.ResponseStatusIncomplete},
		{"tool_calls", api.ResponseStatusCompleted},
		{"content_filter", api.ResponseStatusCompleted}, // Unknown maps to completed.
	}

	for _, tt := range tests {
		t.Run(tt.finishReason, func(t *testing.T) {
			resp := &chatCompletionResponse{
				Model: "m",
				Choices: []chatChoice{
					{
						Message:      chatMessage{Role: "assistant", Content: "ok"},
						FinishReason: tt.finishReason,
					},
				},
				Usage: &chatUsage{},
			}
			pr := translateResponse(resp)
			if pr.Status != tt.expected {
				t.Errorf("finish_reason %q: expected status %q, got %q",
					tt.finishReason, tt.expected, pr.Status)
			}
		})
	}
}

func TestTranslateResponse_NoChoices(t *testing.T) {
	resp := &chatCompletionResponse{
		Model:   "m",
		Choices: []chatChoice{},
		Usage:   &chatUsage{PromptTokens: 5, CompletionTokens: 0, TotalTokens: 5},
	}

	pr := translateResponse(resp)

	if len(pr.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(pr.Items))
	}
	if pr.Usage.InputTokens != 5 {
		t.Errorf("expected input_tokens 5, got %d", pr.Usage.InputTokens)
	}
}

func TestTranslateResponse_NilUsage(t *testing.T) {
	resp := &chatCompletionResponse{
		Model: "m",
		Choices: []chatChoice{
			{
				Message:      chatMessage{Role: "assistant", Content: "hi"},
				FinishReason: "stop",
			},
		},
		Usage: nil,
	}

	pr := translateResponse(resp)

	if pr.Usage.InputTokens != 0 || pr.Usage.OutputTokens != 0 {
		t.Errorf("expected zero usage, got %+v", pr.Usage)
	}
}
