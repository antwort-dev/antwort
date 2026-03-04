package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rhuss/antwort/pkg/storage"
)

func TestMiddleware_BypassEndpoint(t *testing.T) {
	chain := &AuthChain{DefaultDecision: No}
	mw := Middleware(chain, nil, []string{"/healthz"}, nil)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("bypass endpoint: status = %d, want 200", rec.Code)
	}
}

func TestMiddleware_NoAuth_Rejects(t *testing.T) {
	chain := &AuthChain{DefaultDecision: No}
	mw := Middleware(chain, nil, DefaultBypassEndpoints, nil)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/v1/responses", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("no auth: status = %d, want 401", rec.Code)
	}
}

func TestMiddleware_ValidAuth_Passes(t *testing.T) {
	chain := &AuthChain{
		Authenticators: []Authenticator{
			&mockAuthn{result: AuthResult{
				Decision: Yes,
				Identity: &Identity{Subject: "alice", Metadata: map[string]string{"tenant_id": "org-1"}},
			}},
		},
		DefaultDecision: No,
	}
	mw := Middleware(chain, nil, DefaultBypassEndpoints, nil)

	var gotTenant string
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenant = storage.GetTenant(r.Context())
		id := IdentityFromContext(r.Context())
		if id == nil || id.Subject != "alice" {
			t.Error("expected identity 'alice' in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/v1/responses", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("valid auth: status = %d, want 200", rec.Code)
	}
	if gotTenant != "org-1" {
		t.Errorf("tenant = %q, want %q", gotTenant, "org-1")
	}
}

func TestMiddleware_RateLimit_Exceeded(t *testing.T) {
	chain := &AuthChain{
		Authenticators: []Authenticator{
			&mockAuthn{result: AuthResult{
				Decision: Yes,
				Identity: &Identity{Subject: "alice", ServiceTier: "limited"},
			}},
		},
		DefaultDecision: No,
	}

	limiter := NewInProcessLimiter(map[string]TierConfig{
		"limited": {RequestsPerMinute: 2},
	}, 100)

	mw := Middleware(chain, limiter, DefaultBypassEndpoints, nil)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First 2 requests should pass.
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/v1/responses", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("request %d: status = %d, want 200", i+1, rec.Code)
		}
	}

	// 3rd should be rate limited.
	req := httptest.NewRequest("POST", "/v1/responses", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("rate limited request: status = %d, want 429", rec.Code)
	}
}

func TestMiddleware_NoLimiter_AllAllowed(t *testing.T) {
	chain := &AuthChain{
		Authenticators: []Authenticator{
			&mockAuthn{result: AuthResult{Decision: Yes, Identity: &Identity{Subject: "alice"}}},
		},
	}

	// nil limiter = no limiting.
	mw := Middleware(chain, nil, DefaultBypassEndpoints, nil)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 100; i++ {
		req := httptest.NewRequest("POST", "/v1/responses", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("request %d: status = %d, want 200", i+1, rec.Code)
			break
		}
	}
}

// Reuse mockAuthn from auth_test.go (same package).
var _ Authenticator = (*mockAuthn)(nil)

// --- Audit event tests (T009, T021, T022) ---

// testAuditLogger implements AuditLogger for testing.
type testAuditLogger struct {
	buf *bytes.Buffer
}

func newTestAuditLogger() (*testAuditLogger, *bytes.Buffer) {
	var buf bytes.Buffer
	return &testAuditLogger{buf: &buf}, &buf
}

func (l *testAuditLogger) Log(_ context.Context, event string, attrs ...any) {
	h := slog.NewJSONHandler(l.buf, nil)
	logger := slog.New(h)
	logger.Info(event, append([]any{"event", event}, attrs...)...)
}

func (l *testAuditLogger) LogWarn(_ context.Context, event string, attrs ...any) {
	h := slog.NewJSONHandler(l.buf, nil)
	logger := slog.New(h)
	logger.Warn(event, append([]any{"event", event}, attrs...)...)
}

func TestMiddleware_AuditAuthSuccess(t *testing.T) {
	al, buf := newTestAuditLogger()
	chain := &AuthChain{
		Authenticators: []Authenticator{
			&mockAuthn{result: AuthResult{
				Decision: Yes,
				Identity: &Identity{Subject: "alice", Metadata: map[string]string{"api_key": "true"}},
			}},
		},
		DefaultDecision: No,
	}
	mw := Middleware(chain, nil, DefaultBypassEndpoints, al)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/v1/responses", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("failed to parse audit JSON: %v\nbody: %s", err, buf.String())
	}

	if m["event"] != "auth.success" {
		t.Errorf("event = %q, want %q", m["event"], "auth.success")
	}
	if m["auth_method"] != "apikey" {
		t.Errorf("auth_method = %q, want %q", m["auth_method"], "apikey")
	}
	if m["remote_addr"] != "10.0.0.1:12345" {
		t.Errorf("remote_addr = %q, want %q", m["remote_addr"], "10.0.0.1:12345")
	}
	if m["level"] != "INFO" {
		t.Errorf("level = %q, want %q", m["level"], "INFO")
	}
}

func TestMiddleware_AuditAuthSuccess_JWT(t *testing.T) {
	al, buf := newTestAuditLogger()
	chain := &AuthChain{
		Authenticators: []Authenticator{
			&mockAuthn{result: AuthResult{
				Decision: Yes,
				Identity: &Identity{Subject: "bob", Scopes: []string{"responses:read"}},
			}},
		},
		DefaultDecision: No,
	}
	mw := Middleware(chain, nil, DefaultBypassEndpoints, al)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/v1/responses", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("failed to parse audit JSON: %v", err)
	}
	if m["auth_method"] != "jwt" {
		t.Errorf("auth_method = %q, want %q", m["auth_method"], "jwt")
	}
}

func TestMiddleware_AuditAuthFailure(t *testing.T) {
	al, buf := newTestAuditLogger()
	chain := &AuthChain{DefaultDecision: No}
	mw := Middleware(chain, nil, DefaultBypassEndpoints, al)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/v1/responses", nil)
	req.RemoteAddr = "192.168.1.1:9999"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("failed to parse audit JSON: %v\nbody: %s", err, buf.String())
	}

	if m["event"] != "auth.failure" {
		t.Errorf("event = %q, want %q", m["event"], "auth.failure")
	}
	if m["auth_method"] != "unknown" {
		t.Errorf("auth_method = %q, want %q", m["auth_method"], "unknown")
	}
	if m["remote_addr"] != "192.168.1.1:9999" {
		t.Errorf("remote_addr = %q, want %q", m["remote_addr"], "192.168.1.1:9999")
	}
	errStr, ok := m["error"].(string)
	if !ok || errStr == "" {
		t.Error("expected non-empty error field")
	}
	if m["level"] != "WARN" {
		t.Errorf("level = %q, want %q", m["level"], "WARN")
	}
}

func TestMiddleware_AuditRateLimited(t *testing.T) {
	al, buf := newTestAuditLogger()
	chain := &AuthChain{
		Authenticators: []Authenticator{
			&mockAuthn{result: AuthResult{
				Decision: Yes,
				Identity: &Identity{Subject: "alice", ServiceTier: "basic"},
			}},
		},
		DefaultDecision: No,
	}

	limiter := NewInProcessLimiter(map[string]TierConfig{
		"basic": {RequestsPerMinute: 1},
	}, 100)

	mw := Middleware(chain, limiter, DefaultBypassEndpoints, al)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request passes.
	req := httptest.NewRequest("POST", "/v1/responses", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Clear buffer for the auth.success event.
	buf.Reset()

	// Second request should be rate limited.
	req = httptest.NewRequest("POST", "/v1/responses", nil)
	req.RemoteAddr = "10.0.0.5:4444"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}

	// The buffer should have auth.success + auth.rate_limited. Find the rate_limited one.
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	var found bool
	for _, line := range lines {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			continue
		}
		if m["event"] == "auth.rate_limited" {
			found = true
			if m["tier"] != "basic" {
				t.Errorf("tier = %q, want %q", m["tier"], "basic")
			}
			if m["remote_addr"] != "10.0.0.5:4444" {
				t.Errorf("remote_addr = %q, want %q", m["remote_addr"], "10.0.0.5:4444")
			}
			if m["level"] != "WARN" {
				t.Errorf("level = %q, want %q", m["level"], "WARN")
			}
			break
		}
	}
	if !found {
		t.Errorf("auth.rate_limited event not found in output: %s", buf.String())
	}
}

func TestMiddleware_NilAuditLogger_NoPanic(t *testing.T) {
	chain := &AuthChain{DefaultDecision: No}
	// nil audit logger should not panic.
	mw := Middleware(chain, nil, DefaultBypassEndpoints, nil)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/v1/responses", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}
