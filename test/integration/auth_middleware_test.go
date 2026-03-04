package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rhuss/antwort/pkg/auth"
	"github.com/rhuss/antwort/pkg/auth/apikey"
	"github.com/rhuss/antwort/pkg/engine"
	"github.com/rhuss/antwort/pkg/provider/vllm"
	"github.com/rhuss/antwort/pkg/storage/memory"
	"github.com/rhuss/antwort/pkg/tools"
	transporthttp "github.com/rhuss/antwort/pkg/transport/http"
)

// authTestEnv holds servers for auth integration tests.
type authTestEnv struct {
	server      *httptest.Server
	mockBackend *httptest.Server
}

func (e *authTestEnv) teardown() {
	if e.server != nil {
		e.server.Close()
	}
	if e.mockBackend != nil {
		e.mockBackend.Close()
	}
}

// setupAuthEnvironment builds a test server with API key auth for two users.
func setupAuthEnvironment(t *testing.T) *authTestEnv {
	t.Helper()

	mockBackend := startMockBackend()

	prov, err := vllm.New(vllm.Config{BaseURL: mockBackend.URL})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	store := memory.New(100)
	store.SetAuditLogger(nil)

	authenticator := apikey.New([]apikey.RawKeyEntry{
		{
			Key:      "alice-key",
			Identity: auth.Identity{Subject: "alice"},
		},
		{
			Key:      "bob-key",
			Identity: auth.Identity{Subject: "bob"},
		},
	})

	chain := &auth.AuthChain{
		Authenticators:  []auth.Authenticator{authenticator},
		DefaultDecision: auth.No,
	}

	eng, err := engine.New(prov, store, engine.Config{
		DefaultModel:    "mock-model",
		MaxAgenticTurns: 10,
		Executors:       []tools.ToolExecutor{&mockToolExecutor{}},
	})
	if err != nil {
		t.Fatalf("creating engine: %v", err)
	}

	adapter := transporthttp.NewAdapter(eng, store, transporthttp.DefaultConfig())
	adapter.SetAuditLogger(nil)

	authMiddleware := auth.Middleware(chain, nil, auth.DefaultBypassEndpoints, nil)

	mux := http.NewServeMux()
	mux.Handle("/", authMiddleware(adapter.Handler()))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok\n"))
	})

	server := httptest.NewServer(mux)

	return &authTestEnv{
		server:      server,
		mockBackend: mockBackend,
	}
}

// authPostJSON sends a POST with JSON body and an Authorization header.
func authPostJSON(t *testing.T, url, apiKey string, body any) *http.Response {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshaling request: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

// authGetURL sends a GET request with an Authorization header.
func authGetURL(t *testing.T, url, apiKey string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return resp
}

// authDeleteURL sends a DELETE request with an Authorization header.
func authDeleteURL(t *testing.T, url, apiKey string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", url, err)
	}
	return resp
}

// responseBody is a minimal struct for decoding response JSON.
type responseBody struct {
	ID     string `json:"id"`
	Object string `json:"object"`
	Status string `json:"status"`
}

func newResponseRequest() map[string]any {
	return map[string]any{
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
}

// Spec 007: Valid API key is accepted.
func TestAuthAPIKeyAccepted(t *testing.T) {
	env := setupAuthEnvironment(t)
	defer env.teardown()

	resp := authPostJSON(t, env.server.URL+"/v1/responses", "alice-key", newResponseRequest())
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
}

// Spec 007: Invalid API key is rejected with 401.
func TestAuthAPIKeyRejected(t *testing.T) {
	env := setupAuthEnvironment(t)
	defer env.teardown()

	resp := authPostJSON(t, env.server.URL+"/v1/responses", "wrong-key", newResponseRequest())
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		body := readBody(t, resp)
		t.Fatalf("expected 401, got %d: %s", resp.StatusCode, body)
	}
}

// Spec 007: Missing Authorization header results in 401.
func TestAuthNoKeyRequired401(t *testing.T) {
	env := setupAuthEnvironment(t)
	defer env.teardown()

	data, _ := json.Marshal(newResponseRequest())
	resp, err := http.Post(env.server.URL+"/v1/responses", "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		body := readBody(t, resp)
		t.Fatalf("expected 401, got %d: %s", resp.StatusCode, body)
	}
}

// Spec 040: Owner isolation prevents cross-user access.
func TestOwnershipIsolation(t *testing.T) {
	env := setupAuthEnvironment(t)
	defer env.teardown()

	// Alice creates a response.
	resp := authPostJSON(t, env.server.URL+"/v1/responses", "alice-key", newResponseRequest())
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("alice create: expected 200, got %d: %s", resp.StatusCode, body)
	}

	var created responseBody
	decodeJSON(t, resp, &created)
	if created.ID == "" {
		t.Fatal("created response has empty ID")
	}

	responseURL := fmt.Sprintf("%s/v1/responses/%s", env.server.URL, created.ID)

	// Bob tries to access Alice's response: should get 404 (ownership denied).
	bobResp := authGetURL(t, responseURL, "bob-key")
	defer bobResp.Body.Close()

	if bobResp.StatusCode != http.StatusNotFound {
		body := readBody(t, bobResp)
		t.Fatalf("bob access: expected 404, got %d: %s", bobResp.StatusCode, body)
	}

	// Alice can access her own response.
	aliceResp := authGetURL(t, responseURL, "alice-key")
	defer aliceResp.Body.Close()

	if aliceResp.StatusCode != http.StatusOK {
		body := readBody(t, aliceResp)
		t.Fatalf("alice access: expected 200, got %d: %s", aliceResp.StatusCode, body)
	}
}

// Spec 041: Verify the full auth middleware chain works end-to-end (create, get, delete).
func TestAuthMiddlewareChainWorks(t *testing.T) {
	env := setupAuthEnvironment(t)
	defer env.teardown()

	// Create a response.
	resp := authPostJSON(t, env.server.URL+"/v1/responses", "alice-key", newResponseRequest())
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("create: expected 200, got %d: %s", resp.StatusCode, body)
	}

	var created responseBody
	decodeJSON(t, resp, &created)

	responseURL := fmt.Sprintf("%s/v1/responses/%s", env.server.URL, created.ID)

	// Get the response.
	getResp := authGetURL(t, responseURL, "alice-key")
	if getResp.StatusCode != http.StatusOK {
		body := readBody(t, getResp)
		t.Fatalf("get: expected 200, got %d: %s", getResp.StatusCode, body)
	}
	getResp.Body.Close()

	// Delete the response.
	delResp := authDeleteURL(t, responseURL, "alice-key")
	defer delResp.Body.Close()

	if delResp.StatusCode != http.StatusNoContent {
		body := readBody(t, delResp)
		t.Fatalf("delete: expected 204, got %d: %s", delResp.StatusCode, body)
	}

	// Verify it is gone.
	goneResp := authGetURL(t, responseURL, "alice-key")
	defer goneResp.Body.Close()

	if goneResp.StatusCode != http.StatusNotFound {
		body := readBody(t, goneResp)
		t.Fatalf("get after delete: expected 404, got %d: %s", goneResp.StatusCode, body)
	}
}
