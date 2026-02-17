# Spec 09: Configuration

**Branch**: `spec/09-configuration`
**Dependencies**: Spec 07 (Deployment)
**Package**: `github.com/rhuss/antwort/pkg/config`

## Purpose

Define a unified configuration model that works across all deployment contexts: local development, container deployment, and Kubernetes-native environments. The configuration system provides clear precedence rules, startup validation, and runtime reload capabilities.

## Configuration Layers (precedence, highest wins)

```
1. Defaults (compiled into binary)
2. Config file (config.yaml)
3. Environment variables (ANTWORT_*)
4. Admin API (runtime hot reload, optional)
```

Each layer can override the previous. The server resolves the final configuration at startup, then optionally watches for changes.

## Configuration Structure

All configuration in a single, well-documented struct:

```go
type Config struct {
    Server        ServerConfig        `yaml:"server"`
    Engine        EngineConfig        `yaml:"engine"`
    Storage       StorageConfig       `yaml:"storage"`
    Auth          AuthConfig          `yaml:"auth"`
    Observability ObservabilityConfig `yaml:"observability"`
}
```

## Full Configuration Reference

```yaml
# Server settings
server:
  host: 0.0.0.0
  port: 8080
  read_timeout: 30s
  write_timeout: 120s          # long for streaming SSE connections
  idle_timeout: 60s
  max_request_body: 10MB
  shutdown_timeout: 30s
  admin_port: 0                # internal admin API (0 = disabled)

# Engine / inference backend
engine:
  backend_type: chat_completions   # "chat_completions" or "responses"
  model_endpoint: ""               # backend URL (required)
  api_key: ""                      # backend auth (prefer env/secret)
  default_model: ""                # model when request omits it
  max_tokens: 4096                 # default max output tokens
  timeout: 120s                    # per-request timeout to backend
  max_tool_calls: 10               # agentic loop iteration limit

  # Multiple providers (optional, enables model-based routing)
  providers: []
  # - name: llama3-8b
  #   type: chat_completions
  #   endpoint: http://vllm-small:8000/v1
  #   models: ["meta-llama/Llama-3-8B"]

# Storage
storage:
  type: memory                     # "memory" or "postgres"

  postgres:
    dsn: ""                        # full connection string
    host: localhost
    port: 5432
    database: antwort
    user: ""
    password: ""                   # prefer env/secret
    sslmode: require
    max_connections: 25
    max_idle: 5
    conn_max_lifetime: 5m

# Authentication
auth:
  type: none                       # "none", "api_key", "jwt", "mtls"

  api_key:
    header: Authorization
    keys_file: ""                  # path to keys YAML
    keys: []

  jwt:
    issuer: ""
    audience: ""
    jwks_url: ""
    user_claim: sub
    scopes_claim: scope

# Observability
observability:
  logging:
    level: info                    # debug, info, warn, error
    format: json                   # json, text

  metrics:
    enabled: true
    path: /metrics

  tracing:
    enabled: false
    exporter: otlp                 # otlp, jaeger, stdout
    endpoint: ""
    sample_rate: 0.1
```

## Environment Variable Mapping

Every config field maps to an environment variable with `ANTWORT_` prefix and `_` separating nested levels:

```
server.port            -> ANTWORT_SERVER_PORT
engine.backend_type    -> ANTWORT_ENGINE_BACKEND_TYPE
engine.model_endpoint  -> ANTWORT_ENGINE_MODEL_ENDPOINT
engine.api_key         -> ANTWORT_ENGINE_API_KEY
storage.type           -> ANTWORT_STORAGE_TYPE
storage.postgres.dsn   -> ANTWORT_STORAGE_POSTGRES_DSN
auth.type              -> ANTWORT_AUTH_TYPE
```

Environment variables always override file values. Sensitive values (api_key, passwords, DSN) should come from env vars or Kubernetes Secrets, not config files.

## Config File Discovery

The server looks for configuration in this order:
1. Path specified by `--config` flag
2. `ANTWORT_CONFIG` environment variable
3. `./config.yaml` (current directory)
4. `/etc/antwort/config.yaml` (standard mount path)
5. No file found: use defaults + environment variables only

## Validation

Configuration is validated at startup. The server fails fast with clear error messages:

```go
func (c *Config) Validate() error {
    if c.Engine.ModelEndpoint == "" {
        return fmt.Errorf("engine.model_endpoint is required")
    }
    if c.Storage.Type == "postgres" && c.Storage.Postgres.DSN == "" {
        if c.Storage.Postgres.Host == "" {
            return fmt.Errorf("storage.postgres.host is required when type=postgres")
        }
    }
    if c.Auth.Type == "jwt" && c.Auth.JWT.JWKSURL == "" {
        return fmt.Errorf("auth.jwt.jwks_url is required when type=jwt")
    }
    // ... etc
}
```

## Runtime Reload

Some configuration can change without restart. Others require restart.

**Hot-reloadable** (via admin API or file watch):
- Provider registrations (add/remove backends)
- Auth keys (add/revoke API keys)
- Log level

**Restart required**:
- Storage type/connection
- Server listen address/port
- Auth type change (api_key to jwt)

## Admin API

When `server.admin_port` is configured, the server exposes an internal API on a separate port (not exposed via Ingress or public Service):

```
POST   /internal/config/reload         # re-read config file
GET    /internal/config                 # dump current config (redacted)
POST   /internal/providers             # register provider
DELETE /internal/providers/{name}      # remove provider
PUT    /internal/log-level             # change log level
GET    /internal/health                # detailed health check
```

## Secrets Handling

Sensitive values should never appear in config files or ConfigMaps. The server supports:

1. **Environment variables**: `ANTWORT_ENGINE_API_KEY=sk-...`
2. **File references**: `api_key_file: /run/secrets/api-key` (reads value from file at startup)
3. **Kubernetes Secrets**: Mounted as env vars or files via Pod spec

The config dump endpoint (`GET /internal/config`) redacts all sensitive fields.

## Kubernetes Configuration Paths

From simple to advanced:

**Path 1: ConfigMap + Secrets (manual)**
```
ConfigMap (config.yaml) + Secret (credentials)
    -> mounted into Pod
    -> server reads at startup
    -> restart to apply changes
```

**Path 2: Helm values (parameterized)**
```
values.yaml
    -> Helm renders ConfigMap + Secret templates
    -> helm upgrade to apply changes
    -> rolling restart
```

## Open Questions

- Should we support TOML or JSON config files in addition to YAML?
- How to handle configuration for multiple providers with overlapping model names?
- Should the admin API require its own authentication?

## Deliverables

- [ ] `pkg/config/config.go` - Config struct with YAML/env parsing
- [ ] `pkg/config/validate.go` - Validation with fail-fast error messages
- [ ] `pkg/config/loader.go` - Config file discovery and layer merging
- [ ] `pkg/config/admin.go` - Admin API endpoints (optional)
- [ ] Tests for validation, loading, env override, and layer precedence
