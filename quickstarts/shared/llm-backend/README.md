# Shared: LLM Backend (vLLM + Qwen 2.5 7B)

This deploys a vLLM inference server with Qwen 2.5 7B Instruct as the backend model for all quickstarts.

## Prerequisites

- Kubernetes cluster with a GPU node (NVIDIA A10G 24GB or better)
- `kubectl` or `oc` CLI configured
- NVIDIA GPU Operator installed (for GPU scheduling)

## Deploy

```bash
# Create namespace
kubectl create namespace llm-serving

# Deploy vLLM with Qwen 2.5 7B
kubectl apply -k quickstarts/shared/llm-backend/ -n llm-serving

# Wait for model download (2-3 minutes)
kubectl wait --for=condition=complete job/download-model -n llm-serving --timeout=300s

# Wait for vLLM to start (1-2 minutes after download)
kubectl rollout status deployment/llm-predictor -n llm-serving --timeout=300s
```

## Verify

```bash
# Check model is serving
kubectl exec -n llm-serving deploy/llm-predictor -- \
  curl -s http://localhost:8080/v1/models | jq '.data[].id'

# Test inference
kubectl exec -n llm-serving deploy/llm-predictor -- \
  curl -s http://localhost:8080/v1/chat/completions \
    -H "Content-Type: application/json" \
    -d '{"model":"/mnt/models","messages":[{"role":"user","content":"Hello"}],"max_tokens":50}' \
  | jq '.choices[0].message.content'
```

## Internal Service URL

Other quickstarts connect to this backend via:

```
http://llm-predictor.llm-serving.svc.cluster.local:8080
```

## Model Details

| Property | Value |
|----------|-------|
| Model | Qwen/Qwen2.5-7B-Instruct |
| Size | ~14GB (FP16) |
| GPU | 1x A10G (24GB) or equivalent |
| Tool calling | Enabled (hermes parser) |
| Max context | 8192 tokens |

## Cleanup

```bash
kubectl delete namespace llm-serving
```
