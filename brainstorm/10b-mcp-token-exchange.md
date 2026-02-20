# Brainstorm 10b: MCP OAuth Token Exchange

**Dependencies**: Spec 10a (OAuth Client Credentials), Spec 007 (Auth)
**Package**: `pkg/tools/mcp/`

## Purpose

Add OAuth 2.0 token exchange (RFC 8693) for MCP servers that need to know which user is making the request. Antwort exchanges the user's token (from the incoming request) for a token scoped to the MCP server, propagating user identity through the agentic loop.

## Architecture

```
user -> antwort (Bearer user_token)
antwort -> token endpoint (exchange user_token for mcp_token)
antwort -> MCP server (Bearer mcp_token) -> tool results
```

## Why This Matters

In multi-tenant deployments, MCP servers enforce per-user access control. A shared service credential (client_credentials) doesn't carry user identity. Token exchange maps the user's identity to an MCP-scoped token, enabling fine-grained authorization at the tool level.

## Scope

- OAuthTokenExchangeAuth implementing MCPAuthProvider
- RFC 8693 token exchange request (subject_token, requested_token_type)
- User token extraction from request context (via auth middleware identity)
- Per-request token exchange (no caching across users)
- Configuration in MCP server auth section

## Configuration

```yaml
mcp:
  servers:
    - name: user-scoped-tools
      url: http://mcp-server:8080/mcp
      auth:
        type: oauth_token_exchange
        token_url: https://auth.example.com/oauth/token
        client_id: antwort-gateway
        client_secret_file: /run/secrets/oauth-client-secret
        audience: mcp-server-audience
        scopes: ["mcp:tools:execute"]
```

## Key Design Decision

Token exchange happens per-request (per-user), not cached across users. This is because:
- Each user gets a different exchanged token
- Tokens are short-lived (minutes)
- Caching per-user tokens adds complexity with limited benefit

## Dependencies

- Spec 007 (Auth): Provides user identity in request context
- Spec 10a: Shares OAuth token endpoint infrastructure (HTTP client, error handling)

## Deliverables

- [ ] OAuthTokenExchangeAuth in `pkg/tools/mcp/auth.go`
- [ ] RFC 8693 token exchange request builder
- [ ] User token extraction from auth.IdentityFromContext
- [ ] Tests with mock token endpoint
