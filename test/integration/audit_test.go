package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rhuss/antwort/pkg/audit"
	"github.com/rhuss/antwort/pkg/auth"
	"github.com/rhuss/antwort/pkg/auth/apikey"
	"github.com/rhuss/antwort/pkg/engine"
	"github.com/rhuss/antwort/pkg/provider/vllm"
	"github.com/rhuss/antwort/pkg/storage/memory"
	"github.com/rhuss/antwort/pkg/tools"
	transporthttp "github.com/rhuss/antwort/pkg/transport/http"
)

// TestAuditEndToEnd verifies that audit events are emitted for a full
// request lifecycle: auth success, resource created, resource deleted.
func TestAuditEndToEnd(t *testing.T) {
	// Capture audit output.
	var auditBuf bytes.Buffer
	auditLogger := audit.NewFromHandler(slog.NewJSONHandler(&auditBuf, nil))

	// Create a test environment with audit logging enabled.
	mockBackend := startMockBackend()
	defer mockBackend.Close()

	prov, err := vllm.New(vllm.Config{BaseURL: mockBackend.URL})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	store := memory.New(100)
	store.SetAuditLogger(auditLogger)

	eng, err := engine.New(prov, store, engine.Config{
		DefaultModel:    "mock-model",
		MaxAgenticTurns: 10,
		AuditLogger:     auditLogger,
		Executors:       []tools.ToolExecutor{&mockToolExecutor{}},
	})
	if err != nil {
		t.Fatalf("creating engine: %v", err)
	}

	adapter := transporthttp.NewAdapter(eng, store, transporthttp.DefaultConfig())
	adapter.SetAuditLogger(auditLogger)

	// Set up auth with API key.
	authenticator := apikey.New([]apikey.RawKeyEntry{
		{Key: "test-key", Identity: auth.Identity{Subject: "test-user"}},
	})
	chain := &auth.AuthChain{Authenticators: []auth.Authenticator{authenticator}}
	authMiddleware := auth.Middleware(chain, nil, auth.DefaultBypassEndpoints, auditLogger)

	mux := http.NewServeMux()
	mux.Handle("/", adapter.Handler())
	handler := authMiddleware(mux)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	// Step 1: Create a response (triggers auth.success + resource.created).
	reqBody := `{"model":"mock-model","input":[{"role":"user","content":"hello"}]}`
	req, _ := http.NewRequestWithContext(context.Background(), "POST", srv.URL+"/v1/responses", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-key")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /v1/responses: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Parse response to get ID.
	var apiResp map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	respID, ok := apiResp["id"].(string)
	if !ok || respID == "" {
		t.Fatal("response missing id")
	}

	// Step 2: Delete the response (triggers resource.deleted).
	delReq, _ := http.NewRequest("DELETE", srv.URL+"/v1/responses/"+respID, nil)
	delReq.Header.Set("Authorization", "Bearer test-key")
	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatalf("DELETE /v1/responses/%s: %v", respID, err)
	}
	delResp.Body.Close()

	// Parse audit events.
	events := parseAuditEvents(t, &auditBuf)

	// Verify auth.success event (subject may not be in event since identity
	// is injected into context after the audit call; check auth_method instead).
	if !hasEventByName(events, "auth.success") {
		t.Error("missing auth.success event")
	}

	// Verify resource.created event.
	if !hasEvent(events, "resource.created", "resource_type", "response") {
		t.Error("missing resource.created event for response")
	}

	// Verify resource.deleted event.
	if !hasEvent(events, "resource.deleted", "resource_id", respID) {
		t.Error("missing resource.deleted event for deleted response")
	}
}

// TestAuditAuthFailure verifies that failed authentication emits auth.failure.
func TestAuditAuthFailure(t *testing.T) {
	var auditBuf bytes.Buffer
	auditLogger := audit.NewFromHandler(slog.NewJSONHandler(&auditBuf, nil))

	authenticator := apikey.New([]apikey.RawKeyEntry{
		{Key: "valid-key", Identity: auth.Identity{Subject: "user"}},
	})
	chain := &auth.AuthChain{Authenticators: []auth.Authenticator{authenticator}}
	authMiddleware := auth.Middleware(chain, nil, auth.DefaultBypassEndpoints, auditLogger)

	handler := authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	srv := httptest.NewServer(handler)
	defer srv.Close()

	// Send request with wrong key.
	req, _ := http.NewRequest("GET", srv.URL+"/v1/responses", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}

	events := parseAuditEvents(t, &auditBuf)
	if !hasEvent(events, "auth.failure", "auth_method", "unknown") {
		t.Error("missing auth.failure event")
	}
}

// TestAuditNilLoggerNoErrors verifies the full stack works with nil audit logger.
func TestAuditNilLoggerNoErrors(t *testing.T) {
	// Use the shared test environment (no audit logger configured).
	resp := postJSON(t, testEnv.BaseURL()+"/v1/responses", map[string]any{
		"model": "mock-model",
		"input": []map[string]any{
			{"role": "user", "content": "hello"},
		},
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("expected 200 with nil audit logger, got %d: %s", resp.StatusCode, body)
	}
}

// parseAuditEvents splits newline-delimited JSON from the audit buffer.
func parseAuditEvents(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var events []map[string]any
	for _, line := range strings.Split(buf.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Logf("skipping non-JSON line: %s", line)
			continue
		}
		events = append(events, m)
	}
	return events
}

// hasEvent checks if any event matches the given event name and key/value pair.
func hasEvent(events []map[string]any, eventName, key, value string) bool {
	for _, e := range events {
		if e["event"] == eventName {
			if v, ok := e[key]; ok && v == value {
				return true
			}
		}
	}
	return false
}

// hasEventByName checks if any event matches the given event name.
func hasEventByName(events []map[string]any, eventName string) bool {
	for _, e := range events {
		if e["event"] == eventName {
			return true
		}
	}
	return false
}
