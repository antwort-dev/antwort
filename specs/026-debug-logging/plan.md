# Implementation Plan: Category-Based Debug Logging

**Branch**: `026-debug-logging` | **Date**: 2026-02-25 | **Spec**: [spec.md](spec.md)

## Summary

Add a `pkg/debug` package with category-based debug logging. Two orthogonal controls: `ANTWORT_DEBUG` (comma-separated categories) and `ANTWORT_LOG_LEVEL` (ERROR/WARN/INFO/DEBUG/TRACE). Instrument the provider layer and engine agentic loop. Zero overhead when disabled.

## Technical Context

**Language/Version**: Go 1.22+
**Primary Dependencies**: Go standard library only (`log/slog`, `os`, `strings`, `sync`)
**Testing**: Go `testing` package
**Project Type**: Single Go project

## Constitution Check

| Principle | Status | Notes |
|---|---|---|
| II. Zero External Dependencies | PASS | Uses stdlib slog only |
| V. Validate Early | PASS | Invalid categories silently ignored |

No violations.

## Project Structure

```text
pkg/debug/
├── debug.go               # Category registry, Enabled(), Log(), Init()
├── debug_test.go           # Unit tests

pkg/provider/openaicompat/
├── translate.go            # Add debug logging around TranslateToChat
├── stream.go               # Add debug logging for stream initiation

pkg/engine/
├── loop.go                 # Add debug logging for agentic loop turns
├── engine.go               # Add debug logging for request handling

pkg/config/
├── types.go                # Add Logging section to Config

cmd/server/
├── main.go                 # Initialize debug system from config
```
