# Implementation Plan: Structured Output (text.format Passthrough)

**Branch**: `029-structured-output` | **Date**: 2026-02-25 | **Spec**: [spec.md](spec.md)

## Summary

Complete the text.format passthrough pipeline so that structured output constraints (json_object, json_schema) reach the Chat Completions backend via `response_format`. Extend TextFormat with schema fields, add ResponseFormat to the provider request chain, and map it through the openaicompat translation layer. Update OpenAPI spec and add integration tests including mock backend changes for schema-conforming responses.

## Technical Context

**Language/Version**: Go 1.22+ (consistent with Specs 001-028)
**Primary Dependencies**: Go standard library only (`encoding/json`)
**Storage**: N/A (passthrough, no persistence changes)
**Testing**: Go `testing` package, integration tests via httptest
**Project Type**: Single Go project

## Constitution Check

| Principle | Status | Notes |
|---|---|---|
| I. Interface-First | PASS | No new interfaces, extending existing translation pipeline |
| II. Zero External Dependencies | PASS | No new deps, standard library only |
| III. Nil-Safe | PASS | nil TextConfig = no response_format sent (FR-004) |
| VI. Protocol-Agnostic Provider | PASS | ResponseFormat added to ProviderRequest, translated by openaicompat adapter |

No violations.

## Project Structure

```text
pkg/api/
├── types.go                         # Extend TextFormat struct

pkg/provider/
├── types.go                         # Add ResponseFormat to ProviderRequest

pkg/provider/openaicompat/
├── types.go                         # Add ResponseFormat to ChatCompletionRequest
├── translate.go                     # Map ResponseFormat in TranslateToChat

pkg/engine/
├── translate.go                     # Forward text.format to ProviderRequest

api/
└── openapi.yaml                     # Update TextFormat schema

test/integration/
├── helpers_test.go                  # Update mock backend for json response_format
└── responses_test.go                # Structured output tests
```
