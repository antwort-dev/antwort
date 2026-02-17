package vllm

import (
	"encoding/json"
	"testing"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/provider"
)

func TestTranslateToChat_BasicMessage(t *testing.T) {
	req := &provider.ProviderRequest{
		Model: "test-model",
		Messages: []provider.ProviderMessage{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello"},
		},
	}

	cr := translateToChat(req)

	if cr.Model != "test-model" {
		t.Errorf("expected model %q, got %q", "test-model", cr.Model)
	}
	if cr.N != 1 {
		t.Errorf("expected N=1, got %d", cr.N)
	}
	if cr.Stream {
		t.Error("expected stream to be false")
	}
	if cr.StreamOptions != nil {
		t.Error("expected nil StreamOptions when not streaming")
	}
	if len(cr.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(cr.Messages))
	}
	if cr.Messages[0].Role != "system" {
		t.Errorf("expected role %q, got %q", "system", cr.Messages[0].Role)
	}
	if cr.Messages[1].Role != "user" {
		t.Errorf("expected role %q, got %q", "user", cr.Messages[1].Role)
	}
}

func TestTranslateToChat_StreamingOptions(t *testing.T) {
	req := &provider.ProviderRequest{
		Model:  "m",
		Stream: true,
		Messages: []provider.ProviderMessage{
			{Role: "user", Content: "Hi"},
		},
	}

	cr := translateToChat(req)

	if !cr.Stream {
		t.Error("expected stream to be true")
	}
	if cr.StreamOptions == nil {
		t.Fatal("expected StreamOptions to be set")
	}
	if !cr.StreamOptions.IncludeUsage {
		t.Error("expected IncludeUsage to be true")
	}
}

func TestTranslateToChat_Parameters(t *testing.T) {
	temp := 0.5
	topP := 0.8
	maxTokens := 200

	req := &provider.ProviderRequest{
		Model:       "m",
		Messages:    []provider.ProviderMessage{{Role: "user", Content: "Hi"}},
		Temperature: &temp,
		TopP:        &topP,
		MaxTokens:   &maxTokens,
		Stop:        []string{"END"},
	}

	cr := translateToChat(req)

	if cr.Temperature == nil || *cr.Temperature != 0.5 {
		t.Errorf("expected temperature 0.5, got %v", cr.Temperature)
	}
	if cr.TopP == nil || *cr.TopP != 0.8 {
		t.Errorf("expected top_p 0.8, got %v", cr.TopP)
	}
	if cr.MaxTokens == nil || *cr.MaxTokens != 200 {
		t.Errorf("expected max_tokens 200, got %v", cr.MaxTokens)
	}
	if len(cr.Stop) != 1 || cr.Stop[0] != "END" {
		t.Errorf("expected stop [END], got %v", cr.Stop)
	}
}

func TestTranslateToChat_NilParameters(t *testing.T) {
	req := &provider.ProviderRequest{
		Model:    "m",
		Messages: []provider.ProviderMessage{{Role: "user", Content: "Hi"}},
	}

	cr := translateToChat(req)

	if cr.Temperature != nil {
		t.Errorf("expected nil temperature, got %v", cr.Temperature)
	}
	if cr.TopP != nil {
		t.Errorf("expected nil top_p, got %v", cr.TopP)
	}
	if cr.MaxTokens != nil {
		t.Errorf("expected nil max_tokens, got %v", cr.MaxTokens)
	}
}

func TestTranslateToChat_Tools(t *testing.T) {
	params := json.RawMessage(`{"type":"object"}`)
	req := &provider.ProviderRequest{
		Model:    "m",
		Messages: []provider.ProviderMessage{{Role: "user", Content: "Hi"}},
		Tools: []provider.ProviderTool{
			{
				Type: "function",
				Function: provider.ProviderFunctionDef{
					Name:        "get_weather",
					Description: "Get weather",
					Parameters:  params,
				},
			},
		},
	}

	cr := translateToChat(req)

	if len(cr.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(cr.Tools))
	}
	if cr.Tools[0].Type != "function" {
		t.Errorf("expected tool type %q, got %q", "function", cr.Tools[0].Type)
	}
	if cr.Tools[0].Function.Name != "get_weather" {
		t.Errorf("expected function name %q, got %q", "get_weather", cr.Tools[0].Function.Name)
	}
}

func TestTranslateToChat_ToolChoiceString(t *testing.T) {
	tc := api.ToolChoiceRequired
	req := &provider.ProviderRequest{
		Model:      "m",
		Messages:   []provider.ProviderMessage{{Role: "user", Content: "Hi"}},
		ToolChoice: &tc,
	}

	cr := translateToChat(req)

	if cr.ToolChoice != "required" {
		t.Errorf("expected tool_choice %q, got %v", "required", cr.ToolChoice)
	}
}

func TestTranslateToChat_ToolChoiceFunction(t *testing.T) {
	tc := api.NewToolChoiceFunction("my_func")
	req := &provider.ProviderRequest{
		Model:      "m",
		Messages:   []provider.ProviderMessage{{Role: "user", Content: "Hi"}},
		ToolChoice: &tc,
	}

	cr := translateToChat(req)

	fn, ok := cr.ToolChoice.(*api.ToolChoiceFunction)
	if !ok {
		t.Fatalf("expected *api.ToolChoiceFunction, got %T", cr.ToolChoice)
	}
	if fn.Name != "my_func" {
		t.Errorf("expected function name %q, got %q", "my_func", fn.Name)
	}
}

func TestTranslateToChat_ToolCalls(t *testing.T) {
	req := &provider.ProviderRequest{
		Model: "m",
		Messages: []provider.ProviderMessage{
			{
				Role:    "assistant",
				Content: nil,
				ToolCalls: []provider.ProviderToolCall{
					{
						ID:   "call_1",
						Type: "function",
						Function: provider.ProviderFunctionCall{
							Name:      "get_time",
							Arguments: "{}",
						},
					},
				},
			},
		},
	}

	cr := translateToChat(req)

	if len(cr.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(cr.Messages))
	}
	if len(cr.Messages[0].ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(cr.Messages[0].ToolCalls))
	}
	tc := cr.Messages[0].ToolCalls[0]
	if tc.ID != "call_1" {
		t.Errorf("expected ID %q, got %q", "call_1", tc.ID)
	}
	if tc.Function.Name != "get_time" {
		t.Errorf("expected function name %q, got %q", "get_time", tc.Function.Name)
	}
}

func TestTranslateToChat_ToolMessage(t *testing.T) {
	req := &provider.ProviderRequest{
		Model: "m",
		Messages: []provider.ProviderMessage{
			{
				Role:       "tool",
				Content:    "result data",
				ToolCallID: "call_1",
			},
		},
	}

	cr := translateToChat(req)

	if len(cr.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(cr.Messages))
	}
	if cr.Messages[0].ToolCallID != "call_1" {
		t.Errorf("expected tool_call_id %q, got %q", "call_1", cr.Messages[0].ToolCallID)
	}
}
