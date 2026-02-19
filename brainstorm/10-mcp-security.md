# Brainstorm: MCP Security (OAuth, Token Exchange)

**Related to**: Spec 10 (MCP Client)
**Priority**: High (first-class value proposition)

## Why This Matters

MCP servers in production environments require authentication. An agentic gateway that can only connect to unauthenticated MCP servers is a toy. Secure MCP connections are a core differentiator for antwort as an enterprise-grade gateway.

## Authentication Scenarios

### 1. API Key Authentication

The simplest case. The MCP server expects a static API key in a header.

- Config: `Headers: {"Authorization": "Bearer sk-mcp-server-key"}`
- Already supported via the `Headers` field in MCPServerConfig
- Credential source: Kubernetes Secret referenced in the ConfigMap

### 2. OAuth 2.0 Client Credentials

The MCP server is behind an OAuth-protected API. Antwort authenticates as a client (not on behalf of a user) using client_credentials grant.

- Antwort obtains an access token from the OAuth token endpoint
- Token is cached and refreshed before expiry
- The access token is sent as a Bearer token in MCP requests

```
antwort -> token endpoint (client_id + client_secret) -> access_token
antwort -> MCP server (Bearer access_token) -> tool results
```

### 3. OAuth 2.0 Token Exchange (RFC 8693)

The MCP server needs to know which user is making the request (not just that antwort is authorized). Antwort exchanges the user's token (from the incoming request) for a token scoped to the MCP server.

```
user -> antwort (Bearer user_token)
antwort -> token endpoint (exchange user_token for mcp_token)
antwort -> MCP server (Bearer mcp_token) -> tool results
```

This is critical for multi-tenant deployments where the MCP server enforces per-user access control.

### 4. Kubernetes Service Account Token (TokenReview)

For MCP servers running in the same cluster, Kubernetes service account tokens provide identity without external OAuth infrastructure.

- Antwort projects a service account token for the MCP server's audience
- The MCP server validates the token via Kubernetes TokenReview API
- No external identity provider needed

### 5. mTLS with SPIFFE/SPIRE

For sandbox-based MCP execution (Spec 11), workload identity via SPIFFE/SPIRE provides mutual authentication without tokens.

## Design Principles

1. **Credential isolation**: MCP server credentials are never exposed to the model or the user. They flow through antwort only.
2. **Per-server auth**: Each MCP server can have its own auth mechanism. One server uses API keys, another uses OAuth.
3. **Token caching**: OAuth tokens are cached and refreshed proactively (before expiry).
4. **User context propagation**: For token exchange, the user's identity flows from the incoming request to the MCP server call.
5. **Secret management**: Credentials come from Kubernetes Secrets, never from ConfigMaps or environment variables.

## Interface Sketch

```go
// MCPAuthProvider obtains credentials for an MCP server connection.
type MCPAuthProvider interface {
    // GetCredentials returns headers to attach to MCP requests.
    // Called before each MCP request (implementations should cache tokens).
    GetCredentials(ctx context.Context) (map[string]string, error)
}

// Implementations:
// - StaticAuthProvider (API key from Secret)
// - OAuthClientCredentials (client_credentials grant)
// - OAuthTokenExchange (RFC 8693, exchanges user token)
// - ServiceAccountToken (Kubernetes projected token)
```

## Phasing

- **P1**: Static API key (already supported via Headers config)
- **P2**: OAuth client_credentials (server-to-server auth)
- **P3**: OAuth token exchange (user context propagation)
- **P4**: Kubernetes service account tokens, SPIFFE/SPIRE

## Open Questions

- Should the MCPAuthProvider be part of the MCP spec (10) or a separate auth-for-tools spec?
- How to handle token refresh failures during an agentic loop? (Fail the tool call, let the model retry?)
- Should antwort support multiple OAuth providers (one per MCP server)?
