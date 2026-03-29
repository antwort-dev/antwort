# Tasks: Quickstart Series

## Phase 1: Already Done

- [x] T001 Create `quickstarts/shared/llm-backend/` with vLLM + Qwen 2.5 7B manifests and README
- [x] T002 Create `quickstarts/01-minimal/` with base + OpenShift overlay and README
- [x] T003 Test 01-minimal on ROSA: health check, text completion, streaming, metrics

---

## Phase 2: 02-Production (P1)

**Goal**: PostgreSQL persistence + Prometheus + Grafana monitoring

- [x] T004 [US2] Create `quickstarts/02-production/postgres/`: StatefulSet, Service, Secret (credentials), PVC for PostgreSQL
- [x] T005 [US2] Create `quickstarts/02-production/monitoring/`: Prometheus Deployment + config (scrape antwort /metrics), Grafana Deployment + ConfigMap with pre-built antwort dashboard JSON
- [x] T006 [US2] Create `quickstarts/02-production/base/`: antwort ConfigMap (storage=postgres, DSN from Secret), Deployment, Service, kustomization.yaml referencing postgres/ and monitoring/
- [x] T007 [US2] Create `quickstarts/02-production/openshift/`: Route overlay
- [x] T008 [US2] Write `quickstarts/02-production/README.md`: deploy instructions, test persistence (create + pod restart + retrieve), access Grafana dashboard, cleanup
- [x] T009 [US2] Validate `kustomize build` for base and openshift overlays

**Checkpoint**: Production quickstart ready. Test on ROSA.

---

## Phase 3: 04-MCP Tools (P2)

**Goal**: MCP server + agentic tool calling demo

- [x] T010 [US4] Create `quickstarts/04-mcp-tools/mcp-server/`: Deployment + Service for antwort MCP test server (quay.io/rhuss/antwort:mcp-test)
- [x] T011 [US4] Create `quickstarts/04-mcp-tools/base/`: antwort ConfigMap with MCP server config, Deployment, Service, kustomization.yaml
- [x] T012 [US4] Create `quickstarts/04-mcp-tools/openshift/`: Route overlay
- [x] T013 [US4] Write `quickstarts/04-mcp-tools/README.md`: deploy instructions, test agentic loop (ask time, echo), verify tool discovery, cleanup
- [x] T014 [US4] Validate `kustomize build` for base and openshift overlays

**Checkpoint**: MCP tools quickstart ready. Test on ROSA.

---

## Phase 4: 03-Multi-User (P2, blocked on JWT)

**Goal**: Keycloak + JWT auth + tenant isolation

- [x] T015 [US3] Create `quickstarts/03-multi-user/keycloak/`: Keycloak Deployment, Service, PostgreSQL, realm import ConfigMap (two users: alice, bob in separate tenants)
- [x] T016 [US3] Create `quickstarts/03-multi-user/base/`: antwort ConfigMap (auth=jwt, JWKS URL from Keycloak), Deployment, Service
- [x] T017 [US3] Write `quickstarts/03-multi-user/README.md`: deploy instructions, obtain JWT from Keycloak, test tenant isolation
- [x] T018 [US3] Validate kustomize builds

**Unblocked**: JWT authenticator implemented (Spec 007 P2, commit 48c7977)

---

## Phase 5: 05-MCP Secured (P3, blocked on token exchange)

- [ ] T019 [US5] Create `quickstarts/05-mcp-secured/`: config-only overlay adding OAuth client_credentials for MCP server via Keycloak
- [ ] T020 [US5] Write README with token exchange demo

**Blocked**: Requires OAuth token exchange (Spec 10b)

---

## Phase 6: 06-RAG (P3, blocked on RAG MCP server)

- [ ] T021 [US6] Create `quickstarts/06-rag/minio/`: MinIO Deployment + Service
- [ ] T022 [US6] Create `quickstarts/06-rag/qdrant/`: Qdrant Deployment + Service
- [ ] T023 [US6] Create `quickstarts/06-rag/rag-mcp-server/`: RAG MCP server Deployment + Service
- [ ] T024 [US6] Write README with document upload + retrieval demo

**Blocked**: Requires RAG MCP server (separate project)

---

## Dependencies

- **Phase 1**: Done.
- **Phase 2 (02-production)**: Ready to implement now.
- **Phase 3 (04-mcp-tools)**: Ready to implement now.
- **Phase 4 (03-multi-user)**: Blocked on JWT auth.
- **Phase 5 (05-mcp-secured)**: Blocked on token exchange + Phase 4.
- **Phase 6 (06-rag)**: Blocked on RAG MCP server.

## Implementation Strategy

### Now (P1-P2)
1. 02-production (PostgreSQL + monitoring)
2. 04-mcp-tools (agentic loop demo)

### After JWT auth
3. 03-multi-user (Keycloak + tenancy)

### After token exchange
4. 05-mcp-secured (OAuth for MCP)

### After RAG MCP server
5. 06-rag (document retrieval)
