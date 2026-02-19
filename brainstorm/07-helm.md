# Spec 07c: Helm Chart

**Branch**: `spec/07c-helm`
**Dependencies**: Spec 07b (Kustomize base)
**Package**: N/A (Helm chart)

## Purpose

Provide a Helm chart as the primary user-facing deployment interface. Parameterized values for provider, storage, auth, observability, and scaling.

## Scope

### In Scope
- Helm chart with values.yaml
- Provider selection (vLLM, LiteLLM)
- Storage configuration (in-memory, PostgreSQL)
- Auth configuration (none, API key)
- Ingress/Route configuration
- Autoscaling configuration
- Multi-tenant deployment via separate releases

### Out of Scope
- Observability dashboards (separate spec)
- Operator CRDs (future)

## Deliverables

- [ ] `deploy/helm/antwort/` - Complete Helm chart
