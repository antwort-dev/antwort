# Tasks: Authentication & Authorization

**Input**: Design documents from `/specs/007-auth/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, quickstart.md

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup (Core Types & Interface)

**Purpose**: Define the authenticator interface, AuthResult, Identity, and AuthChain.

- [x] T001 (antwort-98a.1) Create `pkg/auth/doc.go` with package documentation. Create `pkg/auth/auth.go` with: AuthDecision enum (Yes/No/Abstain), AuthResult struct, Identity struct, Authenticator interface (single Authenticate method taking context and *http.Request), AuthChain struct with ordered authenticators and default voter. Implement AuthChain.Authenticate that evaluates in order, stops on Yes/No, uses default voter on all-abstain (FR-001, FR-002, FR-003, FR-004).
- [x] T002 (antwort-98a.2) Write AuthChain tests in `pkg/auth/auth_test.go`: first-yes stops chain, first-no stops chain, all-abstain uses default voter (reject), all-abstain with default-accept (NoOp), empty chain uses default voter. Test Identity subject validation (FR-001, FR-002, FR-003).
- [x] T003 (antwort-98a.3) [P] Create `pkg/auth/context.go` with IdentityFromContext and SetIdentity context helpers using private key type.

**Checkpoint**: Auth interface and chain voting ready.

---

## Phase 2: User Story 2 - NoOp Authenticator (Priority: P1)

**Goal**: Development mode with no auth.

- [x] T004 (antwort-gy4.1) [US2] Implement NoOp authenticator in `pkg/auth/noop/noop.go`: always returns Yes with a default identity (subject "anonymous", tier "default"). Write test in `pkg/auth/noop/noop_test.go` (FR-018).

**Checkpoint**: NoOp authenticator works.

---

## Phase 3: User Story 1 - API Key Authenticator (Priority: P1)

**Goal**: Bearer token auth with static key store.

- [x] T005 (antwort-dep.1) [US1] Implement API key authenticator in `pkg/auth/apikey/apikey.go`: extract Bearer token from Authorization header, hash with SHA-256, look up in static key store using constant-time compare. Return Yes (valid), No (invalid token present), Abstain (no Bearer header). Write tests in `pkg/auth/apikey/apikey_test.go` covering valid key, invalid key, no header, tenant mapping (FR-010, FR-011, FR-012).

**Checkpoint**: API key auth works.

---

## Phase 4: User Story 3 + 6 - Auth Middleware (Priority: P1)

**Goal**: Wire auth chain into transport, bypass infra endpoints, inject tenant.

- [x] T006 (antwort-1xg.1) [US3] [US6] Implement auth middleware in `pkg/auth/middleware.go`: HTTP middleware that checks bypass list first, runs AuthChain, extracts tenant from Identity.Metadata["tenant_id"], calls storage.SetTenant, sets Identity in context, logs decisions via slog. Return 401 for auth failures (FR-005, FR-006, FR-007, FR-008, FR-009, FR-026, FR-027).
- [x] T007 (antwort-1xg.2) [US3] [US6] Write middleware tests in `pkg/auth/middleware_test.go`: bypass endpoint passes without auth, valid credentials pass, invalid credentials return 401, tenant injected into context, no auth config = NoOp behavior. Test chain routing (API key handled, JWT handled, both abstain = reject).
- [x] T008 (antwort-1xg.3) [US3] Wire auth middleware into `cmd/server/main.go`: read ANTWORT_AUTH_TYPE env var, create appropriate authenticator(s), wrap HTTP handler with auth middleware. Support "none", "apikey", "jwt", "chain" types (FR-006).

**Checkpoint**: Auth middleware works end-to-end.

---

## Phase 5: User Story 4 - JWT/OIDC Authenticator (Priority: P2)

**Goal**: JWT validation against JWKS endpoint.

- [x] T009 (antwort-gw2.1) [US4] Implement JWT authenticator in `pkg/auth/jwt/jwt.go`: extract Bearer token, parse JWT, fetch and cache JWKS from configured URL, validate signature/expiration/issuer/audience, extract subject/tenant/scopes from configurable claims. Return Yes/No/Abstain. Write tests in `pkg/auth/jwt/jwt_test.go` with test JWKS server (httptest) (FR-013, FR-014, FR-015, FR-016, FR-017).
- [x] T010 (antwort-gw2.2) [US4] Add JWT dependency (golang-jwt/jwt/v5) to go.mod.

**Checkpoint**: JWT auth works.

---

## Phase 6: User Story 5 - Rate Limiting (Priority: P2)

**Goal**: Per-tier request rate limiting.

- [ ] T011 (antwort-6mj.1) [US5] Implement in-process rate limiter in `pkg/auth/ratelimit.go`: sliding window counter per (subject, tier) using sync.Map. Configurable requests-per-minute per tier. Fail open on internal errors. Write tests in `pkg/auth/ratelimit_test.go` covering within-limit, over-limit, unknown tier (use default), nil limiter (FR-021, FR-022, FR-023, FR-024, FR-025).
- [x] T012 (antwort-6mj.2) [US5] Integrate rate limiter into auth middleware in `pkg/auth/middleware.go`: after successful authentication, check rate limit. Return 429 if exceeded.

**Checkpoint**: Rate limiting works.

---

## Phase 7: Polish

- [x] T013 (antwort-ggj.1) [P] Run `go vet ./...` and `go test ./...` across all packages.
- [x] T014 (antwort-ggj.2) [P] Verify conformance tests still pass with auth disabled (NoOp default).
- [x] T015 (antwort-ggj.3) Run `make conformance` to verify no regressions.

---

## Dependencies & Execution Order

- **Phase 1 (Setup)**: No dependencies.
- **Phase 2 (NoOp)**: Depends on Phase 1.
- **Phase 3 (API Key)**: Depends on Phase 1. Parallel with Phase 2.
- **Phase 4 (Middleware)**: Depends on Phases 2+3 (needs at least one authenticator).
- **Phase 5 (JWT)**: Depends on Phase 1. Independent of Phases 2-4.
- **Phase 6 (Rate Limit)**: Depends on Phase 4 (middleware).
- **Phase 7 (Polish)**: Depends on all.

### Parallel Opportunities

- Phase 2 (NoOp) and Phase 3 (API Key) in parallel
- Phase 5 (JWT) can start after Phase 1, parallel with Phases 2-4

## Implementation Strategy

### MVP First

1. Phase 1: Core types
2. Phase 2+3: NoOp + API Key (parallel)
3. Phase 4: Middleware integration
4. **STOP**: Auth works with API keys

### Incremental

5. Phase 5: JWT (enterprise SSO)
6. Phase 6: Rate limiting
7. Phase 7: Polish
