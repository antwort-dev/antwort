# Implementation Plan: Documentation Site

**Branch**: `032-documentation-site` | **Date**: 2026-02-28 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/032-documentation-site/spec.md`

## Summary

Build a comprehensive Antora documentation site with six modules (ROOT, tutorial, reference, extensions, quickstarts, operations), all content written in the kubernetes-patterns voice via the prose plugin. Includes playbook setup, `make docs` target, lunr search, and full content for ~25 AsciiDoc pages.

## Technical Context

**Language/Version**: AsciiDoc (content), YAML (Antora config), JavaScript (Antora build via npx)
**Primary Dependencies**: Antora 3.x, @antora/lunr-extension, npx (Node.js 18+)
**Storage**: N/A (static site generator)
**Testing**: Visual verification, `make docs` build success, link validation
**Target Platform**: Browser (HTML output), developer workstations (local preview)
**Project Type**: Documentation site
**Performance Goals**: Build in under 60 seconds
**Constraints**: Semantic line breaks (one sentence per line), kubernetes-patterns voice
**Scale/Scope**: ~25 AsciiDoc pages across 6 modules

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Interface-First Design | N/A | No Go code |
| II. Zero External Dependencies | PASS | npx is a build-time dependency, not runtime |
| III. Nil-Safe Composition | N/A | No Go code |
| IV. Typed Error Domain | N/A | No Go code |
| V. Validate Early, Fail Fast | N/A | No Go code |

No violations. Documentation-only feature.

## Project Structure

### Documentation (this feature)

```text
specs/032-documentation-site/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Design decisions
└── tasks.md             # Task breakdown
```

### Source Code (repository root)

```text
docs/
├── antora.yml                        # Component descriptor (UPDATE)
├── antora-playbook.yml               # Build playbook (NEW)
├── modules/
│   ├── ROOT/
│   │   ├── nav.adoc                  # Root navigation (UPDATE)
│   │   └── pages/
│   │       ├── index.adoc            # Overview (UPDATE)
│   │       ├── architecture.adoc     # Architecture deep-dive (REWRITE)
│   │       └── api-reference.adoc    # API wire format (REWRITE)
│   ├── tutorial/
│   │   ├── nav.adoc                  # Tutorial navigation (NEW)
│   │   └── pages/
│   │       ├── getting-started.adoc  # First deployment (NEW)
│   │       ├── first-tools.adoc      # Adding tools (NEW)
│   │       ├── code-execution.adoc   # Code interpreter (NEW)
│   │       └── going-production.adoc # Production setup (NEW)
│   ├── reference/
│   │   ├── nav.adoc                  # Reference navigation (NEW)
│   │   └── pages/
│   │       ├── configuration.adoc    # Annotated examples (NEW)
│   │       ├── config-reference.adoc # Complete reference table (NEW)
│   │       ├── environment-variables.adoc # Env var mapping (NEW)
│   │       └── api-endpoints.adoc    # HTTP endpoints (NEW)
│   ├── extensions/
│   │   ├── nav.adoc                  # Extensions navigation (NEW)
│   │   └── pages/
│   │       ├── overview.adoc         # Extension architecture (NEW)
│   │       ├── providers.adoc        # Provider interface (NEW)
│   │       ├── storage.adoc          # ResponseStore interface (NEW)
│   │       ├── auth.adoc             # Authenticator interface (NEW)
│   │       └── tools.adoc            # FunctionProvider interface (NEW)
│   ├── quickstarts/
│   │   ├── nav.adoc                  # Quickstarts navigation (NEW)
│   │   └── pages/
│   │       ├── index.adoc            # Quickstart overview (NEW)
│   │       ├── qs-01-minimal.adoc    # Converted from 01-minimal (NEW)
│   │       ├── qs-02-production.adoc # Converted from 02-production (NEW)
│   │       ├── qs-03-multi-user.adoc # Converted from 03-multi-user (NEW)
│   │       ├── qs-04-mcp-tools.adoc  # Converted from 04-mcp-tools (NEW)
│   │       ├── qs-05-code-interpreter.adoc # Converted from 05 (NEW)
│   │       └── qs-06-responses-proxy.adoc  # Converted from 06 (NEW)
│   └── operations/
│       ├── nav.adoc                  # Operations navigation (NEW)
│       └── pages/
│           ├── monitoring.adoc       # Prometheus + Grafana (NEW)
│           ├── deployment.adoc       # K8s + OpenShift (NEW)
│           ├── troubleshooting.adoc  # Debug logging + FAQ (NEW)
│           └── security.adoc         # Auth + secrets + RBAC (NEW)
Makefile                              # Add docs target (UPDATE)
```

**Structure Decision**: Multi-module Antora layout. Each module has its own nav.adoc and pages/ directory. The ROOT module gets the landing page and architecture overview. The existing single-module structure is replaced entirely.

## Implementation Phases

### Phase 1: Infrastructure Setup (US6)

Set up the Antora build system:
- Create `docs/antora-playbook.yml` with component source, lunr extension
- Update `docs/antora.yml` to register all six modules
- Create module directories with nav.adoc stubs
- Add `make docs` and `make docs-serve` targets to Makefile
- Verify build produces output

### Phase 2: ROOT Module (US1 partial)

Write the foundational pages:
- `index.adoc`: Project overview, what is antwort, key capabilities
- `architecture.adoc`: Request flow (transport -> engine -> provider), streaming lifecycle, agentic loop diagram
- `api-reference.adoc`: Responses API wire format, SSE event catalog, error codes

### Phase 3: Tutorial Module (US1)

Write the progressive tutorial:
- `getting-started.adoc`: Prerequisites, deploy shared LLM backend + 01-minimal, first request
- `first-tools.adoc`: MCP tools setup, agentic loop demo, tool lifecycle
- `code-execution.adoc`: Sandbox server setup, code interpreter, Python execution
- `going-production.adoc`: PostgreSQL, JWT auth, monitoring, OpenShift deployment

### Phase 4: Reference Module (US2)

Write the configuration documentation:
- `configuration.adoc`: Annotated YAML with callouts for each section (server, engine, storage, auth, mcp, providers, observability, logging)
- `config-reference.adoc`: Complete table of all 40+ config keys
- `environment-variables.adoc`: ANTWORT_* env var mapping table
- `api-endpoints.adoc`: HTTP endpoints (/v1/responses, /healthz, /metrics, /v1/models)

### Phase 5: Extensions Module (US3)

Write the extension guides:
- `overview.adoc`: Interface-first philosophy, extension architecture
- `providers.adoc`: Provider interface (6 methods), ProviderCapabilities, translation patterns, registration
- `storage.adoc`: ResponseStore interface (8 methods), tenant context, PostgreSQL adapter reference
- `auth.adoc`: Authenticator interface, three-outcome voting, AuthChain, Identity, middleware
- `tools.adoc`: FunctionProvider interface (7 methods), Route registration, MCP integration

### Phase 6: Quickstarts Module (US4)

Convert all 6 quickstarts from Markdown to AsciiDoc:
- `index.adoc`: Quickstart overview with progression table
- `qs-01-minimal.adoc` through `qs-06-responses-proxy.adoc`: Full conversions with xrefs to reference pages

### Phase 7: Operations Module (US5)

Write the operations guides:
- `monitoring.adoc`: All 16 Prometheus metrics with labels, Grafana dashboard setup, alert rule examples
- `deployment.adoc`: Kustomize patterns, OpenShift overlay, SCC requirements, image registry
- `troubleshooting.adoc`: Debug logging categories, common errors, health check interpretation
- `security.adoc`: Auth setup (API key + JWT), TLS termination, secrets management, RBAC

### Phase 8: Polish

- Verify all cross-references (xrefs) resolve
- Run `make docs` and verify clean build
- Visual review of all pages in browser
- Remove old stub pages that were replaced

## Implementation Order

Phases 1-4 are sequential (infrastructure before content, ROOT before tutorials, tutorials before reference).
Phases 5, 6, 7 are independent of each other and can run in parallel after Phase 4.
Phase 8 depends on all content phases.
