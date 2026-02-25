# Brainstorm 27: API Gaps and OpenAI SDK Compatibility

## Source

Analysis based on "OpenAI Responses API - Strategy, Adoption, and Usage Analysis" (internal doc) which maps Llama Stack, vLLM, Ollama, and llama.cpp implementations against the full spec, plus framework usage data (Agents SDK, LangChain, CrewAI, etc.).

## Antwort's Position

Antwort is already the most feature-complete self-hosted implementation after Llama Stack. It exceeds vLLM and Ollama on every dimension. Key advantages:

- 33 SSE event types (more than any self-hosted impl except Llama Stack)
- Full stateful support (previous_response_id, PostgreSQL storage)
- Auth chain with JWT/API key
- MCP tool integration with OAuth
- code_interpreter with sandbox execution (unique, Llama Stack doesn't have this)
- parallel_tool_calls, max_tool_calls enforcement
- include response filtering, stream_options
- Reasoning streaming events

## Gaps to Close (Ranked by Impact)

### 1. List Responses Endpoint (HIGH)

`GET /v1/responses` with cursor pagination, model filter, ordering.

Every stateful deployment needs this. The Agents SDK and Codex CLI use it for conversation management. Simple CRUD on the storage layer.

### 2. Input Items Endpoint (HIGH)

`GET /v1/responses/{id}/input_items` with pagination.

Returns the input items of a stored response. Used by frameworks that want to inspect what went into a response without fetching the full response object.

### 3. Structured Output / text.format (HIGH)

`text.format: {"type": "json_schema", "json_schema": {...}}`

Enables constrained decoding: the model's output is guaranteed to match a JSON schema. vLLM supports this. Requires forwarding `response_format` to the Chat Completions backend and passing through the result.

Used by: LangChain, CrewAI (response_format), Haystack (text_format), Codex CLI (--response-schema).

### 4. Logprobs (MEDIUM)

When `include: ["message.output_text.logprobs"]` is set, return token-level probabilities. Requires:
- Forwarding `logprobs: true` and `top_logprobs: N` to the Chat Completions backend
- Including logprobs in OutputContentPart (already has the field, just not populated)

Used by: Llama Stack (tested), LangChain (config), Pydantic AI (config).

### 5. Conversations API (LOW)

`GET/POST /v1/conversations`, `GET /v1/conversations/{id}`.

Groups responses into named conversations. Llama Stack implements this but the doc notes: "previous_response_id covers the same use case more simply." Defer.

### 6. Guardrails (LOW for now)

Input/output safety validation. Llama Stack has this via Llama Guard integration. Not in scope for the gateway layer (should be handled by the provider or a dedicated guardrails service). The auth chain could be extended for this.

## OpenAI SDK as Integration Test Suite

### Why

The doc states: "If your /v1/responses endpoint works with `client.responses.create()`, it will work with 80%+ of the ecosystem."

Direct SDK usage dominates: 125M monthly downloads for openai SDK vs 35M for langchain-openai. Every framework ultimately calls the same API.

### How

A container-based test that uses the official OpenAI Python SDK against antwort + mock backend:

```python
from openai import OpenAI

client = OpenAI(base_url="http://localhost:8080/v1", api_key="test")

# Test 1: Basic create
response = client.responses.create(model="mock-model", input="Hello")

# Test 2: Streaming
for event in client.responses.create(model="mock-model", input="Hello", stream=True):
    print(event.type)

# Test 3: Tools
response = client.responses.create(
    model="mock-model",
    input="What's the weather?",
    tools=[{"type": "function", "name": "get_weather", ...}],
)

# Test 4: Multi-turn
response2 = client.responses.create(
    model="mock-model",
    input="Tell me more",
    previous_response_id=response.id,
)

# Test 5: Retrieve
stored = client.responses.retrieve(response.id)

# Test 6: Delete
client.responses.delete(response.id)
```

This validates:
- JSON serialization matches SDK expectations
- Field names, types, optionality correct
- SSE parsing works with SDK's stream handler
- Error responses match SDK error parsing
- Item type polymorphism (message, function_call, reasoning) deserializes correctly

### Implementation

Similar to the existing Zod conformance suite: a Containerfile that installs Python + openai SDK, runs the tests against antwort + mock-backend.

```
test/sdk-compat/
├── Containerfile
├── test_openai_sdk.py
└── run.sh
```

Add as `make sdk-test` target.

## Phasing

1. **Spec 028**: List responses + input items endpoints (2 new CRUD endpoints)
2. **Spec 029**: Structured output (text.format passthrough to provider)
3. **Spec 030**: OpenAI SDK compatibility tests (container-based)
4. **Spec 031**: Logprobs population in output content parts

## The Viability Bar

Per the document, a self-hosted implementation passes when:
1. `client.responses.create()` works (streaming + non-streaming) ✅
2. Multi-turn via previous_response_id works ✅
3. Agent frameworks work with function calling ✅
4. web_search and file_search work ✅
5. MCP tools work ✅
6. Stored responses retrievable via GET ✅ and deletable via DELETE ✅
7. 40+ integration tests pass ✅

Antwort already meets 7/7 criteria. The gaps above are about going from "passes the bar" to "best implementation."
