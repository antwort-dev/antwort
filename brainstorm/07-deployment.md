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
- Helm chart and Kustomize overlays (Helm for users, Kustomize for the base)
- Health checks (liveness, readiness, startup probes)
- Observability (metrics, structured logging, distributed tracing)
- Configuration management (env vars, config file, ConfigMap)
- Horizontal scaling considerations
- Multi-tenant deployment patterns

### Out of Scope
- CI/CD pipeline definition (project-specific)
- Full operator implementation (see "Future: Operator Pattern" section for direction)
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

Single binary and image:
- `antwort-server` - standalone HTTP/gRPC server

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
    TraceSampleRate float64 `env:"ANTWORT_TRACE_SAMPLE_RATE" default:"0.1"`
    TraceExporter string `env:"ANTWORT_TRACE_EXPORTER" default:"otlp"`
}
```

### Tracing Configuration

Tracing uses OpenTelemetry with configurable sampling and export:

| Variable | Description | Default |
|----------|-------------|---------|
| `ANTWORT_TRACE_ENDPOINT` | Collector address (e.g., `otel-collector:4317`) | (disabled) |
| `ANTWORT_TRACE_SAMPLE_RATE` | Fraction of requests to trace | `0.1` (10%) |
| `ANTWORT_TRACE_EXPORTER` | Export format: `otlp`, `jaeger`, or `stdout` | `otlp` |

Use 10% sampling in production to keep overhead low while still capturing representative traces. Increase to `1.0` in dev/staging for full visibility.

## Kubernetes Resources

### Kustomize Base

```
deploy/kubernetes/
├── base/
│   ├── kustomization.yaml
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── configmap.yaml
│   └── serviceaccount.yaml
├── overlays/
│   ├── dev/
│   │   ├── kustomization.yaml
│   │   └── config-patch.yaml
│   ├── production/
│   │   ├── kustomization.yaml
│   │   ├── config-patch.yaml
│   │   ├── hpa.yaml
│   │   └── pdb.yaml
│   └── openshift/
│       ├── kustomization.yaml
│       ├── route.yaml
│       └── servicemonitor.yaml
```

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

### Production Overlay: HorizontalPodAutoscaler

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: antwort
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: antwort
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
    - type: Pods
      pods:
        metric:
          name: antwort_streaming_connections_active
        target:
          type: AverageValue
          averageValue: "50"
```

### Production Overlay: PodDisruptionBudget

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: antwort
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: antwort
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

## Helm Chart

The Helm chart provides a user-facing, parameterized deployment. The Kustomize base remains the underlying structure. Helm is the primary interface for users, while Kustomize serves as the foundation for base manifests and overlay customization.

```
deploy/helm/antwort/
├── Chart.yaml
├── values.yaml
├── templates/
│   ├── _helpers.tpl
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── configmap.yaml
│   ├── secret.yaml
│   ├── serviceaccount.yaml
│   ├── hpa.yaml
│   ├── pdb.yaml
│   ├── ingress.yaml
│   └── servicemonitor.yaml
```

### values.yaml

```yaml
replicaCount: 2

image:
  repository: ghcr.io/rhuss/antwort
  tag: latest

server:
  httpAddr: ":8080"
  grpcAddr: ":9090"
  metricsAddr: ":9091"

provider:
  backendType: chat_completions
  modelEndpoint: ""
  apiKeySecret: ""
  maxTokens: 4096
  timeout: 60s

storage:
  type: postgres
  postgres:
    host: ""
    port: 5432
    database: antwort
    existingSecret: ""

auth:
  enabled: false
  type: api_key

observability:
  metrics:
    enabled: true
    serviceMonitor: false
  tracing:
    enabled: false
    exporter: otlp
    endpoint: ""
    sampleRate: 0.1
  logging:
    level: info
    format: json

ingress:
  enabled: false
  className: nginx
  hosts: []
  tls: []

autoscaling:
  enabled: false
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70

postgresql:
  enabled: false  # deploy PostgreSQL alongside (uses subchart)
```

### Multi-Tenant Deployment

Each tenant gets a separate Helm release with its own namespace, backend, and database:

```bash
# Tenant A: team-alpha, uses Llama 3 on their own vLLM
helm install team-alpha deploy/helm/antwort/ \
  --namespace team-alpha --create-namespace \
  --set provider.modelEndpoint=http://vllm-alpha.team-alpha.svc:8000/v1 \
  --set storage.postgres.host=pg.team-alpha.svc

# Tenant B: team-beta, uses Mistral on a shared vLLM
helm install team-beta deploy/helm/antwort/ \
  --namespace team-beta --create-namespace \
  --set provider.modelEndpoint=http://vllm-shared.inference.svc:8000/v1 \
  --set storage.postgres.host=pg.team-beta.svc
```

Isolation is enforced by Kubernetes:
- Separate namespaces
- Separate Deployments
- Separate databases (or same database with different schemas)
- NetworkPolicy restricts cross-namespace traffic
- RBAC restricts who can modify each tenant's resources

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

#### LLM-Tuned Histogram Buckets

Standard Prometheus default buckets are designed for typical web services and are too fine-grained for LLM inference latencies. Use these custom bucket boundaries for duration histograms:

```go
// LLM-tuned buckets: captures fast validation (0.1s), prompt processing (0.5-2s),
// standard inference (5-10s), long generation (30-60s), and timeouts (120s).
var llmDurationBuckets = []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120}
```

These buckets apply to `antwort_request_duration_seconds`, `antwort_provider_latency_seconds`, and `antwort_storage_latency_seconds`. They are specifically chosen for LLM workloads where inference calls commonly range from sub-second to two minutes.

### Structured Logging

JSON-formatted logs with:
- Request ID correlation
- Model and provider context
- Streaming event counts
- Error details with stack traces (debug level)

Library: `slog` (Go standard library).

#### Request Log Field Reference

Every completed request should emit a structured log entry with these fields:

| Field | Type | Description |
|-------|------|-------------|
| `request_id` | string | Unique request identifier (generated or from `X-Request-ID` header) |
| `tenant_id` | string | Tenant identifier (from auth context) |
| `model` | string | Model name used for inference |
| `duration_ms` | int64 | Total request duration in milliseconds |
| `input_tokens` | int | Number of input tokens consumed |
| `output_tokens` | int | Number of output tokens generated |
| `tool_calls` | int | Number of tool calls executed during the request |
| `status` | string | Final response status (e.g., `completed`, `failed`, `cancelled`) |

Example log entry:

```go
logger.Info("request completed",
    "request_id", requestID,
    "tenant_id", tenantID,
    "model", model,
    "duration_ms", duration.Milliseconds(),
    "input_tokens", usage.InputTokens,
    "output_tokens", usage.OutputTokens,
    "tool_calls", toolCallCount,
    "status", responseStatus,
)
```

### Distributed Tracing

OpenTelemetry integration:
- Span per request
- Child spans for provider calls, tool executions, storage operations
- Trace context propagation via W3C headers

#### Trace Span Hierarchy

The full request lifecycle produces the following nested span structure:

```
HTTP Request
├── validate_request
├── resolve_conversation
│   └── storage.GetResponse (previous_response_id lookup)
├── expand_tools
│   ├── expand_mcp_tools
│   │   ├── mcp.ListTools
│   │   └── mcp.CallTool (per tool call)
│   └── expand_file_search_tools
├── inference (agentic loop)
│   ├── backend.CreateResponseStream (iteration 1)
│   │   └── sse_stream_processing
│   ├── execute_tool_calls
│   │   ├── mcp.CallTool
│   │   └── vectorstore.Search
│   └── backend.CreateResponseStream (iteration 2)
├── storage.SaveResponse
└── stream_to_client
```

Each span carries attributes for the model name, request ID, and (where applicable) tool name or storage operation type. This hierarchy makes it straightforward to identify whether latency originates from the provider, tool execution, storage, or streaming.

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
- **Custom deployment patterns**: Adapt manifests for specific infrastructure needs

## Future: Operator Pattern

As a potential evolution beyond Helm, an operator-based approach could manage antwort instances declaratively via Kubernetes custom resources. This section captures the direction without committing to a full design.

### CRD Concepts

Two primary custom resources would define the operator's API surface:

- **AntwortGateway**: The primary CR that configures a gateway instance, including replica count, backend provider, storage, auth, and observability settings. The controller would create or update the underlying Deployment, Service, ConfigMap, and wiring of Secrets.

- **ProviderBinding**: Registers additional backend providers dynamically. Enables runtime provider switching without redeploying the gateway.

### Why Consider This

- Enables GitOps workflows where CRs in a git repository are applied by ArgoCD or Flux
- Supports dynamic reconfiguration (adding providers, rotating secrets) without manual Helm upgrades
- Fits naturally into environments that already use the operator pattern for other infrastructure

### Implementation Notes

If pursued, the controller would be a separate binary (`cmd/controller/main.go`), built with `controller-runtime`, using standard reconciliation loops and leader election. The gateway itself would remain unaware of CRDs.

This is explicitly out of scope for the initial deployment story. It becomes relevant only if demand warrants the additional complexity.

## Open Questions

- Should we include a PostgreSQL StatefulSet in the manifests or assume external database?
- Network policies for restricting provider access?

## Deliverables

- [ ] `Containerfile` - Multi-stage build for server
- [ ] `deploy/base/` - Kustomize base (Deployment, Service, ConfigMap)
- [ ] `deploy/overlays/openshift/` - OpenShift overlay (Route, ServiceMonitor)
- [ ] `deploy/overlays/production/` - Production overlay (HPA, PDB, tuned resources)
- [ ] `deploy/overlays/dev/` - Development overlay (reduced resources, debug logging)
- [ ] `deploy/helm/antwort/` - Helm chart with values.yaml
- [ ] `cmd/server/main.go` - Server entrypoint with config loading
- [ ] `pkg/health/health.go` - Health check registry
- [ ] `pkg/observability/metrics.go` - Prometheus metrics with LLM-tuned buckets
- [ ] `pkg/observability/tracing.go` - OpenTelemetry setup with configurable sampling
