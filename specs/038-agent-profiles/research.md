# Research: 038-agent-profiles

**Date**: 2026-03-03

## R1: Profile Storage Location

**Decision**: Profiles live in the existing `config.yaml` under an `agents` section. No separate file, no database, no API CRUD.

**Rationale**: The config system (Spec 012) already handles YAML loading, env var overrides, and validation. Adding profiles there requires zero new infrastructure. Config-file profiles match the Kubernetes-native pattern where agent configurations are managed via ConfigMaps or Kustomize overlays.

## R2: Profile Resolution in the Engine

**Decision**: Profile resolution happens at the start of `CreateResponse`, before provider translation. A `ProfileResolver` interface (1 method: `Resolve(name) -> AgentProfile`) is injected into the engine.

**Rationale**: The engine already handles `previous_response_id` and `conversation_id` resolution at the same point. Profile resolution is another pre-processing step. The interface allows swapping implementations (config-based in v1, CRD-based in the future).

## R3: Prompt Parameter Mapping

**Decision**: The `prompt` parameter maps to profile resolution: `prompt.id` is the profile name/ID, `prompt.variables` become the template variables. The `prompt.version` field is accepted but ignored (v1 has no versioning).

**Rationale**: A prompt IS a profile where only instructions are set. Rather than maintaining two separate systems, the prompt parameter is a thin adapter on top of profile resolution. This reduces code duplication and provides a single source of truth for agent configurations.

## R4: Template Substitution

**Decision**: Use simple string replacement for `{{variable}}` patterns. No template engine library. The substitution happens after profile resolution and before the request is passed to the provider.

**Rationale**: Simple `strings.ReplaceAll` for each variable is sufficient. The `{{variable}}` syntax is compatible with Mustache/Handlebars conventions. No external template library is needed (constitution Principle II).

## R5: Merge Strategy

**Decision**: Profile fields provide defaults. Request fields override. Tools are merged (union).

| Field | Profile | Request | Result |
|-------|---------|---------|--------|
| model | model-a | (empty) | model-a |
| model | model-a | model-b | model-b |
| tools | [web_search] | [code_interpreter] | [web_search, code_interpreter] |
| temperature | 0.3 | 0.7 | 0.7 |
| instructions | template | explicit | explicit |

## R6: Package Structure

**Decision**: New `pkg/agent/` package with profile types, resolver, template engine, and config parsing. Keeps agent concerns isolated from the engine.

```
pkg/agent/
├── profile.go      # AgentProfile type, ProfileResolver interface
├── config.go       # Config-based resolver (loads from agents section)
├── template.go     # {{variable}} substitution
├── merge.go        # Profile-to-request merge logic
└── *_test.go       # Tests
```
