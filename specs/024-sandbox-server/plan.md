# Implementation Plan: Sandbox Server for Code Execution

**Branch**: `024-sandbox-server` | **Date**: 2026-02-25 | **Spec**: [spec.md](spec.md)

## Summary

Build a standalone HTTP server binary (`cmd/sandbox-server/`) that accepts Python code execution requests, runs them in subprocesses with timeout enforcement, and returns results. Build a container image (`Containerfile.sandbox`) packaging the server with Python 3.12 and `uv`.

## Technical Context

**Language/Version**: Go 1.22+ for the server binary
**Primary Dependencies**: Go standard library (`net/http`, `os/exec`, `context`, `encoding/json`, `encoding/base64`, `sync/atomic`)
**External Runtime Dependencies**: Python 3.12+, `uv` package manager (in container image only)
**Testing**: Go `testing` package, integration tests using `os/exec` to test the binary
**Target Platform**: Kubernetes (runs inside agent-sandbox pods)
**Project Type**: New binary in existing Go project + new container image

## Constitution Check

| Principle | Status | Notes |
|---|---|---|
| IX. Kubernetes-Native Execution | PASS | Core purpose: execution backend for sandbox pods |
| II. Zero External Dependencies | N/A | Separate binary, not part of core packages |
| V. Validate Early | PASS | Timeout enforced before execution, request validated |

No violations.

## Project Structure

```text
cmd/sandbox-server/
├── main.go                # HTTP server, route handlers

Containerfile.sandbox       # Container image: Go binary + Python + uv

test/sandbox/
├── sandbox_test.go         # Integration tests for the sandbox server
```
