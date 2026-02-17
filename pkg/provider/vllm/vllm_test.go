package vllm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/provider"
)

func TestVLLMProvider_Complete_TextResponse(t *testing.T) {
	// Set up a test server that returns a fixed chatCompletionResponse.
	chatResp := chatCompletionResponse{
		ID:    "chatcmpl-test-123",
		Model: "test-model",
		Choices: []chatChoice{
			{
				Index: 0,
				Message: chatMessage{
					Role:    "assistant",
					Content: "Hello! How can I help you today?",
				},
				FinishReason: "stop",
			},
		},
		Usage: &chatUsage{
			PromptTokens:     12,
			CompletionTokens: 9,
			TotalTokens:      21,
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request.
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected path /v1/chat/completions, got %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Parse the request body to verify translation.
		var chatReq chatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&chatReq); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		if chatReq.Model != "test-model" {
			t.Errorf("expected model %q, got %q", "test-model", chatReq.Model)
		}
		if chatReq.N != 1 {
			t.Errorf("expected N=1, got %d", chatReq.N)
		}
		if chatReq.Stream {
			t.Error("expected stream to be false")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResp)
	}))
	defer srv.Close()

	// Create provider.
	cfg := Config{
		BaseURL: srv.URL,
	}
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer p.Close()

	// Verify Name and Capabilities.
	if p.Name() != "vllm" {
		t.Errorf("expected name %q, got %q", "vllm", p.Name())
	}
	caps := p.Capabilities()
	if !caps.Streaming || !caps.ToolCalling {
		t.Errorf("expected streaming and tool_calling to be true, got streaming=%v, tool_calling=%v",
			caps.Streaming, caps.ToolCalling)
	}

	// Make request.
	req := &provider.ProviderRequest{
		Model: "test-model",
		Messages: []provider.ProviderMessage{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello"},
		},
	}

	resp, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	// Verify response.
	if resp.Model != "test-model" {
		t.Errorf("expected model %q, got %q", "test-model", resp.Model)
	}
	if resp.Status != api.ResponseStatusCompleted {
		t.Errorf("expected status %q, got %q", api.ResponseStatusCompleted, resp.Status)
	}
	if resp.Usage.InputTokens != 12 {
		t.Errorf("expected input_tokens 12, got %d", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 9 {
		t.Errorf("expected output_tokens 9, got %d", resp.Usage.OutputTokens)
	}

	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}

	item := resp.Items[0]
	if item.Type != api.ItemTypeMessage {
		t.Errorf("expected type %q, got %q", api.ItemTypeMessage, item.Type)
	}
	if item.Message == nil || len(item.Message.Output) != 1 {
		t.Fatal("expected message with 1 output part")
	}
	if item.Message.Output[0].Text != "Hello! How can I help you today?" {
		t.Errorf("expected text %q, got %q", "Hello! How can I help you today?", item.Message.Output[0].Text)
	}
}

func TestVLLMProvider_Complete_ToolCallResponse(t *testing.T) {
	chatResp := chatCompletionResponse{
		ID:    "chatcmpl-tc-1",
		Model: "tool-model",
		Choices: []chatChoice{
			{
				Index: 0,
				Message: chatMessage{
					Role:    "assistant",
					Content: nil,
					ToolCalls: []chatToolCall{
						{
							ID:   "call_weather_1",
							Type: "function",
							Function: chatFunctionCall{
								Name:      "get_weather",
								Arguments: `{"location":"Berlin"}`,
							},
						},
					},
				},
				FinishReason: "tool_calls",
			},
		},
		Usage: &chatUsage{PromptTokens: 20, CompletionTokens: 15, TotalTokens: 35},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResp)
	}))
	defer srv.Close()

	p, err := New(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer p.Close()

	req := &provider.ProviderRequest{
		Model: "tool-model",
		Messages: []provider.ProviderMessage{
			{Role: "user", Content: "What's the weather in Berlin?"},
		},
		Tools: []provider.ProviderTool{
			{
				Type: "function",
				Function: provider.ProviderFunctionDef{
					Name:        "get_weather",
					Description: "Get weather",
				},
			},
		},
	}

	resp, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}
	item := resp.Items[0]
	if item.Type != api.ItemTypeFunctionCall {
		t.Errorf("expected type %q, got %q", api.ItemTypeFunctionCall, item.Type)
	}
	if item.FunctionCall.Name != "get_weather" {
		t.Errorf("expected function name %q, got %q", "get_weather", item.FunctionCall.Name)
	}
	if item.FunctionCall.CallID != "call_weather_1" {
		t.Errorf("expected call_id %q, got %q", "call_weather_1", item.FunctionCall.CallID)
	}
}

func TestVLLMProvider_Complete_AuthorizationHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key-123" {
			t.Errorf("expected Authorization %q, got %q", "Bearer test-key-123", auth)
		}

		resp := chatCompletionResponse{
			Model: "m",
			Choices: []chatChoice{
				{Message: chatMessage{Role: "assistant", Content: "ok"}, FinishReason: "stop"},
			},
			Usage: &chatUsage{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p, err := New(Config{BaseURL: srv.URL, APIKey: "test-key-123"})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer p.Close()

	req := &provider.ProviderRequest{
		Model:    "m",
		Messages: []provider.ProviderMessage{{Role: "user", Content: "Hi"}},
	}

	_, err = p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
}

func TestVLLMProvider_Complete_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(chatErrorResponse{
			Error: struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    any    `json:"code"`
			}{
				Message: "internal server error",
				Type:    "server_error",
			},
		})
	}))
	defer srv.Close()

	p, err := New(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer p.Close()

	req := &provider.ProviderRequest{
		Model:    "m",
		Messages: []provider.ProviderMessage{{Role: "user", Content: "Hi"}},
	}

	_, err = p.Complete(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}

	apiErr, ok := err.(*api.APIError)
	if !ok {
		t.Fatalf("expected *api.APIError, got %T", err)
	}
	if apiErr.Type != api.ErrorTypeServerError {
		t.Errorf("expected error type %q, got %q", api.ErrorTypeServerError, apiErr.Type)
	}
	if apiErr.Message != "internal server error" {
		t.Errorf("expected error message %q, got %q", "internal server error", apiErr.Message)
	}
}

func TestVLLMProvider_Complete_RateLimitError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	p, err := New(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer p.Close()

	req := &provider.ProviderRequest{
		Model:    "m",
		Messages: []provider.ProviderMessage{{Role: "user", Content: "Hi"}},
	}

	_, err = p.Complete(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for 429 response")
	}

	apiErr, ok := err.(*api.APIError)
	if !ok {
		t.Fatalf("expected *api.APIError, got %T", err)
	}
	if apiErr.Type != api.ErrorTypeTooManyRequests {
		t.Errorf("expected error type %q, got %q", api.ErrorTypeTooManyRequests, apiErr.Type)
	}
}

func TestVLLMProvider_Complete_ConnectionRefused(t *testing.T) {
	// Point at a URL that will refuse connections.
	p, err := New(Config{BaseURL: "http://127.0.0.1:1"})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer p.Close()

	req := &provider.ProviderRequest{
		Model:    "m",
		Messages: []provider.ProviderMessage{{Role: "user", Content: "Hi"}},
	}

	_, err = p.Complete(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for connection refused")
	}

	apiErr, ok := err.(*api.APIError)
	if !ok {
		t.Fatalf("expected *api.APIError, got %T", err)
	}
	if apiErr.Type != api.ErrorTypeServerError {
		t.Errorf("expected error type %q, got %q", api.ErrorTypeServerError, apiErr.Type)
	}
}

func TestVLLMProvider_New_MissingBaseURL(t *testing.T) {
	_, err := New(Config{})
	if err == nil {
		t.Fatal("expected error for missing BaseURL")
	}
}

func TestVLLMProvider_Stream_NotImplemented(t *testing.T) {
	p, err := New(Config{BaseURL: "http://localhost:8000"})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	_, err = p.Stream(context.Background(), &provider.ProviderRequest{})
	if err == nil {
		t.Fatal("expected error for unimplemented streaming")
	}
}

func TestVLLMProvider_ListModels_Stub(t *testing.T) {
	p, err := New(Config{BaseURL: "http://localhost:8000"})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels should not error: %v", err)
	}
	if models != nil {
		t.Errorf("expected nil models, got %v", models)
	}
}
