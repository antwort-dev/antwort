# Feature Specification: Authentication & Authorization

**Feature Branch**: `007-auth`
**Created**: 2026-02-19
**Status**: Draft
**Input**: User description: "Authentication and authorization for the Antwort OpenResponses gateway: pluggable authenticator chain with three-outcome voting, API key and JWT adapters, authorization, rate limiting, tenant injection."

## Overview

This specification defines the authentication and authorization layer for antwort. Auth is implemented as transport middleware, keeping it decoupled from business logic. The authenticator uses a chain-of-responsibility pattern with three-outcome voting (yes/no/abstain): each authenticator examines the request and either authenticates it, rejects it, or passes to the next in the chain. A configurable default voter handles the case where all authenticators abstain.

The spec delivers three authenticator adapters (NoOp, API key, JWT/OIDC), an ownership-based authorizer, an optional rate limiter tied to service tiers, and the middleware that wires it all together. The auth middleware also injects the tenant identity into the request context, bridging auth with storage multi-tenancy (Spec 005).

## Clarifications

### Session 2026-02-19

- Q: Should rate limiting be part of the auth spec or separate? -> A: Part of auth. Rate limiting needs the Identity (who) and service tier (what), which are auth concerns. It's an optional component: nil rate limiter = no limiting.
- Q: Should we support chained authenticators? -> A: Yes, with three-outcome voting. Each authenticator returns Yes (identity found, stop), No (credentials present but invalid, stop and reject), or Abstain (can't handle, try next). A default voter decides if all abstain (typically reject, unless NoOp is the default).
- Q: Audit logging: auth spec or observability? -> A: Auth middleware logs security decisions via slog (subject, action, result, remote_addr). Detailed audit trails are Spec 07.
- Q: Do we need all adapters in the initial spec? -> A: P1: NoOp + API key. P2: JWT/OIDC. P3: OAuth proxy + mTLS (deferred to future spec).
- Q: How does auth connect to storage tenant isolation? -> A: The auth middleware extracts tenant from Identity.Metadata["tenant_id"] and calls storage.SetTenant(ctx, tenantID). This bridges auth and storage multi-tenancy.
- Q: What is the relationship between subject and tenant? -> A: Subject is the individual user (e.g., "alice"). Tenant is the organization or scope (e.g., "org-1"). In multi-tenant deployments, multiple users share a tenant and can see each other's responses within that tenant. In single-user deployments, the subject can serve as the tenant. The storage layer scopes by tenant, not by subject. Ownership isolation within a tenant is a future enhancement.

## User Scenarios & Testing

### User Story 1 - API Key Authentication (Priority: P1)

A developer configures antwort with a set of API keys. Each request must include a valid API key in the Authorization header (`Bearer sk-...`). Requests with invalid or missing keys are rejected with a 401 error. Each key maps to an identity with a subject, service tier, and optional tenant.

**Why this priority**: API key auth is the simplest production auth mechanism. It enables immediate deployment with access control.

**Independent Test**: Configure two API keys, send requests with valid key (accepted), invalid key (rejected), and no key (rejected). Verify identity is correctly populated.

**Acceptance Scenarios**:

1. **Given** API key auth is configured with key "sk-abc" mapped to user "alice", **When** a request with `Authorization: Bearer sk-abc` is sent, **Then** the request is authenticated with subject "alice"
2. **Given** API key auth is configured, **When** a request with an invalid key is sent, **Then** the request is rejected with 401 Unauthorized
3. **Given** API key auth is configured, **When** a request with no Authorization header is sent, **Then** the request is rejected with 401 Unauthorized
4. **Given** API key auth is configured with tenant "org-1" on a key, **When** the request is authenticated, **Then** the tenant "org-1" is injected into the context for storage scoping

---

### User Story 2 - No Authentication (Development Mode) (Priority: P1)

A developer runs antwort without any authentication configured. All requests are accepted. This is the default behavior and matches the current state (Specs 001-006 have no auth).

**Why this priority**: Development mode must work out of the box. NoOp is the fallback when no auth is configured.

**Independent Test**: Start antwort with no auth config, send requests, verify all are accepted.

**Acceptance Scenarios**:

1. **Given** no authenticator is configured, **When** any request is sent, **Then** it is accepted (NoOp behavior)
2. **Given** NoOp is the default voter in the chain, **When** all configured authenticators abstain, **Then** the request is accepted

---

### User Story 3 - Auth Chain with Voting (Priority: P1)

An operator configures multiple authenticators (e.g., API key + JWT). The auth middleware tries each in order using three-outcome voting. If the first authenticator recognizes the credentials (Yes or No), the chain stops. If it abstains, the next authenticator is tried. If all abstain, the default voter decides.

**Why this priority**: Chain support is necessary for environments that accept both API keys and JWTs.

**Independent Test**: Configure API key + JWT authenticators. Send request with API key (handled by first). Send request with JWT (first abstains, second handles). Send request with no credentials (both abstain, default voter decides).

**Acceptance Scenarios**:

1. **Given** a chain [API key, JWT] with default reject, **When** a valid API key is sent, **Then** the API key authenticator returns Yes and the chain stops
2. **Given** a chain [API key, JWT] with default reject, **When** an invalid API key is sent, **Then** the API key authenticator returns No, the chain stops, and the request is rejected
3. **Given** a chain [API key, JWT] with default reject, **When** a valid JWT is sent, **Then** the API key authenticator abstains, the JWT authenticator returns Yes
4. **Given** a chain [API key, JWT] with default reject, **When** no credentials are sent, **Then** both abstain, the default voter rejects with 401

---

### User Story 4 - JWT/OIDC Authentication (Priority: P2)

An operator configures antwort with a JWT/OIDC provider. JWTs are validated against a JWKS endpoint. The subject, tenant, and scopes are extracted from configurable claims. Expired or invalid tokens are rejected.

**Why this priority**: JWT/OIDC enables SSO integration for enterprise deployments.

**Independent Test**: Configure JWT auth with a test JWKS endpoint. Send request with valid JWT (accepted). Send expired JWT (rejected). Send JWT with wrong audience (rejected).

**Acceptance Scenarios**:

1. **Given** JWT auth configured with issuer and audience, **When** a valid JWT is sent, **Then** the identity is populated from claims (sub, tenant, scopes)
2. **Given** JWT auth configured, **When** an expired JWT is sent, **Then** the authenticator returns No (rejected)
3. **Given** JWT auth configured, **When** a JWT with wrong audience is sent, **Then** the authenticator returns No (rejected)
4. **Given** JWT auth configured, **When** a request with no bearer token is sent, **Then** the authenticator returns Abstain

---

### User Story 5 - Rate Limiting by Service Tier (Priority: P2)

An operator configures rate limits per service tier. After authentication, the rate limiter checks whether the identity's tier has capacity. Requests exceeding the limit are rejected with 429 Too Many Requests.

**Why this priority**: Rate limiting prevents abuse and enables tiered service offerings.

**Independent Test**: Configure a "standard" tier with 10 req/min. Send 11 requests within a minute. Verify the 11th is rejected with 429.

**Acceptance Scenarios**:

1. **Given** a "standard" tier with 10 req/min, **When** 10 requests are sent within a minute, **Then** all are accepted
2. **Given** a "standard" tier with 10 req/min, **When** the 11th request is sent, **Then** it is rejected with 429 Too Many Requests
3. **Given** no rate limiter is configured, **When** any number of requests are sent, **Then** all are accepted (no limiting)

---

### User Story 6 - Auth Bypass for Infrastructure Endpoints (Priority: P2)

An operator deploys antwort with auth enabled. Kubernetes liveness and readiness probes must work without authentication. The auth middleware bypasses auth for configured infrastructure endpoints.

**Why this priority**: Auth bypass is essential for Kubernetes deployments with health probes.

**Independent Test**: Configure auth with API keys. Access /healthz without credentials. Verify it succeeds. Access /v1/responses without credentials. Verify it fails with 401.

**Acceptance Scenarios**:

1. **Given** auth is enabled, **When** `/healthz` is accessed without credentials, **Then** the request is accepted (bypassed)
2. **Given** auth is enabled, **When** `/v1/responses` is accessed without credentials, **Then** the request is rejected with 401
3. **Given** a custom bypass list ["healthz", "/custom"], **When** `/custom` is accessed, **Then** it is bypassed

---

### Edge Cases

- What happens when the JWKS endpoint is unreachable? The JWT authenticator returns a server error (500), not a 401. The error is logged. Cached JWKS continue to be used if available.
- What happens when an API key is valid but the identity has no service tier? The default tier is used (configurable, defaults to "default").
- What happens when the rate limiter storage is full? The rate limiter should fail open (allow requests) rather than fail closed (reject). This prevents rate limiter failures from causing total service outage.
- What happens when two authenticators both return Yes for the same request? This shouldn't happen with proper abstain logic. If it does, the first Yes wins (chain stops immediately).
- What happens when Identity.Subject is empty after successful auth? The middleware rejects the request. A valid identity must have a non-empty subject.

## Requirements

### Functional Requirements

**Authenticator Interface**

- **FR-001**: The system MUST define an authenticator interface with a single method that returns one of three outcomes: Yes (identity), No (rejected), or Abstain (not my credentials)
- **FR-002**: The system MUST support chaining multiple authenticators. The chain evaluates authenticators in order, stopping on the first Yes or No. If all abstain, a configurable default voter decides.
- **FR-003**: The default voter MUST be configurable: reject (production default) or accept (development default, equivalent to NoOp)

**Identity**

- **FR-004**: An authenticated identity MUST contain a non-empty subject (unique identifier), an optional service tier, optional scopes, and optional metadata
- **FR-005**: The tenant identifier (if present in identity metadata) MUST be injected into the request context via `storage.SetTenant` for storage multi-tenancy scoping

**Auth Middleware**

- **FR-006**: Auth MUST be implemented as transport middleware, running before the engine processes the request
- **FR-007**: The middleware MUST check the bypass list before attempting authentication. Bypassed endpoints skip all auth checks.
- **FR-008**: The middleware MUST log authentication decisions (success/failure) with structured fields: subject, action, result, remote_addr
- **FR-009**: The middleware MUST return 401 Unauthorized for authentication failures and 404 Not Found for authorization failures (ownership-based isolation, not 403)

**API Key Authenticator**

- **FR-010**: The system MUST provide an API key authenticator that validates bearer tokens against a key store
- **FR-011**: The key store MUST support static configuration (configured list of keys with identity mappings)
- **FR-012**: The API key authenticator MUST return Yes when a valid key is found, No when a bearer token is present but invalid, and Abstain when no bearer token is present

**JWT/OIDC Authenticator**

- **FR-013**: The system MUST provide a JWT authenticator that validates tokens against a JWKS endpoint
- **FR-014**: The JWT authenticator MUST validate signature, expiration, not-before, issuer, and audience
- **FR-015**: The JWT authenticator MUST extract subject, tenant, and scopes from configurable claim names
- **FR-016**: The JWT authenticator MUST cache the JWKS to avoid fetching on every request
- **FR-017**: The JWT authenticator MUST return Yes when a valid JWT is found, No when a bearer token is present but invalid JWT, and Abstain when no bearer token is present

**NoOp Authenticator**

- **FR-018**: The system MUST provide a NoOp authenticator that always returns Yes with a default identity. Used for development and as a default voter in the chain.

**Authorization**

- **FR-019**: The system MUST enforce tenant-based isolation: users can only access responses within their tenant scope. Cross-tenant access MUST return 404, not 403. In single-user deployments, the subject serves as the tenant.
- **FR-020**: Authorization MUST use the identity's tenant (from Identity.Metadata["tenant_id"]) to scope storage queries via `storage.SetTenant`. When no tenant is present, storage operates without scoping (backward compatible with single-tenant mode).

**Rate Limiting**

- **FR-021**: The system MUST support optional rate limiting per service tier
- **FR-022**: Rate limits MUST be configurable per tier: requests per minute
- **FR-023**: When rate limits are exceeded, the system MUST return 429 Too Many Requests
- **FR-024**: When no rate limiter is configured, all requests are allowed (nil-safe)
- **FR-025**: The rate limiter MUST fail open: if the limiter itself errors, the request is allowed

**Auth Bypass**

- **FR-026**: The system MUST support a configurable list of endpoints that bypass authentication
- **FR-027**: Default bypass endpoints MUST include `/healthz` and `/readyz`

### Key Entities

- **Authenticator**: A pluggable component that examines request credentials and returns Yes/No/Abstain.
- **Identity**: The authenticated caller's identity with subject, service tier, scopes, and metadata.
- **AuthChain**: An ordered list of authenticators with a default voter for when all abstain.
- **AuthMiddleware**: Transport middleware that runs the auth chain, injects identity, and enforces authorization.
- **RateLimiter**: An optional component that enforces request rate limits per service tier.

## Success Criteria

### Measurable Outcomes

- **SC-001**: API key authentication correctly accepts valid keys and rejects invalid keys with 401
- **SC-002**: The auth chain with three-outcome voting correctly routes requests through multiple authenticators
- **SC-003**: JWT authentication validates tokens against a JWKS endpoint, extracting subject and tenant
- **SC-004**: Tenant injection bridges auth identity to storage multi-tenancy scoping
- **SC-005**: Rate limiting enforces per-tier request limits with 429 responses
- **SC-006**: Infrastructure endpoints (healthz, readyz) are accessible without authentication
- **SC-007**: Cross-user resource access returns 404 (not 403) for ownership-based isolation
- **SC-008**: An engine with no auth configured preserves existing behavior (all requests accepted)

## Assumptions

- API key validation uses constant-time comparison to prevent timing attacks.
- The JWKS cache has a configurable TTL (default 1 hour) and refreshes in the background.
- Rate limiting is in-process (per instance). Distributed rate limiting (Redis-backed) is a future enhancement.
- The NoOp authenticator is used for development. In production, at least one real authenticator should be configured.
- OAuth proxy and mTLS adapters are deferred to a future spec (P3). The interface supports them when needed.

## Dependencies

- **Spec 001 (Core Protocol)**: Error types (401, 403, 429 mapping).
- **Spec 002 (Transport Layer)**: Middleware chain integration.
- **Spec 005 (Storage)**: Tenant context injection via `storage.SetTenant`.
- **Spec 006 (Conformance)**: Server binary needs auth wiring.

## Scope Boundaries

### In Scope

- Authenticator interface with three-outcome voting (Yes/No/Abstain)
- Auth chain with default voter
- API key authenticator with static key store
- JWT/OIDC authenticator with JWKS validation
- NoOp authenticator (development/default voter)
- Auth middleware (bypass, identity injection, tenant bridging)
- Ownership-based authorization (404 for cross-user access)
- Rate limiting by service tier
- Structured auth logging

### Out of Scope

- User management / key provisioning (external concern)
- OAuth proxy adapter (future)
- mTLS adapter (future)
- Distributed rate limiting (Redis)
- Fine-grained RBAC beyond ownership isolation
- Audit trail persistence (Spec 07)
