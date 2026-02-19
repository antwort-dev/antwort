# Spec 07b: Kubernetes Deployment (Kustomize)

**Branch**: `spec/07b-kustomize`
**Dependencies**: Spec 07a (Container Image)
**Package**: N/A (Kubernetes manifests)

## Purpose

Define the Kubernetes deployment manifests using Kustomize. Base manifests for any cluster, with overlays for dev, production, and OpenShift.

## Scope

### In Scope
- Kustomize base: Deployment, Service, ConfigMap, ServiceAccount
- Dev overlay: reduced resources, debug logging
- Production overlay: HPA, PDB, tuned resources
- OpenShift overlay: Route, ServiceMonitor
- Health checks (liveness, readiness, startup probes)
- Configuration via ConfigMap + Secret references

### Out of Scope
- Helm chart (separate spec)
- Observability stack (separate spec)
- Multi-tenant deployment patterns (Helm concern)
- Operator CRDs (future)

## Directory Structure

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

## Core Deployment

The base Deployment runs the antwort server with:
- Health probes on /healthz and /readyz
- Configuration from ConfigMap + Secret
- Resource requests/limits suitable for a Go HTTP server
- Non-root security context

## Open Questions

- Should we include a PostgreSQL StatefulSet or assume external database?
  -> Assume external. In-memory store for dev, external PostgreSQL for production.

## Deliverables

- [ ] `deploy/kubernetes/base/` - Kustomize base
- [ ] `deploy/kubernetes/overlays/dev/` - Development overlay
- [ ] `deploy/kubernetes/overlays/production/` - Production overlay
- [ ] `deploy/kubernetes/overlays/openshift/` - OpenShift overlay
