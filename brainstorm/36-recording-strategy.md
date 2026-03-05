# Brainstorm 36: Full E2E Recording Strategy

**Date**: 2026-03-05
**Participants**: Roland Huss
**Goal**: Define a comprehensive recording strategy that (a) reuses Llama Stack recordings where possible and (b) sets up a ROSA recording session for remaining scenarios.

## Current State

The E2E test infrastructure is operational:
- Replay backend with SHA256 hash matching (streaming + non-streaming)
- 7 recording files in `test/e2e/recordings/`
- 5 E2E tests passing, 4 skipping (auth not configured), 2 skipping (tool call recordings missing)
- E2E coverage: 15% of deployed binary code paths
- CI integration: lint-test job runs E2E tests with replay backend

## Recording Gap Analysis

### What we have recordings for

| Scenario | Recording | Source |
|----------|-----------|--------|
| Non-streaming Chat Completion | `97dc...json` | Recorded from antwort -> deterministic mock |
| Streaming Chat Completion | `a484...json` | Recorded from antwort -> deterministic mock |
| Handcrafted non-streaming | `chat-basic.json` | Manual (hash doesn't match antwort's format) |
| Handcrafted streaming | `chat-streaming.json` | Manual (hash doesn't match) |
| Handcrafted tool call | `chat-tool-call.json` | Manual (hash doesn't match) |
| Handcrafted tool result | `chat-tool-result.json` | Manual (hash doesn't match) |
| Responses API | `responses-api-basic.json` | Manual (for direct replay tests) |

### What we need recordings for

| Scenario | Why needed | Recording source |
|----------|-----------|-----------------|
| Tool call turn 1 (LLM requests function call) | TestE2EToolCallNonStreaming | Need antwort with tool executor |
| Tool call turn 2 (LLM final answer after tool result) | Same test, second turn | Same session |
| Streaming tool call turn 1 | TestE2EToolCallStreaming | Same setup, stream=true |
| Streaming tool call turn 2 | Same test, second turn | Same session |
| Reasoning response | Future: reasoning items in E2E | Model with reasoning support |
| Structured output (json_schema) | Future: structured output E2E | Model with constrained decoding |
| Multi-turn conversation | Future: conversation chaining E2E | Any model, 2+ turns |
| Embedding (for file search) | Future: RAG E2E | Embedding model |

## Strategy: Two-Phase Recording

### Phase 1: Local recording against deterministic mock

**What**: Record tool calling sequences against the deterministic mock-backend, which already returns proper `tool_calls` responses. This captures the exact request format antwort sends for multi-turn agentic scenarios.

**Requirement**: Antwort needs a tool executor configured. The mock-backend's `get_weather` tool call response needs a matching executor in antwort.

**Approach**: Create a dedicated recording script or test that:
1. Starts the deterministic mock on port A
2. Starts the recording proxy on port B, forwarding to port A
3. Starts antwort with a built-in function executor (get_weather) on port C, pointing at port B
4. Sends requests that trigger tool calling
5. Extracts recording files

**Challenge**: Antwort's `get_weather` executor is a mock in tests (`test/integration/helpers_test.go`), not a production binary feature. Options:
- Add a `--test-tools` flag to the server binary that registers mock tool executors
- Create a recording-specific Go test that wires everything up programmatically
- Use the MCP test server (`cmd/mcp-test-server`) which provides `get_time` and `echo` tools

**Recommended**: Use the MCP test server. It's a real binary, provides real tools, and exercises the MCP client path. The recording captures both the LLM interaction (Chat Completions) and the tool execution flow.

```bash
# Recording with MCP test server
PORT=9095 cmd/mcp-test-server &         # MCP server with get_time, echo
MOCK_PORT=9092 bin/mock-backend &        # deterministic LLM
MOCK_PORT=9091 bin/mock-backend \        # recording proxy
  --recordings-dir /tmp/tool-rec \
  --mode record \
  --record-target http://localhost:9092 &
ANTWORT_BACKEND_URL=http://localhost:9091 \
ANTWORT_MCP_SERVERS='[{"name":"test","transport":"sse","url":"http://localhost:9095"}]' \
bin/server &

# Trigger tool call
curl -X POST http://localhost:8080/v1/responses \
  -d '{"model":"mock-model","input":[{"role":"user","content":"what time is it?"}],"tools":[{"type":"function","function":{"name":"get_time"}}]}'
```

### Phase 2: ROSA recording session with real LLM

**What**: Record against a real LLM (Llama 3.2 on vLLM) for scenarios where the deterministic mock's responses are too simplistic. This produces recordings with real model behavior: varied token counts, natural language, realistic tool call argument formatting.

**When**: After Phase 1 is working. One-time recording session, results committed to repo.

**ROSA Setup**:

1. **Cluster**: Create a ROSA HCP cluster with GPU machinepool
   ```bash
   /rosa:create antwort-recording
   /rosa:gpu add antwort-recording
   ```

2. **Model**: Deploy vLLM with Llama 3.2 3B Instruct (smallest model that supports tool calling)
   ```bash
   /rosa:install model --cluster antwort-recording \
     --model meta-llama/Llama-3.2-3B-Instruct \
     --gpu-type g5.xlarge
   ```

3. **Recording infrastructure**: Deploy via kustomize overlay
   ```
   quickstarts/01-minimal/recording/
   ├── kustomization.yaml    # extends base
   ├── recording-proxy.yaml  # mock-backend in record mode
   └── vllm-backend.yaml     # points at vLLM service
   ```

   The recording proxy sits between antwort and vLLM:
   ```
   antwort -> recording-proxy (port 9090) -> vLLM (port 8000)
   ```

4. **Recording scenarios** (run each, extract recordings):

   | Scenario | Request | Expected model behavior |
   |----------|---------|------------------------|
   | Basic chat | Simple greeting | Short text response |
   | Streaming chat | Same, stream=true | Token-by-token response |
   | Tool call (function) | "What's the weather?" + get_weather tool | Model returns tool_calls |
   | Tool result | Second turn after tool execution | Model uses tool result in response |
   | Structured output | "Give me JSON" + response_format | Model follows schema |
   | Reasoning | "Think step by step" | reasoning_content in response |
   | Long response | "Write a haiku" | Multi-token completion |
   | Multi-turn | 2-3 turn conversation | Context-dependent responses |

5. **Extract recordings**:
   ```bash
   kubectl cp antwort/recording-proxy-xxx:/recordings ./real-recordings/
   cp real-recordings/*.json test/e2e/recordings/
   ```

6. **Tear down** (GPU nodes are expensive):
   ```bash
   /rosa:delete antwort-recording
   ```

**Cost estimate**: ~$5-10 for a 1-hour recording session on g5.xlarge.

## Reusing Llama Stack Recordings

### What's available

Llama Stack has 1,738 recordings covering:
- Chat Completions (streaming + non-streaming)
- Tool calling (function calls with schemas)
- Embeddings
- Multi-turn conversations
- Web search tool invocations (Tavily)

### What's reusable

| Llama Stack category | Reusable? | Why |
|---------------------|-----------|-----|
| Chat Completions (non-streaming) | Yes | Same protocol, just unwrap `__type__`/`__data__` |
| Chat Completions (streaming) | Yes | Reconstruct SSE chunks from chunk arrays |
| Tool calling (function calls) | Partially | Request body format differs (antwort adds `n`, `stream_options`) |
| Embeddings | No | Different endpoint, not used by antwort's vLLM provider |
| Ollama-native format | No | Different protocol entirely |
| Tavily/web search | No | Tool runtime, not LLM interaction |

### Conversion challenges

1. **Hash mismatch**: Llama Stack recordings store the request as the Python SDK sent it. Antwort's vLLM provider sends requests with different field ordering and additional fields (`n: 1`, `stream_options`). The request hash won't match.

2. **Solution**: The conversion script (`scripts/convert-llamastack-recordings.go`) should:
   - Extract the response body only
   - NOT try to match by hash
   - Instead, store converted recordings with descriptive filenames (`llama-chat-basic.json`)
   - Create a "fixture" mode where recordings are matched by request content (model + messages) rather than exact hash

3. **Alternative**: Use Llama Stack recordings as reference data (verify format compatibility) rather than direct replay. Record fresh against deterministic mock or real LLM for actual E2E replay.

### Recommendation

**Don't try to reuse Llama Stack recordings for replay.** The request format mismatch makes hash matching impossible without significant normalization changes. Instead:
- Use the conversion script to validate format compatibility
- Use converted responses as templates for handcrafting recordings
- Record fresh for all E2E replay scenarios

## Proposed Recording File Organization

```
test/e2e/recordings/
├── README.md
├── basic/
│   ├── chat-nonstreaming.json     # Recorded from antwort -> mock
│   └── chat-streaming.json        # Recorded from antwort -> mock
├── tools/
│   ├── tool-call-turn1.json       # Recorded from antwort -> mock (with MCP)
│   ├── tool-call-turn2.json       # Second turn after tool execution
│   ├── streaming-tool-turn1.json  # Streaming variant
│   └── streaming-tool-turn2.json  # Streaming second turn
├── advanced/                      # From ROSA recording session
│   ├── reasoning.json             # Real model reasoning
│   ├── structured-output.json     # Real model with json_schema
│   └── multi-turn.json            # Multi-turn conversation
└── llama-stack/                   # Converted for reference
    └── README.md                  # Notes on conversion
```

## Next Steps

1. **Now**: Fix tool call E2E tests to skip cleanly (done)
2. **Next session**: Run Phase 1 recording with MCP test server for tool call scenarios
3. **When needed**: ROSA recording session for real model behavior
4. **Ongoing**: Add recordings as new test scenarios are needed

## Open Questions

1. Should recordings be organized in subdirectories or flat? The hash-based naming makes subdirectories awkward (the loader reads all .json from a directory). Could add recursive loading.

2. Should we add a `make record` target that automates the Phase 1 recording setup? Would make it easy to regenerate recordings after protocol changes.

3. Should we track which recordings are "synthetic" (from deterministic mock) vs "real" (from actual LLM) via metadata? This helps understand test fidelity.
