---
description: Install Antwort (Minimal)
---

# Install Antwort (Minimal)

## Prerequisites Check

Check the `<rosa-auth>` context in the system reminder.

**If AWS Valid is False:**
- Inform the user they need to authenticate with AWS first

**If ROSA Valid is False:**
- Inform the user they need to run `rosa login`

### Component-Specific Prerequisites

Verify a model InferenceService is running:

```bash
oc get inferenceservice -n llm-models -o name
```

If no InferenceService exists, inform the user they need to install a model first via `/rosa:install model`.

## Configuration

Determine the backend URL from the model InferenceService:

```bash
# Get the internal service URL for the first available InferenceService
MODEL_NAME=$(oc get inferenceservice -n llm-models -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
BACKEND_URL="http://${MODEL_NAME}.llm-models.svc.cluster.local/v1"
```

If `backend_url` parameter is provided and non-empty, use that instead of auto-detection.

Set namespace from parameter (default: `antwort`).

## Installation

1. Create the namespace if it doesn't exist:

```bash
oc create namespace ${NAMESPACE} --dry-run=client -o yaml | oc apply -f -
```

2. Deploy Antwort using the quickstart manifests. Write a kustomization overlay to `/tmp/antwort-kustomization.yaml`:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - github.com/antwort-dev/antwort/quickstarts/01-minimal/openshift?ref=main
patches:
  - patch: |-
      - op: replace
        path: /data/ANTWORT_BACKEND_URL
        value: "${BACKEND_URL}"
    target:
      kind: ConfigMap
      name: antwort-config
namespace: ${NAMESPACE}
```

Write this to `/tmp/antwort-kustomization.yaml`, then apply:

```bash
kustomize build /tmp/ | oc apply -f - -n ${NAMESPACE}
```

Alternatively, if the antwort repo is checked out locally, use the local quickstart:

```bash
cd quickstarts/01-minimal/openshift
kustomize build . | oc apply -f - -n ${NAMESPACE}
```

3. Wait for rollout:

```bash
oc rollout status deployment/antwort -n ${NAMESPACE} --timeout=120s
```

## Verification

Check the route is accessible:

```bash
ROUTE_URL=$(oc get route antwort -n ${NAMESPACE} -o jsonpath='{.spec.host}')
curl -sf "https://${ROUTE_URL}/v1/healthz" && echo "Antwort is healthy"
```

## Post-Install

Report the Antwort URL to the user:

```
Antwort deployed successfully.
  URL: https://${ROUTE_URL}/v1
  Backend: ${BACKEND_URL}
  Storage: in-memory (not persistent)

Set environment variables for cluster validation:
  export CLUSTER_ANTWORT_URL=https://${ROUTE_URL}/v1
  export CLUSTER_MODEL=${MODEL_NAME}
```
