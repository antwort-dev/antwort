# Quickstart 06: Responses API Proxy (Gateway Architecture)

Deploy antwort in a two-tier proxy chain: a frontend instance using the `vllm-responses` provider forwards Responses API requests to a backend instance using the standard `vllm` provider. Both instances run the same antwort image with different configurations, demonstrating how antwort can proxy through its own Responses API for gateway architectures.

```
User --> Frontend (vllm-responses provider) --> Backend (vllm provider) --> LLM
```

**Time to deploy**: 5 minutes (after LLM backend is running)

## Prerequisites

- [Shared LLM Backend](../shared/llm-backend/) deployed and running
- `kubectl` or `oc` CLI configured

## Deploy

```bash
# Create namespace
kubectl create namespace antwort

# Deploy backend and frontend
kubectl apply -k quickstarts/06-responses-proxy/base/ -n antwort

# Wait for both pods to be ready
kubectl rollout status deployment/antwort-backend -n antwort --timeout=60s
kubectl rollout status deployment/antwort-frontend -n antwort --timeout=60s
```

### OpenShift / ROSA

For external access via Route (frontend only, backend stays cluster-internal):

```bash
# Apply with OpenShift overlay
kubectl apply -k quickstarts/06-responses-proxy/openshift/ -n antwort

# Get the route URL
ROUTE=$(kubectl get route antwort-frontend -n antwort -o jsonpath='{.spec.host}')
echo "Antwort URL: https://$ROUTE"
```

## Test

### Basic Text Completion (through proxy)

```bash
# Using port-forward to the frontend (vanilla Kubernetes)
kubectl port-forward -n antwort svc/antwort-frontend 8080:8080 &

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

### Streaming (through proxy chain)

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

### Direct Backend Test (optional)

To compare, you can test the backend directly:

```bash
kubectl port-forward -n antwort svc/antwort-backend 8081:8080 &

curl -s -X POST "http://localhost:8081/v1/responses" \
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

## What's Deployed

| Component | Description |
|-----------|-------------|
| antwort-backend | Antwort with `vllm` provider, connects to the LLM backend (1 pod) |
| antwort-frontend | Antwort with `vllm-responses` provider, proxies to backend (1 pod) |
| ConfigMaps | Separate configs for backend (vllm) and frontend (vllm-responses) |
| Services | ClusterIP on port 8080 for each instance |
| Route (OpenShift) | Edge TLS for external access to frontend only |

## Configuration

This quickstart uses two provider types to create the proxy chain:

- **Backend** (`engine.provider: vllm`): Connects directly to the LLM serving endpoint and translates between the Responses API and the vLLM-compatible Chat Completions API.
- **Frontend** (`engine.provider: vllm-responses`): Forwards requests to another Responses API endpoint (the backend). This enables gateway patterns where a frontend instance can add authentication, routing, or other middleware before proxying to one or more backend instances.

The frontend's `engine.backend_url` points to the backend's in-cluster Service (`http://antwort-backend:8080`), so all traffic stays within the cluster.

## Next Steps

This is the final quickstart in the current series. See the [quickstarts overview](../) for the full list.

## Cleanup

```bash
kubectl delete -k quickstarts/06-responses-proxy/base/ -n antwort
```
