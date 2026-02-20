# Brainstorm 10c: MCP Kubernetes Identity

**Dependencies**: Spec 011 (MCP Client)
**Package**: `pkg/tools/mcp/`

## Purpose

Add Kubernetes-native identity for MCP servers running in the same cluster. Uses projected service account tokens (audience-scoped) or SPIFFE/SPIRE workload identity for mutual authentication without external OAuth infrastructure.

## Architecture

### Service Account Tokens

```
antwort pod (projected token with MCP audience) -> MCP server
MCP server -> TokenReview API (validate token) -> identity confirmed
```

### SPIFFE/SPIRE

```
antwort (SPIFFE SVID) <-> MCP server (SPIFFE SVID)
mutual TLS, no tokens needed
```

## Scope

### Service Account Tokens
- ServiceAccountTokenAuth implementing MCPAuthProvider
- Read projected token from filesystem path
- Automatic token refresh (kubelet rotates tokens)
- Configuration: audience, token path

### SPIFFE/SPIRE (future)
- mTLS configuration for MCP HTTP transport
- SPIFFE trust bundle management
- Deferred until sandbox execution (Spec 11) requires it

## Configuration

```yaml
mcp:
  servers:
    - name: cluster-tools
      url: http://mcp-server.tools.svc.cluster.local:8080/mcp
      auth:
        type: service_account_token
        audience: mcp-server.tools.svc.cluster.local
        token_path: /var/run/secrets/tokens/mcp-token
```

## Pod Spec for Projected Token

```yaml
volumes:
  - name: mcp-token
    projected:
      sources:
        - serviceAccountToken:
            path: mcp-token
            expirationSeconds: 3600
            audience: mcp-server.tools.svc.cluster.local
```

## Deliverables

- [ ] ServiceAccountTokenAuth in `pkg/tools/mcp/auth.go`
- [ ] Token file reading with automatic refresh
- [ ] Configuration in MCP server auth section
- [ ] Tests
