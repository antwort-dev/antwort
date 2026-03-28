---
description: Install Antwort (Production)
---

# Install Antwort (Production)

## Prerequisites Check

Check the `<rosa-auth>` context in the system reminder. Verify AWS and ROSA authentication.

Verify a model InferenceService is running:

```bash
oc get inferenceservice -n llm-models -o name
```

## Configuration

1. Detect backend URL from model InferenceService (same as antwort-minimal).
2. If `postgres_url` is empty, deploy a co-located PostgreSQL instance.
3. Set replicas from parameter (default: 2).

## Installation

1. Create namespace, deploy using quickstart 02-production overlay:

```bash
oc create namespace ${NAMESPACE} --dry-run=client -o yaml | oc apply -f -
cd quickstarts/02-production/openshift
kustomize build . | oc apply -f - -n ${NAMESPACE}
```

2. If deploying co-located PostgreSQL, apply the postgres overlay:

```bash
kustomize build quickstarts/02-production/postgres | oc apply -f - -n ${NAMESPACE}
oc rollout status statefulset/postgres -n ${NAMESPACE} --timeout=120s
```

3. Wait for Antwort rollout:

```bash
oc rollout status deployment/antwort -n ${NAMESPACE} --timeout=120s
```

## Verification

```bash
ROUTE_URL=$(oc get route antwort -n ${NAMESPACE} -o jsonpath='{.spec.host}')
curl -sf "https://${ROUTE_URL}/v1/healthz" && echo "Antwort (Production) is healthy"
```

## Post-Install

Report URL and storage configuration to the user.
