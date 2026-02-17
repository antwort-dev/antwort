package engine

import (
	"encoding/json"
	"testing"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/provider"
)

func TestTranslateRequest_Instructions(t *testing.T) {
	req := &api.CreateResponseRequest{
		Model:        "test-model",
		Instructions: "You are a helpful assistant.",
		Input:        []api.Item{},
	}

	pr := translateRequest(req)

	if pr.Model != "test-model" {
		t.Errorf("expected model %q, got %q", "test-model", pr.Model)
	}
	if len(pr.Messages) != 1 {
		t.Fatalf("expected 1 message (system), got %d", len(pr.Messages))
	}
	if pr.Messages[0].Role != "system" {
		t.Errorf("expected role %q, got %q", "system", pr.Messages[0].Role)
	}
	if pr.Messages[0].Content != "You are a helpful assistant." {
		t.Errorf("expected instructions content, got %v", pr.Messages[0].Content)
	}
}

func TestTranslateRequest_NoInstructions(t *testing.T) {
	req := &api.CreateResponseRequest{
		Model: "test-model",
		Input: []api.Item{
			{
				Type: api.ItemTypeMessage,
				Message: &api.MessageData{
					Role:    api.RoleUser,
					Content: []api.ContentPart{{Type: "input_text", Text: "Hello"}},
				},
			},
		},
	}

	pr := translateRequest(req)

	if len(pr.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(pr.Messages))
	}
	if pr.Messages[0].Role != "user" {
		t.Errorf("expected role %q, got %q", "user", pr.Messages[0].Role)
	}
}

func TestTranslateRequest_UserMessage(t *testing.T) {
	req := &api.CreateResponseRequest{
		Model: "m",
		Input: []api.Item{
			{
				Type: api.ItemTypeMessage,
				Message: &api.MessageData{
					Role: api.RoleUser,
					Content: []api.ContentPart{
						{Type: "input_text", Text: "Hello "},
						{Type: "input_text", Text: "World"},
					},
				},
			},
		},
	}

	pr := translateRequest(req)

	if len(pr.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(pr.Messages))
	}
	if pr.Messages[0].Content != "Hello World" {
		t.Errorf("expected concatenated content %q, got %v", "Hello World", pr.Messages[0].Content)
	}
}

func TestTranslateRequest_AssistantMessage(t *testing.T) {
	req := &api.CreateResponseRequest{
		Model: "m",
		Input: []api.Item{
			{
				Type: api.ItemTypeMessage,
				Message: &api.MessageData{
					Role: api.RoleAssistant,
					Output: []api.OutputContentPart{
						{Type: "output_text", Text: "I can help!"},
					},
				},
			},
		},
	}

	pr := translateRequest(req)

	if len(pr.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(pr.Messages))
	}
	if pr.Messages[0].Role != "assistant" {
		t.Errorf("expected role %q, got %q", "assistant", pr.Messages[0].Role)
	}
	if pr.Messages[0].Content != "I can help!" {
		t.Errorf("expected content %q, got %v", "I can help!", pr.Messages[0].Content)
	}
}

func TestTranslateRequest_SystemMessage(t *testing.T) {
	req := &api.CreateResponseRequest{
		Model: "m",
		Input: []api.Item{
			{
				Type: api.ItemTypeMessage,
				Message: &api.MessageData{
					Role:    api.RoleSystem,
					Content: []api.ContentPart{{Type: "input_text", Text: "Be concise."}},
				},
			},
		},
	}

	pr := translateRequest(req)

	if len(pr.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(pr.Messages))
	}
	if pr.Messages[0].Role != "system" {
		t.Errorf("expected role %q, got %q", "system", pr.Messages[0].Role)
	}
	if pr.Messages[0].Content != "Be concise." {
		t.Errorf("expected content %q, got %v", "Be concise.", pr.Messages[0].Content)
	}
}

func TestTranslateRequest_FunctionCall(t *testing.T) {
	req := &api.CreateResponseRequest{
		Model: "m",
		Input: []api.Item{
			{
				Type: api.ItemTypeFunctionCall,
				FunctionCall: &api.FunctionCallData{
					Name:      "get_weather",
					CallID:    "call_123",
					Arguments: `{"city":"Berlin"}`,
				},
			},
		},
	}

	pr := translateRequest(req)

	if len(pr.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(pr.Messages))
	}
	msg := pr.Messages[0]
	if msg.Role != "assistant" {
		t.Errorf("expected role %q, got %q", "assistant", msg.Role)
	}
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(msg.ToolCalls))
	}
	tc := msg.ToolCalls[0]
	if tc.ID != "call_123" {
		t.Errorf("expected tool call ID %q, got %q", "call_123", tc.ID)
	}
	if tc.Type != "function" {
		t.Errorf("expected tool call type %q, got %q", "function", tc.Type)
	}
	if tc.Function.Name != "get_weather" {
		t.Errorf("expected function name %q, got %q", "get_weather", tc.Function.Name)
	}
	if tc.Function.Arguments != `{"city":"Berlin"}` {
		t.Errorf("expected arguments %q, got %q", `{"city":"Berlin"}`, tc.Function.Arguments)
	}
}

func TestTranslateRequest_FunctionCallOutput(t *testing.T) {
	req := &api.CreateResponseRequest{
		Model: "m",
		Input: []api.Item{
			{
				Type: api.ItemTypeFunctionCallOutput,
				FunctionCallOutput: &api.FunctionCallOutputData{
					CallID: "call_123",
					Output: `{"temp": 20}`,
				},
			},
		},
	}

	pr := translateRequest(req)

	if len(pr.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(pr.Messages))
	}
	msg := pr.Messages[0]
	if msg.Role != "tool" {
		t.Errorf("expected role %q, got %q", "tool", msg.Role)
	}
	if msg.ToolCallID != "call_123" {
		t.Errorf("expected tool_call_id %q, got %q", "call_123", msg.ToolCallID)
	}
	if msg.Content != `{"temp": 20}` {
		t.Errorf("expected content %q, got %v", `{"temp": 20}`, msg.Content)
	}
}

func TestTranslateRequest_ReasoningSkipped(t *testing.T) {
	req := &api.CreateResponseRequest{
		Model: "m",
		Input: []api.Item{
			{
				Type: api.ItemTypeReasoning,
				Reasoning: &api.ReasoningData{
					Content: "thinking...",
				},
			},
		},
	}

	pr := translateRequest(req)

	if len(pr.Messages) != 0 {
		t.Errorf("expected 0 messages (reasoning skipped), got %d", len(pr.Messages))
	}
}

func TestTranslateRequest_Tools(t *testing.T) {
	params := json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`)
	req := &api.CreateResponseRequest{
		Model: "m",
		Input: []api.Item{},
		Tools: []api.ToolDefinition{
			{
				Type:        "function",
				Name:        "get_weather",
				Description: "Get weather for a city",
				Parameters:  params,
			},
		},
	}

	pr := translateRequest(req)

	if len(pr.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(pr.Tools))
	}
	tool := pr.Tools[0]
	if tool.Type != "function" {
		t.Errorf("expected tool type %q, got %q", "function", tool.Type)
	}
	if tool.Function.Name != "get_weather" {
		t.Errorf("expected function name %q, got %q", "get_weather", tool.Function.Name)
	}
	if tool.Function.Description != "Get weather for a city" {
		t.Errorf("expected description, got %q", tool.Function.Description)
	}
}

func TestTranslateRequest_ToolChoice(t *testing.T) {
	tc := api.ToolChoiceRequired
	req := &api.CreateResponseRequest{
		Model:      "m",
		Input:      []api.Item{},
		ToolChoice: &tc,
	}

	pr := translateRequest(req)

	if pr.ToolChoice == nil {
		t.Fatal("expected ToolChoice to be set")
	}
	if pr.ToolChoice.String != "required" {
		t.Errorf("expected ToolChoice %q, got %q", "required", pr.ToolChoice.String)
	}
}

func TestTranslateRequest_Parameters(t *testing.T) {
	temp := 0.7
	topP := 0.9
	maxTokens := 100

	req := &api.CreateResponseRequest{
		Model:           "m",
		Input:           []api.Item{},
		Temperature:     &temp,
		TopP:            &topP,
		MaxOutputTokens: &maxTokens,
		Stream:          true,
	}

	pr := translateRequest(req)

	if pr.Temperature == nil || *pr.Temperature != 0.7 {
		t.Errorf("expected temperature 0.7, got %v", pr.Temperature)
	}
	if pr.TopP == nil || *pr.TopP != 0.9 {
		t.Errorf("expected top_p 0.9, got %v", pr.TopP)
	}
	if pr.MaxTokens == nil || *pr.MaxTokens != 100 {
		t.Errorf("expected max_tokens 100, got %v", pr.MaxTokens)
	}
	if !pr.Stream {
		t.Error("expected stream to be true")
	}
}

func TestTranslateRequest_OmitsNilParameters(t *testing.T) {
	req := &api.CreateResponseRequest{
		Model: "m",
		Input: []api.Item{},
	}

	pr := translateRequest(req)

	if pr.Temperature != nil {
		t.Errorf("expected nil temperature, got %v", pr.Temperature)
	}
	if pr.TopP != nil {
		t.Errorf("expected nil top_p, got %v", pr.TopP)
	}
	if pr.MaxTokens != nil {
		t.Errorf("expected nil max_tokens, got %v", pr.MaxTokens)
	}
	if pr.ToolChoice != nil {
		t.Errorf("expected nil tool_choice, got %v", pr.ToolChoice)
	}
}

func TestTranslateRequest_FullConversation(t *testing.T) {
	req := &api.CreateResponseRequest{
		Model:        "gpt-4",
		Instructions: "Be helpful.",
		Input: []api.Item{
			{
				Type: api.ItemTypeMessage,
				Message: &api.MessageData{
					Role:    api.RoleUser,
					Content: []api.ContentPart{{Type: "input_text", Text: "What is the weather?"}},
				},
			},
			{
				Type: api.ItemTypeFunctionCall,
				FunctionCall: &api.FunctionCallData{
					Name:      "get_weather",
					CallID:    "call_1",
					Arguments: `{"city":"Berlin"}`,
				},
			},
			{
				Type: api.ItemTypeFunctionCallOutput,
				FunctionCallOutput: &api.FunctionCallOutputData{
					CallID: "call_1",
					Output: "20C sunny",
				},
			},
			{
				Type: api.ItemTypeMessage,
				Message: &api.MessageData{
					Role: api.RoleAssistant,
					Output: []api.OutputContentPart{
						{Type: "output_text", Text: "It's 20C and sunny in Berlin."},
					},
				},
			},
			{
				Type: api.ItemTypeReasoning,
				Reasoning: &api.ReasoningData{
					Content: "should not appear",
				},
			},
		},
	}

	pr := translateRequest(req)

	// Instructions + user + function_call + function_call_output + assistant = 5
	// Reasoning is skipped.
	expected := []struct {
		role string
	}{
		{"system"},    // instructions
		{"user"},      // user message
		{"assistant"}, // function_call
		{"tool"},      // function_call_output
		{"assistant"}, // assistant message
	}

	if len(pr.Messages) != len(expected) {
		t.Fatalf("expected %d messages, got %d", len(expected), len(pr.Messages))
	}
	for i, e := range expected {
		if pr.Messages[i].Role != e.role {
			t.Errorf("message[%d]: expected role %q, got %q", i, e.role, pr.Messages[i].Role)
		}
	}

	// Verify the function_call assistant message has tool_calls.
	if len(pr.Messages[2].ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call on message[2], got %d", len(pr.Messages[2].ToolCalls))
	}

	// Verify tool message has tool_call_id.
	if pr.Messages[3].ToolCallID != "call_1" {
		t.Errorf("expected tool_call_id %q, got %q", "call_1", pr.Messages[3].ToolCallID)
	}
}

// Ensure the function compiles with the correct types.
var _ = func() {
	var pr *provider.ProviderRequest
	_ = pr
}
