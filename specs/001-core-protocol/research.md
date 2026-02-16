# Research: Core Protocol & Data Model

**Feature**: 001-core-protocol
**Date**: 2026-02-16

## Research Tasks

### RT-1: Go Polymorphic Type Serialization Patterns

**Decision**: Use a flat struct with discriminator field and optional type-specific embedded structs.

**Rationale**: The OpenResponses Item is polymorphic, discriminated by the `type` field. Go does not have sum types or sealed interfaces. Three patterns were evaluated:

**Alternatives considered**:

1. **Interface with concrete types** (e.g., `Item` interface, `MessageItem`/`FunctionCallItem` structs): Clean type safety, but JSON deserialization requires a custom `UnmarshalJSON` that peeks at the `type` field to determine which concrete type to create. Round-tripping through `json.RawMessage` adds complexity.

2. **Flat struct with optional fields** (chosen): Single `Item` struct with `Type` discriminator and pointer fields (`*MessageData`, `*FunctionCallData`, etc.) that are nil when not applicable. Simple JSON marshaling via `omitempty`. Straightforward to validate. Used by many Go API libraries (e.g., the Stripe Go SDK, Slack Go SDK).

3. **`json.RawMessage` everywhere**: Store type-specific data as raw JSON and decode on demand. Defers type errors to access time rather than parse time. Poor developer experience.

Decision (2) was chosen because it provides correct JSON serialization out of the box, avoids custom unmarshalers for the common case, and matches the "flat with discriminator" pattern used by the OpenAI Go SDK.

### RT-2: ID Generation Strategy

**Decision**: Use `crypto/rand` to generate 24-character alphanumeric strings with type prefixes (`resp_`, `item_`).

**Rationale**: Matches OpenAI's ID format (e.g., `resp_67ccf18ef5fc8190b42dbcb8a6626432`). Using `crypto/rand` provides sufficient uniqueness without requiring a database sequence or external service. The prefixed format aids debugging and log correlation.

**Alternatives considered**:

1. **UUID v4**: Standard but longer (36 chars with hyphens), does not match OpenAI convention, harder to type.
2. **UUID v7**: Time-ordered, good for database indexing, but overkill for a data model library that does not own storage.
3. **ULID**: Similar benefits to UUID v7, but adds a dependency.

### RT-3: State Machine Implementation Pattern

**Decision**: State machine as a validation function, not an object with methods.

**Rationale**: The state machines in this spec are simple (3-5 states, no conditional transitions). A function `ValidateTransition(from, to Status) error` is sufficient. A full state machine library would be over-engineering for transitions this straightforward.

**Alternatives considered**:

1. **State machine library** (e.g., `looplab/fsm`): Adds dependency for a trivial state graph.
2. **Method on the struct** (e.g., `response.TransitionTo(status)`): Couples mutation with validation. Better to keep validation separate so the core engine controls when transitions happen.

### RT-4: JSON Serialization for `tool_choice` Union Type

**Decision**: Custom `ToolChoice` type implementing `json.Marshaler`/`json.Unmarshaler`.

**Rationale**: `tool_choice` can be a string (`"auto"`, `"required"`, `"none"`) or a structured object (`{"type":"function","name":"..."}`). This requires a union type in Go. A custom type with `MarshalJSON`/`UnmarshalJSON` handles both forms cleanly.

**Alternatives considered**:

1. **`any` / `interface{}`**: Loses type safety entirely.
2. **Separate fields** (`ToolChoiceString` + `ToolChoiceObject`): Awkward API, does not map to the JSON wire format.

### RT-5: Configurable Validation Limits

**Decision**: Define a `ValidationConfig` struct passed to the validator, with sensible defaults.

**Rationale**: FR-040 requires configurable limits (max input items, max content size). Encoding limits in a config struct keeps validation pure (no global state) and testable (tests can pass different configs).

## Summary

All research items resolved. No external dependencies required beyond Go standard library (`encoding/json`, `crypto/rand`, `errors`, `fmt`). No NEEDS CLARIFICATION remains.
