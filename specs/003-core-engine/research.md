# Research: Core Engine & Provider Abstraction

**Feature**: 003-core-engine
**Date**: 2026-02-17

## Research Tasks

### RT-1: Provider Interface Design (Protocol-Agnostic)

**Decision**: Define the Provider interface with 5 methods: `Name()`, `Capabilities()`, `Complete()`, `Stream()`, `ListModels()`, and `Close()`. The interface operates on Antwort's own types, not on any backend protocol.

**Rationale**: The spec requires protocol-agnostic design (FR-002). A Chat Completions adapter and a Responses API proxy adapter must both implement the same interface. By defining operations in terms of `ProviderRequest`, `ProviderResponse`, and `ProviderEvent`, each adapter handles its own protocol translation internally.

**Alternatives considered**:

1. **Protocol-specific interface** (e.g., methods that accept Chat Completions types directly): Simpler for the first adapter, but locks out future adapters that use different protocols. Violates constitution principle VI.

2. **Generic interface with `any` types**: Maximum flexibility but loses type safety. The engine would need runtime type assertions, making errors harder to catch at compile time.

3. **Separate interfaces per operation** (e.g., `Completer`, `Streamer`, `ModelLister`): Follows Interface Segregation Principle but adds composition complexity. The Provider needs all these capabilities together; splitting them gains nothing for this domain.

### RT-2: Streaming Event Translation Architecture

**Decision**: The engine maps `ProviderEvent` to `api.StreamEvent`. The provider adapter returns `ProviderEvent` types on a channel. The engine consumes from the channel, generates synthetic lifecycle events, and writes `api.StreamEvent` values through the `ResponseWriter`.

**Rationale**: This keeps the provider layer independent of the transport layer (constitution: layer dependencies). The engine is the natural location for lifecycle event generation because it owns the Response object and its state machine.

**Alternatives considered**:

1. **Provider returns `api.StreamEvent` directly**: Simpler mapping, but creates a dependency from `pkg/provider` to `pkg/transport` (via `StreamEvent`). Violates the layer dependency constraint.

2. **Translator interface handles event mapping**: The brainstorm proposed `TranslateStreamEvent()` on the Translator interface. This was rejected because it couples provider adapters to transport types.

3. **Event bus / pubsub pattern**: Over-engineering for a single-producer, single-consumer channel pattern. Go channels are the natural fit.

### RT-3: Chat Completions SSE Parsing Strategy

**Decision**: Use `bufio.Scanner` with a custom split function to parse SSE lines from the HTTP response body. Each `data: {...}` line is parsed as JSON into internal Chat Completions chunk types. The `data: [DONE]` sentinel terminates the stream.

**Rationale**: Go's `bufio.Scanner` handles line-by-line reading efficiently. A custom split function handles the SSE line protocol (lines separated by `\n\n`, with `data:` prefix stripping). This avoids any external SSE library while keeping the parsing correct.

**Alternatives considered**:

1. **Third-party SSE library** (e.g., `r3labs/sse`): Adds an external dependency, violating constitution principle II.

2. **Raw `io.Reader` with manual line splitting**: Works but loses the ergonomics of `bufio.Scanner`. More error-prone buffer management.

3. **`net/http` streaming with `json.Decoder`**: Cannot handle the SSE `data:` prefix or the `[DONE]` sentinel. Requires preprocessing.

### RT-4: Tool Call Argument Buffering

**Decision**: Buffer incremental tool call arguments in a `map[int]*strings.Builder` keyed by tool call index. Each `delta.tool_calls[i].function.arguments` fragment is appended to the builder. When the stream indicates the tool call is complete (finish_reason or new tool call starting), emit `ProviderEventToolCallDone` with the fully assembled string.

**Rationale**: Chat Completions returns tool call arguments as incremental JSON fragments across multiple SSE chunks. The vLLM adapter must buffer these fragments because the engine expects complete arguments in the done event. A `strings.Builder` per tool call index is the simplest approach, with constant overhead per active tool call.

**Alternatives considered**:

1. **Pass fragments through to engine**: The engine would need to understand Chat Completions tool call fragment semantics. This leaks protocol details into the engine.

2. **Buffer in a single concatenated string**: Doesn't work with parallel tool calls (different indices interleaved in the same stream).

### RT-5: Conversation History Reconstruction

**Decision**: Follow `previous_response_id` links iteratively (not recursively) through the ResponseStore, collecting responses in a stack. Reverse to get chronological order. For each response, extract input Items (user turn) and output Items (assistant turn) and flatten to ProviderMessages. Track visited IDs to detect cycles.

**Rationale**: Iterative traversal with a visited-ID set prevents stack overflow on deep chains and catches cycles. The algorithm is O(n) in chain length with O(n) memory for the visited set and message array.

**Alternatives considered**:

1. **Recursive traversal**: Simpler code but risks stack overflow on deep chains (Go default stack is 1MB, grows to 1GB, but deep recursion is still undesirable).

2. **Pre-computed conversation view in storage**: Would require a different storage interface that returns conversation history directly. Optimizes read but couples the engine to a specific storage strategy. Better to keep reconstruction in the engine and optimize storage later.

### RT-6: Nil-Safe Composition Pattern

**Decision**: The Engine constructor accepts required dependencies (Provider) and optional dependencies (ResponseStore) as separate parameters. Required dependencies are validated; nil causes a constructor error. Optional dependencies are accepted as-is; nil means the feature is disabled.

**Rationale**: This pattern comes from openresponses-gw and aligns with constitution principle III. It enables the engine to operate in minimal mode (no store, no tools) or full mode (store + tools) without conditional compilation or feature flags.

**Alternatives considered**:

1. **Options pattern** (`func WithStore(s ResponseStore) EngineOption`): More flexible for many optional parameters but adds boilerplate. The engine currently has only 1-2 optional deps, making options premature.

2. **Builder pattern**: Even more boilerplate for the same outcome. Not justified for this complexity level.

3. **Feature flags**: A config struct with boolean fields like `EnableStorage`. Requires checking flags in multiple places and doesn't prevent nil pointer dereferences if the flag disagrees with the injected dependency.

### RT-7: Response Input Field Amendment

**Decision**: Add `Input []Item` field to `api.Response` type in `pkg/api/types.go`. The field is optional (uses `json:"input,omitempty"`).

**Rationale**: The OpenResponses specification includes an `input` field on the Response object. Conversation history reconstruction requires both input and output from prior responses. Without this field, the engine cannot reconstruct the user's messages from stored responses. This is a backwards-compatible change; existing code that doesn't set the field continues to work.

**Alternatives considered**:

1. **Separate InputStore interface**: Would require a new interface and storage implementation just for inputs. Adds interface complexity without benefit.

2. **Store full CreateResponseRequest**: Stores too much data (tools, config, etc.) that isn't needed for history reconstruction. The input Items are the minimal required data.
