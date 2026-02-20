# Brainstorm 10a: MCP OAuth Client Credentials

**Dependencies**: Spec 011 (MCP Client)
**Package**: `pkg/tools/mcp/` (extends existing MCPAuthProvider)

## Purpose

Add OAuth 2.0 client_credentials grant as an authentication method for MCP servers. Antwort authenticates as a service client (not on behalf of a user) and obtains an access token from an OAuth token endpoint. The token is cached and refreshed before expiry.

## Architecture

```
antwort -> token endpoint (client_id + client_secret) -> access_token (cached)
antwort -> MCP server (Bearer access_token) -> tool results
```

## Scope

- OAuthClientCredentialsAuth implementing MCPAuthProvider
- Token endpoint call with client_id/client_secret
- Token caching with automatic refresh before expiry
- Per-server OAuth config in config.yaml
- Credential sourcing via `_file` references (Kubernetes Secrets)

## Configuration

```yaml
mcp:
  servers:
    - name: enterprise-tools
      url: http://mcp-server:8080/mcp
      auth:
        type: oauth_client_credentials
        token_url: https://auth.example.com/oauth/token
        client_id: antwort-gateway
        client_id_file: /run/secrets/oauth-client-id
        client_secret_file: /run/secrets/oauth-client-secret
        scopes: ["mcp:tools:execute"]
```

## Token Lifecycle

1. First MCP request: obtain token from token endpoint
2. Cache token with TTL (from `expires_in` response field)
3. Refresh proactively when token is 80% through its lifetime
4. On refresh failure: use existing token if still valid, error if expired
5. On token endpoint unreachable: fail the tool call (model can retry)

## Decisions

- Token refresh failures during an agentic loop fail the tool call. The model retries or uses a different approach. Same as MCP server disconnection.
- Multiple OAuth providers supported. Each MCP server has its own auth config.
- Token caching is in-process (per instance). No shared cache.

## Deliverables

- [ ] OAuthClientCredentialsAuth in `pkg/tools/mcp/auth.go`
- [ ] Token caching with automatic refresh
- [ ] Config integration (auth section in MCP server config)
- [ ] Tests with mock token endpoint (httptest)
