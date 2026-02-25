# Implementation Plan: Sandbox Multi-Runtime Modes

**Branch**: `027-sandbox-modes` | **Date**: 2026-02-25 | **Spec**: [spec.md](spec.md)

## Summary

Add a mode system to the sandbox server binary. SANDBOX_MODE env var selects the runtime (python/golang/node/shell). Auto-detect as fallback. Each mode maps to interpreter command, file extension, and package installer. REST API unchanged. Health reports mode and version.

## Technical Context

**Language/Version**: Go 1.22+ for the server binary
**Primary Dependencies**: Go standard library (`net/http`, `os/exec`, `encoding/json`)
**Testing**: Go `testing` package, table-driven tests for mode config
**Project Type**: Existing binary extension (cmd/sandbox-server/)

## Constitution Check

| Principle | Status | Notes |
|---|---|---|
| II. Zero External Dependencies | N/A | Separate binary, not core |
| V. Validate Early | PASS | Invalid mode fails at startup |

No violations.

## Project Structure

```text
cmd/sandbox-server/
├── main.go                # Add mode field, modeConfig(), detectMode()
├── main_test.go           # Unit tests for mode selection and auto-detection
```
