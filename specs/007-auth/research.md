# Research: Authentication & Authorization

**Feature**: 007-auth
**Date**: 2026-02-19

## R1: Three-Outcome Voting Return Type

**Decision**: Use a result struct with an enum Decision field and optional Identity.

```go
type AuthResult struct {
    Decision AuthDecision  // Yes, No, Abstain
    Identity *Identity     // populated only when Decision == Yes
    Err      error         // populated only when Decision == No
}
```

**Rationale**: A struct is cleaner than overloading error semantics. `Yes` + Identity, `No` + error reason, `Abstain` + nil identity. The chain logic is straightforward: check Decision, not error type.

**Alternatives considered**:
- Return `(*Identity, error)` with sentinel errors: Ambiguous (nil identity + nil error = abstain vs error?).
- Return `(Decision, *Identity, error)`: Too many return values.

## R2: JWT Library Choice

**Decision**: Use `github.com/golang-jwt/jwt/v5` for JWT parsing and validation, with a custom JWKS fetcher using `net/http`.

**Rationale**: `golang-jwt` is the most widely used Go JWT library (successor to `dgrijalva/jwt-go`). It supports RS256/ES256, custom key functions, and claim validation. The JWKS fetcher is simple enough to implement with stdlib `net/http` + `encoding/json`.

**Alternatives considered**:
- `github.com/lestrrat-go/jwx`: More features but heavier dependency.
- `github.com/coreos/go-oidc`: OIDC-specific, more than we need.
- Custom JWT parsing: Too much crypto code to maintain.

## R3: Rate Limiter Algorithm

**Decision**: Token bucket per (subject, tier) using `golang.org/x/time/rate` or a simple counter with sliding window.

**Rationale**: A simple sliding window counter (requests in the last N seconds) is sufficient for in-process rate limiting. No external state needed. The `sync.Map` provides per-identity counters.

**Alternatives considered**:
- `golang.org/x/time/rate`: Good but adds an external dep for stdlib alternative.
- Redis-backed: Over-engineering for initial implementation.

## R4: API Key Storage Security

**Decision**: Store API key hashes (SHA-256) in the static key store, not plaintext keys. Compare using `crypto/subtle.ConstantTimeCompare` on the hashes.

**Rationale**: Even for static config, storing plaintext keys in memory is a risk if memory is dumped. Hashing on startup and comparing hashes prevents timing attacks and limits exposure.

## R5: Middleware Integration Point

**Decision**: Auth middleware wraps the HTTP handler, not the engine. It runs in the transport layer before the request reaches the engine.

**Rationale**: Auth is a transport concern (HTTP headers, status codes). The engine should not know about auth. The middleware extracts identity, injects tenant, and passes a clean context to the engine.
