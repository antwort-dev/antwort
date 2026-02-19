package apikey

import (
	"context"
	"net/http"
	"testing"

	"github.com/rhuss/antwort/pkg/auth"
)

func newTestAuth() *Authenticator {
	return New([]RawKeyEntry{
		{
			Key: "sk-test-key-1",
			Identity: auth.Identity{
				Subject:     "alice",
				ServiceTier: "standard",
				Metadata:    map[string]string{"tenant_id": "org-1"},
			},
		},
		{
			Key: "sk-test-key-2",
			Identity: auth.Identity{
				Subject:     "bob",
				ServiceTier: "premium",
			},
		},
	})
}

func TestValidKey(t *testing.T) {
	a := newTestAuth()
	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer sk-test-key-1")

	result := a.Authenticate(context.Background(), r)

	if result.Decision != auth.Yes {
		t.Fatalf("Decision = %d, want Yes", result.Decision)
	}
	if result.Identity.Subject != "alice" {
		t.Errorf("Subject = %q, want %q", result.Identity.Subject, "alice")
	}
	if result.Identity.ServiceTier != "standard" {
		t.Errorf("ServiceTier = %q, want %q", result.Identity.ServiceTier, "standard")
	}
	if result.Identity.TenantID() != "org-1" {
		t.Errorf("TenantID = %q, want %q", result.Identity.TenantID(), "org-1")
	}
}

func TestInvalidKey(t *testing.T) {
	a := newTestAuth()
	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer sk-wrong-key")

	result := a.Authenticate(context.Background(), r)

	if result.Decision != auth.No {
		t.Fatalf("Decision = %d, want No", result.Decision)
	}
}

func TestNoHeader(t *testing.T) {
	a := newTestAuth()
	r, _ := http.NewRequest("GET", "/", nil)

	result := a.Authenticate(context.Background(), r)

	if result.Decision != auth.Abstain {
		t.Fatalf("Decision = %d, want Abstain", result.Decision)
	}
}

func TestNonBearerHeader(t *testing.T) {
	a := newTestAuth()
	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Basic dXNlcjpwYXNz")

	result := a.Authenticate(context.Background(), r)

	if result.Decision != auth.Abstain {
		t.Fatalf("Decision = %d, want Abstain (non-Bearer)", result.Decision)
	}
}

func TestEmptyBearerToken(t *testing.T) {
	a := newTestAuth()
	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer ")

	result := a.Authenticate(context.Background(), r)

	if result.Decision != auth.No {
		t.Fatalf("Decision = %d, want No (empty token)", result.Decision)
	}
}

func TestSecondKey(t *testing.T) {
	a := newTestAuth()
	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer sk-test-key-2")

	result := a.Authenticate(context.Background(), r)

	if result.Decision != auth.Yes {
		t.Fatalf("Decision = %d, want Yes", result.Decision)
	}
	if result.Identity.Subject != "bob" {
		t.Errorf("Subject = %q, want %q", result.Identity.Subject, "bob")
	}
}
