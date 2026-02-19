# Implementation Plan: Authentication & Authorization

**Branch**: `007-auth` | **Date**: 2026-02-19 | **Spec**: [spec.md](spec.md)

## Summary

Implement pluggable authentication with three-outcome voting (Yes/No/Abstain), chained authenticators, API key and JWT/OIDC adapters, ownership-based authorization via tenant scoping, optional rate limiting by service tier, and auth bypass for infrastructure endpoints. Auth is transport middleware, bridging to storage multi-tenancy via tenant context injection.

## Technical Context

**Language/Version**: Go 1.22+ (consistent with Specs 001-006)
**Primary Dependencies**: Go stdlib for core + API key. `golang.org/x/crypto` for constant-time comparison (optional). JWT validation needs a JWKS library (adapter package only).
**Testing**: `go test` with table-driven tests. JWT tests use test JWKS server (httptest).
**Constraints**: Core auth interface stdlib-only (Constitution II). JWT adapter is the only external dep.

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Interface-First | PASS | Authenticator (1 method), RateLimiter (1 method) |
| II. Zero Dependencies | PASS | Core stdlib-only. JWT lib in adapter package. |
| III. Nil-Safe Composition | PASS | No auth = NoOp. No rate limiter = no limiting. |
| IV. Typed Error Domain | PASS | 401, 404, 429 via APIError |
| V. Validate Early | PASS | Auth runs before engine |
| VIII. Context Propagation | PASS | Identity + tenant via context |

## Project Structure

```text
pkg/
├── auth/
│   ├── doc.go                    # Package documentation
│   ├── auth.go                   # Authenticator interface, AuthResult, Identity, AuthChain
│   ├── auth_test.go              # Chain voting tests
│   ├── middleware.go             # HTTP middleware (bypass, chain, tenant injection, logging)
│   ├── middleware_test.go        # Middleware integration tests
│   ├── ratelimit.go              # RateLimiter interface + in-process tiered limiter
│   ├── ratelimit_test.go         # Rate limit tests
│   │
│   ├── apikey/
│   │   ├── apikey.go             # API key authenticator + static key store
│   │   └── apikey_test.go        # API key tests
│   │
│   ├── jwt/
│   │   ├── jwt.go                # JWT/OIDC authenticator with JWKS
│   │   └── jwt_test.go           # JWT tests (test JWKS server)
│   │
│   └── noop/
│       └── noop.go               # NoOp authenticator (always Yes)
│
└── engine/                       # No changes (auth is middleware, not engine)

cmd/
└── server/
    └── main.go                   # MODIFIED: wire auth middleware from env config
```

## Complexity Tracking

No constitutional violations. No complexity justifications needed.
