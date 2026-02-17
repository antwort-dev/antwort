# Implementation Plan: Core Engine & Provider Abstraction

**Branch**: `003-core-engine` | **Date**: 2026-02-17 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/003-core-engine/spec.md`

## Summary

Implement the core orchestration engine and provider abstraction layer for Antwort. This spec bridges the transport layer (Spec 002) to LLM inference backends. Three packages are introduced: `pkg/provider` (protocol-agnostic provider interface and types), `pkg/engine` (orchestration engine implementing `ResponseCreator`), and `pkg/provider/vllm` (Chat Completions adapter for vLLM and any OpenAI-compatible backend). A prerequisite amendment adds an `Input` field to `api.Response` in `pkg/api`.

## Technical Context

**Language/Version**: Go 1.22+ (required for `http.ServeMux` method+pattern routing, consistent with Spec 002)
**Primary Dependencies**: None (Go standard library only: `net/http`, `log/slog`, `encoding/json`, `context`, `sync`, `io`, `bufio`, `bytes`, `fmt`, `strings`, `time`, `net/url`)
**Storage**: N/A (engine uses `transport.ResponseStore` interface; no storage implementation in this spec)
**Testing**: `go test` with table-driven tests + mock HTTP server for Chat Completions + mock Provider/ResponseStore interfaces
**Target Platform**: Library packages, cross-platform
**Project Type**: Single Go module (`github.com/rhuss/antwort`)
**Performance Goals**: Translation overhead negligible relative to inference latency; streaming events forwarded with no application-layer buffering
**Constraints**: Zero external dependencies; must maintain unidirectional dependency flow (transport -> engine -> provider -> api); provider interface must be protocol-agnostic
**Scale/Scope**: ~8 new entities, 40 functional requirements, 3 packages, ~20 source files

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Interface-First Design | PASS | Provider (5 methods), Engine implements ResponseCreator. Function adapters for testing. |
| II. Zero External Dependencies | PASS | stdlib only across all three packages |
| III. Nil-Safe Composition | PASS | Engine accepts optional ResponseStore (nil = no storage). Future tools/auth follow same pattern. |
| IV. Typed Error Domain | PASS | Reuses api.APIError. Provider errors mapped to error types via constructors. |
| V. Validate Early, Fail Fast | PASS | Engine validates capabilities before provider call. Adapter validates backend response. |
| VI. Protocol-Agnostic Provider | PASS | Interface uses ProviderRequest/Response/Event types. Chat Completions is adapter-internal. |
| VII. Streaming First-Class | PASS | Engine generates synthetic lifecycle events, maps ProviderEvent to StreamEvent. |
| VIII. Context Propagation | PASS | Engine propagates ctx to all provider calls. No middleware-specific dependencies. |
| Layer Dependencies | PASS | provider imports api only. engine imports api + transport (interfaces). No reverse deps. |

No violations. All gates pass.

## Project Structure

### Documentation (this feature)

```text
specs/003-core-engine/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0: design decisions
├── data-model.md        # Phase 1: entity definitions
├── quickstart.md        # Phase 1: usage guide
├── contracts/
│   └── chat-completions-mapping.md  # Phase 1: translation rules
└── checklists/
    └── requirements.md  # Spec quality checklist
```

### Source Code (repository root)

```text
pkg/
├── api/
│   └── types.go                  # AMENDED: Add Input field to Response
│
├── provider/
│   ├── doc.go                    # Package documentation
│   ├── provider.go               # Provider interface, Translator interface
│   ├── types.go                  # ProviderRequest, ProviderResponse, ProviderEvent,
│   │                             # ProviderMessage, ProviderCapabilities, ModelInfo
│   ├── capabilities.go           # Capability validation logic
│   ├── provider_test.go          # Interface contract tests (with mock)
│   ├── types_test.go             # Type serialization tests
│   ├── capabilities_test.go      # Capability validation tests
│   │
│   └── vllm/
│       ├── doc.go                # Package documentation
│       ├── vllm.go               # VLLMProvider struct, Complete, Stream, ListModels, Close
│       ├── config.go             # VLLMConfig struct, defaults
│       ├── translate.go          # Request translation (Items -> Chat messages)
│       ├── response.go           # Response translation (Chat response -> Items)
│       ├── stream.go             # SSE chunk parsing, streaming event production
│       ├── types.go              # Chat Completions request/response types (internal)
│       ├── errors.go             # HTTP error mapping (status code -> APIError)
│       ├── vllm_test.go          # Integration tests with mock HTTP server
│       ├── translate_test.go     # Translation unit tests (table-driven)
│       ├── response_test.go      # Response translation tests
│       └── stream_test.go        # Streaming SSE parsing tests
│
└── engine/
    ├── doc.go                    # Package documentation
    ├── engine.go                 # Engine struct, CreateResponse (implements ResponseCreator)
    ├── config.go                 # EngineConfig struct
    ├── translate.go              # CreateResponseRequest -> ProviderRequest translation
    ├── events.go                 # ProviderEvent -> api.StreamEvent mapping
    ├── history.go                # Conversation history reconstruction
    ├── validate.go               # Capability validation, request pre-checks
    ├── engine_test.go            # Engine orchestration tests (mock provider + mock store)
    ├── translate_test.go         # Request translation tests (table-driven)
    ├── events_test.go            # Event mapping tests
    ├── history_test.go           # Conversation reconstruction tests
    └── validate_test.go          # Capability validation tests
```

**Structure Decision**: Three new packages following the layer dependency graph. `pkg/provider/` defines the interface and types at the top level; `pkg/provider/vllm/` contains the first adapter as a sub-package. `pkg/engine/` contains the orchestration logic. One amendment to `pkg/api/types.go` adds the `Input` field to `Response`.

## Complexity Tracking

No constitutional violations. No complexity justifications needed.
