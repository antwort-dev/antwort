# Implementation Plan: Code Interpreter Tool

**Branch**: `025-code-interpreter` | **Date**: 2026-02-25 | **Spec**: [spec.md](spec.md)

## Summary

Add a `code_interpreter` FunctionProvider that executes Python code in sandbox pods. Two modes: SandboxClaim mode (creates/deletes CRDs for pod-level isolation) and static URL mode (direct HTTP to a sandbox server, for development). New `code_interpreter_call` item type for structured output with logs and file references.

## Technical Context

**Language/Version**: Go 1.22+
**Primary Dependencies**: Go standard library for core. `k8s.io/client-go` for SandboxClaim CRUD (adapter package only).
**Testing**: Go `testing` package, integration tests with mock sandbox HTTP server
**Target Platform**: Kubernetes with agent-sandbox controller
**Project Type**: Single Go project with layered architecture

## Constitution Check

| Principle | Status | Notes |
|---|---|---|
| I. Interface-First | PASS | Uses existing FunctionProvider interface |
| II. Zero External Dependencies | PASS | client-go in adapter package `pkg/tools/builtins/codeinterpreter/kubernetes/` |
| III. Nil-Safe | PASS | Provider disabled = not registered |
| V. Validate Early | PASS | Config validation at startup (mutual exclusion) |
| IX. Kubernetes-Native | PASS | Core purpose: sandbox pods via CRDs |

No violations.

## Project Structure

```text
pkg/api/
├── types.go                          # Add CodeInterpreterCallData, CodeInterpreterOutput types

pkg/tools/builtins/codeinterpreter/
├── provider.go                       # CodeInterpreter FunctionProvider (implements FunctionProvider)
├── client.go                         # Sandbox HTTP client (calls /execute on sandbox pods)
├── types.go                          # Internal types for sandbox request/response
└── kubernetes/
    └── sandbox.go                    # SandboxClaim client (adapter with client-go)

pkg/engine/
├── loop.go                           # Update classifyToolType for code_interpreter events

cmd/server/
├── main.go                           # Wire code_interpreter provider

test/integration/
├── helpers_test.go                   # Add mock sandbox server for integration tests
├── codeinterpreter_test.go           # Integration tests for code_interpreter tool
```
