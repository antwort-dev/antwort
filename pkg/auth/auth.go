package auth

import (
	"context"
	"errors"
	"net/http"
)

// AuthDecision represents the three possible outcomes of authentication.
type AuthDecision int

const (
	// Yes means credentials are valid. The chain stops and the identity is used.
	Yes AuthDecision = iota

	// No means credentials are present but invalid. The chain stops and the
	// request is rejected.
	No

	// Abstain means this authenticator cannot handle the credentials type.
	// The chain continues to the next authenticator.
	Abstain
)

// AuthResult carries the outcome of an authentication attempt.
type AuthResult struct {
	Decision AuthDecision
	Identity *Identity // populated only when Decision == Yes
	Err      error     // populated only when Decision == No
}

// Identity represents an authenticated caller.
type Identity struct {
	// Subject is the unique identifier (required, non-empty).
	Subject string

	// ServiceTier determines rate limits and priority.
	ServiceTier string

	// Scopes lists the authorization scopes granted.
	Scopes []string

	// Metadata carries auth-provider-specific data.
	// The key "tenant_id" is used for storage multi-tenancy scoping.
	Metadata map[string]string
}

// TenantID returns the tenant identifier from metadata, or empty string.
func (id *Identity) TenantID() string {
	if id == nil || id.Metadata == nil {
		return ""
	}
	return id.Metadata["tenant_id"]
}

// Authenticator examines request credentials and returns a three-outcome vote.
type Authenticator interface {
	Authenticate(ctx context.Context, r *http.Request) AuthResult
}

// Sentinel errors.
var (
	ErrUnauthenticated = errors.New("authentication required")
	ErrForbidden       = errors.New("access denied")
	ErrTooManyRequests = errors.New("rate limit exceeded")
)

// AuthChain evaluates authenticators in order using three-outcome voting.
type AuthChain struct {
	// Authenticators are evaluated left to right.
	Authenticators []Authenticator

	// DefaultDecision is used when all authenticators abstain.
	// Use Yes for development (NoOp behavior) or No for production.
	DefaultDecision AuthDecision
}

// Authenticate runs the chain. Stops on the first Yes or No.
// If all abstain, returns the default decision.
func (c *AuthChain) Authenticate(ctx context.Context, r *http.Request) AuthResult {
	for _, authn := range c.Authenticators {
		result := authn.Authenticate(ctx, r)
		if result.Decision != Abstain {
			return result
		}
	}

	// All abstained: use default.
	if c.DefaultDecision == Yes {
		return AuthResult{
			Decision: Yes,
			Identity: &Identity{Subject: "anonymous", ServiceTier: "default"},
		}
	}

	return AuthResult{
		Decision: No,
		Err:      ErrUnauthenticated,
	}
}
