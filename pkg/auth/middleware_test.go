package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rhuss/antwort/pkg/storage"
)

func TestMiddleware_BypassEndpoint(t *testing.T) {
	chain := &AuthChain{DefaultDecision: No}
	mw := Middleware(chain, nil, []string{"/healthz"})

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
	mw := Middleware(chain, nil, DefaultBypassEndpoints)

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
	mw := Middleware(chain, nil, DefaultBypassEndpoints)

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

	mw := Middleware(chain, limiter, DefaultBypassEndpoints)
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
	mw := Middleware(chain, nil, DefaultBypassEndpoints)
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
