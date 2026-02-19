# Tasks: Kubernetes Deployment (Kustomize)

## Phase 1: Base Manifests (P1)

- [ ] T001 [US1] Create `deploy/kubernetes/base/kustomization.yaml` referencing all base resources.
- [ ] T002 [US1] [P] Create `deploy/kubernetes/base/deployment.yaml`: single-replica Deployment with health probes, non-root security context, resource limits, envFrom ConfigMap.
- [ ] T003 [US1] [P] Create `deploy/kubernetes/base/service.yaml`: ClusterIP Service exposing port 8080.
- [ ] T004 [US1] [P] Create `deploy/kubernetes/base/configmap.yaml`: ANTWORT_BACKEND_URL, ANTWORT_MODEL, ANTWORT_PROVIDER, ANTWORT_STORAGE, ANTWORT_PORT.
- [ ] T005 [US1] [P] Create `deploy/kubernetes/base/serviceaccount.yaml`.
- [ ] T006 [US1] Verify `kustomize build deploy/kubernetes/base/` produces valid YAML.

**Checkpoint**: Base manifests ready.

---

## Phase 2: OpenShift Overlay (P1)

- [ ] T007 [US2] Create `deploy/kubernetes/overlays/openshift/kustomization.yaml` extending base.
- [ ] T008 [US2] [P] Create `deploy/kubernetes/overlays/openshift/route.yaml`: Route with edge TLS.
- [ ] T009 [US2] [P] Create `deploy/kubernetes/overlays/openshift/servicemonitor.yaml`.
- [ ] T010 [US2] Verify `kustomize build deploy/kubernetes/overlays/openshift/` produces valid YAML.

**Checkpoint**: OpenShift overlay ready for ROSA test drive.

---

## Phase 3: Dev + Production Overlays (P2)

- [ ] T011 [US3] [P] Create `deploy/kubernetes/overlays/dev/` with reduced resources and debug logging.
- [ ] T012 [US3] [P] Create `deploy/kubernetes/overlays/production/` with HPA and PDB.

---

## Phase 4: Makefile + Polish

- [ ] T013 Add `deploy` and `deploy-openshift` Makefile targets.
- [ ] T014 Verify all overlays build cleanly with `kustomize build`.
