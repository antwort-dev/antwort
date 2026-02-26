# Implementation Plan: Code Interpreter Tool

**Branch**: `025-code-interpreter` | **Date**: 2026-02-26 | **Spec**: [spec.md](spec.md)

## Summary

Add a `code_interpreter` FunctionProvider that executes Python code in sandbox pods. Two modes: SandboxClaim mode (creates/deletes CRDs via agent-sandbox for pod-level isolation) and static URL mode (direct HTTP to a sandbox server, for development). The core provider, HTTP client, API types, and SSE event support are already implemented. This plan covers the remaining work: the SandboxClaim Kubernetes adapter, integration tests with the real sandbox-server binary, and polish.

## Technical Context

**Language/Version**: Go 1.22+
**Primary Dependencies**: Go standard library for core. `sigs.k8s.io/controller-runtime` + `sigs.k8s.io/agent-sandbox` for SandboxClaim adapter (adapter package only).
**Storage**: N/A (no persistence in this feature)
**Testing**: Go `testing` package. controller-runtime fake client for K8s boundary. Real sandbox-server binary as subprocess for integration tests.
**Target Platform**: Kubernetes with agent-sandbox controller (v0.1.1+)
**Project Type**: Single Go project with layered architecture
**Constraints**: CI on GitHub Actions free-tier (2 cores, 7GB RAM, no container runtime)

## Constitution Check

| Principle | Status | Notes |
|---|---|---|
| I. Interface-First | PASS | `SandboxAcquirer` interface already defined, `claimAcquirer` implements it |
| II. Zero External Dependencies (Core) | PASS | controller-runtime + agent-sandbox in adapter package `kubernetes/` only |
| III. Nil-Safe | PASS | Provider not registered when config missing; SandboxClaim mode requires non-nil client |
| IV. Typed Error Domain | PASS | Errors wrapped with context, surfaced as tool results |
| V. Validate Early | PASS | Config mutual exclusion validated in `New()` at startup |
| IX. Kubernetes-Native | PASS | SandboxClaim CRDs consumed via agent-sandbox |
| Testing: Real over fakes | PASS | Real sandbox-server binary; fake only at K8s API boundary |
| Testing: CI-friendly | PASS | No container runtime needed; Python pre-installed on GH Actions |

No violations.

## Project Structure

### Source Code

```text
pkg/tools/builtins/codeinterpreter/
├── provider.go                       # CodeInterpreter FunctionProvider (existing, update New() for claimAcquirer)
├── client.go                         # Sandbox HTTP client (existing, no changes)
├── client_test.go                    # Client unit tests (existing, no changes)
├── types.go                          # Sandbox request/response types (existing, no changes)
└── kubernetes/
    ├── acquirer.go                   # claimAcquirer: SandboxClaim create/watch/delete (NEW)
    └── acquirer_test.go              # Tests with controller-runtime fake client (NEW)

test/integration/
├── codeinterpreter_test.go           # Integration test with real sandbox-server (NEW)
```

### Dependencies (go.mod additions)

```text
sigs.k8s.io/agent-sandbox v0.1.1        # SandboxClaim + Sandbox API types
sigs.k8s.io/controller-runtime v0.22.x  # Typed K8s client (Create, Get, Watch, Delete)
```

## Design Decisions

### D1: claimAcquirer implements SandboxAcquirer

The existing `SandboxAcquirer` interface (`Acquire(ctx) -> (url, release, error)`) maps cleanly to the SandboxClaim lifecycle. No interface changes needed. The `release` function returned by `Acquire` deletes the SandboxClaim. See [research.md](research.md#r3-sandboxacquirer-interface-fit).

### D2: Watch Sandbox (not SandboxClaim) for readiness

The agent-sandbox controller creates a Sandbox with the same name as the SandboxClaim. The Sandbox's status contains `serviceFQDN` and the `Ready` condition. This matches the Python client's approach. See [research.md](research.md#r1-agent-sandbox-crd-api-types).

### D3: Integration tests start real sandbox-server

The sandbox-server binary is built from this repo (`cmd/sandbox-server`). Integration tests build it, start it as a subprocess on a random port, point the provider at it, and run code through the full execution path. No mocks for the execution flow. See [research.md](research.md#r2-testing-strategy).

### D4: provider.go wiring update

`New()` in `provider.go` currently returns an error for `sandbox_template` mode. The update instantiates a `claimAcquirer` instead. The rest of the provider code is unchanged because it works against the `SandboxAcquirer` interface.
