# Implementation Plan: Function Provider Registry

**Branch**: `016-function-registry` | **Date**: 2026-02-20 | **Spec**: [spec.md](spec.md)

## Project Structure

```text
pkg/tools/registry/
├── provider.go        # FunctionProvider interface + Route type
├── registry.go        # FunctionRegistry (ToolExecutor + HTTP handler + metrics)
├── middleware.go       # Auth + metrics wrapping for provider routes
└── registry_test.go   # Tests with mock provider

pkg/config/
└── config.go          # MODIFIED: Add ProvidersConfig map
```
