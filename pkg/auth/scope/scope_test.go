package scope

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rhuss/antwort/pkg/auth"
)

// --- T010: Role expansion tests ---

func TestExpandRoles_Basic(t *testing.T) {
	config := map[string][]string{
		"viewer": {"responses:read", "files:read"},
	}
	result, err := ExpandRoles(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result["viewer"]["responses:read"] {
		t.Error("expected viewer to have responses:read")
	}
	if !result["viewer"]["files:read"] {
		t.Error("expected viewer to have files:read")
	}
	if len(result["viewer"]) != 2 {
		t.Errorf("expected 2 scopes, got %d", len(result["viewer"]))
	}
}

func TestExpandRoles_Inheritance(t *testing.T) {
	config := map[string][]string{
		"viewer": {"responses:read", "files:read"},
		"user":   {"viewer", "responses:create"},
	}
	result, err := ExpandRoles(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// user should have viewer's scopes + own
	if !result["user"]["responses:read"] {
		t.Error("expected user to inherit responses:read from viewer")
	}
	if !result["user"]["files:read"] {
		t.Error("expected user to inherit files:read from viewer")
	}
	if !result["user"]["responses:create"] {
		t.Error("expected user to have responses:create")
	}
	if len(result["user"]) != 3 {
		t.Errorf("expected 3 scopes for user, got %d", len(result["user"]))
	}
}

func TestExpandRoles_CycleDetection(t *testing.T) {
	config := map[string][]string{
		"a": {"b"},
		"b": {"a"},
	}
	_, err := ExpandRoles(config)
	if err == nil {
		t.Fatal("expected error for circular reference, got nil")
	}
}

func TestExpandRoles_UndefinedReference(t *testing.T) {
	config := map[string][]string{
		"user": {"nonexistent_role"},
	}
	_, err := ExpandRoles(config)
	if err == nil {
		t.Fatal("expected error for undefined reference, got nil")
	}
}

func TestExpandRoles_WildcardPreserved(t *testing.T) {
	config := map[string][]string{
		"admin": {"*"},
	}
	result, err := ExpandRoles(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result["admin"]["*"] {
		t.Error("expected wildcard to be preserved")
	}
	if len(result["admin"]) != 1 {
		t.Errorf("expected 1 scope, got %d", len(result["admin"]))
	}
}

// --- T011: Scope middleware tests ---

func TestMiddleware_ScopeMatch(t *testing.T) {
	expandedRoles := map[string]map[string]bool{
		"creator": {"responses:create": true},
	}
	handler := Middleware(expandedRoles, DefaultEndpointScopes)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest("POST", "/v1/responses", nil)
	ctx := auth.SetIdentity(req.Context(), &auth.Identity{
		Subject:  "user1",
		Metadata: map[string]string{"roles": "creator"},
	})
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestMiddleware_MissingScope(t *testing.T) {
	expandedRoles := map[string]map[string]bool{
		"viewer": {"responses:read": true},
	}
	handler := Middleware(expandedRoles, DefaultEndpointScopes)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest("POST", "/v1/responses", nil)
	ctx := auth.SetIdentity(req.Context(), &auth.Identity{
		Subject:  "user1",
		Metadata: map[string]string{"roles": "viewer"},
	})
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
	body := rr.Body.String()
	if body != `{"error":{"type":"forbidden","message":"insufficient scope: requires responses:create"}}` {
		t.Errorf("unexpected body: %s", body)
	}
}

func TestMiddleware_Wildcard(t *testing.T) {
	expandedRoles := map[string]map[string]bool{
		"admin": {"*": true},
	}
	handler := Middleware(expandedRoles, DefaultEndpointScopes)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest("DELETE", "/v1/responses/some-id", nil)
	ctx := auth.SetIdentity(req.Context(), &auth.Identity{
		Subject:  "admin1",
		Metadata: map[string]string{"roles": "admin"},
	})
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestMiddleware_NilRoles_PassThrough(t *testing.T) {
	handler := Middleware(nil, DefaultEndpointScopes)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest("POST", "/v1/responses", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestMiddleware_NoIdentity_PassThrough(t *testing.T) {
	expandedRoles := map[string]map[string]bool{
		"viewer": {"responses:read": true},
	}
	handler := Middleware(expandedRoles, DefaultEndpointScopes)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	// No identity in context.
	req := httptest.NewRequest("POST", "/v1/responses", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 (pass through), got %d", rr.Code)
	}
}

func TestMiddleware_403IncludesRequiredScope(t *testing.T) {
	expandedRoles := map[string]map[string]bool{
		"viewer": {"responses:read": true},
	}
	handler := Middleware(expandedRoles, DefaultEndpointScopes)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest("DELETE", "/v1/files/file-123", nil)
	ctx := auth.SetIdentity(req.Context(), &auth.Identity{
		Subject:  "user1",
		Metadata: map[string]string{"roles": "viewer"},
	})
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
	body := rr.Body.String()
	expected := `{"error":{"type":"forbidden","message":"insufficient scope: requires files:delete"}}`
	if body != expected {
		t.Errorf("expected body %q, got %q", expected, body)
	}
}

func TestMiddleware_DirectScopes(t *testing.T) {
	// User has direct scopes (not via roles).
	expandedRoles := map[string]map[string]bool{}
	handler := Middleware(expandedRoles, DefaultEndpointScopes)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest("GET", "/v1/agents", nil)
	ctx := auth.SetIdentity(req.Context(), &auth.Identity{
		Subject: "user1",
		Scopes:  []string{"agents:read"},
	})
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestMiddleware_PathParamMatching(t *testing.T) {
	expandedRoles := map[string]map[string]bool{
		"reader": {"conversations:read": true},
	}
	handler := Middleware(expandedRoles, DefaultEndpointScopes)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest("GET", "/v1/conversations/conv-abc-123/items", nil)
	ctx := auth.SetIdentity(req.Context(), &auth.Identity{
		Subject:  "user1",
		Metadata: map[string]string{"roles": "reader"},
	})
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}
