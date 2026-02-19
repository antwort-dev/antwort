# Research: LiteLLM Provider Adapter

**Feature**: 008-provider-litellm
**Date**: 2026-02-19

## R1: Shared Base Package Location

**Decision**: `pkg/provider/openaicompat/` as a new package alongside vLLM and LiteLLM.

**Rationale**: The shared base is provider-agnostic OpenAI-compatible logic. Putting it in its own package under `provider/` makes the dependency clear: both `vllm/` and `litellm/` import `openaicompat/`. It's not a "provider" itself (no Provider interface implementation), just reusable translation logic.

**Alternatives**: Putting it in `provider/` directly (pollutes the interface package), or in `vllm/` (wrong ownership).

## R2: Extraction Strategy

**Decision**: Move, don't copy. Extract types.go, translate.go, stream.go, response.go, errors.go from `pkg/provider/vllm/` into `pkg/provider/openaicompat/`. The vLLM adapter becomes a thin wrapper that creates an openaicompat.Client with its config.

**Rationale**: The vLLM adapter currently has ~600 lines of translation logic that is 100% OpenAI Chat Completions format. Moving it (not copying) ensures a single source of truth.

## R3: Customization Points

**Decision**: The shared Client accepts functional options for customization:
- `ModelMapper func(string) string` - transforms model names before sending
- `ExtraParams func(*api.CreateResponseRequest) map[string]any` - adds provider-specific body params
- `ExtractExtensions func(*http.Response) map[string]json.RawMessage` - extracts provider-specific response data

**Rationale**: These three hooks cover all known differences between vLLM and LiteLLM without adding provider-specific code to the shared base.

## R4: Server Provider Selection

**Decision**: `ANTWORT_PROVIDER` env var with values "vllm" (default) and "litellm". When not set, defaults to "vllm" for backward compatibility. The backend URL is still `ANTWORT_BACKEND_URL` regardless of provider.

**Rationale**: Simple, backward-compatible. The provider selection is a deployment-time decision, not per-request.
