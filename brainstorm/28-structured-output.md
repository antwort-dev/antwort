# Brainstorm 28: Structured Output (text.format Passthrough)

## Problem

Antwort accepts `text.format` in requests and echoes it in responses, but never forwards it to the Chat Completions backend. This means constrained decoding (JSON mode, JSON schema mode) doesn't work. The model ignores the format constraint and returns free-form text.

This blocks:
- `client.responses.parse()` in the OpenAI Python SDK
- LangChain's `response_format` config
- CrewAI's structured output
- Haystack's `text_format` setting
- Codex CLI's `--response-schema` flag
- Pydantic AI's structured config

## Current State

The pipeline has 4 layers, and `text.format` flows through the first and last but skips the middle two:

```
Request (text.format) --> Engine (translate) --> Provider (ProviderRequest) --> Chat Completions (response_format)
         OK                   MISSING                 MISSING                        MISSING
```

The echo path works: `text.format` from the request is copied to the response via `getTextConfig()`. But the actual constraint is never sent to the model.

## Responses API vs Chat Completions API

The Responses API `text.format` maps to Chat Completions `response_format`:

| Responses API | Chat Completions API | Effect |
|---|---|---|
| `{"type": "text"}` | omit (default) | Free-form text output |
| `{"type": "json_object"}` | `{"type": "json_object"}` | JSON mode: model outputs valid JSON |
| `{"type": "json_schema", "json_schema": {...}}` | `{"type": "json_schema", "json_schema": {...}}` | Schema mode: output matches the schema |

The mapping is nearly 1:1 for `json_object` and `json_schema`. For `text`, the `response_format` field is simply omitted (it's the default).

## json_schema Mode

The `json_schema` format carries additional fields:

```json
{
  "type": "json_schema",
  "json_schema": {
    "name": "math_response",
    "strict": true,
    "schema": {
      "type": "object",
      "properties": {
        "answer": {"type": "number"},
        "reasoning": {"type": "string"}
      },
      "required": ["answer", "reasoning"],
      "additionalProperties": false
    }
  }
}
```

These fields must flow through to the provider unchanged:
- `name` (required): Schema identifier
- `strict` (optional, default false): Enables strict schema adherence
- `schema` (required): The JSON Schema object

## Design

### Approach: Passthrough

No validation of `text.format` values. The provider (vLLM, LiteLLM, OpenAI) validates schema correctness, supported types, and model capability. Antwort just passes it through, consistent with how it handles all other passthrough fields (temperature, top_p, etc.).

### Go Type Changes

Extend `TextFormat` to carry the full json_schema payload:

```go
// TextFormat specifies the output text format.
type TextFormat struct {
    Type       string          `json:"type"`
    Name       string          `json:"name,omitempty"`        // json_schema mode
    Strict     *bool           `json:"strict,omitempty"`      // json_schema mode
    Schema     json.RawMessage `json:"schema,omitempty"`      // json_schema mode
}
```

Using `json.RawMessage` for `Schema` avoids re-parsing the JSON schema object. It flows through as opaque bytes.

### Pipeline Changes

1. **ProviderRequest** (pkg/provider/types.go): Add `ResponseFormat *api.TextFormat` field
2. **translateRequest** (pkg/engine/translate.go): Forward `req.Text.Format` to `provReq.ResponseFormat` (skip if type is "text")
3. **ChatCompletionRequest** (pkg/provider/openaicompat/types.go): Add `ResponseFormat any` field
4. **TranslateToChat** (pkg/provider/openaicompat/translate.go): Map `ProviderRequest.ResponseFormat` to `ChatCompletionRequest.ResponseFormat`

### SDK parse() Compatibility

The OpenAI Python SDK's `client.responses.parse()` method:
1. Sends `text.format` with `json_schema` type
2. Receives the response with `output_text` content
3. Parses the text content against the provided schema
4. Returns a typed Python object

For this to work, Antwort needs:
- The model to actually respect `response_format` (provider forwarding)
- The output text to contain valid JSON matching the schema
- The response structure to be standard (already works)

No special server-side support needed for parse(). The SDK does the parsing client-side. The only requirement is that the format constraint reaches the model.

## Provider Considerations

### vLLM
- Supports `response_format` with `json_object` and `json_schema`
- Uses outlines or xgrammar for constrained decoding
- Schema validation happens server-side

### LiteLLM
- Forwards `response_format` to the underlying provider
- Provider support varies (OpenAI, Anthropic, etc.)
- Falls back to prompt-based JSON if provider doesn't support native constrained decoding

### Both
- Use the standard Chat Completions `response_format` field
- The openaicompat layer handles both

## What This Doesn't Cover

- **Logprobs**: Separate concern, separate spec
- **Refusal handling**: When structured output is refused (content policy), the response has a `refusal` field. Not in scope.
- **Schema caching**: Some providers cache compiled schemas. Transparent to the gateway.

## Complexity Assessment

Low. This is a 4-file passthrough change plus test coverage. No new endpoints, no new storage, no new dependencies. The hardest part is making sure the json_schema object flows through without mutation.

## Testing Strategy

1. Unit tests: TextFormat serialization round-trip (text, json_object, json_schema modes)
2. Integration test: Create response with `text.format: {type: "json_object"}`, verify mock backend receives `response_format`
3. Integration test: Create response with `text.format: {type: "json_schema", ...}`, verify schema flows through
4. SDK compat test (if added): `client.responses.parse()` returns a typed result

## Files to Change

```
pkg/api/types.go                         # Extend TextFormat struct
pkg/provider/types.go                    # Add ResponseFormat to ProviderRequest
pkg/engine/translate.go                  # Forward text.format
pkg/provider/openaicompat/types.go       # Add ResponseFormat to ChatCompletionRequest
pkg/provider/openaicompat/translate.go   # Map ResponseFormat
api/openapi.yaml                         # Update TextFormat schema
test/integration/responses_test.go       # Integration tests
```
