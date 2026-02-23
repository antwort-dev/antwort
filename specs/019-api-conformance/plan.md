# Implementation Plan: API Conformance & Integration Testing

**Branch**: `019-api-conformance` | **Date**: 2026-02-23 | **Spec**: [spec.md](spec.md)

## Summary

Own OpenAPI spec + oasdiff against upstream + Go integration tests + Zod compliance + container-based CI.

## Project Structure

```text
api/
├── openapi.yaml                # Antwort's OpenAPI spec (OpenResponses + side-APIs)
└── openresponses-ref.json      # Downloaded upstream spec (gitignored, fetched in CI)

test/
├── integration/
│   ├── responses_test.go       # POST/GET/DELETE /v1/responses
│   ├── streaming_test.go       # SSE event validation
│   ├── vectorstores_test.go    # /v1/vector_stores CRUD
│   ├── health_test.go          # /healthz, /metrics
│   ├── errors_test.go          # Error cases (400, 401, 404)
│   └── helpers_test.go         # Test server setup (antwort + mock backend)
├── Containerfile               # Test runner container
└── run.sh                      # Pipeline script

.github/
└── workflows/
    └── api-conformance.yml     # GitHub Actions workflow
```
