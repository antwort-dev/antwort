# Research: Unified Configuration System

**Feature**: 012-configuration
**Date**: 2026-02-20

## R1: YAML Library

**Decision**: Use `gopkg.in/yaml.v3` for YAML parsing.

**Rationale**: Standard Go YAML library. Supports struct tags, nested types, and custom unmarshalers. Already widely used in the Kubernetes ecosystem.

## R2: Env Override Strategy

**Decision**: Flatten the config struct path with `_` separator and `ANTWORT_` prefix. Use reflection or a helper to map env vars to struct fields.

**Rationale**: Standard pattern (Docker, 12-factor apps). `server.port` becomes `ANTWORT_SERVER_PORT`. Existing env vars (ANTWORT_BACKEND_URL) map to `engine.backend_url` for backward compatibility.

**Backward compatibility mapping**:
- `ANTWORT_BACKEND_URL` -> `engine.backend_url`
- `ANTWORT_MODEL` -> `engine.default_model`
- `ANTWORT_PORT` -> `server.port`
- `ANTWORT_PROVIDER` -> `engine.provider`
- `ANTWORT_STORAGE` -> `storage.type`
- `ANTWORT_AUTH_TYPE` -> `auth.type`
- `ANTWORT_API_KEYS` -> `auth.api_keys` (JSON array)
- `ANTWORT_MCP_SERVERS` -> `mcp.servers` (JSON array)

## R3: File Watching

**Decision**: Use `github.com/fsnotify/fsnotify` for OS-level file notifications. Fall back to polling (every 5s) if fsnotify is unavailable or the filesystem doesn't support inotify.

**Rationale**: fsnotify is the standard Go file watching library. However, Kubernetes volume mounts use symlinks that may not trigger inotify correctly on all OS versions. The polling fallback ensures reliability.

**Alternative**: Pure polling. Simpler but less responsive. Acceptable if we want zero external deps.

## R4: _file Resolution

**Decision**: After YAML parsing and env override, scan all `_file` fields. For each non-empty `_file` field, read the file and populate the corresponding value field. Trim whitespace/newlines from file content (common with mounted Secrets).

**Rationale**: File content from Kubernetes Secrets often has trailing newlines. Trimming prevents auth failures from invisible whitespace.
