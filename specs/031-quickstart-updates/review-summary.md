# Review Summary: Quickstart Updates

**Spec:** specs/031-quickstart-updates/spec.md | **Plan:** specs/031-quickstart-updates/plan.md
**Generated:** 2026-02-28

---

## Executive Summary

The antwort project has a progressive series of Kubernetes quickstarts (01 through 04) that walk developers from a minimal deployment up through production monitoring, JWT authentication, and MCP tool calling. Two major features shipped recently but have no quickstart coverage: sandbox code execution (Specs 024/025) and the Responses API provider (Spec 030). Additionally, newer API capabilities like structured output and reasoning are not demonstrated in any existing quickstart.

This feature adds two new quickstarts to the series. Quickstart 05 (code-interpreter) deploys a Python sandbox server alongside antwort so the LLM can write and execute code during response generation. It demonstrates data analysis, package installation, and computation. Quickstart 06 (responses-proxy) deploys two antwort instances in a proxy chain where a frontend instance forwards requests to a backend instance, demonstrating the middleware/gateway deployment pattern.

Both quickstarts follow the established Kustomize-based structure with base manifests and OpenShift overlays. Each includes a README with step-by-step deploy instructions and copy-pasteable curl test commands.

The feature also refreshes all four existing quickstart READMEs (01-04) by adding structured output and reasoning test sections, plus "Next Steps" links that create a continuous progression through the entire series.

All work is infrastructure and documentation. No Go code changes are needed.

## PR Contents

| Artifact | Description |
|----------|-------------|
| `spec.md` | 3 user stories, 14 functional requirements for new quickstarts and README refresh |
| `plan.md` | 3-phase implementation with exact file layouts for both new quickstarts |
| `tasks.md` | 32 tasks across 5 phases, synced to beads |
| `research.md` | Design decisions: image naming, provider config keys, resource naming |
| `quickstart.md` | Verification commands for all new features |
| `review-summary.md` | This file |

## Technical Decisions

### Sandbox Deployment Mode: Static URL (not SandboxClaim)
- **Chosen approach:** Deploy sandbox-server as a standalone pod with `sandbox_url` pointing to it
- **Alternatives considered:**
  - SandboxClaim via agent-sandbox operator: Rejected for quickstart because it requires CRD installation and operator setup, adding significant complexity
- **Trade-off:** Simpler quickstart experience at the cost of not demonstrating production pod pooling. An "Advanced: SandboxClaim" documentation section covers the production approach.

### Responses Provider Selection: `vllm-responses`
- **Chosen approach:** Use `engine.provider: vllm-responses` for the frontend
- **Alternatives considered:**
  - Generic HTTP proxy: Not available in antwort
  - Separate provider name: Would require code changes
- **Trade-off:** None. This is the only supported provider name for Responses API proxying.

### Resource Naming: `antwort-backend` / `antwort-frontend`
- **Chosen approach:** Prefix resources with their role in the proxy chain
- **Alternatives considered:**
  - `antwort-upstream`/`antwort-gateway`: Less universally understood
  - `antwort-1`/`antwort-2`: Not self-documenting
- **Trade-off:** Slightly longer names but immediately clear architecture.

## Critical References

| Reference | Why it needs attention |
|-----------|----------------------|
| `spec.md` FR-002: code_interpreter config | Defines how sandbox_url is wired, must match actual config.go parsing |
| `plan.md` Phase 2: antwort-configmap.yaml | The providers.code_interpreter config structure must match codeinterpreter.New() settings parsing |
| `spec.md` FR-007: streaming through proxy | Streaming through two antwort instances is the most complex test scenario |
| `plan.md` Phase 3: frontend-configmap.yaml | `engine.provider: vllm-responses` must exactly match the createProvider switch case |

## Reviewer Checklist

### Verify
- [ ] The `providers.code_interpreter.settings` keys match `pkg/tools/builtins/codeinterpreter/provider.go` lines 67-81
- [ ] The `engine.provider: vllm-responses` value matches `cmd/server/main.go` line 248
- [ ] Container image references are consistent across all quickstarts
- [ ] Each README follows the same structure as existing quickstarts (01-04)

### Question
- [ ] Should the sandbox-server have resource limits different from 256Mi/512Mi?
- [ ] Should the 06-responses-proxy quickstart also demonstrate adding auth on the frontend while backend is unauthenticated?

### Watch out for
- [ ] Sandbox container image (`quay.io/rhuss/antwort-sandbox:latest`) must actually be built and pushed before quickstart works
- [ ] Structured output examples depend on model capability. Qwen 2.5 7B supports json_schema but smaller models may not.

## Scope Boundaries
- **In scope:** Two new quickstarts (05, 06), four README updates (01-04), OpenShift overlays
- **Out of scope:** SandboxClaim deployment, MCP OAuth quickstart, RAG quickstart, container image building
- **Why these boundaries:** SandboxClaim requires operator infrastructure, OAuth and RAG are blocked on external dependencies

## Naming & Schema Decisions

| Item | Name | Context |
|------|------|---------|
| Sandbox image | `quay.io/rhuss/antwort-sandbox:latest` | Built from Containerfile.sandbox |
| Sandbox service | `sandbox-server` | Matches code_interpreter config expectation |
| Backend antwort | `antwort-backend` | Role-based naming in proxy chain |
| Frontend antwort | `antwort-frontend` | Role-based naming in proxy chain |
| Provider type | `vllm-responses` | Existing provider name in server main.go |

## Risk Areas

| Risk | Impact | Mitigation |
|------|--------|------------|
| Sandbox image not in registry | High | Document image build in README, use kustomize image overrides |
| Model can't generate valid Python | Med | Choose well-tested prompts, note model requirements in prerequisites |
| Streaming through proxy chain fails | Med | Test with both streaming and non-streaming, document both in README |
| Structured output not supported by model | Low | Note in README that json_schema requires model support |

---
*Share this with reviewers. Full context in linked spec and plan.*
