# Implementation Plan: List Responses and Input Items Endpoints

**Branch**: `028-list-endpoints` | **Date**: 2026-02-25 | **Spec**: [spec.md](spec.md)

## Summary

Add GET /v1/responses (list with cursor pagination, model filter, ordering) and GET /v1/responses/{id}/input_items (input items with pagination). Extend ResponseStore interface with ListResponses and GetInputItems methods. Implement in the in-memory store. Update OpenAPI spec.

## Technical Context

**Language/Version**: Go 1.22+
**Primary Dependencies**: Go standard library only
**Testing**: Go `testing` package, integration tests via httptest
**Project Type**: Single Go project

## Constitution Check

| Principle | Status | Notes |
|---|---|---|
| I. Interface-First | PASS | Extend existing ResponseStore interface |
| II. Zero External Dependencies | PASS | No new deps |
| III. Nil-Safe | PASS | No store = 501 |

No violations.

## Project Structure

```text
pkg/transport/
├── store.go               # Extend ResponseStore interface

pkg/transport/http/
├── adapter.go             # Add list and input_items handlers

pkg/storage/memory/
├── memory.go              # Implement ListResponses, GetInputItems

api/
└── openapi.yaml           # Add two new endpoints

test/integration/
└── responses_test.go      # List and input_items tests
```
