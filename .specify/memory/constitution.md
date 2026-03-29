<!--
Sync Impact Report
  Version change: 1.0.0 → 1.1.0 (MINOR: new principle added)
  Modified principles:
    All principles renumbered (II-IX) to make room for new Principle I
  Added principles:
    I. Enterprise Production First (NEW)
  Templates requiring updates:
    ✅ constitution.md (this file)
    ⚠ plan-template.md - no changes needed (generic Constitution Check)
    ⚠ spec-template.md - no changes needed (generic)
    ⚠ tasks-template.md - no changes needed (generic)
  Follow-up TODOs: none
-->

# Antwort Constitution

## Core Principles

### I. Enterprise Production First

Enterprise non-functional requirements are first-class design concerns,
not afterthoughts bolted on after the happy path works. Every feature
spec MUST address applicable production concerns from the start:

- **Multi-user isolation**: Per-user resource ownership and access control
- **Multi-tenancy**: Per-group isolation within a shared instance
- **Scalability**: Horizontal scaling via Kubernetes HPA, stateless
  gateway mode, connection pooling
- **High availability**: Graceful shutdown, in-flight request draining,
  health checks, no single points of failure in the request path
- **Resilience**: Error propagation without cascading failures, bounded
  retries, timeout enforcement, stale detection for background workers
- **Security**: Authentication, authorization, audit logging, input
  validation at boundaries (see Principle IX for details)
- **Observability**: Metrics, structured logging, debug categories
  (see Principle VIII for details)

A spec that delivers functionality without addressing these concerns is
incomplete. The Constitution Check in every plan MUST verify that
applicable enterprise concerns are designed in, not deferred.

Rationale: Retrofitting multi-tenancy, security, or scalability into
existing code is orders of magnitude harder than designing for it.
Antwort serves production agentic workloads where these properties are
table stakes, not nice-to-haves.

### II. Go Standard Library Core

Core packages (`pkg/`) MUST depend only on the Go standard library.
External dependencies are permitted only in adapter packages that bridge
to specific technologies (pgx for PostgreSQL, prometheus for metrics,
jwt for authentication, MCP SDK for tool protocol).

Rationale: Maximum portability, minimal attack surface, and predictable
behavior. Adapter isolation means swapping implementations never touches
core logic.

Boundary rule: If a new package under `pkg/` needs an external import,
it MUST be an adapter package (named after the technology it adapts) or
the dependency MUST be justified in the spec's Constitution Check.

### III. Interface-First Architecture

Every major subsystem MUST be defined as a Go interface with at least
one implementation. New features extend existing interfaces or introduce
new ones rather than modifying concrete types.

Key interfaces:
- `provider.Provider` (LLM backends)
- `transport.ResponseStore` (persistence)
- `transport.ResponseCreator` (request processing)
- `tools.ToolExecutor` (tool execution)
- `vectorstore.Backend` (vector databases)
- `registry.FunctionProvider` (pluggable tool providers)

Rationale: Interface boundaries enable independent testing, pluggable
implementations, and clear package responsibilities.

### IV. Kubernetes-Native Deployment

Antwort targets Kubernetes exclusively. All deployment patterns MUST
work with Kustomize overlays, support HPA scaling, and integrate with
Prometheus scraping. Single-binary architecture (one image, multiple
modes via `--mode` flag).

Deployment modes:
- `integrated` (gateway + worker in one process)
- `gateway` (stateless request handling)
- `worker` (background response processing)

Rationale: Kubernetes provides the scaling, scheduling, and service
discovery that Antwort deliberately does not implement.

### V. Specification-Driven Development

Every feature MUST start as a formal specification before code is
written. The SDD workflow is: brainstorm, specify, plan, tasks, review,
implement, verify. Spec artifacts live in `specs/NNN-feature-name/`.

Required artifacts per spec: `spec.md`, `plan.md`, `tasks.md`.
Generated artifacts: `research.md`, `data-model.md`, `REVIEWERS.md`.

Rationale: Specifications capture decisions, enable review before
implementation, and provide traceability from requirements to code.

### VI. Comprehensive Testing (NON-NEGOTIABLE)

Every feature MUST include tests at all levels:

- **Unit tests**: Per-package in `pkg/*/` using Go standard `testing`.
  Test files live alongside the code they test.
- **Integration tests**: In `test/integration/` exercising the compiled
  server binary against the deterministic mock backend.
- **E2E tests**: In `test/e2e/` using the instrumented server with
  recorded LLM responses for deterministic replay. Build tag `e2e`.
- **CI validation**: All tests MUST pass in the GitHub Actions pipeline
  before merge. No skipping, no `t.Skip()` without a tracking issue.

Rationale: The agentic loop, tool execution, and streaming make manual
testing insufficient. Automated tests at every level catch regressions
that unit tests alone miss.

### VII. Documentation Completeness (NON-NEGOTIABLE)

Every feature PR MUST include documentation updates:

- **Antora docs** (`docs/`): Reference pages, tutorials, or quickstart
  guides as appropriate for the feature scope. Navigation (`nav.adoc`)
  MUST be updated when adding new pages.
- **README.md**: The implemented specs list MUST be updated to reflect
  the new feature.
- **Landing page**: For larger features (new subsystems, user-facing
  capabilities), the landing page at `../antwort.github.io` MUST be
  updated with feature cards or architecture diagrams.
- **Configuration reference**: New configuration options MUST be
  documented in `docs/modules/reference/pages/config-reference.adoc`
  and `docs/modules/reference/pages/environment-variables.adoc`.

A feature is not complete until its documentation is merged.

Rationale: Undocumented features do not exist for users. Documentation
drift erodes trust and increases support burden.

### VIII. Observability by Default

Every subsystem MUST expose Prometheus metrics. Metrics are always
registered at startup (no feature flags). The `pkg/observability/`
package is the single source of truth for metric definitions.

Requirements:
- Metric names follow Prometheus conventions: `antwort_` prefix,
  `_total` suffix for counters, unit in name (`_seconds`, `_bytes`).
- Duration histograms use `LLMBuckets` unless domain-specific buckets
  are justified.
- Structured logging via `log/slog` with category-based debug logging
  via `pkg/debug/`.
- Audit logging for security events via `pkg/audit/`.

Rationale: Production visibility is not optional. Operators need metrics
and logs to diagnose issues without access to source code.

### IX. Security at Every Layer

Authentication and authorization MUST be enforced at system boundaries.
The three-level isolation model applies to all resources:

- **Owner** (`Identity.Subject`): Per-user isolation
- **Group** (`tenant_id`): Per-tenant isolation within an instance
- **Instance**: Separate Deployments for hard isolation

Requirements:
- All external input validated at the boundary (transport layer).
- No secrets in source, logs, metrics, or error messages.
- Parameterized queries for all database operations.
- Scope-based authorization with role-to-scope mapping.

Rationale: An API gateway handling LLM inference is a high-value target.
Defense in depth prevents single-point-of-failure security.

## Architecture Constraints

| Constraint | Rule | Rationale |
|-----------|------|-----------|
| Single binary | One `cmd/server` entry point, mode via flag | Simplifies deployment, container images |
| No ORM | Raw SQL with pgx, no query builders | Predictable queries, no hidden N+1 |
| No web framework | `net/http` with `http.ServeMux` (Go 1.22+ patterns) | Standard library sufficiency |
| No global state | Dependencies injected via constructors | Testability, no init() side effects beyond metric registration |
| Additive changes | New metrics, endpoints, and features MUST NOT break existing behavior | Backwards compatibility for SDK users |

## Development Workflow

### Feature Development

1. Brainstorm and specify (`/spex:brainstorm`, `/speckit.specify`)
2. Plan and generate tasks (`/speckit.plan`, `/speckit.tasks`)
3. Review spec and plan (`/spex:review-spec`, `/spex:review-plan`)
4. Implement (`/speckit.implement`)
5. Deep review and verify (`/spex:deep-review`, `/spex:verify`)
6. Documentation update (Antora docs, README.md, landing page if applicable)
7. Commit and PR

### Commit Conventions

- Attribution: `Assisted-By: 🤖 Claude Code`
- Feature commits: descriptive message with spec reference
- Spec artifact commits: separate from implementation commits

### Branch Naming

Feature branches: `NNN-feature-name` (matching spec directory)

## Governance

- This constitution supersedes all ad-hoc practices.
- Amendments require documentation in a spec or constitution update commit.
- Every PR review MUST verify compliance with applicable principles.
- Complexity beyond these principles MUST be justified in the spec's
  Constitution Check section.
- The implemented specs list in README.md is the canonical record of
  what Antwort supports.

**Version**: 1.1.0 | **Ratified**: 2026-03-29 | **Last Amended**: 2026-03-29
