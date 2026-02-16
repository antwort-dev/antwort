# Spec 07: Deployment & Operations

**Branch**: `spec/07-deployment`
**Dependencies**: All previous specs (this is the integration spec)
**Package**: N/A (deployment artifacts, not Go code)

## Purpose

Define the deployment model, container build, Kubernetes/OpenShift manifests, and observability stack for running antwort in production.

## Scope

### In Scope
- Multi-stage container image build (Containerfile)
- Kubernetes manifests (Deployment, Service, ConfigMap, Secret)
- OpenShift-specific resources (Route, ServiceMonitor)
- Helm chart or Kustomize overlays
- Health checks (liveness, readiness, startup probes)
- Observability (metrics, structured logging, distributed tracing)
- Configuration management (env vars, config file, ConfigMap)
- Horizontal scaling considerations

### Out of Scope
- CI/CD pipeline definition (project-specific)
- Operator pattern (future, if demand warrants)
- Multi-cluster deployment

## Container Image

```dockerfile
# Multi-stage build
FROM golang:1.22 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /antwort ./cmd/server

FROM gcr.io/distroless/static-debian12
COPY --from=builder /antwort /antwort
ENTRYPOINT ["/antwort"]
```

Two binaries, two images:
- `antwort-server` - standalone HTTP/gRPC server
- `antwort-extproc` - Envoy ext_proc sidecar

## Configuration

All configuration via environment variables with optional config file override:

```go
type Config struct {
    // Server
    HTTPAddr  string `env:"ANTWORT_HTTP_ADDR" default:":8080"`
    GRPCAddr  string `env:"ANTWORT_GRPC_ADDR" default:":9090"`
    MetricsAddr string `env:"ANTWORT_METRICS_ADDR" default:":9091"`

    // Provider
    Provider ProviderConfig

    // Storage
    Database PostgresConfig

    // Auth
    Auth AuthConfig

    // Observability
    LogLevel  string `env:"ANTWORT_LOG_LEVEL" default:"info"`
    LogFormat string `env:"ANTWORT_LOG_FORMAT" default:"json"`
    TraceEndpoint string `env:"ANTWORT_TRACE_ENDPOINT"`
}
```

## Kubernetes Resources

### Core Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: antwort
spec:
  replicas: 2
  selector:
    matchLabels:
      app: antwort
  template:
    spec:
      containers:
        - name: antwort
          image: ghcr.io/rhuss/antwort:latest
          ports:
            - name: http
              containerPort: 8080
            - name: grpc
              containerPort: 9090
            - name: metrics
              containerPort: 9091
          envFrom:
            - configMapRef:
                name: antwort-config
            - secretRef:
                name: antwort-secrets
          livenessProbe:
            httpGet:
              path: /healthz
              port: http
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /readyz
              port: http
            periodSeconds: 5
          startupProbe:
            httpGet:
              path: /healthz
              port: http
            failureThreshold: 30
            periodSeconds: 2
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              memory: 512Mi
```

### Health Endpoints

```
GET /healthz  - Liveness: process is alive
GET /readyz   - Readiness: database connected, provider reachable
```

Readiness checks:
- PostgreSQL connection pool has available connections
- At least one provider is reachable
- (Optional) MCP servers connected

## Observability

### Metrics (Prometheus)

```
# Request metrics
antwort_requests_total{method, status, model}
antwort_request_duration_seconds{method, model}
antwort_streaming_connections_active{model}

# Provider metrics
antwort_provider_requests_total{provider, model, status}
antwort_provider_latency_seconds{provider, model}
antwort_provider_tokens_total{provider, model, direction}

# Agentic loop metrics
antwort_agent_loop_turns_total{model}
antwort_tool_executions_total{tool_name, status}

# Storage metrics
antwort_storage_operations_total{operation, status}
antwort_storage_latency_seconds{operation}

# Rate limiting
antwort_ratelimit_rejected_total{tier}
```

### Structured Logging

JSON-formatted logs with:
- Request ID correlation
- Model and provider context
- Streaming event counts
- Error details with stack traces (debug level)

Library: `slog` (Go standard library).

### Distributed Tracing

OpenTelemetry integration:
- Span per request
- Child spans for provider calls, tool executions, storage operations
- Trace context propagation via W3C headers

## OpenShift Specific

```yaml
# Route for external access
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: antwort
spec:
  tls:
    termination: edge
  to:
    kind: Service
    name: antwort

# ServiceMonitor for Prometheus scraping
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: antwort
spec:
  endpoints:
    - port: metrics
      interval: 15s
```

## Scaling Considerations

- **Horizontal scaling**: Stateless tier scales freely. Stateful tier scales with connection pool limits.
- **Database connections**: Each pod gets `maxConns / replicas` connections.
- **Streaming**: Long-lived SSE connections consume goroutines. Monitor `streaming_connections_active`.
- **Provider bottleneck**: vLLM throughput is usually the limit, not antwort itself.

## Extension Points

- **Custom health checks**: Register additional readiness checks via `HealthRegistry`
- **Custom metrics**: Register Prometheus collectors for internal tools or custom providers
- **Sidecar injection**: The ext_proc image can be used as an Envoy sidecar

## Open Questions

- Helm chart vs Kustomize? Kustomize aligns better with OpenShift patterns.
- Should we include a PostgreSQL StatefulSet in the manifests or assume external database?
- Pod disruption budget and rolling update strategy?
- Network policies for restricting provider access?

## Deliverables

- [ ] `Containerfile` - Multi-stage build for server
- [ ] `Containerfile.extproc` - Build for ext_proc binary
- [ ] `deploy/base/` - Kustomize base (Deployment, Service, ConfigMap)
- [ ] `deploy/overlays/openshift/` - OpenShift overlay (Route, ServiceMonitor)
- [ ] `deploy/overlays/dev/` - Development overlay (reduced resources, debug logging)
- [ ] `cmd/server/main.go` - Server entrypoint with config loading
- [ ] `cmd/extproc/main.go` - ext_proc entrypoint
- [ ] `pkg/health/health.go` - Health check registry
- [ ] `pkg/observability/metrics.go` - Prometheus metrics
- [ ] `pkg/observability/tracing.go` - OpenTelemetry setup
