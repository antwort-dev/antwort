---
description: Install Antwort (Background)
---

# Install Antwort (Background)

## Prerequisites Check

Verify AWS/ROSA authentication and model InferenceService availability.

## Configuration

1. Detect backend URL from model InferenceService.
2. If `postgres_url` is empty, deploy co-located PostgreSQL.
3. Set worker count from parameter (default: 1).

## Installation

1. Create namespace and deploy using quickstart 09-background overlay:

```bash
oc create namespace ${NAMESPACE} --dry-run=client -o yaml | oc apply -f -
cd quickstarts/09-background
kustomize build . | oc apply -f - -n ${NAMESPACE}
```

2. If deploying co-located PostgreSQL:

```bash
oc rollout status statefulset/postgres -n ${NAMESPACE} --timeout=120s
```

3. Wait for gateway and worker rollouts:

```bash
oc rollout status deployment/antwort-gateway -n ${NAMESPACE} --timeout=120s
oc rollout status deployment/antwort-worker -n ${NAMESPACE} --timeout=120s
```

## Verification

```bash
ROUTE_URL=$(oc get route antwort -n ${NAMESPACE} -o jsonpath='{.spec.host}')
curl -sf "https://${ROUTE_URL}/v1/healthz" && echo "Antwort gateway is healthy"

# Test background mode
curl -s -X POST "https://${ROUTE_URL}/v1/responses" \
  -H "Content-Type: application/json" \
  -d '{"model":"'${MODEL_NAME}'","input":"test","background":true}' | jq '.status'
```

## Post-Install

Report gateway URL and worker status. Show example background request command.
