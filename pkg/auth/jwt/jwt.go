// Package jwt provides a JWT/OIDC authenticator that validates
// bearer tokens against a JWKS (JSON Web Key Set) endpoint.
//
// It supports RSA-signed JWTs with configurable issuer, audience,
// and custom claim extraction for subject, tenant, and scopes.
package jwt

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/rhuss/antwort/pkg/auth"
)

// Config holds the JWT authenticator configuration.
type Config struct {
	// Issuer is the expected JWT issuer (iss claim). If empty, issuer is not validated.
	Issuer string

	// Audience is the expected JWT audience (aud claim). If empty, audience is not validated.
	Audience string

	// JWKSURL is the URL to fetch the JSON Web Key Set for signature verification.
	JWKSURL string

	// UserClaim is the JWT claim used as the identity subject. Default: "sub".
	UserClaim string

	// TenantClaim is the JWT claim used for the tenant_id metadata. Default: "tenant_id".
	TenantClaim string

	// ScopesClaim is the JWT claim used for authorization scopes. Default: "scope".
	// The value can be a space-separated string or a JSON array.
	ScopesClaim string

	// CacheTTL controls how long JWKS keys are cached. Default: 1 hour.
	CacheTTL time.Duration

	// HTTPClient allows injecting a custom HTTP client (useful for testing).
	// If nil, http.DefaultClient is used.
	HTTPClient *http.Client
}

// applyDefaults fills in zero-value fields with sensible defaults.
func (c *Config) applyDefaults() {
	if c.UserClaim == "" {
		c.UserClaim = "sub"
	}
	if c.TenantClaim == "" {
		c.TenantClaim = "tenant_id"
	}
	if c.ScopesClaim == "" {
		c.ScopesClaim = "scope"
	}
	if c.CacheTTL == 0 {
		c.CacheTTL = 1 * time.Hour
	}
	if c.HTTPClient == nil {
		c.HTTPClient = http.DefaultClient
	}
}

// Authenticator validates JWT bearer tokens against a JWKS endpoint.
type Authenticator struct {
	config    Config
	jwksCache *jwksCache
}

// New creates a JWT authenticator with the given configuration.
func New(cfg Config) *Authenticator {
	cfg.applyDefaults()
	return &Authenticator{
		config: cfg,
		jwksCache: &jwksCache{
			keys:    make(map[string]*rsa.PublicKey),
			ttl:     cfg.CacheTTL,
			jwksURL: cfg.JWKSURL,
			client:  cfg.HTTPClient,
		},
	}
}

// Authenticate extracts a bearer token from the Authorization header,
// validates it as a JWT, and returns an identity on success.
//
// Decision outcomes:
//   - Abstain: no Authorization header or not a Bearer scheme
//   - No: bearer token present but invalid (expired, wrong issuer, bad signature, etc.)
//   - Yes: valid JWT with populated Identity
func (a *Authenticator) Authenticate(ctx context.Context, r *http.Request) auth.AuthResult {
	header := r.Header.Get("Authorization")
	if header == "" {
		return auth.AuthResult{Decision: auth.Abstain}
	}

	// Must be Bearer token.
	if !strings.HasPrefix(header, "Bearer ") {
		return auth.AuthResult{Decision: auth.Abstain}
	}

	tokenStr := strings.TrimPrefix(header, "Bearer ")
	if tokenStr == "" {
		return auth.AuthResult{
			Decision: auth.No,
			Err:      fmt.Errorf("empty bearer token"),
		}
	}

	// Parse the token to extract the key ID (kid) from the header.
	// We use ParseUnverified first to get the kid, then verify with the
	// matching JWKS key.
	token, err := jwtlib.Parse(tokenStr, func(token *jwtlib.Token) (interface{}, error) {
		// Ensure the signing method is RSA.
		if _, ok := token.Method.(*jwtlib.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		kid, ok := token.Header["kid"].(string)
		if !ok || kid == "" {
			return nil, fmt.Errorf("token missing kid header")
		}

		// Fetch the public key for this kid from the JWKS cache.
		key, fetchErr := a.jwksCache.getKey(ctx, kid)
		if fetchErr != nil {
			return nil, fmt.Errorf("fetching JWKS key for kid %q: %w", kid, fetchErr)
		}

		return key, nil
	}, a.parserOptions()...)
	if err != nil {
		slog.Debug("JWT validation failed", "error", err)
		return auth.AuthResult{
			Decision: auth.No,
			Err:      fmt.Errorf("invalid JWT: %w", err),
		}
	}

	claims, ok := token.Claims.(jwtlib.MapClaims)
	if !ok || !token.Valid {
		return auth.AuthResult{
			Decision: auth.No,
			Err:      fmt.Errorf("invalid JWT claims"),
		}
	}

	// Extract subject.
	subject := claimString(claims, a.config.UserClaim)
	if subject == "" {
		return auth.AuthResult{
			Decision: auth.No,
			Err:      fmt.Errorf("JWT missing %q claim", a.config.UserClaim),
		}
	}

	// Build identity.
	identity := &auth.Identity{
		Subject:  subject,
		Metadata: make(map[string]string),
	}

	// Extract tenant.
	if tenant := claimString(claims, a.config.TenantClaim); tenant != "" {
		identity.Metadata["tenant_id"] = tenant
	}

	// Extract scopes.
	identity.Scopes = extractScopes(claims, a.config.ScopesClaim)

	return auth.AuthResult{
		Decision: auth.Yes,
		Identity: identity,
	}
}

// parserOptions builds JWT parser options based on the configuration.
func (a *Authenticator) parserOptions() []jwtlib.ParserOption {
	opts := []jwtlib.ParserOption{
		jwtlib.WithValidMethods([]string{"RS256", "RS384", "RS512"}),
	}

	if a.config.Issuer != "" {
		opts = append(opts, jwtlib.WithIssuer(a.config.Issuer))
	}

	if a.config.Audience != "" {
		opts = append(opts, jwtlib.WithAudience(a.config.Audience))
	}

	return opts
}

// claimString extracts a string value from JWT claims.
// Returns empty string if the claim is missing or not a string.
func claimString(claims jwtlib.MapClaims, key string) string {
	val, ok := claims[key]
	if !ok {
		return ""
	}
	s, ok := val.(string)
	if !ok {
		return ""
	}
	return s
}

// extractScopes extracts scopes from JWT claims.
// The scope claim can be either a space-separated string or a JSON array.
func extractScopes(claims jwtlib.MapClaims, key string) []string {
	val, ok := claims[key]
	if !ok {
		return nil
	}

	// Case 1: space-separated string (e.g., "read write admin")
	if s, ok := val.(string); ok {
		parts := strings.Fields(s)
		if len(parts) == 0 {
			return nil
		}
		return parts
	}

	// Case 2: JSON array (e.g., ["read", "write", "admin"])
	if arr, ok := val.([]interface{}); ok {
		var scopes []string
		for _, item := range arr {
			if s, ok := item.(string); ok {
				scopes = append(scopes, s)
			}
		}
		if len(scopes) == 0 {
			return nil
		}
		return scopes
	}

	return nil
}

// jwksCache caches RSA public keys fetched from a JWKS endpoint.
// It is thread-safe and supports TTL-based cache invalidation.
type jwksCache struct {
	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey // kid -> public key
	fetchedAt time.Time
	ttl       time.Duration
	jwksURL   string
	client    *http.Client
}

// getKey returns the RSA public key for the given kid.
// It fetches from the JWKS endpoint if the cache is expired or the kid is unknown.
func (c *jwksCache) getKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	// Try cache first with read lock.
	c.mu.RLock()
	if key, ok := c.keys[kid]; ok && time.Since(c.fetchedAt) < c.ttl {
		c.mu.RUnlock()
		return key, nil
	}
	c.mu.RUnlock()

	// Cache miss or expired: refresh with write lock.
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock (another goroutine may have refreshed).
	if key, ok := c.keys[kid]; ok && time.Since(c.fetchedAt) < c.ttl {
		return key, nil
	}

	if err := c.fetchJWKS(ctx); err != nil {
		return nil, err
	}

	key, ok := c.keys[kid]
	if !ok {
		return nil, fmt.Errorf("key %q not found in JWKS", kid)
	}

	return key, nil
}

// fetchJWKS fetches the JWKS from the configured URL and populates the key cache.
// Must be called with the write lock held.
func (c *jwksCache) fetchJWKS(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.jwksURL, nil)
	if err != nil {
		return fmt.Errorf("creating JWKS request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetching JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading JWKS response: %w", err)
	}

	var jwks jwksDocument
	if err := json.Unmarshal(body, &jwks); err != nil {
		return fmt.Errorf("parsing JWKS: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, jwk := range jwks.Keys {
		if jwk.Kty != "RSA" {
			continue
		}
		if jwk.Use != "" && jwk.Use != "sig" {
			continue
		}

		pubKey, err := parseRSAPublicKey(jwk)
		if err != nil {
			slog.Warn("skipping JWKS key", "kid", jwk.Kid, "error", err)
			continue
		}

		keys[jwk.Kid] = pubKey
	}

	c.keys = keys
	c.fetchedAt = time.Now()

	slog.Debug("JWKS cache refreshed", "keys", len(keys), "url", c.jwksURL)
	return nil
}

// jwksDocument represents the JSON Web Key Set response.
type jwksDocument struct {
	Keys []jwkKey `json:"keys"`
}

// jwkKey represents a single JSON Web Key.
type jwkKey struct {
	Kty string `json:"kty"` // Key type (e.g., "RSA")
	Kid string `json:"kid"` // Key ID
	Use string `json:"use"` // Key use (e.g., "sig")
	N   string `json:"n"`   // RSA modulus (base64url-encoded)
	E   string `json:"e"`   // RSA public exponent (base64url-encoded)
}

// parseRSAPublicKey constructs an *rsa.PublicKey from a JWK.
func parseRSAPublicKey(jwk jwkKey) (*rsa.PublicKey, error) {
	// Decode modulus (n).
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("decoding modulus: %w", err)
	}

	// Decode exponent (e).
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("decoding exponent: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)

	if !e.IsInt64() {
		return nil, fmt.Errorf("RSA exponent too large")
	}

	return &rsa.PublicKey{
		N: n,
		E: int(e.Int64()),
	}, nil
}
