# Quickstart 02: Production Setup

Deploy antwort with PostgreSQL persistence, Prometheus metrics collection, and a pre-built Grafana dashboard. This builds on the concepts from [01-minimal](../01-minimal/) by adding durable storage and observability.

**Time to deploy**: 5 minutes (after LLM backend is running)

## Prerequisites

- [Shared LLM Backend](../shared/llm-backend/) deployed and running
- `kubectl` or `oc` CLI configured
- Familiarity with [01-minimal](../01-minimal/) concepts

## Deploy

```bash
# Create namespace
kubectl create namespace antwort

# Deploy everything (antwort + PostgreSQL + Prometheus + Grafana)
kubectl apply -k quickstarts/02-production/base/ -n antwort

# Wait for PostgreSQL to be ready
kubectl rollout status statefulset/postgres -n antwort --timeout=120s

# Wait for antwort to be ready
kubectl rollout status deployment/antwort -n antwort --timeout=60s

# Wait for monitoring stack
kubectl rollout status deployment/prometheus -n antwort --timeout=60s
kubectl rollout status deployment/grafana -n antwort --timeout=60s
```

### OpenShift / ROSA

For external access via Routes:

```bash
# Apply with OpenShift overlay (includes Routes for antwort and Grafana)
kubectl apply -k quickstarts/02-production/openshift/ -n antwort

# Get the route URLs
ANTWORT_ROUTE=$(kubectl get route antwort -n antwort -o jsonpath='{.spec.host}')
GRAFANA_ROUTE=$(kubectl get route grafana -n antwort -o jsonpath='{.spec.host}')
echo "Antwort URL: https://$ANTWORT_ROUTE"
echo "Grafana URL: https://$GRAFANA_ROUTE"
```

## Test

### Setup Port-Forwards

```bash
# Antwort API (vanilla Kubernetes)
kubectl port-forward -n antwort svc/antwort 8080:8080 &

# Grafana dashboard
kubectl port-forward -n antwort svc/grafana 3000:3000 &

# Or use Route URLs on OpenShift (see above)
export URL=http://localhost:8080
```

### Create a Response

```bash
RESPONSE=$(curl -s -X POST "$URL/v1/responses" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "/mnt/models",
    "input": [
      {
        "type": "message",
        "role": "user",
        "content": [{"type": "input_text", "text": "What is the capital of France? Answer in one sentence."}]
      }
    ]
  }')

echo "$RESPONSE" | jq '{id: .id, status: .status, answer: .output[0].content[0].text}'
```

### Test Persistence

Verify that responses survive pod restarts:

```bash
# Save the response ID
RESPONSE_ID=$(echo "$RESPONSE" | jq -r '.id')
echo "Response ID: $RESPONSE_ID"

# Get the current antwort pod name
POD=$(kubectl get pod -n antwort -l app.kubernetes.io/name=antwort -o jsonpath='{.items[0].metadata.name}')
echo "Current pod: $POD"

# Delete the pod (Kubernetes will create a new one)
kubectl delete pod -n antwort "$POD"

# Wait for the new pod to be ready
kubectl rollout status deployment/antwort -n antwort --timeout=60s

# Re-establish port-forward
kubectl port-forward -n antwort svc/antwort 8080:8080 &

# Retrieve the response by ID (should succeed, proving PostgreSQL persistence)
curl -s "$URL/v1/responses/$RESPONSE_ID" | jq '{id: .id, status: .status, answer: .output[0].content[0].text}'
```

### Access Grafana Dashboard

1. Open [http://localhost:3000](http://localhost:3000) in your browser (or the OpenShift Route URL)
2. Anonymous access is enabled for viewing. Admin login: `admin` / `admin`
3. Navigate to **Dashboards** and open **Antwort - OpenResponses Gateway**
4. Send a few requests to see metrics populate the panels

The dashboard includes panels for:

| Panel | Metric |
|-------|--------|
| Request Rate | `antwort_requests_total` |
| Request Duration | `antwort_request_duration_seconds` (p50/p95/p99) |
| Provider Latency | `antwort_provider_latency_seconds` (p50/p95/p99) |
| Active Streaming | `antwort_streaming_connections_active` |
| Token Usage | `gen_ai_client_token_usage` |
| Error Rate | 5xx / total ratio |

## What's Deployed

| Component | Description |
|-----------|-------------|
| antwort | OpenResponses gateway (1 pod) |
| PostgreSQL | Persistent storage (StatefulSet, 5Gi PVC) |
| Prometheus | Metrics collection, scrapes antwort every 15s |
| Grafana | Dashboard UI with pre-built antwort dashboard |
| ConfigMap | PostgreSQL-backed config with DSN from Secret |
| Secret | PostgreSQL credentials and connection string |
| Routes (OpenShift) | Edge TLS for antwort and Grafana |

## Configuration

The `config.yaml` in the ConfigMap uses PostgreSQL storage with the DSN loaded from a mounted Secret file:

```yaml
server:
  port: 8080

engine:
  provider: vllm
  backend_url: http://llm-predictor.llm-serving.svc.cluster.local:8080
  default_model: /mnt/models
  max_turns: 10

storage:
  type: postgres
  postgres:
    dsn_file: /run/secrets/postgres/dsn
    migrate_on_start: true

auth:
  type: none

observability:
  metrics:
    enabled: true
    path: /metrics
```

The PostgreSQL credentials are stored in the `postgres-credentials` Secret. For a real production deployment, replace the default password with a strong one or use a Secret management solution.

## Next Steps

- [04-mcp-tools](../04-mcp-tools/) - Add MCP server for agentic tool calling

## Cleanup

```bash
kubectl delete namespace antwort
```

This also removes the PostgreSQL PVC and all stored data.
