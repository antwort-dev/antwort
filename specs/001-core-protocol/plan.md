# Implementation Plan: Core Protocol & Data Model

**Branch**: `001-core-protocol` | **Date**: 2026-02-16 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/001-core-protocol/spec.md`

## Summary

Implement the foundational data model for the Antwort OpenResponses proxy. This is a pure Go library package (`pkg/api`) containing all core data types (Items, Content, Request, Response), streaming event types, error types, state machine logic, request validation, and ID generation. The package has zero external dependencies (Go stdlib only) and zero I/O. All other specs depend on it.

## Technical Context

**Language/Version**: Go 1.22+
**Primary Dependencies**: None (Go standard library only: `encoding/json`, `crypto/rand`, `errors`, `fmt`, `strings`, `regexp`)
**Storage**: N/A (pure data types, no persistence)
**Testing**: `go test` with table-driven tests
**Target Platform**: Library package, cross-platform
**Project Type**: Single Go module (`github.com/rhuss/antwort`)
**Performance Goals**: N/A (data types, no I/O)
**Constraints**: Must produce JSON identical to OpenAI Responses API for client compatibility (SC-001)
**Scale/Scope**: ~8 core entities, ~40 functional requirements, ~15 streaming event types

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Constitution is a template placeholder (not yet ratified). No gates to enforce. Proceeding.

## Project Structure

### Documentation (this feature)

```text
specs/001-core-protocol/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0: design decisions
├── data-model.md        # Phase 1: entity definitions
├── quickstart.md        # Phase 1: usage guide
├── contracts/
│   └── openapi.yaml     # Phase 1: OpenAPI 3.1 schema
└── checklists/
    └── requirements.md  # Spec quality checklist
```

### Source Code (repository root)

```text
pkg/
└── api/
    ├── types.go           # Item, MessageData, FunctionCallData, ReasoningData,
    │                      # ContentPart, OutputContentPart, CreateResponseRequest,
    │                      # Response, Usage, ToolDefinition, ToolChoice
    ├── events.go          # StreamEvent, StreamEventType constants, constructors
    ├── errors.go          # APIError, ErrorType constants, error constructors
    ├── validation.go      # ValidateRequest, ValidateItem, ValidationConfig
    ├── state.go           # ValidateResponseTransition, ValidateItemTransition
    ├── id.go              # NewResponseID, NewItemID (prefixed random IDs)
    ├── types_test.go      # JSON round-trip tests for all types
    ├── events_test.go     # Event serialization tests
    ├── errors_test.go     # Error construction tests
    ├── validation_test.go # Request validation tests
    ├── state_test.go      # State machine transition tests
    └── id_test.go         # ID format and uniqueness tests
```

**Structure Decision**: Single `pkg/api` package. All core protocol types in one package to avoid circular dependencies. Sub-packages would create unnecessary import chains since all types reference each other (e.g., Response contains Items, Items contain ContentParts).

## Design Decisions

See [research.md](research.md) for full rationale on each decision:

1. **Polymorphic Items**: Flat struct with discriminator field and optional pointer fields (not interfaces)
2. **ID generation**: `crypto/rand` with `resp_`/`item_` prefixes, 24-char alphanumeric
3. **State machine**: Validation function, not state machine object
4. **ToolChoice**: Custom type with `json.Marshaler`/`json.Unmarshaler` for string/object union
5. **Validation limits**: `ValidationConfig` struct with configurable max values

## Complexity Tracking

No constitution violations to justify.
