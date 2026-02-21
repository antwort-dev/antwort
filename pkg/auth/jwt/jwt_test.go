package jwt

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/rhuss/antwort/pkg/auth"
)

// testKeyPair holds the RSA key pair used throughout the tests.
var testKeyPair *rsa.PrivateKey

func init() {
	var err error
	testKeyPair, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(fmt.Sprintf("generating test RSA key: %v", err))
	}
}

// testKID is the key ID used for the test key pair.
const testKID = "test-key-1"

// jwksHandler returns an HTTP handler that serves the test public key as a JWKS.
// It also increments fetchCount each time the handler is called.
func jwksHandler(fetchCount *atomic.Int32) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if fetchCount != nil {
			fetchCount.Add(1)
		}

		pubKey := testKeyPair.PublicKey
		nBase64 := base64.RawURLEncoding.EncodeToString(pubKey.N.Bytes())
		eBase64 := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pubKey.E)).Bytes())

		jwks := map[string]interface{}{
			"keys": []map[string]string{
				{
					"kty": "RSA",
					"kid": testKID,
					"use": "sig",
					"n":   nBase64,
					"e":   eBase64,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	}
}

// createSignedToken creates a JWT signed with the test private key.
func createSignedToken(t *testing.T, claims jwtlib.MapClaims) string {
	t.Helper()
	token := jwtlib.NewWithClaims(jwtlib.SigningMethodRS256, claims)
	token.Header["kid"] = testKID

	tokenStr, err := token.SignedString(testKeyPair)
	if err != nil {
		t.Fatalf("signing test token: %v", err)
	}
	return tokenStr
}

// newTestAuthenticator creates a test JWKS server and JWT authenticator.
func newTestAuthenticator(t *testing.T, cfgOverride func(*Config), fetchCount *atomic.Int32) (*Authenticator, *httptest.Server) {
	t.Helper()

	server := httptest.NewServer(jwksHandler(fetchCount))
	t.Cleanup(server.Close)

	cfg := Config{
		Issuer:   "https://auth.example.com",
		Audience: "my-api",
		JWKSURL:  server.URL + "/.well-known/jwks.json",
		CacheTTL: 1 * time.Hour,
	}

	if cfgOverride != nil {
		cfgOverride(&cfg)
	}

	authn := New(cfg)
	return authn, server
}

func TestJWT_ValidToken(t *testing.T) {
	authn, _ := newTestAuthenticator(t, nil, nil)

	claims := jwtlib.MapClaims{
		"sub": "user-123",
		"iss": "https://auth.example.com",
		"aud": "my-api",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := createSignedToken(t, claims)

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer "+token)

	result := authn.Authenticate(context.Background(), r)

	if result.Decision != auth.Yes {
		t.Fatalf("Decision = %d, want Yes; err=%v", result.Decision, result.Err)
	}
	if result.Identity == nil {
		t.Fatal("Identity is nil")
	}
	if result.Identity.Subject != "user-123" {
		t.Errorf("Subject = %q, want %q", result.Identity.Subject, "user-123")
	}
}

func TestJWT_ExpiredToken(t *testing.T) {
	authn, _ := newTestAuthenticator(t, nil, nil)

	claims := jwtlib.MapClaims{
		"sub": "user-123",
		"iss": "https://auth.example.com",
		"aud": "my-api",
		"exp": time.Now().Add(-1 * time.Hour).Unix(),
		"iat": time.Now().Add(-2 * time.Hour).Unix(),
	}
	token := createSignedToken(t, claims)

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer "+token)

	result := authn.Authenticate(context.Background(), r)

	if result.Decision != auth.No {
		t.Fatalf("Decision = %d, want No (expired)", result.Decision)
	}
}

func TestJWT_WrongAudience(t *testing.T) {
	authn, _ := newTestAuthenticator(t, nil, nil)

	claims := jwtlib.MapClaims{
		"sub": "user-123",
		"iss": "https://auth.example.com",
		"aud": "wrong-api",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := createSignedToken(t, claims)

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer "+token)

	result := authn.Authenticate(context.Background(), r)

	if result.Decision != auth.No {
		t.Fatalf("Decision = %d, want No (wrong audience)", result.Decision)
	}
}

func TestJWT_WrongIssuer(t *testing.T) {
	authn, _ := newTestAuthenticator(t, nil, nil)

	claims := jwtlib.MapClaims{
		"sub": "user-123",
		"iss": "https://evil.example.com",
		"aud": "my-api",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := createSignedToken(t, claims)

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer "+token)

	result := authn.Authenticate(context.Background(), r)

	if result.Decision != auth.No {
		t.Fatalf("Decision = %d, want No (wrong issuer)", result.Decision)
	}
}

func TestJWT_NoBearerToken(t *testing.T) {
	authn, _ := newTestAuthenticator(t, nil, nil)

	tests := []struct {
		name   string
		header string
	}{
		{"no header", ""},
		{"basic auth", "Basic dXNlcjpwYXNz"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			if tc.header != "" {
				r.Header.Set("Authorization", tc.header)
			}

			result := authn.Authenticate(context.Background(), r)

			if result.Decision != auth.Abstain {
				t.Fatalf("Decision = %d, want Abstain", result.Decision)
			}
		})
	}
}

func TestJWT_InvalidToken(t *testing.T) {
	authn, _ := newTestAuthenticator(t, nil, nil)

	tests := []struct {
		name  string
		token string
	}{
		{"garbage", "not-a-jwt"},
		{"empty bearer", ""},
		{"partial jwt", "eyJhbGciOiJSUzI1NiJ9.invalidpayload"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.Header.Set("Authorization", "Bearer "+tc.token)

			result := authn.Authenticate(context.Background(), r)

			if result.Decision != auth.No {
				t.Fatalf("Decision = %d, want No (invalid token)", result.Decision)
			}
		})
	}
}

func TestJWT_TenantExtraction(t *testing.T) {
	authn, _ := newTestAuthenticator(t, nil, nil)

	claims := jwtlib.MapClaims{
		"sub":       "user-123",
		"iss":       "https://auth.example.com",
		"aud":       "my-api",
		"exp":       time.Now().Add(1 * time.Hour).Unix(),
		"iat":       time.Now().Unix(),
		"tenant_id": "org-456",
	}
	token := createSignedToken(t, claims)

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer "+token)

	result := authn.Authenticate(context.Background(), r)

	if result.Decision != auth.Yes {
		t.Fatalf("Decision = %d, want Yes; err=%v", result.Decision, result.Err)
	}
	if result.Identity.TenantID() != "org-456" {
		t.Errorf("TenantID = %q, want %q", result.Identity.TenantID(), "org-456")
	}
}

func TestJWT_ScopesExtraction(t *testing.T) {
	t.Run("space-separated string", func(t *testing.T) {
		authn, _ := newTestAuthenticator(t, nil, nil)

		claims := jwtlib.MapClaims{
			"sub":   "user-123",
			"iss":   "https://auth.example.com",
			"aud":   "my-api",
			"exp":   time.Now().Add(1 * time.Hour).Unix(),
			"iat":   time.Now().Unix(),
			"scope": "read write admin",
		}
		token := createSignedToken(t, claims)

		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("Authorization", "Bearer "+token)

		result := authn.Authenticate(context.Background(), r)

		if result.Decision != auth.Yes {
			t.Fatalf("Decision = %d, want Yes; err=%v", result.Decision, result.Err)
		}

		expected := []string{"read", "write", "admin"}
		if len(result.Identity.Scopes) != len(expected) {
			t.Fatalf("Scopes = %v, want %v", result.Identity.Scopes, expected)
		}
		for i, s := range expected {
			if result.Identity.Scopes[i] != s {
				t.Errorf("Scopes[%d] = %q, want %q", i, result.Identity.Scopes[i], s)
			}
		}
	})

	t.Run("json array", func(t *testing.T) {
		authn, _ := newTestAuthenticator(t, nil, nil)

		claims := jwtlib.MapClaims{
			"sub":   "user-123",
			"iss":   "https://auth.example.com",
			"aud":   "my-api",
			"exp":   time.Now().Add(1 * time.Hour).Unix(),
			"iat":   time.Now().Unix(),
			"scope": []interface{}{"read", "write"},
		}
		token := createSignedToken(t, claims)

		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("Authorization", "Bearer "+token)

		result := authn.Authenticate(context.Background(), r)

		if result.Decision != auth.Yes {
			t.Fatalf("Decision = %d, want Yes; err=%v", result.Decision, result.Err)
		}

		expected := []string{"read", "write"}
		if len(result.Identity.Scopes) != len(expected) {
			t.Fatalf("Scopes = %v, want %v", result.Identity.Scopes, expected)
		}
		for i, s := range expected {
			if result.Identity.Scopes[i] != s {
				t.Errorf("Scopes[%d] = %q, want %q", i, result.Identity.Scopes[i], s)
			}
		}
	})
}

func TestJWT_JWKSCaching(t *testing.T) {
	var fetchCount atomic.Int32
	authn, _ := newTestAuthenticator(t, nil, &fetchCount)

	claims := jwtlib.MapClaims{
		"sub": "user-123",
		"iss": "https://auth.example.com",
		"aud": "my-api",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := createSignedToken(t, claims)

	// Make multiple requests with the same token.
	for i := 0; i < 5; i++ {
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("Authorization", "Bearer "+token)

		result := authn.Authenticate(context.Background(), r)

		if result.Decision != auth.Yes {
			t.Fatalf("request %d: Decision = %d, want Yes; err=%v", i, result.Decision, result.Err)
		}
	}

	// JWKS should have been fetched only once (the cache TTL is 1 hour).
	count := fetchCount.Load()
	if count != 1 {
		t.Errorf("JWKS fetch count = %d, want 1 (caching broken)", count)
	}
}

func TestJWT_CustomClaims(t *testing.T) {
	cfgOverride := func(cfg *Config) {
		cfg.UserClaim = "email"
		cfg.TenantClaim = "org_id"
		cfg.ScopesClaim = "permissions"
	}

	authn, _ := newTestAuthenticator(t, cfgOverride, nil)

	claims := jwtlib.MapClaims{
		"email":       "alice@example.com",
		"iss":         "https://auth.example.com",
		"aud":         "my-api",
		"exp":         time.Now().Add(1 * time.Hour).Unix(),
		"iat":         time.Now().Unix(),
		"org_id":      "org-custom",
		"permissions": "read write",
	}
	token := createSignedToken(t, claims)

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer "+token)

	result := authn.Authenticate(context.Background(), r)

	if result.Decision != auth.Yes {
		t.Fatalf("Decision = %d, want Yes; err=%v", result.Decision, result.Err)
	}
	if result.Identity.Subject != "alice@example.com" {
		t.Errorf("Subject = %q, want %q", result.Identity.Subject, "alice@example.com")
	}
	if result.Identity.TenantID() != "org-custom" {
		t.Errorf("TenantID = %q, want %q", result.Identity.TenantID(), "org-custom")
	}
	if len(result.Identity.Scopes) != 2 || result.Identity.Scopes[0] != "read" || result.Identity.Scopes[1] != "write" {
		t.Errorf("Scopes = %v, want [read write]", result.Identity.Scopes)
	}
}

func TestJWT_MissingSubClaim(t *testing.T) {
	authn, _ := newTestAuthenticator(t, nil, nil)

	claims := jwtlib.MapClaims{
		"iss": "https://auth.example.com",
		"aud": "my-api",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
		// no "sub" claim
	}
	token := createSignedToken(t, claims)

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer "+token)

	result := authn.Authenticate(context.Background(), r)

	if result.Decision != auth.No {
		t.Fatalf("Decision = %d, want No (missing sub)", result.Decision)
	}
}

func TestJWT_NoIssuerValidation(t *testing.T) {
	// When Issuer is empty, any issuer should be accepted.
	cfgOverride := func(cfg *Config) {
		cfg.Issuer = ""
	}

	authn, _ := newTestAuthenticator(t, cfgOverride, nil)

	claims := jwtlib.MapClaims{
		"sub": "user-123",
		"iss": "https://any-issuer.example.com",
		"aud": "my-api",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := createSignedToken(t, claims)

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer "+token)

	result := authn.Authenticate(context.Background(), r)

	if result.Decision != auth.Yes {
		t.Fatalf("Decision = %d, want Yes (no issuer validation); err=%v", result.Decision, result.Err)
	}
}

func TestJWT_NoAudienceValidation(t *testing.T) {
	// When Audience is empty, any audience should be accepted.
	cfgOverride := func(cfg *Config) {
		cfg.Audience = ""
	}

	authn, _ := newTestAuthenticator(t, cfgOverride, nil)

	claims := jwtlib.MapClaims{
		"sub": "user-123",
		"iss": "https://auth.example.com",
		"aud": "any-api",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := createSignedToken(t, claims)

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer "+token)

	result := authn.Authenticate(context.Background(), r)

	if result.Decision != auth.Yes {
		t.Fatalf("Decision = %d, want Yes (no audience validation); err=%v", result.Decision, result.Err)
	}
}
