# Tasks: MCP OAuth Client Credentials

## Phase 1: OAuth Provider Implementation (P1)

- [ ] T001 [US1] Implement OAuthClientCredentialsAuth in `pkg/tools/mcp/auth.go`: GetHeaders() obtains/caches token from token endpoint. Token cache with expiry tracking. Proactive refresh at 80% lifetime. Mutex for concurrent refresh (FR-001 to FR-009).
- [ ] T002 [US1] [US2] Write OAuth auth tests in `pkg/tools/mcp/auth_test.go`: mock token endpoint (httptest), verify token acquisition, caching (no re-fetch), proactive refresh, expired token error, concurrent refresh serialization, invalid credentials error (FR-001 to FR-009).

---

## Phase 2: Config Integration (P1)

- [ ] T003 [US1] Update `pkg/tools/mcp/config.go`: add MCPAuthConfig struct with type, token_url, client_id, client_id_file, client_secret, client_secret_file, scopes. Update ServerConfig with Auth field (FR-008, FR-010).
- [ ] T004 [US1] Update MCP executor/client creation in `cmd/server/main.go` to construct OAuthClientCredentialsAuth when auth.type is "oauth_client_credentials" (FR-010).

---

## Phase 3: Polish

- [ ] T005 [P] Run `go vet ./...` and `go test ./...`.
- [ ] T006 [P] Update config.example.yaml with OAuth MCP server example.

---

## Dependencies

- Phase 1: No dependencies.
- Phase 2: Depends on Phase 1.
- Phase 3: Depends on all.
