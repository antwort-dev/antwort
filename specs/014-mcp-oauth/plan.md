# Implementation Plan: MCP OAuth Client Credentials

**Branch**: `014-mcp-oauth` | **Date**: 2026-02-20 | **Spec**: [spec.md](spec.md)

## Summary

Add OAuthClientCredentialsAuth to the MCP auth provider system. Token endpoint integration, caching, proactive refresh, per-server config.

## Technical Context

**Dependencies**: None new. Uses stdlib `net/http` for token endpoint calls, `sync.Mutex` for concurrent refresh serialization.

## Project Structure

```text
pkg/tools/mcp/
├── auth.go              # MODIFIED: Add OAuthClientCredentialsAuth
├── auth_test.go         # NEW: OAuth token tests with mock endpoint
├── config.go            # MODIFIED: Add auth section to MCPServerConfig
```
