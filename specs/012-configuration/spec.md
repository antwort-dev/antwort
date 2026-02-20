# Feature Specification: Unified Configuration System

**Feature Branch**: `012-configuration`
**Created**: 2026-02-20
**Status**: Draft

## Overview

This specification replaces the ad-hoc environment variable configuration in `cmd/server/main.go` with a unified configuration system. Configuration is loaded from a YAML file with environment variable overrides. Sensitive values use file-based references (`_file` suffix) that work transparently with any Kubernetes secret management system (mounted Secrets, External Secrets Operator, Secrets Store CSI, Vault sidecar).

The system watches both config files and secret files for changes, enabling hot reload of credentials and settings without pod restarts. This makes antwort operator-ready: a future operator renders CRDs into ConfigMaps and Secrets, antwort detects the changes and reconfigures itself.

## Clarifications

### Session 2026-02-20

- Q: ConfigMap API watch or volume mount + file watch? -> A: Volume mount + file watch. More portable (works locally and on Kubernetes), no Kubernetes API dependency in the binary, kubelet handles ConfigMap/Secret updates atomically.
- Q: ConfigMap or Secret? -> A: Both. ConfigMap for non-sensitive config (server settings, MCP URLs, log level). Secret for credentials (API keys, passwords). Config references secrets via `_file` fields.
- Q: Integration with secret management systems? -> A: The `_file` pattern is SMS-agnostic. Antwort reads files at configured paths. The infrastructure (mounted Secret, CSI driver, Vault sidecar, External Secrets) delivers files transparently. No proprietary client libraries needed.
- Q: Watch secrets for rotation? -> A: Yes. Watch both config and secret file paths. Secret rotation (API key expiry, credential compromise) is hot-reloadable without restart.
- Q: TOML/JSON config? -> A: YAML only. Kubernetes standard.
- Q: Admin API for runtime config? -> A: Deferred. File watch is sufficient.
- Q: Operator integration? -> A: Operator renders CRD into ConfigMap + Secret. Antwort watches the mounted files. Clean separation: antwort never knows about CRDs.

## User Scenarios & Testing

### User Story 1 - Configure via YAML File (Priority: P1)

An operator creates a `config.yaml` file with all settings (server, engine, storage, auth, MCP) and starts antwort with `--config config.yaml`. The server reads the file, validates it, and starts with the configured settings. Invalid configuration causes a clear error at startup.

**Why this priority**: Replaces the current env var spaghetti with a structured, documented config file.

**Acceptance Scenarios**:

1. **Given** a valid config.yaml, **When** the server starts with `--config config.yaml`, **Then** it applies all settings from the file
2. **Given** a config file with a missing required field (backend URL), **When** the server starts, **Then** it fails with a clear error message identifying the missing field
3. **Given** no config file and no env vars, **When** the server starts, **Then** it fails with an error about the required backend URL

---

### User Story 2 - Environment Variable Override (Priority: P1)

An operator sets environment variables to override specific config file values. Environment variables use the `ANTWORT_` prefix with underscore-separated path segments. Env vars take precedence over file values.

**Acceptance Scenarios**:

1. **Given** a config file with `server.port: 8080` and env `ANTWORT_SERVER_PORT=9090`, **When** the server starts, **Then** it listens on port 9090 (env wins)
2. **Given** only env vars (no config file), **When** the server starts, **Then** it works with env vars alone (backward compatible)

---

### User Story 3 - File-Based Secret References (Priority: P1)

An operator stores sensitive values (API keys, passwords) in files (mounted Kubernetes Secrets) and references them in config via `_file` suffix fields. The server reads the file content at startup.

**Acceptance Scenarios**:

1. **Given** `engine.api_key_file: /run/secrets/backend-key`, **When** the server starts, **Then** it reads the API key from that file path
2. **Given** both `engine.api_key` and `engine.api_key_file`, **When** the server starts, **Then** the `_file` reference takes precedence
3. **Given** a `_file` reference pointing to a non-existent file, **When** the server starts, **Then** it fails with a clear error

---

### User Story 4 - Hot Reload on Config and Secret Changes (Priority: P2)

An operator updates a ConfigMap or rotates a Secret. The kubelet updates the mounted volume files. Antwort detects the file changes and applies hot-reloadable settings without restart.

**Acceptance Scenarios**:

1. **Given** a running server, **When** the config file is updated with a new log level, **Then** the server applies the new log level within seconds
2. **Given** a running server, **When** a secret file (API key) is rotated, **Then** the server uses the new credential for subsequent requests
3. **Given** a hot-reload with invalid config, **When** the server detects the change, **Then** it logs a warning and keeps the previous valid config

---

### User Story 5 - Config File Discovery (Priority: P2)

The server discovers its config file from multiple sources in priority order: `--config` flag, `ANTWORT_CONFIG` env var, `./config.yaml`, `/etc/antwort/config.yaml`. If no file is found, it works with env vars and defaults only.

**Acceptance Scenarios**:

1. **Given** `--config /custom/path.yaml`, **When** the server starts, **Then** it uses that file
2. **Given** no flag but `ANTWORT_CONFIG=/custom/path.yaml`, **When** the server starts, **Then** it uses that file
3. **Given** no flag and no env, **When** `./config.yaml` exists, **Then** it uses that file
4. **Given** no config file found anywhere, **When** the server starts, **Then** it works with env vars and defaults

---

### Edge Cases

- What happens when the config file is deleted while the server is running? The server keeps the last valid config and logs a warning.
- What happens when a secret file becomes empty? The server logs a warning and keeps the previous value (empty credential would break things).
- What happens when the config file has unknown fields? They are silently ignored (forward compatibility for operator-generated configs).
- What happens when env vars and config file have conflicting types (e.g., env string for a numeric field)? The env var is parsed according to the field's type. Parse errors are reported at startup.

## Requirements

### Functional Requirements

**Config Loading**

- **FR-001**: The system MUST support a YAML configuration file with a structured schema covering server, engine, storage, auth, and MCP settings
- **FR-002**: The system MUST support environment variable overrides with `ANTWORT_` prefix and underscore-separated path (e.g., `ANTWORT_SERVER_PORT` for `server.port`)
- **FR-003**: The precedence order MUST be: env vars > config file > compiled defaults
- **FR-004**: The system MUST discover the config file from: `--config` flag, `ANTWORT_CONFIG` env var, `./config.yaml`, `/etc/antwort/config.yaml` (in priority order)
- **FR-005**: If no config file is found, the system MUST work with env vars and defaults only

**Secret References**

- **FR-006**: For every sensitive config field, the system MUST support a `_file` variant that reads the value from a file path (e.g., `engine.api_key_file`)
- **FR-007**: The `_file` variant MUST take precedence over the inline value when both are set
- **FR-008**: The `_file` pattern MUST work with any Kubernetes secret delivery mechanism (mounted Secret, CSI volume, sidecar-injected file) without requiring proprietary client libraries

**Validation**

- **FR-009**: The system MUST validate all configuration at startup and fail fast with clear error messages identifying the specific invalid field
- **FR-010**: Required fields MUST be validated (e.g., backend URL is required)
- **FR-011**: Type mismatches MUST be reported with the field path and expected type

**Hot Reload**

- **FR-012**: The system MUST watch config files and secret files for changes after startup
- **FR-013**: When a watched file changes, the system MUST re-read all config and secret files, validate the new config, and apply hot-reloadable settings
- **FR-014**: Hot-reloadable settings MUST include: MCP server connections, auth credentials, log level, rate limit tiers
- **FR-015**: Settings that require restart MUST include: server listen address, storage type, auth mechanism type
- **FR-016**: If a hot-reload produces invalid config, the system MUST keep the previous valid config and log a warning

**Server Integration**

- **FR-017**: The `cmd/server/main.go` MUST be refactored to use the config system instead of raw `os.Getenv` calls
- **FR-018**: The config system MUST support the `--config` CLI flag for specifying the config file path
- **FR-019**: The system MUST maintain backward compatibility: existing env var names MUST continue to work

### Key Entities

- **Config**: The unified configuration struct with sections for server, engine, storage, auth, MCP, and observability.
- **ConfigLoader**: Discovers and loads config from file + env vars + defaults, resolves `_file` references.
- **ConfigWatcher**: Watches config and secret file paths for changes, triggers reload callbacks.

## Success Criteria

### Measurable Outcomes

- **SC-001**: The server starts correctly from a YAML config file with all settings
- **SC-002**: Environment variables override config file values with correct precedence
- **SC-003**: Secret rotation (file change) is detected and applied within 10 seconds without restart
- **SC-004**: Invalid configuration at startup produces a clear error with field path
- **SC-005**: The `cmd/server/main.go` no longer contains raw `os.Getenv` calls (all config via the config system)
- **SC-006**: Existing env var names (ANTWORT_BACKEND_URL, ANTWORT_MODEL, etc.) continue to work

## Assumptions

- YAML is the only supported config format (Kubernetes standard).
- File watching uses OS-level file notification (fsnotify) or polling with a reasonable interval.
- The `_file` pattern works with any file on the filesystem, regardless of how it was delivered (mounted Secret, CSI, sidecar, manually placed).
- The admin API for runtime config changes is deferred to a future spec.
- An operator rendering CRDs to ConfigMaps is a separate future spec. This spec provides the bridge (file watching).

## Dependencies

- **Spec 006 (Conformance)**: The `cmd/server/main.go` that gets refactored.
- **Spec 007 (Auth)**: Auth config section (type, API keys, JWT settings).
- **Spec 011 (MCP Client)**: MCP server config section.

## Scope Boundaries

### In Scope

- Config struct with YAML parsing
- Environment variable override with `ANTWORT_` prefix
- Config file discovery (flag, env, standard paths)
- `_file` pattern for secret references
- Startup validation with clear errors
- File watching for config and secret hot reload
- Server refactoring to use config system
- Backward compatibility with existing env vars

### Out of Scope

- Admin API (future)
- Operator CRD rendering (future)
- TOML/JSON config formats
- Config encryption at rest
- Remote config sources (consul, etcd)
