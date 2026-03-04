package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rhuss/antwort/pkg/engine"
	"github.com/rhuss/antwort/pkg/provider/vllm"
	"github.com/rhuss/antwort/pkg/storage/memory"
	"github.com/rhuss/antwort/pkg/tools"
	transporthttp "github.com/rhuss/antwort/pkg/transport/http"
)

// convTestEnv holds servers for conversation integration tests.
type convTestEnv struct {
	server      *httptest.Server
	mockBackend *httptest.Server
}

func (e *convTestEnv) teardown() {
	if e.server != nil {
		e.server.Close()
	}
	if e.mockBackend != nil {
		e.mockBackend.Close()
	}
}

// setupConversationEnvironment builds a test server with a conversation store.
func setupConversationEnvironment(t *testing.T) *convTestEnv {
	t.Helper()

	mockBackend := startMockBackend()

	prov, err := vllm.New(vllm.Config{BaseURL: mockBackend.URL})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	store := memory.New(100)
	store.SetAuditLogger(nil)

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
	adapter.SetConversationStore(memory.NewConversationStore())

	mux := http.NewServeMux()
	mux.Handle("/", adapter.Handler())

	server := httptest.NewServer(mux)

	return &convTestEnv{
		server:      server,
		mockBackend: mockBackend,
	}
}

// conversationResponse is a minimal struct for decoding conversation JSON.
type conversationResponse struct {
	ID        string `json:"id"`
	Object    string `json:"object"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"created_at"`
}

// conversationListResponse is a minimal struct for decoding a conversation list.
type conversationListResponse struct {
	Object string                 `json:"object"`
	Data   []conversationResponse `json:"data"`
}

// deleteResponse is a minimal struct for decoding a delete confirmation.
type deleteResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Deleted bool   `json:"deleted"`
}

func TestCreateConversation(t *testing.T) {
	env := setupConversationEnvironment(t)
	defer env.teardown()

	resp := postJSON(t, env.server.URL+"/v1/conversations", map[string]any{})
	if resp.StatusCode != http.StatusCreated {
		body := readBody(t, resp)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
	}

	var conv conversationResponse
	decodeJSON(t, resp, &conv)

	if conv.ID == "" {
		t.Error("conversation ID is empty")
	}
	if conv.Object != "conversation" {
		t.Errorf("object = %q, want %q", conv.Object, "conversation")
	}
	if conv.CreatedAt == 0 {
		t.Error("created_at is zero")
	}
}

func TestGetConversation(t *testing.T) {
	env := setupConversationEnvironment(t)
	defer env.teardown()

	// Create a conversation.
	createResp := postJSON(t, env.server.URL+"/v1/conversations", map[string]any{
		"name": "test-conv",
	})
	if createResp.StatusCode != http.StatusCreated {
		body := readBody(t, createResp)
		t.Fatalf("create: expected 201, got %d: %s", createResp.StatusCode, body)
	}

	var created conversationResponse
	decodeJSON(t, createResp, &created)

	// Get it.
	getResp := getURL(t, fmt.Sprintf("%s/v1/conversations/%s", env.server.URL, created.ID))
	if getResp.StatusCode != http.StatusOK {
		body := readBody(t, getResp)
		t.Fatalf("get: expected 200, got %d: %s", getResp.StatusCode, body)
	}

	var fetched conversationResponse
	decodeJSON(t, getResp, &fetched)

	if fetched.ID != created.ID {
		t.Errorf("fetched ID = %q, want %q", fetched.ID, created.ID)
	}
	if fetched.Name != "test-conv" {
		t.Errorf("fetched name = %q, want %q", fetched.Name, "test-conv")
	}
}

func TestGetConversationNotFound(t *testing.T) {
	env := setupConversationEnvironment(t)
	defer env.teardown()

	resp := getURL(t, env.server.URL+"/v1/conversations/conv_nonexistent")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		body := readBody(t, resp)
		t.Fatalf("expected 404, got %d: %s", resp.StatusCode, body)
	}
}

func TestListConversations(t *testing.T) {
	env := setupConversationEnvironment(t)
	defer env.teardown()

	// Create 2 conversations.
	for i := 0; i < 2; i++ {
		resp := postJSON(t, env.server.URL+"/v1/conversations", map[string]any{
			"name": fmt.Sprintf("conv-%d", i),
		})
		if resp.StatusCode != http.StatusCreated {
			body := readBody(t, resp)
			t.Fatalf("create %d: expected 201, got %d: %s", i, resp.StatusCode, body)
		}
		resp.Body.Close()
	}

	// List all.
	listResp := getURL(t, env.server.URL+"/v1/conversations")
	if listResp.StatusCode != http.StatusOK {
		body := readBody(t, listResp)
		t.Fatalf("list: expected 200, got %d: %s", listResp.StatusCode, body)
	}

	var list conversationListResponse
	if err := json.NewDecoder(listResp.Body).Decode(&list); err != nil {
		t.Fatalf("decoding list: %v", err)
	}
	listResp.Body.Close()

	if list.Object != "list" {
		t.Errorf("list object = %q, want %q", list.Object, "list")
	}
	if len(list.Data) != 2 {
		t.Errorf("list length = %d, want 2", len(list.Data))
	}
}

func TestDeleteConversation(t *testing.T) {
	env := setupConversationEnvironment(t)
	defer env.teardown()

	// Create a conversation.
	createResp := postJSON(t, env.server.URL+"/v1/conversations", map[string]any{})
	if createResp.StatusCode != http.StatusCreated {
		body := readBody(t, createResp)
		t.Fatalf("create: expected 201, got %d: %s", createResp.StatusCode, body)
	}

	var created conversationResponse
	decodeJSON(t, createResp, &created)

	convURL := fmt.Sprintf("%s/v1/conversations/%s", env.server.URL, created.ID)

	// Delete it.
	delResp := deleteURL(t, convURL)
	if delResp.StatusCode != http.StatusOK {
		body := readBody(t, delResp)
		t.Fatalf("delete: expected 200, got %d: %s", delResp.StatusCode, body)
	}

	var del deleteResponse
	decodeJSON(t, delResp, &del)

	if !del.Deleted {
		t.Error("deleted field should be true")
	}

	// Verify it is gone.
	getResp := getURL(t, convURL)
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusNotFound {
		body := readBody(t, getResp)
		t.Fatalf("get after delete: expected 404, got %d: %s", getResp.StatusCode, body)
	}
}
