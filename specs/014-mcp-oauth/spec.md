# Feature Specification: MCP OAuth Client Credentials

**Feature Branch**: `014-mcp-oauth`
**Created**: 2026-02-20
**Status**: Draft

## Overview

This specification adds OAuth 2.0 client_credentials authentication for MCP servers. When an MCP server is behind an OAuth-protected API, antwort authenticates as a service client by obtaining an access token from the OAuth token endpoint. Tokens are cached and refreshed proactively before expiry to avoid interrupting agentic loops.

This extends the existing `MCPAuthProvider` interface from Spec 011. The `StaticKeyAuth` (API key) provider already works. This adds `OAuthClientCredentialsAuth` as a second provider.

## Clarifications

### Session 2026-02-20

- Q: Token caching strategy? -> A: In-process cache per provider instance. Cache token with TTL from `expires_in`. Refresh at 80% of lifetime.
- Q: Token refresh during agentic loop? -> A: If refresh fails and token is still valid, use existing token. If token is expired and refresh fails, fail the tool call (model can retry).
- Q: Multiple OAuth providers? -> A: Yes. Each MCP server has its own auth config. One server can use API key, another OAuth.
- Q: Credentials sourcing? -> A: client_id and client_secret via `_file` references to mounted Kubernetes Secrets (Spec 012 pattern).

## User Scenarios & Testing

### User Story 1 - Connect to OAuth-Protected MCP Server (Priority: P1)

An operator configures an MCP server with OAuth client_credentials authentication. Antwort obtains an access token from the token endpoint, caches it, and includes it as a Bearer token in all MCP requests to that server. The agentic loop works transparently.

**Acceptance Scenarios**:

1. **Given** an MCP server configured with OAuth client_credentials, **When** the first tool call is made, **Then** antwort obtains an access token and sends it as Bearer token
2. **Given** a cached token, **When** subsequent tool calls are made, **Then** the cached token is reused (no token endpoint call)
3. **Given** a token near expiry (>80% lifetime), **When** a tool call is made, **Then** the token is refreshed proactively before the request

---

### User Story 2 - Handle Token Failures Gracefully (Priority: P1)

An operator's OAuth token endpoint becomes unreachable. Antwort handles the failure gracefully within the agentic loop.

**Acceptance Scenarios**:

1. **Given** a valid cached token and unreachable token endpoint, **When** proactive refresh fails, **Then** the existing token is used (still valid)
2. **Given** an expired token and unreachable token endpoint, **When** a tool call is made, **Then** the tool call fails with a clear error (model can retry or choose a different approach)
3. **Given** invalid credentials (wrong client_secret), **When** token acquisition is attempted, **Then** a clear error is returned identifying the authentication failure

---

### Edge Cases

- What happens when the token endpoint returns an unexpected response format? The provider returns an error. The tool call fails.
- What happens when `expires_in` is missing from the token response? Default to 3600 seconds (1 hour), the OAuth standard default.
- What happens when two concurrent tool calls both trigger a token refresh? Only one refresh is performed (mutex-protected). The second call waits for the first refresh to complete.

## Requirements

### Functional Requirements

- **FR-001**: The system MUST provide an OAuth client_credentials auth provider implementing the existing MCPAuthProvider interface
- **FR-002**: The provider MUST obtain an access token from the configured token endpoint using the client_credentials grant type
- **FR-003**: The provider MUST cache the access token and reuse it for subsequent requests within its validity period
- **FR-004**: The provider MUST refresh the token proactively when 80% of the token's lifetime has elapsed
- **FR-005**: If proactive refresh fails, the provider MUST continue using the existing token if it is still valid
- **FR-006**: If the token is expired and refresh fails, the provider MUST return an error for the tool call
- **FR-007**: The provider MUST support configurable scopes in the token request
- **FR-008**: The provider MUST source client_id and client_secret from `_file` references (Kubernetes Secrets) or inline config values
- **FR-009**: Concurrent token refreshes MUST be serialized (only one refresh at a time)
- **FR-010**: The MCP server config MUST support an `auth` section with `type: oauth_client_credentials` and fields for token_url, client_id, client_secret, and scopes

### Key Entities

- **OAuthClientCredentialsAuth**: MCPAuthProvider implementation that obtains and caches OAuth access tokens.
- **TokenCache**: In-process cache holding the current access token and its expiry.

## Success Criteria

- **SC-001**: An MCP tool call to an OAuth-protected server succeeds with a valid access token
- **SC-002**: Token caching avoids redundant token endpoint calls (one token per validity period)
- **SC-003**: Token refresh failures degrade gracefully (use existing valid token, fail only when expired)

## Assumptions

- The OAuth token endpoint follows the standard RFC 6749 client_credentials grant response format (access_token, token_type, expires_in).
- Token caching is in-process (per antwort instance). No distributed token cache.
- The MCP SDK's HTTP transport accepts custom headers (already verified in Spec 011 with StaticKeyAuth).

## Dependencies

- **Spec 011 (MCP Client)**: MCPAuthProvider interface and MCP client infrastructure.
- **Spec 012 (Configuration)**: `_file` pattern for credential sourcing.

## Scope Boundaries

### In Scope

- OAuthClientCredentialsAuth provider
- Token endpoint integration (RFC 6749 client_credentials)
- Token caching with proactive refresh
- Per-server OAuth config in config.yaml
- Concurrent refresh serialization
- Error handling (expired token, unreachable endpoint)

### Out of Scope

- Token exchange (RFC 8693) (Spec 10b)
- Kubernetes service account tokens (Spec 10c)
- PKCE or authorization code grants (not applicable for service-to-service)
- Distributed token caching
