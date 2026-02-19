# Feature Specification: Kubernetes Deployment (Kustomize)

**Feature Branch**: `010-kustomize-deploy`
**Created**: 2026-02-19
**Status**: Draft

## Overview

This specification defines the Kubernetes deployment manifests for antwort using Kustomize. It provides a base set of manifests (Deployment, Service, ConfigMap, ServiceAccount) with overlays for dev, production, and OpenShift environments. The manifests enable deploying antwort to any Kubernetes cluster with a single `kustomize build | kubectl apply` command.

## User Scenarios & Testing

### User Story 1 - Deploy to Kubernetes (Priority: P1)

An operator deploys antwort to a Kubernetes cluster by applying the Kustomize base manifests. The deployment connects to a backend inference server (vLLM or LiteLLM) and accepts OpenResponses API requests via a Service.

**Why this priority**: This is the fundamental deployment path. Without it, antwort cannot run on Kubernetes.

**Acceptance Scenarios**:

1. **Given** the base manifests, **When** applied to a cluster, **Then** antwort pods start and pass health checks
2. **Given** a ConfigMap with `ANTWORT_BACKEND_URL`, **When** the pod starts, **Then** it connects to the configured backend
3. **Given** the Deployment, **When** the pod is running, **Then** liveness, readiness, and startup probes are configured and healthy

---

### User Story 2 - Deploy to OpenShift/ROSA (Priority: P1)

An operator deploys antwort to an OpenShift or ROSA cluster using the OpenShift overlay. This adds a Route for external HTTPS access and a ServiceMonitor for Prometheus metrics scraping.

**Why this priority**: OpenShift/ROSA is the primary deployment target for the test drive.

**Acceptance Scenarios**:

1. **Given** the OpenShift overlay, **When** applied, **Then** a Route is created with edge TLS termination
2. **Given** the Route, **When** a client sends a request to the Route URL, **Then** it reaches the antwort Service
3. **Given** the ServiceMonitor, **When** Prometheus scrapes metrics, **Then** it collects from the antwort metrics port

---

### User Story 3 - Production Hardening (Priority: P2)

An operator deploys antwort to production using the production overlay. This adds horizontal pod autoscaling, pod disruption budgets, and tuned resource limits.

**Acceptance Scenarios**:

1. **Given** the production overlay, **When** applied, **Then** HPA scales pods based on CPU utilization
2. **Given** the PDB, **When** a node is drained, **Then** at least one pod remains available

---

### Edge Cases

- What happens when `ANTWORT_BACKEND_URL` is not set? The pod starts but fails readiness checks (no backend). The Deployment shows 0/N ready pods.
- What happens when the backend is unreachable? The readiness probe fails, traffic is not routed to the pod, but the pod stays alive (liveness still passes).

## Requirements

### Functional Requirements

**Base Manifests**

- **FR-001**: The project MUST provide Kustomize base manifests: Deployment, Service, ConfigMap, and ServiceAccount
- **FR-002**: The Deployment MUST configure liveness probe (GET /healthz), readiness probe (GET /healthz), and startup probe with appropriate thresholds
- **FR-003**: The ConfigMap MUST contain all configuration environment variables with sensible defaults
- **FR-004**: The Service MUST expose the HTTP port (8080) for API access
- **FR-005**: The Deployment MUST run as non-root with a restricted security context
- **FR-006**: The Deployment MUST set resource requests and limits appropriate for a Go HTTP server

**Overlays**

- **FR-007**: The project MUST provide a dev overlay with reduced resource limits and debug-level logging
- **FR-008**: The project MUST provide a production overlay with HPA, PDB, and production-tuned resources
- **FR-009**: The project MUST provide an OpenShift overlay with Route (edge TLS) and ServiceMonitor

**Deployment Experience**

- **FR-010**: Deploying MUST work with a single command: `kustomize build deploy/kubernetes/overlays/<env> | kubectl apply -f -`
- **FR-011**: The ConfigMap MUST support customization of the backend URL, provider type, model name, storage type, and auth configuration without editing manifests

## Success Criteria

- **SC-001**: The base manifests deploy successfully to a vanilla Kubernetes cluster and pods pass health checks
- **SC-002**: The OpenShift overlay creates a working Route that accepts external HTTPS requests
- **SC-003**: A developer can deploy to a new cluster in under 5 minutes using only the manifests and a backend URL

## Assumptions

- The container image is already built and available in a registry (Spec 009).
- The backend inference server (vLLM/LiteLLM) is deployed separately. Antwort connects to it via the backend URL.
- PostgreSQL (if used) is deployed separately. The default storage type is "memory" for simplicity.
- No Ingress controller is needed for OpenShift (Route handles external access).

## Dependencies

- **Spec 009 (Container Image)**: The image that the Deployment references.
- **Spec 006 (Conformance)**: The server binary with health endpoints.

## Scope Boundaries

### In Scope

- Kustomize base manifests (Deployment, Service, ConfigMap, ServiceAccount)
- Dev overlay (reduced resources, debug logging)
- Production overlay (HPA, PDB)
- OpenShift overlay (Route, ServiceMonitor)
- Makefile target for deploying (`make deploy`)

### Out of Scope

- Helm chart (Spec 07c)
- PostgreSQL deployment (external)
- CI/CD pipeline
- Network policies
- Observability stack (Spec 07d)
