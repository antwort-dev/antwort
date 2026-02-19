# Data Model: Authentication & Authorization

**Feature**: 007-auth
**Date**: 2026-02-19

## Core Types

### AuthDecision (Enum)

| Value | Meaning |
|-------|---------|
| `Yes` | Credentials valid, identity populated. Chain stops. |
| `No` | Credentials present but invalid. Chain stops, reject. |
| `Abstain` | Can't handle these credentials. Try next authenticator. |

### AuthResult (Value)

| Field | Type | Description |
|-------|------|-------------|
| Decision | AuthDecision | The three-outcome vote |
| Identity | *Identity | Populated when Decision == Yes |
| Err | error | Populated when Decision == No (reason for rejection) |

### Identity (Value)

| Field | Type | Description |
|-------|------|-------------|
| Subject | string | Unique user identifier (required, non-empty) |
| ServiceTier | string | Rate limit tier (default: "default") |
| Scopes | []string | Authorization scopes (optional) |
| Metadata | map[string]string | Auth-provider-specific data, includes tenant_id |

### AuthChain

| Field | Type | Description |
|-------|------|-------------|
| Authenticators | []Authenticator | Ordered list, evaluated left to right |
| DefaultVoter | AuthDecision | What to do when all abstain (Yes or No) |

### TierConfig

| Field | Type | Description |
|-------|------|-------------|
| RequestsPerMinute | int | Max requests per minute for this tier |

## Interfaces

### Authenticator

| Method | Signature | Description |
|--------|-----------|-------------|
| Authenticate | `(ctx, *http.Request) -> AuthResult` | Examine request, return Yes/No/Abstain |

### RateLimiter

| Method | Signature | Description |
|--------|-----------|-------------|
| Allow | `(ctx, *Identity) -> error` | Check if request is within limits. nil = allowed. |
