package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rhuss/antwort/pkg/agent"
	"github.com/rhuss/antwort/pkg/config"
	"github.com/rhuss/antwort/pkg/engine"
	"github.com/rhuss/antwort/pkg/provider/vllm"
	"github.com/rhuss/antwort/pkg/storage/memory"
	"github.com/rhuss/antwort/pkg/tools"
	transporthttp "github.com/rhuss/antwort/pkg/transport/http"
)

// agentTestEnv holds servers for agent profile integration tests.
type agentTestEnv struct {
	server      *httptest.Server
	mockBackend *httptest.Server
}

func (e *agentTestEnv) teardown() {
	if e.server != nil {
		e.server.Close()
	}
	if e.mockBackend != nil {
		e.mockBackend.Close()
	}
}

// setupAgentEnvironment builds a test server with agent profile resolution.
func setupAgentEnvironment(t *testing.T) *agentTestEnv {
	t.Helper()

	mockBackend := startMockBackend()

	prov, err := vllm.New(vllm.Config{BaseURL: mockBackend.URL})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	store := memory.New(100)
	store.SetAuditLogger(nil)

	resolver, err := agent.NewConfigResolver(map[string]config.AgentProfileConfig{
		"helper": {
			Description:  "A helpful assistant",
			Instructions: "You are a helpful assistant",
			Model:        "mock-model",
		},
	})
	if err != nil {
		t.Fatalf("creating resolver: %v", err)
	}

	eng, err := engine.New(prov, store, engine.Config{
		DefaultModel:    "mock-model",
		MaxAgenticTurns: 10,
		Executors:       []tools.ToolExecutor{&mockToolExecutor{}},
		ProfileResolver: resolver,
	})
	if err != nil {
		t.Fatalf("creating engine: %v", err)
	}

	adapter := transporthttp.NewAdapter(eng, store, transporthttp.DefaultConfig())
	adapter.SetAuditLogger(nil)
	adapter.SetProfileResolver(resolver)

	mux := http.NewServeMux()
	mux.Handle("/", adapter.Handler())

	server := httptest.NewServer(mux)

	return &agentTestEnv{
		server:      server,
		mockBackend: mockBackend,
	}
}

func TestAgentProfileResolution(t *testing.T) {
	env := setupAgentEnvironment(t)
	defer env.teardown()

	reqBody := map[string]any{
		"model": "mock-model",
		"agent": "helper",
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

	resp := postJSON(t, env.server.URL+"/v1/responses", reqBody)
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var response responseBody
	decodeJSON(t, resp, &response)

	if response.ID == "" {
		t.Error("response ID is empty")
	}
	if response.Status != "completed" {
		t.Errorf("status = %q, want %q", response.Status, "completed")
	}
}

func TestAgentProfileNotFound(t *testing.T) {
	env := setupAgentEnvironment(t)
	defer env.teardown()

	reqBody := map[string]any{
		"model": "mock-model",
		"agent": "nonexistent",
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

	resp := postJSON(t, env.server.URL+"/v1/responses", reqBody)
	defer resp.Body.Close()

	// Agent profile not found should return an error (400 or 404).
	if resp.StatusCode == http.StatusOK {
		t.Fatal("expected error status for nonexistent agent, got 200")
	}
}

func TestListAgentProfiles(t *testing.T) {
	env := setupAgentEnvironment(t)
	defer env.teardown()

	resp := getURL(t, env.server.URL+"/v1/agents")
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var list struct {
		Object string `json:"object"`
		Data   []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Model       string `json:"model"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decoding list: %v", err)
	}
	resp.Body.Close()

	if list.Object != "list" {
		t.Errorf("object = %q, want %q", list.Object, "list")
	}
	if len(list.Data) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(list.Data))
	}
	if list.Data[0].Name != "helper" {
		t.Errorf("profile name = %q, want %q", list.Data[0].Name, "helper")
	}
	if list.Data[0].Description != "A helpful assistant" {
		t.Errorf("description = %q, want %q", list.Data[0].Description, "A helpful assistant")
	}
}
