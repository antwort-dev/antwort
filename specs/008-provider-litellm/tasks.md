# Tasks: LiteLLM Provider Adapter

**Input**: Design documents from `/specs/008-provider-litellm/`

## Format: `[ID] [P?] [Story] Description`

## Phase 1: User Story 1 - Extract Shared Base (Priority: P1) MVP

**Goal**: Extract OpenAI-compatible translation from vLLM into shared package.

**Independent Test**: All existing vLLM tests pass after extraction.

### Implementation for User Story 1

- [ ] T001 (antwort-zg5.1) [US1] Create `pkg/provider/openaicompat/` package. Move types.go (chatCompletionRequest/Response/Chunk types) from vLLM into openaicompat/types.go. Update vLLM to import from openaicompat. Verify vLLM tests pass (FR-001, FR-004).
- [ ] T002 (antwort-zg5.2) [US1] Move translate.go (translateToChat function) from vLLM into openaicompat/translate.go. Add ModelMapper and ExtraParams hooks. Update vLLM to call openaicompat. Verify tests pass (FR-002, FR-003).
- [ ] T003 (antwort-zg5.3) [US1] Move response.go (translateResponse, mapFinishReason) from vLLM into openaicompat/response.go. Add ExtractExtensions hook. Update vLLM. Verify tests pass (FR-002).
- [ ] T004 (antwort-zg5.4) [US1] Move stream.go (parseSSEStream, tool call buffering) from vLLM into openaicompat/stream.go. Update vLLM. Verify stream tests pass (FR-002).
- [ ] T005 (antwort-zg5.5) [US1] Move errors.go (mapHTTPError, mapNetworkError) from vLLM into openaicompat/errors.go. Update vLLM. Verify error tests pass (FR-002).
- [ ] T006 (antwort-zg5.6) [US1] Create openaicompat/client.go: Client struct with Complete, Stream, ListModels methods that use the shared translate/response/stream/errors. The vLLM adapter embeds this Client. Verify all vLLM tests pass unchanged (FR-001, FR-004).

**Checkpoint**: Shared base extracted. vLLM tests green. No behavior changes.

---

## Phase 2: User Story 2 + 3 - LiteLLM Adapter (Priority: P1)

**Goal**: LiteLLM adapter with model mapping, non-streaming, and streaming.

### Implementation for User Story 2 + 3

- [ ] T007 (antwort-5js.1) [US2] [P] Create `pkg/provider/litellm/config.go` with LiteLLMConfig (BaseURL, APIKey, Timeout, DefaultModel, ModelMapping).
- [ ] T008 (antwort-5js.2) [US2] Implement LiteLLM adapter in `pkg/provider/litellm/litellm.go`: embed openaicompat.Client with model mapping hook. Implement Provider interface (Name, Capabilities, Complete, Stream, ListModels, Close). Write tests with mock HTTP server in `pkg/provider/litellm/litellm_test.go` covering non-streaming, streaming, model mapping, and API key auth (FR-005, FR-006, FR-007, FR-008, FR-009).

**Checkpoint**: LiteLLM adapter works with mock server.

---

## Phase 3: User Story 4 - Extensions (Priority: P2)

**Goal**: LiteLLM-specific extensions (fallbacks, cost metadata).

### Implementation for User Story 4

- [ ] T009 (antwort-cq2.1) [US4] Implement LiteLLM extension handling in `pkg/provider/litellm/extensions.go`: ExtraParams hook that extracts `litellm:*` from request extensions and maps to extra_body. ExtractExtensions hook that reads cost/model info from response. Write tests (FR-010, FR-011).

**Checkpoint**: LiteLLM extensions flow through.

---

## Phase 4: Server Integration + Polish

**Goal**: Provider selection in server binary.

- [ ] T010 (antwort-0kt.1) Update `cmd/server/main.go`: add ANTWORT_PROVIDER env var ("vllm" default, "litellm"). Create appropriate provider based on selection. Add ANTWORT_LITELLM_MODEL_MAPPING env var for JSON model mapping (FR-012, FR-013).
- [ ] T011 (antwort-0kt.2) [P] Run `go vet ./...` and `go test ./...` to verify zero regressions.
- [ ] T012 (antwort-0kt.3) [P] Run `make conformance` to verify conformance score unchanged.

---

## Dependencies

- **Phase 1**: No dependencies. Start immediately.
- **Phase 2**: Depends on Phase 1 (shared base must exist).
- **Phase 3**: Depends on Phase 2 (adapter must exist).
- **Phase 4**: Depends on Phase 2.

## Implementation Strategy

### MVP: Phase 1 + 2

1. Extract shared base (zero regressions)
2. LiteLLM adapter works
3. **STOP**: Two providers validated

### Full: + Phase 3 + 4

4. Extensions support
5. Server provider selection
