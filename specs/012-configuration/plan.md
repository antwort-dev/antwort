# Implementation Plan: Unified Configuration System

**Branch**: `012-configuration` | **Date**: 2026-02-20 | **Spec**: [spec.md](spec.md)

## Summary

Replace ad-hoc env var configuration with a unified config system: YAML file + env override + `_file` secret references + file watching for hot reload. Refactor cmd/server/main.go to use config.

## Technical Context

**Language/Version**: Go 1.25+
**Dependencies**: `gopkg.in/yaml.v3` for YAML parsing. `github.com/fsnotify/fsnotify` for file watching (or stdlib polling).
**Testing**: `go test` with temp files for config loading, env override, file watch tests.

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| II. Zero Dependencies | PASS | YAML + fsnotify in config package (adapter) |
| III. Nil-Safe | PASS | No config file = env + defaults |
| IX. Kubernetes-Native | PASS | File-based, no k8s API dependency |

## Project Structure

```text
pkg/
└── config/
    ├── config.go          # Config struct with all sections
    ├── loader.go          # Load from file + env + defaults, resolve _file refs
    ├── watcher.go         # File watcher for hot reload
    ├── validate.go        # Startup validation
    ├── config_test.go     # Loading, override, validation tests
    └── watcher_test.go    # File watch tests

cmd/
└── server/
    └── main.go            # REFACTORED: use config.Load() instead of os.Getenv
```
