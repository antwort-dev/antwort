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
- JWT/OIDC adapter
- OpenShift OAuth proxy integration
- mTLS client certificate adapter
- Rate limiting tied to `service_tier`
- Auth middleware for the transport layer
- Auth bypass for infrastructure endpoints

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

    // Endpoint is the logical endpoint being accessed.
    // Used by the middleware to determine whether auth should be bypassed.
    Endpoint string
}

// Identity represents an authenticated caller.
type Identity struct {
    // Subject is the unique identifier (API key hash, username, cert CN, JWT sub).
    Subject string

    // ServiceTier determines rate limits and priority.
    ServiceTier string

    // Scopes lists the authorization scopes granted to this identity
    // (e.g., "responses:read", "responses:write"). Used for fine-grained
    // access control when the auth provider supplies scope information.
    Scopes []string

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
        // 1. Check if the endpoint is in the bypass list; if so, skip auth
        // 2. Extract AuthRequest from transport context
        // 3. Call authn.Authenticate()
        // 4. Store Identity in context
        // 5. Call authz.Authorize() for the specific action
        // 6. If OK, call next.Handle()
    }
}

// IdentityFromContext retrieves the authenticated identity.
func IdentityFromContext(ctx context.Context) *Identity
```

## Auth Bypass

Certain infrastructure endpoints must be accessible without authentication. These are used by orchestrators (Kubernetes liveness/readiness probes) and monitoring systems, and requiring auth on them would break health checking and metrics collection.

The following endpoints bypass authentication entirely:

| Endpoint   | Purpose                        |
|------------|--------------------------------|
| `/healthz` | Liveness probe                 |
| `/readyz`  | Readiness probe                |
| `/metrics` | Prometheus metrics collection  |

The bypass check happens at the beginning of the auth middleware, before any credential extraction or validation. The bypass list is configurable to allow operators to add or remove endpoints as needed.

```go
// DefaultBypassEndpoints lists endpoints that skip authentication.
var DefaultBypassEndpoints = []string{"/healthz", "/readyz", "/metrics"}

// BypassChecker determines whether a request should skip authentication.
// The transport layer populates the Endpoint field in AuthRequest,
// keeping this check protocol-agnostic.
type BypassChecker struct {
    Endpoints []string
}

func (b *BypassChecker) ShouldBypass(endpoint string) bool {
    for _, e := range b.Endpoints {
        if e == endpoint {
            return true
        }
    }
    return false
}
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

### JWT/OIDC

The JWT/OIDC adapter validates JSON Web Tokens using a JWKS endpoint. It supports standard OIDC providers and allows custom claim extraction for mapping tokens to antwort identities.

```go
// JWTAuthenticator validates JWTs via JWKS and extracts identity claims.
type JWTAuthenticator struct {
    // Issuer is the expected "iss" claim value.
    Issuer string

    // Audience is the expected "aud" claim value.
    Audience string

    // JWKSURL is the endpoint serving the JSON Web Key Set
    // used for signature validation.
    JWKSURL string

    // SubjectClaim specifies which JWT claim maps to Identity.Subject.
    // Defaults to "sub" if empty.
    SubjectClaim string

    // TenantClaim specifies which JWT claim maps to tenant/org identity
    // (stored in Identity.Metadata["tenant_id"]).
    // Common values: "org_id", "tenant", "team_id".
    TenantClaim string

    // ScopesClaim specifies which JWT claim provides authorization scopes
    // (stored in Identity.Scopes).
    // Defaults to "scope" if empty. Supports both space-delimited strings
    // and JSON arrays.
    ScopesClaim string
}
```

The authenticator performs the following validation steps:

1. Extract the bearer token from `AuthRequest.BearerToken`
2. Fetch and cache the JWKS from the configured endpoint
3. Validate the JWT signature against the key set
4. Verify expiration (`exp`), not-before (`nbf`), issuer (`iss`), and audience (`aud`)
5. Extract the subject, tenant, and scopes from the configured claims
6. Return a populated `Identity`

```go
// Config example:
// auth:
//   type: jwt
//   jwt:
//     issuer: https://auth.example.com
//     audience: antwort
//     jwks_url: https://auth.example.com/.well-known/jwks.json
//     tenant_claim: org_id
//     scopes_claim: scope
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

## Authorization

### Ownership-Based Isolation

When a user references a resource owned by another user (via `previous_response_id`, `conversation_id`, or similar fields), the authorizer must return HTTP 404 (Not Found) rather than HTTP 403 (Forbidden). Returning 403 would confirm the resource exists, leaking information about other users' data. Returning 404 is a security best practice that makes cross-user resource references indistinguishable from references to nonexistent resources.

The storage layer supports this naturally: queries scoped by the authenticated user's identity simply return no rows for another user's resources, which surfaces as a "not found" error.

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
    // Type selects the authenticator: "api_key", "jwt", "oauth_proxy", "mtls", "none"
    Type string `json:"type" env:"ANTWORT_AUTH_TYPE"`

    // APIKeys for static key authentication.
    APIKeys []APIKeyEntry `json:"api_keys,omitempty"`

    // JWT configuration for JWT/OIDC authentication.
    JWT *JWTConfig `json:"jwt,omitempty"`

    // BypassEndpoints lists endpoints that skip authentication.
    // Defaults to DefaultBypassEndpoints if not set.
    BypassEndpoints []string `json:"bypass_endpoints,omitempty"`

    // RateLimits per service tier.
    RateLimits map[string]TierConfig `json:"rate_limits,omitempty"`
}

type JWTConfig struct {
    Issuer       string `json:"issuer"`
    Audience     string `json:"audience"`
    JWKSURL      string `json:"jwks_url"`
    SubjectClaim string `json:"subject_claim,omitempty"` // defaults to "sub"
    TenantClaim  string `json:"tenant_claim,omitempty"`
    ScopesClaim  string `json:"scopes_claim,omitempty"` // defaults to "scope"
}
```

## Extension Points

- **Custom authenticators**: Implement `Authenticator` for LDAP, SAML, or other providers
- **Custom authorizers**: Implement `Authorizer` for OPA, Casbin, or policy-as-code
- **Custom key stores**: Implement `KeyStore` for database-backed or vault-backed keys
- **Custom rate limiters**: Implement `RateLimiter` for Redis-backed distributed limiting

## Future Considerations

### Per-User Resource Limits

Beyond global rate limiting by service tier, a future iteration could support per-user resource limits. This would allow operators to constrain individual users based on their plan or role:

- **Per-user rate limiting**: Separate request-per-minute and token-per-minute quotas per user, in addition to the tier-level limits
- **Max stored responses**: Limit the total number of responses a single user can have stored at any time
- **Model access control**: Restrict which models a user is allowed to request (e.g., only allow certain users access to expensive models)

These limits would integrate with the existing `Identity` and `Authorizer` interfaces. Per-user configuration could be stored in the `Identity.Metadata` map or in a dedicated user-limits store.

## Open Questions

- Should rate limiting be part of the auth spec or a separate concern?
- Should we support chained authenticators (try API key, fall back to OAuth proxy)?
- Audit logging of auth decisions: part of this spec or observability (Spec 07)?

## Deliverables

- [ ] `pkg/auth/auth.go` - Authenticator, Authorizer, Identity interfaces
- [ ] `pkg/auth/middleware.go` - Transport middleware (including bypass logic)
- [ ] `pkg/auth/apikey/apikey.go` - API key authenticator
- [ ] `pkg/auth/jwt/jwt.go` - JWT/OIDC authenticator
- [ ] `pkg/auth/oauthproxy/oauthproxy.go` - OAuth proxy authenticator
- [ ] `pkg/auth/mtls/mtls.go` - mTLS authenticator
- [ ] `pkg/auth/noop/noop.go` - No-op authenticator
- [ ] `pkg/auth/ratelimit/ratelimit.go` - Rate limiter
- [ ] `pkg/auth/config.go` - Configuration
- [ ] Tests for each adapter
