---
description: Install Antwort (RAG)
---

# Install Antwort (RAG)

## Prerequisites Check

Verify AWS/ROSA authentication and model InferenceService availability.

## Configuration

1. Detect backend URL from model InferenceService.
2. Select vector store type from parameter (default: memory).

## Installation

1. Create namespace and deploy using quickstart 08-rag overlay:

```bash
oc create namespace ${NAMESPACE} --dry-run=client -o yaml | oc apply -f -
cd quickstarts/08-rag
kustomize build . | oc apply -f - -n ${NAMESPACE}
```

2. If vector_store=qdrant, deploy Qdrant alongside:

```bash
# Deploy Qdrant StatefulSet
# (Qdrant manifests would be in the quickstart overlay)
oc rollout status statefulset/qdrant -n ${NAMESPACE} --timeout=120s
```

3. Wait for Antwort rollout:

```bash
oc rollout status deployment/antwort -n ${NAMESPACE} --timeout=120s
```

## Verification

```bash
ROUTE_URL=$(oc get route antwort -n ${NAMESPACE} -o jsonpath='{.spec.host}')
curl -sf "https://${ROUTE_URL}/v1/healthz" && echo "Antwort (RAG) is healthy"
# Verify Files API is available
curl -sf "https://${ROUTE_URL}/v1/files" && echo "Files API available"
```

## Post-Install

Report URL and RAG configuration. Suggest uploading a test file to verify the pipeline.
