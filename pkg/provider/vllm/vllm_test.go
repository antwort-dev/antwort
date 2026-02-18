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

func TestVLLMProvider_Stream_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p, err := New(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer p.Close()

	_, err = p.Stream(context.Background(), &provider.ProviderRequest{
		Model:    "m",
		Messages: []provider.ProviderMessage{{Role: "user", Content: "Hi"}},
	})
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

func TestVLLMProvider_ListModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/models" {
			t.Errorf("expected path /v1/models, got %s", r.URL.Path)
		}

		resp := chatModelsResponse{
			Object: "list",
			Data: []chatModel{
				{ID: "meta-llama/Llama-3-8B", Object: "model", OwnedBy: "meta"},
				{ID: "mistral-7b", Object: "model", OwnedBy: "mistral"},
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
	if models[0].ID != "meta-llama/Llama-3-8B" {
		t.Errorf("model[0].ID = %q, want %q", models[0].ID, "meta-llama/Llama-3-8B")
	}
	if models[1].ID != "mistral-7b" {
		t.Errorf("model[1].ID = %q, want %q", models[1].ID, "mistral-7b")
	}
}

// T026: vLLM streaming integration tests with mock HTTP server.

func TestVLLMProvider_Stream_TextResponse(t *testing.T) {
	// Mock server returning Chat Completions SSE chunks for a text response.
	sseData := `data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"test-model","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"test-model","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"test-model","choices":[{"index":0,"delta":{"content":" there"},"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"test-model","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":3,"total_tokens":13}}

data: [DONE]

`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify streaming request properties.
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected path /v1/chat/completions, got %s", r.URL.Path)
		}

		// Verify that stream=true and stream_options are set.
		var chatReq chatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&chatReq); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		if !chatReq.Stream {
			t.Error("expected stream=true in request")
		}
		if chatReq.StreamOptions == nil || !chatReq.StreamOptions.IncludeUsage {
			t.Error("expected stream_options.include_usage=true")
		}

		// Send SSE response.
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(sseData))
	}))
	defer srv.Close()

	p, err := New(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer p.Close()

	req := &provider.ProviderRequest{
		Model: "test-model",
		Messages: []provider.ProviderMessage{
			{Role: "user", Content: "Hi"},
		},
		Stream: true,
	}

	ch, err := p.Stream(context.Background(), req)
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	// Collect all events.
	var events []provider.ProviderEvent
	for ev := range ch {
		events = append(events, ev)
	}

	// Verify event sequence.
	if len(events) < 5 {
		t.Fatalf("expected at least 5 events, got %d", len(events))
	}

	// First event: role-only text delta (empty).
	if events[0].Type != provider.ProviderEventTextDelta {
		t.Errorf("event[0] type = %d, want TextDelta", events[0].Type)
	}

	// Text deltas.
	var textContent string
	for _, ev := range events {
		if ev.Type == provider.ProviderEventTextDelta {
			textContent += ev.Delta
		}
	}
	if textContent != "Hello there!" {
		t.Errorf("accumulated text = %q, want %q", textContent, "Hello there!")
	}

	// Verify there's a TextDone event.
	var textDoneFound bool
	for _, ev := range events {
		if ev.Type == provider.ProviderEventTextDone {
			textDoneFound = true
		}
	}
	if !textDoneFound {
		t.Error("expected TextDone event, not found")
	}

	// Last event should be Done with usage.
	last := events[len(events)-1]
	if last.Type != provider.ProviderEventDone {
		t.Errorf("last event type = %d, want Done", last.Type)
	}
	if last.Usage == nil {
		t.Fatal("expected usage in done event")
	}
	if last.Usage.InputTokens != 10 {
		t.Errorf("usage input_tokens = %d, want 10", last.Usage.InputTokens)
	}
	if last.Usage.OutputTokens != 3 {
		t.Errorf("usage output_tokens = %d, want 3", last.Usage.OutputTokens)
	}
}

func TestVLLMProvider_Stream_AuthorizationHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer stream-key-42" {
			t.Errorf("expected Authorization %q, got %q", "Bearer stream-key-42", auth)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"ok\"},\"finish_reason\":null}]}\n\ndata: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n"))
	}))
	defer srv.Close()

	p, err := New(Config{BaseURL: srv.URL, APIKey: "stream-key-42"})
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

	// Drain the channel.
	for range ch {
	}
}

func TestVLLMProvider_Stream_ContextCancellation(t *testing.T) {
	// Server that sends chunks slowly.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected http.Flusher")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// Send first chunk.
		w.Write([]byte("data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hi\"},\"finish_reason\":null}]}\n\n"))
		flusher.Flush()

		// Wait for context cancellation (the client will disconnect).
		<-r.Context().Done()
	}))
	defer srv.Close()

	p, err := New(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer p.Close()

	ctx, cancel := context.WithCancel(context.Background())

	ch, err := p.Stream(ctx, &provider.ProviderRequest{
		Model:    "m",
		Messages: []provider.ProviderMessage{{Role: "user", Content: "Hi"}},
		Stream:   true,
	})
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	// Read at least one event, then cancel.
	ev := <-ch
	if ev.Type != provider.ProviderEventTextDelta {
		t.Errorf("first event type = %d, want TextDelta", ev.Type)
	}

	cancel()

	// Drain remaining events. The channel should close.
	var count int
	for range ch {
		count++
	}
	// Should close without hanging.
	t.Logf("received %d events after cancellation", count)
}

func TestVLLMProvider_Stream_FinishReasonLength(t *testing.T) {
	sseData := `data: {"id":"1","object":"chat.completion.chunk","model":"m","choices":[{"index":0,"delta":{"content":"truncated"},"finish_reason":null}]}

data: {"id":"1","object":"chat.completion.chunk","model":"m","choices":[{"index":0,"delta":{},"finish_reason":"length"}]}

data: [DONE]

`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
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

	var events []provider.ProviderEvent
	for ev := range ch {
		events = append(events, ev)
	}

	// Find the done event and verify incomplete status.
	var doneFound bool
	for _, ev := range events {
		if ev.Type == provider.ProviderEventDone && ev.Item != nil {
			doneFound = true
			if ev.Item.Status != api.ItemStatusIncomplete {
				t.Errorf("item status = %q, want %q", ev.Item.Status, api.ItemStatusIncomplete)
			}
		}
	}
	if !doneFound {
		t.Error("expected Done event with incomplete item status")
	}
}
