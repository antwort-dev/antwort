package auth

import (
	"context"
	"net/http"
	"testing"
)

// mockAuthn is a test authenticator with configurable behavior.
type mockAuthn struct {
	result AuthResult
}

func (m *mockAuthn) Authenticate(_ context.Context, _ *http.Request) AuthResult {
	return m.result
}

func TestAuthChain_FirstYesStops(t *testing.T) {
	chain := &AuthChain{
		Authenticators: []Authenticator{
			&mockAuthn{result: AuthResult{Decision: Yes, Identity: &Identity{Subject: "alice"}}},
			&mockAuthn{result: AuthResult{Decision: No, Err: ErrUnauthenticated}},
		},
		DefaultDecision: No,
	}

	r, _ := http.NewRequest("GET", "/", nil)
	result := chain.Authenticate(context.Background(), r)

	if result.Decision != Yes {
		t.Errorf("Decision = %d, want Yes", result.Decision)
	}
	if result.Identity.Subject != "alice" {
		t.Errorf("Subject = %q, want %q", result.Identity.Subject, "alice")
	}
}

func TestAuthChain_FirstNoStops(t *testing.T) {
	chain := &AuthChain{
		Authenticators: []Authenticator{
			&mockAuthn{result: AuthResult{Decision: No, Err: ErrUnauthenticated}},
			&mockAuthn{result: AuthResult{Decision: Yes, Identity: &Identity{Subject: "bob"}}},
		},
		DefaultDecision: No,
	}

	r, _ := http.NewRequest("GET", "/", nil)
	result := chain.Authenticate(context.Background(), r)

	if result.Decision != No {
		t.Errorf("Decision = %d, want No", result.Decision)
	}
}

func TestAuthChain_AllAbstain_DefaultReject(t *testing.T) {
	chain := &AuthChain{
		Authenticators: []Authenticator{
			&mockAuthn{result: AuthResult{Decision: Abstain}},
			&mockAuthn{result: AuthResult{Decision: Abstain}},
		},
		DefaultDecision: No,
	}

	r, _ := http.NewRequest("GET", "/", nil)
	result := chain.Authenticate(context.Background(), r)

	if result.Decision != No {
		t.Errorf("Decision = %d, want No (default reject)", result.Decision)
	}
}

func TestAuthChain_AllAbstain_DefaultAccept(t *testing.T) {
	chain := &AuthChain{
		Authenticators: []Authenticator{
			&mockAuthn{result: AuthResult{Decision: Abstain}},
		},
		DefaultDecision: Yes,
	}

	r, _ := http.NewRequest("GET", "/", nil)
	result := chain.Authenticate(context.Background(), r)

	if result.Decision != Yes {
		t.Errorf("Decision = %d, want Yes (default accept)", result.Decision)
	}
	if result.Identity.Subject != "anonymous" {
		t.Errorf("Subject = %q, want %q", result.Identity.Subject, "anonymous")
	}
}

func TestAuthChain_Empty_DefaultReject(t *testing.T) {
	chain := &AuthChain{DefaultDecision: No}

	r, _ := http.NewRequest("GET", "/", nil)
	result := chain.Authenticate(context.Background(), r)

	if result.Decision != No {
		t.Errorf("Decision = %d, want No (empty chain)", result.Decision)
	}
}

func TestAuthChain_AbstainThenYes(t *testing.T) {
	chain := &AuthChain{
		Authenticators: []Authenticator{
			&mockAuthn{result: AuthResult{Decision: Abstain}},
			&mockAuthn{result: AuthResult{Decision: Yes, Identity: &Identity{Subject: "jwt-user"}}},
		},
		DefaultDecision: No,
	}

	r, _ := http.NewRequest("GET", "/", nil)
	result := chain.Authenticate(context.Background(), r)

	if result.Decision != Yes {
		t.Errorf("Decision = %d, want Yes", result.Decision)
	}
	if result.Identity.Subject != "jwt-user" {
		t.Errorf("Subject = %q, want %q", result.Identity.Subject, "jwt-user")
	}
}

func TestIdentity_TenantID(t *testing.T) {
	id := &Identity{Subject: "alice", Metadata: map[string]string{"tenant_id": "org-1"}}
	if id.TenantID() != "org-1" {
		t.Errorf("TenantID = %q, want %q", id.TenantID(), "org-1")
	}

	// No metadata.
	id2 := &Identity{Subject: "bob"}
	if id2.TenantID() != "" {
		t.Errorf("TenantID = %q, want empty", id2.TenantID())
	}

	// Nil identity.
	var id3 *Identity
	if id3.TenantID() != "" {
		t.Errorf("TenantID on nil = %q, want empty", id3.TenantID())
	}
}

func TestIdentityContext(t *testing.T) {
	ctx := context.Background()

	// No identity set.
	if IdentityFromContext(ctx) != nil {
		t.Error("expected nil identity from empty context")
	}

	// Set and retrieve.
	id := &Identity{Subject: "alice"}
	ctx = SetIdentity(ctx, id)
	got := IdentityFromContext(ctx)
	if got == nil || got.Subject != "alice" {
		t.Errorf("got %v, want alice", got)
	}
}
