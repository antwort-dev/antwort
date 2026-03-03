# Implementation Plan: Agent Profiles & Prompt Templates

**Branch**: `038-agent-profiles` | **Date**: 2026-03-03 | **Spec**: [spec.md](spec.md)

## Summary

Add server-side agent profiles to antwort. Profiles are named configuration bundles (model, instructions with `{{variable}}` templates, tools, constraints) loaded from `config.yaml`. The engine resolves profiles via the `agent` field or OpenAI `prompt` parameter on create response requests. A new `pkg/agent/` package contains the profile types, resolver, template engine, and merge logic.

## Technical Context

**Language/Version**: Go 1.22+
**Primary Dependencies**: Go standard library only (strings, regexp for template substitution)
**Storage**: Config file only (no database, no runtime CRUD)
**Testing**: `go test` with table-driven tests
**Project Type**: Web service (engine pre-processing + config extension)

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Interface-First | PASS | ProfileResolver interface (1 method) |
| II. Zero External Deps | PASS | stdlib strings for template substitution |
| III. Nil-Safe | PASS | No resolver = agent/prompt fields rejected |
| V. Validate Early | PASS | Profile config validated at startup |
| Documentation | PASS | FR-019/020/021 |

## Design Decisions

### D1: Package Layout

```
pkg/agent/
├── profile.go      # AgentProfile type, ProfileResolver interface
├── config.go       # ConfigResolver (loads profiles from config)
├── template.go     # {{variable}} substitution
├── merge.go        # Profile-to-request merge logic
└── *_test.go
```

### D2: Engine Integration

In `engine.CreateResponse()`, before provider translation:
1. If `agent` is set: resolve profile by name
2. If `prompt` is set: resolve profile by `prompt.id`, extract variables
3. Substitute `{{variables}}` in instructions
4. Merge profile into request (request fields win)
5. Continue with normal request processing

### D3: Config Schema

```yaml
agents:
  profile-name:
    description: "..."
    model: "..."
    instructions: "... {{variable}} ..."
    tools: [...]
    temperature: 0.3
    max_output_tokens: 4096
```

Parsed into `map[string]*AgentProfile` at startup.

### D4: List Endpoint

`GET /v1/agents` returns profile summaries. Mounted on the HTTP adapter mux alongside responses and conversations routes.

## Project Structure

```text
pkg/agent/profile.go           # NEW: AgentProfile, ProfileResolver
pkg/agent/config.go            # NEW: ConfigResolver
pkg/agent/template.go          # NEW: Template substitution
pkg/agent/merge.go             # NEW: Merge logic
pkg/agent/*_test.go            # NEW: Tests
pkg/api/types.go               # Add PromptReference, agent/prompt/variables fields
pkg/config/config.go           # Add AgentProfiles to Config
pkg/engine/engine.go           # Accept ProfileResolver, resolve in CreateResponse
pkg/transport/http/adapter.go  # Add GET /v1/agents route
pkg/transport/http/agents.go   # NEW: Agent list handler
cmd/server/main.go             # Wire ProfileResolver
```
