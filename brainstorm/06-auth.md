# Spec 06: Authentication & Authorization

**Branch**: `spec/06-auth`
**Dependencies**: Spec 01 (Core Protocol), Spec 02 (Transport)
**Package**: `github.com/rhuss/antwort/pkg/auth`

## Purpose

Define the pluggable authentication and authorization interface, and implement adapters for common auth mechanisms. Auth is implemented as transport middleware, keeping it decoupled from business logic.

## Scope

### In Scope
- Auth interface definition (authentication + authorization)
- Bearer token (API key) adapter
- OpenShift OAuth proxy integration
- mTLS client certificate adapter
- Rate limiting tied to `service_tier`
- Auth middleware for the transport layer

### Out of Scope
- User management / key provisioning (external concern)
- Multi-tenancy data isolation (future, built on top of auth identity)
- Fine-grained RBAC beyond basic allow/deny

## Auth Interface

```go
// Authenticator extracts and validates identity from a request.
type Authenticator interface {
    // Authenticate extracts credentials from the request context
    // and returns an Identity if valid.
    // Returns ErrUnauthenticated if credentials are missing or invalid.
    Authenticate(ctx context.Context, req AuthRequest) (*Identity, error)
}

// AuthRequest carries protocol-agnostic authentication data
// extracted by the transport layer.
type AuthRequest struct {
    // BearerToken from the Authorization header (if present).
    BearerToken string

    // ClientCert from mTLS (if present).
    ClientCert *x509.Certificate

    // Headers for custom auth schemes (e.g., X-Forwarded-User from OAuth proxy).
    Headers map[string]string

    // RemoteAddr is the client IP.
    RemoteAddr string
}

// Identity represents an authenticated caller.
type Identity struct {
    // Subject is the unique identifier (API key hash, username, cert CN).
    Subject string

    // ServiceTier determines rate limits and priority.
    ServiceTier string

    // Metadata carries auth-provider-specific data.
    Metadata map[string]string
}

// Authorizer checks whether an identity is allowed to perform an action.
type Authorizer interface {
    // Authorize checks if the identity can perform the given action.
    // Returns ErrForbidden if not allowed.
    Authorize(ctx context.Context, identity *Identity, action Action) error
}

// Action describes what the caller wants to do.
type Action struct {
    Resource string // "response"
    Verb     string // "create", "get", "delete"
    Model    string // requested model (for model-level access control)
}

// Sentinel errors
var (
    ErrUnauthenticated = errors.New("authentication required")
    ErrForbidden       = errors.New("access denied")
)
```

## Auth Middleware

```go
// AuthMiddleware creates transport middleware from an Authenticator and Authorizer.
func AuthMiddleware(authn Authenticator, authz Authorizer) transport.Middleware {
    return func(next transport.Handler) transport.Handler {
        // 1. Extract AuthRequest from transport context
        // 2. Call authn.Authenticate()
        // 3. Store Identity in context
        // 4. Call authz.Authorize() for the specific action
        // 5. If OK, call next.Handle()
    }
}

// IdentityFromContext retrieves the authenticated identity.
func IdentityFromContext(ctx context.Context) *Identity
```

## Adapters

### Bearer Token (API Key)

```go
// APIKeyAuthenticator validates bearer tokens against a key store.
type APIKeyAuthenticator struct {
    keys KeyStore
}

// KeyStore looks up API keys. Implementations can use
// static config, database, or external service.
type KeyStore interface {
    Lookup(ctx context.Context, token string) (*Identity, error)
}

// StaticKeyStore uses a configured list of API keys.
type StaticKeyStore struct {
    keys map[string]*Identity // token hash -> identity
}

// Config example:
// api_keys:
//   - key: "sk-..."
//     subject: "user-1"
//     service_tier: "standard"
```

### OpenShift OAuth Proxy

When deployed behind an OpenShift OAuth proxy, auth is already handled. The adapter extracts identity from forwarded headers:

```go
// OAuthProxyAuthenticator trusts headers set by the OAuth proxy.
type OAuthProxyAuthenticator struct {
    userHeader  string // default: "X-Forwarded-User"
    emailHeader string // default: "X-Forwarded-Email"
    groupHeader string // default: "X-Forwarded-Groups"
}
```

### mTLS

```go
// MTLSAuthenticator extracts identity from client certificates.
type MTLSAuthenticator struct {
    trustedCAs *x509.CertPool
}
```

### No-Op (Development)

```go
// NoOpAuthenticator allows all requests. For development only.
type NoOpAuthenticator struct{}
```

## Rate Limiting

```go
// RateLimiter enforces request limits based on identity and service tier.
type RateLimiter interface {
    // Allow checks if the request should proceed.
    // Returns ErrTooManyRequests if the limit is exceeded.
    Allow(ctx context.Context, identity *Identity) error
}

// TieredRateLimiter applies different limits per service_tier.
type TieredRateLimiter struct {
    tiers map[string]TierConfig
}

type TierConfig struct {
    RequestsPerMinute int
    TokensPerMinute   int
    ConcurrentStreams  int
}
```

## Configuration

```go
type AuthConfig struct {
    // Type selects the authenticator: "api_key", "oauth_proxy", "mtls", "none"
    Type string `json:"type" env:"ANTWORT_AUTH_TYPE"`

    // APIKeys for static key authentication.
    APIKeys []APIKeyEntry `json:"api_keys,omitempty"`

    // RateLimits per service tier.
    RateLimits map[string]TierConfig `json:"rate_limits,omitempty"`
}
```

## Extension Points

- **Custom authenticators**: Implement `Authenticator` for JWT, OIDC, LDAP, etc.
- **Custom authorizers**: Implement `Authorizer` for OPA, Casbin, or policy-as-code
- **Custom key stores**: Implement `KeyStore` for database-backed or vault-backed keys
- **Custom rate limiters**: Implement `RateLimiter` for Redis-backed distributed limiting

## Open Questions

- Should rate limiting be part of the auth spec or a separate concern?
- Should we support chained authenticators (try API key, fall back to OAuth proxy)?
- Audit logging of auth decisions: part of this spec or observability (Spec 07)?

## Deliverables

- [ ] `pkg/auth/auth.go` - Authenticator, Authorizer, Identity interfaces
- [ ] `pkg/auth/middleware.go` - Transport middleware
- [ ] `pkg/auth/apikey/apikey.go` - API key authenticator
- [ ] `pkg/auth/oauthproxy/oauthproxy.go` - OAuth proxy authenticator
- [ ] `pkg/auth/mtls/mtls.go` - mTLS authenticator
- [ ] `pkg/auth/noop/noop.go` - No-op authenticator
- [ ] `pkg/auth/ratelimit/ratelimit.go` - Rate limiter
- [ ] `pkg/auth/config.go` - Configuration
- [ ] Tests for each adapter
