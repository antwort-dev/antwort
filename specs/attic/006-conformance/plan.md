# Implementation Plan: OpenResponses Conformance Testing

**Branch**: `006-conformance` | **Date**: 2026-02-18 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/006-conformance/spec.md`

## Summary

Create the infrastructure to run antwort as a standalone server and validate it against the official OpenResponses compliance test suite. This includes a server wiring layer (`cmd/server`), a deterministic mock Chat Completions backend (`cmd/mock-backend`), integration of the official TypeScript compliance suite via containerized runner, test profiles for layered conformance validation, and a CI-ready pipeline script.

## Technical Context

**Language/Version**: Go 1.22+ for server and mock binaries. TypeScript/bun for the official compliance suite (run via container).
**Primary Dependencies**: Existing antwort packages (api, transport, engine, provider, storage, tools). No new Go dependencies.
**Container Runtime**: Podman for running the compliance suite container.
**Testing**: Official OpenResponses compliance suite (TypeScript/Zod) + Go integration tests for server and mock.
**Target Platform**: Linux/macOS development, CI pipelines
**Project Type**: Single Go module + Makefile + shell scripts
**Constraints**: Compliance suite runs as-is from upstream. No forking. Profile filtering is post-hoc.

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Interface-First | N/A | Infrastructure, no new interfaces |
| II. Zero Dependencies | PASS | Server uses existing packages. Mock uses stdlib. |
| III. Nil-Safe Composition | PASS | Storage optional in server wiring |
| IX. Kubernetes-Native | PASS | Podman for containers, health endpoints |

No violations.

## Project Structure

### Source Code

```text
cmd/
├── server/
│   └── main.go              # Antwort server wiring: env config -> transport + engine + provider + storage
│
└── mock-backend/
    └── main.go              # Mock Chat Completions server for deterministic testing

conformance/
├── run.sh                   # Pipeline script: start mock, start antwort, run suite, report
├── profiles.json            # Test profile definitions (core, extended)
├── Containerfile            # Container image for running the TypeScript suite
└── README.md                # Instructions for running conformance tests

Makefile                     # Top-level: `make conformance PROFILE=core`
```

## Complexity Tracking

No constitutional violations. No complexity justifications needed.
