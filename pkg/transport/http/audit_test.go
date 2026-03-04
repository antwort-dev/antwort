package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/audit"
	"github.com/rhuss/antwort/pkg/transport"
)

// auditCapture collects audit log entries for testing.
type auditCapture struct {
	buf bytes.Buffer
}

func newAuditCapture() (*auditCapture, *audit.Logger) {
	ac := &auditCapture{}
	handler := slog.NewJSONHandler(&ac.buf, nil)
	logger := audit.NewFromHandler(handler)
	return ac, logger
}

// entries returns parsed JSON log entries.
func (ac *auditCapture) entries() []map[string]any {
	var entries []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(ac.buf.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err == nil {
			entries = append(entries, m)
		}
	}
	return entries
}

func TestAuditResponseCreated(t *testing.T) {
	respID := "resp_testABC12345678901234567"
	creator := &mockCreator{
		response: &api.Response{
			ID:     respID,
			Object: "response",
			Status: api.ResponseStatusCompleted,
			Model:  "test-model",
		},
	}

	store := &mockStore{}
	adapter := newTestAdapter(creator, store)
	ac, logger := newAuditCapture()
	adapter.SetAuditLogger(logger)

	srv := httptest.NewServer(adapter.Handler())
	defer srv.Close()

	req := api.CreateResponseRequest{
		Model: "test-model",
		Input: []api.Item{{Type: api.ItemTypeMessage}},
	}
	resp := postJSON(t, srv, req)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	entries := ac.entries()
	if len(entries) == 0 {
		t.Fatal("expected audit log entry for resource.created")
	}

	found := false
	for _, e := range entries {
		if e["event"] == "resource.created" && e["resource_type"] == "response" && e["resource_id"] == respID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("audit log missing resource.created event for response %s, entries: %v", respID, entries)
	}
}

func TestAuditResponseDeleted(t *testing.T) {
	respID := "resp_testABC12345678901234567"
	store := &mockStore{
		responses: map[string]*api.Response{
			respID: {ID: respID, Object: "response", Status: api.ResponseStatusCompleted},
		},
	}
	creator := &mockCreator{}
	adapter := newTestAdapter(creator, store)
	ac, logger := newAuditCapture()
	adapter.SetAuditLogger(logger)

	srv := httptest.NewServer(adapter.Handler())
	defer srv.Close()

	req, _ := http.NewRequest("DELETE", srv.URL+"/v1/responses/"+respID, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE error: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	entries := ac.entries()
	found := false
	for _, e := range entries {
		if e["event"] == "resource.deleted" && e["resource_type"] == "response" && e["resource_id"] == respID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("audit log missing resource.deleted event for response %s", respID)
	}
}

func TestAuditConversationCreatedDeleted(t *testing.T) {
	creator := &mockCreator{}
	store := &mockStore{}
	adapter := newTestAdapter(creator, store)
	ac, logger := newAuditCapture()
	adapter.SetAuditLogger(logger)

	convStore := &mockConvStore{conversations: make(map[string]*api.Conversation)}
	adapter.SetConversationStore(convStore)

	srv := httptest.NewServer(adapter.Handler())
	defer srv.Close()

	// Create conversation.
	body := `{"name": "test-conv"}`
	resp, err := http.Post(srv.URL+"/v1/conversations", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST error: %v", err)
	}
	var conv api.Conversation
	json.NewDecoder(resp.Body).Decode(&conv)
	resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	// Check created event.
	entries := ac.entries()
	createdFound := false
	for _, e := range entries {
		if e["event"] == "resource.created" && e["resource_type"] == "conversation" {
			createdFound = true
			break
		}
	}
	if !createdFound {
		t.Error("audit log missing resource.created event for conversation")
	}

	// Delete conversation.
	req, _ := http.NewRequest("DELETE", srv.URL+"/v1/conversations/"+conv.ID, nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE error: %v", err)
	}
	resp.Body.Close()

	// Check deleted event.
	entries = ac.entries()
	deletedFound := false
	for _, e := range entries {
		if e["event"] == "resource.deleted" && e["resource_type"] == "conversation" && e["resource_id"] == conv.ID {
			deletedFound = true
			break
		}
	}
	if !deletedFound {
		t.Error("audit log missing resource.deleted event for conversation")
	}
}

func TestAuditNilLoggerNoOp(t *testing.T) {
	respID := "resp_testABC12345678901234567"
	creator := &mockCreator{
		response: &api.Response{
			ID:     respID,
			Object: "response",
			Status: api.ResponseStatusCompleted,
			Model:  "test-model",
		},
	}

	// Do not set an audit logger (nil by default).
	adapter := newTestAdapter(creator, &mockStore{})
	srv := httptest.NewServer(adapter.Handler())
	defer srv.Close()

	req := api.CreateResponseRequest{
		Model: "test-model",
		Input: []api.Item{{Type: api.ItemTypeMessage}},
	}
	resp := postJSON(t, srv, req)
	resp.Body.Close()

	// Should not panic with nil audit logger.
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestAuditStreamingResponseCreated(t *testing.T) {
	respID := "resp_testABC12345678901234567"
	creator := &mockCreator{
		events: []api.StreamEvent{
			{
				Type: api.EventResponseCreated,
				Response: &api.Response{
					ID:     respID,
					Object: "response",
					Status: api.ResponseStatusInProgress,
				},
			},
			{
				Type: api.EventResponseCompleted,
				Response: &api.Response{
					ID:     respID,
					Object: "response",
					Status: api.ResponseStatusCompleted,
				},
			},
		},
	}

	adapter := newTestAdapter(creator, &mockStore{})
	ac, logger := newAuditCapture()
	adapter.SetAuditLogger(logger)

	srv := httptest.NewServer(adapter.Handler())
	defer srv.Close()

	req := api.CreateResponseRequest{
		Model:  "test-model",
		Stream: true,
		Input:  []api.Item{{Type: api.ItemTypeMessage}},
	}
	data, _ := json.Marshal(req)
	resp, err := http.Post(srv.URL+"/v1/responses", "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("POST error: %v", err)
	}
	// Read full body to ensure handler completes.
	io.ReadAll(resp.Body)
	resp.Body.Close()

	entries := ac.entries()
	found := false
	for _, e := range entries {
		if e["event"] == "resource.created" && e["resource_type"] == "response" && e["resource_id"] == respID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("audit log missing resource.created event for streaming response %s, entries: %v", respID, entries)
	}
}

// mockConvStore is a simple in-memory conversation store for tests.
type mockConvStore struct {
	conversations map[string]*api.Conversation
}

func (m *mockConvStore) SaveConversation(_ context.Context, conv *api.Conversation) error {
	m.conversations[conv.ID] = conv
	return nil
}

func (m *mockConvStore) GetConversation(_ context.Context, id string) (*api.Conversation, error) {
	conv, ok := m.conversations[id]
	if !ok {
		return nil, api.NewNotFoundError("conversation not found")
	}
	return conv, nil
}

func (m *mockConvStore) ListConversations(_ context.Context, _ transport.ListOptions) (*transport.ConversationList, error) {
	return &transport.ConversationList{Object: "list"}, nil
}

func (m *mockConvStore) DeleteConversation(_ context.Context, id string) error {
	if _, ok := m.conversations[id]; !ok {
		return api.NewNotFoundError("conversation not found")
	}
	delete(m.conversations, id)
	return nil
}

func (m *mockConvStore) AddItems(_ context.Context, _ string, _ []api.ConversationItem) error {
	return nil
}

func (m *mockConvStore) ListItems(_ context.Context, _ string, _ transport.ListOptions) (*transport.ItemList, error) {
	return &transport.ItemList{Object: "list"}, nil
}

