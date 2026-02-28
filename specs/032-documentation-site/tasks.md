# Tasks: Documentation Site

**Input**: Design documents from `/specs/032-documentation-site/`
**Prerequisites**: plan.md, spec.md, research.md

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Infrastructure Setup

**Purpose**: Set up Antora build system with multi-module structure.

- [ ] T001 (antwort-a8l.1) Create `docs/antora-playbook.yml` with local content source pointing to `docs/`, enable `@antora/lunr-extension` for search (FR-001, FR-013)
- [ ] T002 (antwort-a8l.2) Update `docs/antora.yml` to register all six module nav files: ROOT, tutorial, reference, extensions, quickstarts, operations (FR-002)
- [ ] T003 (antwort-a8l.3) [P] Create module directories: `docs/modules/tutorial/pages/`, `docs/modules/reference/pages/`, `docs/modules/extensions/pages/`, `docs/modules/quickstarts/pages/`, `docs/modules/operations/pages/` (FR-002)
- [ ] T004 (antwort-a8l.4) [P] Create stub `nav.adoc` for each new module: `docs/modules/tutorial/nav.adoc`, `docs/modules/reference/nav.adoc`, `docs/modules/extensions/nav.adoc`, `docs/modules/quickstarts/nav.adoc`, `docs/modules/operations/nav.adoc` (FR-003)
- [ ] T005 (antwort-a8l.5) Update `docs/modules/ROOT/nav.adoc` to reference cross-module xrefs for the new modules (FR-003)
- [ ] T006 (antwort-a8l.6) Add `docs` and `docs-serve` targets to `Makefile`: `npx antora docs/antora-playbook.yml` for build, local HTTP server for preview (FR-012)
- [ ] T007 (antwort-a8l.7) Verify `make docs` produces a browsable HTML site with all six modules in navigation
- [ ] T008 (antwort-a8l.8) Remove old stub pages that are being replaced: `docs/modules/ROOT/pages/providers.adoc`, `docs/modules/ROOT/pages/tools.adoc`, `docs/modules/ROOT/pages/auth.adoc`, `docs/modules/ROOT/pages/storage.adoc`, `docs/modules/ROOT/pages/observability.adoc`, `docs/modules/ROOT/pages/deployment.adoc`, `docs/modules/ROOT/pages/configuration.adoc`, `docs/modules/ROOT/pages/getting-started.adoc`

**Checkpoint**: `make docs` builds successfully with empty module pages.

---

## Phase 2: ROOT Module (US1 partial, US6)

**Goal**: Write the foundational overview and architecture pages.

**Independent Test**: The ROOT module pages render with real content covering project overview, architecture diagram, and API reference.

- [ ] T009 (antwort-0mu.1) [US1] Rewrite `docs/modules/ROOT/pages/index.adoc` using kubernetes-patterns voice: project overview, what antwort is (OpenResponses API gateway), key capabilities (streaming, tools, multi-tenant, extensible), links to tutorial and quickstarts (FR-010, FR-011)
- [ ] T010 (antwort-0mu.2) [US1] Rewrite `docs/modules/ROOT/pages/architecture.adoc` using kubernetes-patterns voice: request flow diagram (ASCII: HTTP -> Transport -> Engine -> Provider -> LLM), streaming lifecycle (SSE events), agentic loop (multi-turn tool calling), component layer descriptions (FR-014, FR-010, FR-011)
- [ ] T011 (antwort-91e.2) [US1] Rewrite `docs/modules/ROOT/pages/api-reference.adoc` using kubernetes-patterns voice: Responses API wire format (POST /v1/responses), request schema, response schema, SSE event catalog (all event types from types.go), error codes and types, include parameter reference (FR-015, FR-010, FR-011)

**Checkpoint**: ROOT module has substantive content for all three pages.

---

## Phase 3: Tutorial Module (US1)

**Goal**: Progressive tutorial from zero to production deployment.

**Independent Test**: A developer can follow all four tutorial pages sequentially and end with a working production deployment.

- [ ] T012 (antwort-4lm.1) [US1] Write `docs/modules/tutorial/pages/getting-started.adoc` using kubernetes-patterns voice: prerequisites (Kubernetes cluster, kubectl, shared LLM backend), deploy 01-minimal quickstart, send first request with curl, explain response structure, verify health endpoint (FR-004, FR-010, FR-011)
- [ ] T013 (antwort-4lm.2) [US1] Write `docs/modules/tutorial/pages/first-tools.adoc` using kubernetes-patterns voice: what is the agentic loop, deploy 04-mcp-tools quickstart, send tool-using request, explain function_call/function_call_output lifecycle, show streaming tool events (FR-004, FR-010, FR-011)
- [ ] T014 (antwort-4lm.3) [US1] Write `docs/modules/tutorial/pages/code-execution.adoc` using kubernetes-patterns voice: what is the code interpreter, deploy 05-code-interpreter quickstart, send code execution request, explain sandbox architecture, package installation, show output formats (FR-004, FR-010, FR-011)
- [ ] T015 (antwort-4lm.4) [US1] Write `docs/modules/tutorial/pages/going-production.adoc` using kubernetes-patterns voice: add PostgreSQL (02-production), add JWT auth (03-multi-user), Prometheus metrics and Grafana, OpenShift deployment with Routes, xrefs to reference and operations modules (FR-004, FR-010, FR-011)
- [ ] T016 (antwort-4lm.5) [US1] Update `docs/modules/tutorial/nav.adoc` with final page titles and order

**Checkpoint**: Tutorial module complete with four progressive pages.

---

## Phase 4: Reference Module (US2)

**Goal**: Complete configuration documentation with annotated examples and reference tables.

**Independent Test**: Every config key from config.example.yaml can be found in the reference module with type, default, env var, and description.

- [ ] T017 (antwort-f4h.1) [US2] Write `docs/modules/reference/pages/configuration.adoc` using kubernetes-patterns voice: annotated YAML examples with callouts for all 8 config sections (server, engine, storage, auth, mcp, providers, observability, logging), explain each section's purpose, show common configurations (FR-005, FR-010, FR-011)
- [ ] T018 (antwort-f4h.2) [US2] Write `docs/modules/reference/pages/config-reference.adoc` using kubernetes-patterns voice: complete table of all 40+ config keys with columns: Key, Type, Default, Env Var, Description. Cover _file suffix convention, config discovery order (FR-006, FR-010, FR-011)
- [ ] T019 (antwort-f4h.3) [US2] Write `docs/modules/reference/pages/environment-variables.adoc` using kubernetes-patterns voice: table mapping all ANTWORT_* env vars to config keys, explain override precedence (env > file > default), document JSON-encoded env vars for complex types (FR-017, FR-010, FR-011)
- [ ] T020 (antwort-f4h.4) [US2] Write `docs/modules/reference/pages/api-endpoints.adoc` using kubernetes-patterns voice: all HTTP endpoints (POST /v1/responses, GET /v1/responses/:id, DELETE /v1/responses/:id, GET /v1/responses/:id/input_items, GET /v1/models, GET /healthz, GET /readyz, GET /metrics), request/response examples (FR-010, FR-011)
- [ ] T021 (antwort-f4h.5) [US2] Update `docs/modules/reference/nav.adoc` with final page titles and order

**Checkpoint**: Reference module complete with four pages covering all configuration.

---

## Phase 5: Extensions Module (US3)

**Goal**: Document all extension interfaces for custom adapter development.

**Independent Test**: A developer can understand each interface contract from the documentation without reading source code.

- [ ] T022 (antwort-8z6.1) [P] [US3] Write `docs/modules/extensions/pages/overview.adoc` using kubernetes-patterns voice: extension philosophy (interface-first from constitution), how extensions plug in, adapter vs core distinction, zero-dependency rule for core (FR-007, FR-010, FR-011)
- [ ] T023 (antwort-8z6.2) [P] [US3] Write `docs/modules/extensions/pages/providers.adoc` using kubernetes-patterns voice: Provider interface (6 methods with signatures), ProviderCapabilities struct, ProviderRequest/ProviderResponse types, translation patterns, registration in cmd/server/main.go, existing providers as reference (FR-007, FR-010, FR-011)
- [ ] T024 (antwort-8z6.3) [P] [US3] Write `docs/modules/extensions/pages/storage.adoc` using kubernetes-patterns voice: ResponseStore interface (8 methods with signatures), ListOptions and pagination, tenant context propagation via context.Context, HealthCheck contract, PostgreSQL adapter as reference, migration patterns (FR-007, FR-010, FR-011)
- [ ] T025 (antwort-8z6.4) [P] [US3] Write `docs/modules/extensions/pages/auth.adoc` using kubernetes-patterns voice: Authenticator interface (single method), three-outcome voting (AuthDecision: Yes/No/Abstain), AuthResult and Identity structs, AuthChain composition, middleware integration, existing authenticators (noop, apikey, jwt) as reference (FR-007, FR-010, FR-011)
- [ ] T026 (antwort-8z6.5) [P] [US3] Write `docs/modules/extensions/pages/tools.adoc` using kubernetes-patterns voice: FunctionProvider interface (7 methods with signatures), tool definition format, CanExecute/Execute contract, Route registration for management APIs, Prometheus Collector integration, MCP client as external tool pattern (FR-007, FR-010, FR-011)
- [ ] T027 (antwort-8z6.6) [US3] Update `docs/modules/extensions/nav.adoc` with final page titles and order

**Checkpoint**: Extensions module complete with five pages covering all interfaces.

---

## Phase 6: Quickstarts Module (US4)

**Goal**: Convert all 6 quickstarts from Markdown to AsciiDoc.

**Independent Test**: All six quickstarts render in the Antora site with identical content to the original READMEs.

- [ ] T028 (antwort-lhv.1) [US4] Write `docs/modules/quickstarts/pages/index.adoc`: quickstart overview with progression table (01-06), prerequisites, which quickstart to start with (FR-008, FR-010)
- [ ] T029 (antwort-lhv.2) [P] [US4] Convert `quickstarts/01-minimal/README.md` to `docs/modules/quickstarts/pages/qs-01-minimal.adoc`: full AsciiDoc conversion with admonitions, callouts, xrefs to reference pages (FR-008, FR-016, FR-010, FR-011)
- [ ] T030 (antwort-lhv.3) [P] [US4] Convert `quickstarts/02-production/README.md` to `docs/modules/quickstarts/pages/qs-02-production.adoc` (FR-008, FR-016, FR-010, FR-011)
- [ ] T031 (antwort-lhv.4) [P] [US4] Convert `quickstarts/03-multi-user/README.md` to `docs/modules/quickstarts/pages/qs-03-multi-user.adoc` (FR-008, FR-016, FR-010, FR-011)
- [ ] T032 (antwort-lhv.5) [P] [US4] Convert `quickstarts/04-mcp-tools/README.md` to `docs/modules/quickstarts/pages/qs-04-mcp-tools.adoc` (FR-008, FR-016, FR-010, FR-011)
- [ ] T033 (antwort-lhv.6) [P] [US4] Convert `quickstarts/05-code-interpreter/README.md` to `docs/modules/quickstarts/pages/qs-05-code-interpreter.adoc` (FR-008, FR-016, FR-010, FR-011)
- [ ] T034 (antwort-lhv.7) [P] [US4] Convert `quickstarts/06-responses-proxy/README.md` to `docs/modules/quickstarts/pages/qs-06-responses-proxy.adoc` (FR-008, FR-016, FR-010, FR-011)
- [ ] T035 (antwort-lhv.8) [US4] Update `docs/modules/quickstarts/nav.adoc` with final page titles and order

**Checkpoint**: Quickstarts module complete with seven pages (overview + 6 quickstarts).

---

## Phase 7: Operations Module (US5)

**Goal**: Write production operations guides.

**Independent Test**: An operator can follow the monitoring guide to set up Prometheus scraping and find all available metrics.

- [ ] T036 (antwort-an5.1) [P] [US5] Write `docs/modules/operations/pages/monitoring.adoc` using kubernetes-patterns voice: all 16 Prometheus metrics with names, types, labels, descriptions, histogram buckets; Grafana dashboard setup; ServiceMonitor configuration; alert rule examples; GenAI semantic conventions (FR-009, FR-018, FR-010, FR-011)
- [ ] T037 (antwort-an5.2) [P] [US5] Write `docs/modules/operations/pages/deployment.adoc` using kubernetes-patterns voice: Kustomize base + overlay pattern, OpenShift SCC requirements, Route configuration, image registry setup, resource limits, health probes, container security context (FR-009, FR-010, FR-011)
- [ ] T038 (antwort-an5.3) [P] [US5] Write `docs/modules/operations/pages/troubleshooting.adoc` using kubernetes-patterns voice: debug logging categories and how to enable them, common error messages and resolutions, health check endpoint interpretation, sandbox execution failures, provider connection errors (FR-009, FR-010, FR-011)
- [ ] T039 (antwort-an5.4) [P] [US5] Write `docs/modules/operations/pages/security.adoc` using kubernetes-patterns voice: API key authentication setup, JWT/OIDC configuration with Keycloak, TLS termination via Ingress/Route, secrets management (_file suffix, Kubernetes Secrets), RBAC for multi-tenant deployments (FR-009, FR-010, FR-011)
- [ ] T040 (antwort-an5.5) [US5] Update `docs/modules/operations/nav.adoc` with final page titles and order

**Checkpoint**: Operations module complete with four pages.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Final validation and cleanup.

- [ ] T041 (antwort-tj8.1) Verify all cross-references (xrefs) between modules resolve correctly by running `make docs` and checking for warnings
- [ ] T042 (antwort-tj8.2) Verify search functionality works: build site, search for "configuration", "streaming", "auth", "tools", "deploy" and confirm relevant results
- [ ] T043 (antwort-tj8.3) Remove any remaining old stub pages from `docs/modules/ROOT/pages/` that were migrated to other modules

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Infrastructure)**: No dependencies
- **Phase 2 (ROOT)**: Depends on Phase 1
- **Phase 3 (Tutorial)**: Depends on Phase 2 (needs index for context)
- **Phase 4 (Reference)**: Depends on Phase 1 (needs module structure)
- **Phase 5 (Extensions)**: Depends on Phase 1, independent of Phases 3-4
- **Phase 6 (Quickstarts)**: Depends on Phase 1, independent of Phases 3-5
- **Phase 7 (Operations)**: Depends on Phase 1, independent of Phases 3-6
- **Phase 8 (Polish)**: Depends on all content phases

### Parallel Opportunities

- Phases 4, 5, 6, 7 can all run in parallel after Phase 1 is complete
- Within Phase 5: T022-T026 are independent (different pages, different interfaces)
- Within Phase 6: T029-T034 are independent (different quickstart conversions)
- Within Phase 7: T036-T039 are independent (different operations topics)

---

## Implementation Strategy

### MVP First (US6 + US1)

1. Phase 1: Infrastructure setup (T001-T008)
2. Phase 2: ROOT module (T009-T011)
3. Phase 3: Tutorial module (T012-T016)
4. **STOP**: Developers can find the project, understand the architecture, and follow a tutorial

### Incremental Delivery

5. Phase 4: Reference module (T017-T021)
6. Phases 5+6+7 in parallel: Extensions, Quickstarts, Operations
7. Phase 8: Polish and validation
