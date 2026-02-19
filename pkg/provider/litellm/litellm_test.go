package litellm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/provider"
	"github.com/rhuss/antwort/pkg/provider/openaicompat"
)

func TestLiteLLMProvider_Name(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	p, err := New(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer p.Close()

	if p.Name() != "litellm" {
		t.Errorf("expected name %q, got %q", "litellm", p.Name())
	}
}

func TestLiteLLMProvider_Capabilities(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	p, err := New(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer p.Close()

	caps := p.Capabilities()
	if !caps.Streaming {
		t.Error("expected streaming to be true")
	}
	if !caps.ToolCalling {
		t.Error("expected tool_calling to be true")
	}
}

func TestLiteLLMProvider_New_MissingBaseURL(t *testing.T) {
	_, err := New(Config{})
	if err == nil {
		t.Fatal("expected error for missing BaseURL")
	}
}

func TestLiteLLMProvider_Complete_TextResponse(t *testing.T) {
	chatResp := openaicompat.ChatCompletionResponse{
		ID:    "chatcmpl-litellm-1",
		Model: "openai/gpt-4",
		Choices: []openaicompat.ChatChoice{
			{
				Index: 0,
				Message: openaicompat.ChatMessage{
					Role:    "assistant",
					Content: "Hello from LiteLLM!",
				},
				FinishReason: "stop",
			},
		},
		Usage: &openaicompat.ChatUsage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected path /v1/chat/completions, got %s", r.URL.Path)
		}
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
		Model: "gpt-4",
		Messages: []provider.ProviderMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	resp, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	if resp.Status != api.ResponseStatusCompleted {
		t.Errorf("expected status %q, got %q", api.ResponseStatusCompleted, resp.Status)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}
	if resp.Items[0].Message.Output[0].Text != "Hello from LiteLLM!" {
		t.Errorf("expected text %q, got %q", "Hello from LiteLLM!", resp.Items[0].Message.Output[0].Text)
	}
}

func TestLiteLLMProvider_ModelMapping(t *testing.T) {
	var receivedModel string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var chatReq openaicompat.ChatCompletionRequest
		json.NewDecoder(r.Body).Decode(&chatReq)
		receivedModel = chatReq.Model

		resp := openaicompat.ChatCompletionResponse{
			Model: chatReq.Model,
			Choices: []openaicompat.ChatChoice{
				{
					Message:      openaicompat.ChatMessage{Role: "assistant", Content: "ok"},
					FinishReason: "stop",
				},
			},
			Usage: &openaicompat.ChatUsage{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p, err := New(Config{
		BaseURL: srv.URL,
		ModelMapping: map[string]string{
			"gpt-4":  "openai/gpt-4",
			"claude": "anthropic/claude-3-opus",
		},
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer p.Close()

	// Test mapped model.
	req := &provider.ProviderRequest{
		Model:    "gpt-4",
		Messages: []provider.ProviderMessage{{Role: "user", Content: "Hi"}},
	}
	_, err = p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if receivedModel != "openai/gpt-4" {
		t.Errorf("expected mapped model %q, got %q", "openai/gpt-4", receivedModel)
	}

	// Test unmapped model (pass-through).
	req.Model = "unknown-model"
	_, err = p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if receivedModel != "unknown-model" {
		t.Errorf("expected pass-through model %q, got %q", "unknown-model", receivedModel)
	}
}

func TestLiteLLMProvider_Complete_AuthorizationHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer litellm-key-123" {
			t.Errorf("expected Authorization %q, got %q", "Bearer litellm-key-123", auth)
		}

		resp := openaicompat.ChatCompletionResponse{
			Model: "m",
			Choices: []openaicompat.ChatChoice{
				{Message: openaicompat.ChatMessage{Role: "assistant", Content: "ok"}, FinishReason: "stop"},
			},
			Usage: &openaicompat.ChatUsage{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p, err := New(Config{BaseURL: srv.URL, APIKey: "litellm-key-123"})
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

func TestLiteLLMProvider_Stream_TextResponse(t *testing.T) {
	sseData := `data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"m","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"m","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"m","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"m","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}

data: [DONE]

`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(sseData))
	}))
	defer srv.Close()

	p, err := New(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer p.Close()

	ch, err := p.Stream(context.Background(), &provider.ProviderRequest{
		Model:    "m",
		Messages: []provider.ProviderMessage{{Role: "user", Content: "Hi"}},
		Stream:   true,
	})
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	var textContent string
	var doneFound bool
	for ev := range ch {
		if ev.Type == provider.ProviderEventTextDelta {
			textContent += ev.Delta
		}
		if ev.Type == provider.ProviderEventDone {
			doneFound = true
		}
	}

	if textContent != "Hello world" {
		t.Errorf("accumulated text = %q, want %q", textContent, "Hello world")
	}
	if !doneFound {
		t.Error("expected Done event, not found")
	}
}

func TestLiteLLMProvider_ListModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("expected path /v1/models, got %s", r.URL.Path)
		}

		resp := openaicompat.ChatModelsResponse{
			Object: "list",
			Data: []openaicompat.ChatModel{
				{ID: "openai/gpt-4", Object: "model", OwnedBy: "openai"},
				{ID: "anthropic/claude-3", Object: "model", OwnedBy: "anthropic"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p, err := New(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer p.Close()

	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
	if models[0].ID != "openai/gpt-4" {
		t.Errorf("model[0].ID = %q, want %q", models[0].ID, "openai/gpt-4")
	}
	if models[1].ID != "anthropic/claude-3" {
		t.Errorf("model[1].ID = %q, want %q", models[1].ID, "anthropic/claude-3")
	}
}

func TestLiteLLMProvider_Complete_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
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
}
