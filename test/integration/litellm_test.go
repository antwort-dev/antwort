package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/engine"
	"github.com/rhuss/antwort/pkg/provider/litellm"
	"github.com/rhuss/antwort/pkg/storage/memory"
	transporthttp "github.com/rhuss/antwort/pkg/transport/http"
)

// setupLiteLLMEnv creates a test environment using the LiteLLM provider
// backed by the same mock Chat Completions backend.
func setupLiteLLMEnv(t *testing.T, cfg litellm.Config) *httptest.Server {
	t.Helper()

	prov, err := litellm.New(cfg)
	if err != nil {
		t.Fatalf("creating LiteLLM provider: %v", err)
	}

	store := memory.New(100)
	eng, err := engine.New(prov, store, engine.Config{
		DefaultModel: "mock-model",
	})
	if err != nil {
		t.Fatalf("creating engine: %v", err)
	}

	adapter := transporthttp.NewAdapter(eng, store, transporthttp.DefaultConfig())
	mux := http.NewServeMux()
	mux.Handle("/", adapter.Handler())

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// TestLiteLLMProviderBasic verifies the LiteLLM provider works end-to-end
// using the same mock Chat Completions backend.
func TestLiteLLMProviderBasic(t *testing.T) {
	mock := startMockBackend()
	defer mock.Close()

	srv := setupLiteLLMEnv(t, litellm.Config{BaseURL: mock.URL})

	reqBody := map[string]any{
		"model": "mock-model",
		"input": []map[string]any{
			{
				"type": "message",
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "Hello"},
				},
			},
		},
	}

	resp := postJSON(t, srv.URL+"/v1/responses", reqBody)
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var response api.Response
	decodeJSON(t, resp, &response)

	if response.ID == "" {
		t.Error("response ID is empty")
	}
	if response.Status != api.ResponseStatusCompleted {
		t.Errorf("status = %q, want %q", response.Status, api.ResponseStatusCompleted)
	}
}

// TestLiteLLMProviderModelMapping verifies model name mapping works correctly.
func TestLiteLLMProviderModelMapping(t *testing.T) {
	mock := startMockBackend()
	defer mock.Close()

	srv := setupLiteLLMEnv(t, litellm.Config{
		BaseURL: mock.URL,
		ModelMapping: map[string]string{
			"my-alias": "mock-model",
		},
	})

	reqBody := map[string]any{
		"model": "my-alias",
		"input": []map[string]any{
			{
				"type": "message",
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "Hello"},
				},
			},
		},
	}

	resp := postJSON(t, srv.URL+"/v1/responses", reqBody)
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var response api.Response
	decodeJSON(t, resp, &response)

	if response.Status != api.ResponseStatusCompleted {
		t.Errorf("status = %q, want %q", response.Status, api.ResponseStatusCompleted)
	}
}

// TestLiteLLMProviderConfigValidation verifies that a missing BaseURL returns an error.
func TestLiteLLMProviderConfigValidation(t *testing.T) {
	_, err := litellm.New(litellm.Config{})
	if err == nil {
		t.Error("expected error for empty BaseURL, got nil")
	}
}
