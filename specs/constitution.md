# Antwort Constitution

## Core Principles

### I. Interface-First Design

Every subsystem is defined as a Go interface before any implementation exists. Interfaces are the contracts between layers. Implementations are pluggable.

- Interfaces should have 1-5 methods. If an interface grows beyond 5 methods, split it into cohesive sub-interfaces.
- Provide function adapter types for single-method interfaces (e.g., `ResponseCreatorFunc`) to enable inline usage in tests and simple wiring.
- Dependencies are injected as interfaces, never as concrete types. This enables testing without infrastructure and swapping implementations without changing callers.
- No interface inheritance. Use composition of small interfaces where broader capability is needed.

### II. Zero External Dependencies (Core)

The core packages (`pkg/api`, `pkg/transport`, `pkg/engine`, `pkg/provider`) depend only on the Go standard library. No exceptions.

- Allowed standard library packages: `net/http`, `log/slog`, `encoding/json`, `context`, `sync`, `os/signal`, `crypto/rand`, `fmt`, `strings`, `regexp`, `errors`, `time`, `io`, `bufio`, `bytes`, `strconv`, `net/url`.
- External dependencies are permitted only in adapter implementations (e.g., a future Milvus vector store adapter) and only when no reasonable standard library alternative exists.
- External dependencies must be wrapped behind an interface so the core never imports them.

### III. Nil-Safe Composition

Optional capabilities (storage, tools, vector search, auth) use nil-safe composition, not feature flags or build tags.

- The engine and adapters accept optional dependencies as interface parameters. When nil, the feature is disabled.
- Methods that depend on optional capabilities check for nil before use and return gracefully (no-op, skip, or descriptive error).
- This enables deployments at any scale: a minimal stateless proxy with no storage, a full-featured gateway with PostgreSQL and MCP tools, or anything in between.
- Required dependencies are validated in constructors. Optional dependencies are accepted without validation.

### IV. Typed Error Domain

Errors are first-class domain types, not generic strings or sentinel values.

- All errors surfaced to clients use the `APIError` type with a classified `Type` (server_error, invalid_request, not_found, model_error, too_many_requests).
- Validation errors include a `Param` field that pinpoints the specific request field that failed.
- Constructor functions (`NewInvalidRequestError`, `NewNotFoundError`, etc.) enforce correct error construction.
- Internal errors use `fmt.Errorf("context: %w", err)` to wrap with call-site context while preserving the error chain.
- Error types map deterministically to HTTP status codes. The mapping is defined once and used consistently.

### V. Validate Early, Fail Fast

Request validation happens before any processing, provider calls, or state mutations.

- The transport layer validates well-formed JSON and transport concerns (body size, content type).
- The engine validates business rules (required fields, type constraints, capability compatibility) before calling the provider.
- Each validation failure returns a typed error identifying the exact field and constraint that failed.
- Only the first validation error is returned (fail-fast). Clients fix one issue at a time.
- Validation limits (max input items, max content size, max tools) are configuration-driven with sensible defaults, never hardcoded.

### VI. Protocol-Agnostic Provider Layer

The provider interface makes no assumptions about the backend protocol. Each adapter handles its own protocol translation internally.

- The `Provider` interface defines operations in terms of Antwort's own types (`ProviderRequest`, `ProviderResponse`, `ProviderEvent`), not in terms of Chat Completions, Responses API, or any other protocol.
- A Chat Completions adapter translates to/from `/v1/chat/completions`. A Responses API proxy adapter forwards to/from `/v1/responses`. Both implement the same interface.
- Protocol-specific concerns (SSE chunk parsing, content encoding, finish_reason mapping) live inside the adapter, invisible to the engine.
- Adding a new backend protocol means adding a new adapter, not changing the interface or engine.

### VII. Streaming as First-Class Concern

Streaming is the primary consumption mode for LLM inference. It is not an afterthought bolted onto a synchronous design.

- The `ResponseWriter` abstraction hides streaming protocol details (SSE formatting, flushing, sentinel values) from the engine.
- The engine generates synthetic lifecycle events (response.created, output_item.added, etc.) that the backend does not produce, ensuring clients receive the complete OpenResponses event sequence.
- Streaming state (event ordering, tool call argument buffering, content indexing) is managed by the engine and adapter, not exposed to callers.
- Context cancellation during streaming is detected and produces the correct terminal event (response.cancelled) or error.

### VIII. Context Carries Cross-Cutting Data

Request-scoped data (request IDs, user identity, tracing context) propagates through `context.Context`, not through function parameters or global state.

- Context keys use private types to prevent collisions between packages.
- Middleware injects values into context; downstream code reads them without knowing where they originated.
- The engine and providers never depend on specific middleware being present. Missing context values result in safe defaults (empty request ID, no user identity).

## Development Standards

### Testing

- **Table-driven tests** for all validation, parsing, and translation logic. Each test case is a row with inputs, expected outputs, and a descriptive name.
- **Mock backends** for provider adapter tests. A test HTTP server that speaks Chat Completions (or other protocols) exercises the full adapter path without external infrastructure.
- **Interface mocks** for engine tests. The engine is tested against mock providers and mock stores, verifying orchestration logic in isolation.
- **Nil-safe tests** for every optional capability. Tests verify correct behavior when optional dependencies are present AND when they are nil.
- **Error path coverage** is mandatory. Every error branch in the code has a corresponding test case. Testing only happy paths is insufficient.

### Naming Conventions

- **ID prefixes**: `resp_` for responses, `item_` for items, followed by 24 random alphanumeric characters. Validated by regex before acceptance.
- **Package names**: Short, lowercase, singular (`api`, `transport`, `engine`, `provider`). No stuttering (use `provider.New`, not `provider.NewProvider`).
- **Interface names**: Noun or noun phrase describing the capability (`ResponseCreator`, `ResponseStore`, `Provider`). Single-method interfaces may use `-er` suffix.
- **Error constructors**: `New<Type>Error(params)` pattern (e.g., `NewInvalidRequestError`).
- **Extension types**: `provider:type` format with colon separator (e.g., `vllm:guided_decoding`).

### Configuration

- Configuration is injected via typed structs with sensible defaults.
- Environment variables override config file values. Environment variable names follow the pattern `ANTWORT_<SECTION>_<FIELD>` (e.g., `ANTWORT_VLLM_URL`).
- Zero/nil values in config mean "use default" or "feature disabled", not "error".
- Validation configs (limits, timeouts) have explicit default constructors (e.g., `DefaultValidationConfig()`).

### Specification-Driven Development

- Every feature starts as a formal specification before any code is written.
- Specs define WHAT the system does (functional requirements, acceptance scenarios, edge cases) and WHY (user stories, priorities), never HOW (no implementation details in specs).
- Specs produce artifacts: spec.md, plan.md, tasks.md, data-model.md, research.md, review-summary.md.
- Implementation is verified against the spec. All functional requirements must be traceable to test cases.
- Specs are numbered in dependency order and developed sequentially.

### IX. Kubernetes-Native Execution

Antwort is designed exclusively for Kubernetes. There is no standalone or local execution mode.

- All tool code execution is delegated to isolated sandbox pods managed by the [agent-sandbox](https://github.com/kubernetes-sigs/agent-sandbox) project (Kubernetes SIG). No custom or potentially blocking code executes within the antwort process.
- Antwort consumes Kubernetes CRDs (`Sandbox`, `SandboxWarmPool`, `SandboxClaim`) rather than implementing its own pod management controllers. The agent-sandbox controller handles pod lifecycle, warm pools, and stable identity.
- Communication between antwort and sandbox pods uses mutual TLS with SPIFFE/SPIRE workload identities. No shared secrets.
- Sandbox pods expose a well-defined REST interface for code execution. The container image (Python runtime, `uv` package manager, execution server) is an antwort project deliverable; the pod lifecycle is managed by agent-sandbox.

## Architecture Constraints

### Layer Dependencies

Dependencies flow in one direction: transport -> engine -> provider. No reverse dependencies.

```
transport (pkg/transport/)
    |
    v
engine (pkg/engine/)
    |
    v
provider (pkg/provider/)
    |
    v
api (pkg/api/)  <-- shared types, depended on by all layers
```

- `pkg/api/` defines shared protocol types. All other packages depend on it.
- `pkg/transport/` defines handler interfaces and HTTP/SSE adapter. Depends on `pkg/api/`.
- `pkg/engine/` implements the handler interfaces. Depends on `pkg/api/` and `pkg/transport/` (for interface types only).
- `pkg/provider/` defines the provider interface and adapters. Depends on `pkg/api/` only. Does not import `pkg/transport/`.

### Two-Tier API

Antwort supports two operational modes determined per-request, not per-deployment:

- **Stateless** (`store: false`): No persistence, no response retrieval, no conversation chaining. Suitable for fire-and-forget inference proxying.
- **Stateful** (`store: true`, default): Full CRUD, conversation chaining via `previous_response_id`, response persistence. Requires a storage backend.

The engine handles both modes. Stateless mode is always available. Stateful features degrade gracefully when no store is configured.

## Governance

- This constitution supersedes ad-hoc decisions. When a design question arises, check the constitution first.
- Amendments require updating this document, reviewing impact on existing specs, and migrating any non-compliant code.
- New specs must declare compliance with these principles. Deviations require explicit justification in the spec's Assumptions or Clarifications section.
- Code reviews verify constitutional compliance. Non-compliant code is not merged.

**Version**: 1.1.0 | **Ratified**: 2026-02-17 | **Last Amended**: 2026-02-18
