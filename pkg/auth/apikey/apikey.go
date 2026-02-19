// Package apikey provides an API key authenticator that validates
// bearer tokens against a static key store using SHA-256 hashing
// and constant-time comparison.
package apikey

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/rhuss/antwort/pkg/auth"
)

// KeyEntry maps a key hash to an identity.
type KeyEntry struct {
	KeyHash  [32]byte
	Identity auth.Identity
}

// Authenticator validates bearer tokens against a static key store.
type Authenticator struct {
	keys []KeyEntry
}

// New creates an API key authenticator from a list of raw keys and identities.
// Keys are hashed immediately; plaintext keys are not stored.
func New(entries []RawKeyEntry) *Authenticator {
	a := &Authenticator{}
	for _, e := range entries {
		a.keys = append(a.keys, KeyEntry{
			KeyHash:  sha256.Sum256([]byte(e.Key)),
			Identity: e.Identity,
		})
	}
	return a
}

// RawKeyEntry is the configuration format for API keys.
type RawKeyEntry struct {
	Key      string
	Identity auth.Identity
}

// Authenticate extracts the bearer token and validates it.
// Returns Yes if valid, No if bearer token present but invalid,
// Abstain if no Authorization header or not a Bearer token.
func (a *Authenticator) Authenticate(_ context.Context, r *http.Request) auth.AuthResult {
	header := r.Header.Get("Authorization")
	if header == "" {
		return auth.AuthResult{Decision: auth.Abstain}
	}

	// Must be Bearer token.
	if !strings.HasPrefix(header, "Bearer ") {
		return auth.AuthResult{Decision: auth.Abstain}
	}

	token := strings.TrimPrefix(header, "Bearer ")
	if token == "" {
		return auth.AuthResult{Decision: auth.No, Err: auth.ErrUnauthenticated}
	}

	// Hash the token and compare against stored hashes.
	tokenHash := sha256.Sum256([]byte(token))

	for _, entry := range a.keys {
		if subtle.ConstantTimeCompare(tokenHash[:], entry.KeyHash[:]) == 1 {
			// Copy identity to avoid shared state.
			id := entry.Identity
			return auth.AuthResult{Decision: auth.Yes, Identity: &id}
		}
	}

	// Bearer token present but not found.
	return auth.AuthResult{Decision: auth.No, Err: auth.ErrUnauthenticated}
}
