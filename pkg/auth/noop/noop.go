// Package noop provides a no-op authenticator that accepts all requests.
// Used for development and as a default voter in the auth chain.
package noop

import (
	"context"
	"net/http"

	"github.com/rhuss/antwort/pkg/auth"
)

// Authenticator always returns Yes with a default anonymous identity.
type Authenticator struct{}

func (a *Authenticator) Authenticate(_ context.Context, _ *http.Request) auth.AuthResult {
	return auth.AuthResult{
		Decision: auth.Yes,
		Identity: &auth.Identity{
			Subject:     "anonymous",
			ServiceTier: "default",
		},
	}
}
