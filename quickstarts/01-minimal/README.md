# Quickstart 01: Minimal Setup

Deploy antwort as an OpenResponses gateway with the simplest possible configuration. In-memory storage, no authentication, connecting to the shared LLM backend.

**Time to deploy**: 5 minutes (after LLM backend is running)

## Prerequisites

- [Shared LLM Backend](../shared/llm-backend/) deployed and running
- `kubectl` or `oc` CLI configured

## Deploy

```bash
# Create namespace
kubectl create namespace antwort

# Deploy antwort
kubectl apply -k quickstarts/01-minimal/ -n antwort

# Wait for pod to be ready
kubectl rollout status deployment/antwort -n antwort --timeout=60s
```

### OpenShift / ROSA

For external access via Route:

```bash
# Apply with OpenShift overlay
kubectl apply -k quickstarts/01-minimal/openshift/ -n antwort

# Get the route URL
ROUTE=$(kubectl get route antwort -n antwort -o jsonpath='{.spec.host}')
echo "Antwort URL: https://$ROUTE"
```

## Test

### Basic Text Completion

```bash
# Using port-forward (vanilla Kubernetes)
kubectl port-forward -n antwort svc/antwort 8080:8080 &

# Or using the Route URL (OpenShift)
# export URL=https://$ROUTE

export URL=http://localhost:8080

curl -s -X POST "$URL/v1/responses" \
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
  }' | jq '{status: .status, answer: .output[0].content[0].text}'
```

Expected output:
```json
{
  "status": "completed",
  "answer": "The capital of France is Paris."
}
```

### Streaming

```bash
curl -s -N -X POST "$URL/v1/responses" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "/mnt/models",
    "stream": true,
    "input": [
      {
        "type": "message",
        "role": "user",
        "content": [{"type": "input_text", "text": "Count from 1 to 5."}]
      }
    ]
  }'
```

### Health Check

```bash
curl -s "$URL/healthz"
# ok
```

### Metrics

```bash
curl -s "$URL/metrics" | grep antwort_requests_total
```

### Test Structured Output

Request a response with a JSON schema to get structured output:

```bash
curl -s -X POST "$URL/v1/responses" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "/mnt/models",
    "input": [
      {
        "type": "message",
        "role": "user",
        "content": [{"type": "input_text", "text": "List 3 programming languages with their year of creation"}]
      }
    ],
    "text": {
      "format": {
        "type": "json_schema",
        "name": "languages",
        "schema": {
          "type": "object",
          "properties": {
            "languages": {
              "type": "array",
              "items": {
                "type": "object",
                "properties": {
                  "name": {"type": "string"},
                  "year": {"type": "integer"}
                },
                "required": ["name", "year"]
              }
            }
          },
          "required": ["languages"]
        }
      }
    }
  }' | jq '.output[] | select(.type == "message") | .content[0].text' -r | jq .
```

The `text.format` field constrains the model to produce valid JSON matching the schema. Expected output:

```json
{
  "languages": [
    {"name": "Python", "year": 1991},
    {"name": "JavaScript", "year": 1995},
    {"name": "Go", "year": 2009}
  ]
}
```

### Test Reasoning

Request a response with reasoning to see the model's thought process:

```bash
curl -s -X POST "$URL/v1/responses" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "/mnt/models",
    "input": [
      {
        "type": "message",
        "role": "user",
        "content": [{"type": "input_text", "text": "What is 15% of 240?"}]
      }
    ],
    "reasoning": {"effort": "medium"}
  }' | jq '{
    output_types: [.output[].type],
    reasoning: [.output[] | select(.type == "reasoning") | .summary[0].text],
    answer: [.output[] | select(.type == "message") | .content[0].text]
  }'
```

Reasoning output depends on model support. If the model does not support reasoning, the response will complete normally without reasoning items.

## What's Deployed

| Component | Description |
|-----------|-------------|
| antwort | OpenResponses gateway (1 pod) |
| ConfigMap | Backend URL, model name, in-memory storage |
| Service | ClusterIP on port 8080 |
| Route (OpenShift) | Edge TLS for external access |

## Configuration

The `config.yaml` in the ConfigMap contains the minimal configuration:

```yaml
server:
  port: 8080
engine:
  provider: vllm
  backend_url: http://llm-predictor.llm-serving.svc.cluster.local:8080
  default_model: /mnt/models
storage:
  type: memory
auth:
  type: none
```

## Next Steps

Ready for more? Continue to [Quickstart 02: Production](../02-production/) to add PostgreSQL persistence and Prometheus monitoring.

## Cleanup

```bash
kubectl delete namespace antwort
```
