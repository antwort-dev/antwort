package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/engine"
	"github.com/rhuss/antwort/pkg/provider/responses"
	"github.com/rhuss/antwort/pkg/storage/memory"
	transporthttp "github.com/rhuss/antwort/pkg/transport/http"
)

// startMockResponsesBackend creates a mock server that speaks the Responses API protocol.
func startMockResponsesBackend() *httptest.Server {
	mux := http.NewServeMux()

	// POST /v1/responses returns a canned response.
	mux.HandleFunc("POST /v1/responses", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":     "resp_mock123",
			"object": "response",
			"status": "completed",
			"model":  "mock-model",
			"output": []map[string]any{
				{
					"id":   "item_1",
					"type": "message",
					"role": "assistant",
					"content": []map[string]any{
						{"type": "output_text", "text": "Hello from responses provider!"},
					},
				},
			},
			"usage": map[string]any{
				"input_tokens":  10,
				"output_tokens": 5,
				"total_tokens":  15,
			},
		})
	})

	// GET /v1/responses returns 405 (probe expects this).
	mux.HandleFunc("GET /v1/responses", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	// GET /v1/models for model listing.
	mux.HandleFunc("GET /v1/models", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data": []map[string]any{
				{"id": "mock-model", "object": "model", "owned_by": "test"},
			},
		})
	})

	return httptest.NewServer(mux)
}

// TestResponsesProviderBasic verifies the Responses API provider works end-to-end.
func TestResponsesProviderBasic(t *testing.T) {
	mockBackend := startMockResponsesBackend()
	defer mockBackend.Close()

	prov, err := responses.New(responses.Config{
		BaseURL: mockBackend.URL,
	})
	if err != nil {
		t.Fatalf("creating responses provider: %v", err)
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
	defer srv.Close()

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
	if len(response.Output) == 0 {
		t.Fatal("expected at least one output item")
	}
}

// TestResponsesProviderConfigValidation verifies that a missing BaseURL returns an error.
func TestResponsesProviderConfigValidation(t *testing.T) {
	_, err := responses.New(responses.Config{})
	if err == nil {
		t.Error("expected error for empty BaseURL, got nil")
	}
}

// TestResponsesProviderProbeFailure verifies the provider still starts if the
// backend probe fails (non-fatal warning).
func TestResponsesProviderProbeFailure(t *testing.T) {
	// Create a server that returns 404 for the probe endpoint.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/responses", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	probeSrv := httptest.NewServer(mux)
	defer probeSrv.Close()

	// Provider should still be created (probe failure is a warning, not an error).
	prov, err := responses.New(responses.Config{
		BaseURL: probeSrv.URL,
	})
	if err != nil {
		t.Fatalf("expected provider creation to succeed despite probe failure, got: %v", err)
	}
	if prov == nil {
		t.Error("expected non-nil provider")
	}
}
