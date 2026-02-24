// Package integration provides integration tests for the antwort API.
//
// Tests run against a real antwort HTTP server backed by a mock LLM
// backend, both started in-process using net/http/httptest.
package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"context"

	"github.com/rhuss/antwort/pkg/engine"
	"github.com/rhuss/antwort/pkg/provider/vllm"
	"github.com/rhuss/antwort/pkg/storage/memory"
	"github.com/rhuss/antwort/pkg/tools"
	transporthttp "github.com/rhuss/antwort/pkg/transport/http"
)

// testEnv holds the shared servers for all integration tests.
var testEnv *TestEnvironment

// TestEnvironment holds the antwort server and mock backend for testing.
type TestEnvironment struct {
	AntwortServer *httptest.Server
	MockBackend   *httptest.Server
}

// TestMain starts the mock backend and antwort server before running tests.
func TestMain(m *testing.M) {
	testEnv = setupTestEnvironment()
	code := m.Run()
	testEnv.Teardown()
	os.Exit(code)
}

// setupTestEnvironment creates a mock LLM backend and an antwort server wired to it.
func setupTestEnvironment() *TestEnvironment {
	// Start mock backend.
	mockBackend := startMockBackend()

	// Create provider pointing to the mock backend.
	prov, err := vllm.New(vllm.Config{
		BaseURL: mockBackend.URL,
	})
	if err != nil {
		panic(fmt.Sprintf("creating provider: %v", err))
	}

	// Create in-memory store.
	store := memory.New(100)

	// Create mock tool executor for agentic loop testing.
	mockExecutor := &mockToolExecutor{}

	// Create engine with mock executor for tool lifecycle testing.
	eng, err := engine.New(prov, store, engine.Config{
		DefaultModel:    "mock-model",
		MaxAgenticTurns: 10,
		Executors:       []tools.ToolExecutor{mockExecutor},
	})
	if err != nil {
		panic(fmt.Sprintf("creating engine: %v", err))
	}

	// Create HTTP adapter.
	adapter := transporthttp.NewAdapter(eng, store, transporthttp.DefaultConfig())

	// Build mux matching production layout.
	mux := http.NewServeMux()
	mux.Handle("/", adapter.Handler())
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok\n"))
	})

	antwortServer := httptest.NewServer(mux)

	return &TestEnvironment{
		AntwortServer: antwortServer,
		MockBackend:   mockBackend,
	}
}

// Teardown stops both servers.
func (env *TestEnvironment) Teardown() {
	if env.AntwortServer != nil {
		env.AntwortServer.Close()
	}
	if env.MockBackend != nil {
		env.MockBackend.Close()
	}
}

// BaseURL returns the antwort server base URL.
func (env *TestEnvironment) BaseURL() string {
	return env.AntwortServer.URL
}

// --- HTTP helpers ---

// postJSON sends a POST request with JSON body and returns the response.
func postJSON(t *testing.T, url string, body any) *http.Response {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshaling request: %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

// getURL sends a GET request and returns the response.
func getURL(t *testing.T, url string) *http.Response {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return resp
}

// deleteURL sends a DELETE request and returns the response.
func deleteURL(t *testing.T, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		t.Fatalf("creating DELETE request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", url, err)
	}
	return resp
}

// readBody reads and returns the response body as a string.
func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}
	return string(body)
}

// decodeJSON reads the response body and decodes it into the target.
func decodeJSON(t *testing.T, resp *http.Response, target any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		t.Fatalf("decoding JSON: %v", err)
	}
}

// --- Mock backend ---

// startMockBackend creates an httptest server that mimics a Chat Completions API.
func startMockBackend() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /v1/chat/completions", handleMockChatCompletions)
	mux.HandleFunc("GET /v1/models", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data": []map[string]any{
				{"id": "mock-model", "object": "model", "owned_by": "test"},
			},
		})
	})

	return httptest.NewServer(mux)
}

// handleMockChatCompletions handles chat completion requests with deterministic responses.
func handleMockChatCompletions(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content any    `json:"content"`
		} `json:"messages"`
		Tools  []any `json:"tools"`
		Stream bool  `json:"stream"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":{"message":"invalid request","type":"invalid_request_error"}}`, http.StatusBadRequest)
		return
	}

	// Check trigger words in user messages.
	wantsReasoning := false
	wantsTruncate := false
	for _, msg := range req.Messages {
		if msg.Role == "user" {
			if s, ok := msg.Content.(string); ok {
				lower := strings.ToLower(s)
				if strings.Contains(lower, "reason") {
					wantsReasoning = true
				}
				if strings.Contains(lower, "truncate") {
					wantsTruncate = true
				}
			}
		}
	}

	if req.Stream {
		if wantsTruncate {
			handleMockStreamingTruncated(w, req.Model)
		} else if wantsReasoning {
			handleMockStreamingWithReasoning(w, req.Model)
		} else if len(req.Tools) > 0 {
			// Check if this is a tool result turn (tool role messages present).
			hasToolResults := false
			for _, msg := range req.Messages {
				if msg.Role == "tool" {
					hasToolResults = true
					break
				}
			}
			if hasToolResults {
				// Second turn: return text after tool results.
				handleMockStreamingToolResult(w, req.Model)
			} else {
				// First turn: return a tool call.
				handleMockStreamingToolCall(w, req.Model)
			}
		} else {
			handleMockStreaming(w, req.Model)
		}
		return
	}

	// Non-streaming truncation.
	if wantsTruncate {
		handleMockTruncatedResponse(w, req.Model)
		return
	}

	// Determine response text based on content.
	text := "Hello from mock!"
	for _, msg := range req.Messages {
		if msg.Role == "user" {
			if s, ok := msg.Content.(string); ok && strings.Contains(strings.ToLower(s), "count") {
				text = "1, 2, 3, 4, 5"
			}
		}
	}

	// Check for tool calls.
	if len(req.Tools) > 0 {
		handleMockToolCall(w, req.Model)
		return
	}

	// Non-streaming reasoning response.
	if wantsReasoning {
		handleMockReasoningResponse(w, req.Model)
		return
	}

	model := req.Model
	if model == "" {
		model = "mock-model"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"id":     "chatcmpl-mock",
		"object": "chat.completion",
		"model":  model,
		"choices": []map[string]any{
			{
				"index":         0,
				"message":       map[string]any{"role": "assistant", "content": text},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]any{
			"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15,
		},
	})
}

// handleMockToolCall responds with a tool call for get_weather.
func handleMockToolCall(w http.ResponseWriter, model string) {
	if model == "" {
		model = "mock-model"
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"id":     "chatcmpl-mock-tool",
		"object": "chat.completion",
		"model":  model,
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": nil,
					"tool_calls": []map[string]any{
						{
							"id":   "call_mock_1",
							"type": "function",
							"function": map[string]any{
								"name":      "get_weather",
								"arguments": `{"location":"San Francisco"}`,
							},
						},
					},
				},
				"finish_reason": "tool_calls",
			},
		},
		"usage": map[string]any{
			"prompt_tokens": 20, "completion_tokens": 15, "total_tokens": 35,
		},
	})
}

// handleMockStreaming sends SSE chunks for a streaming response.
func handleMockStreaming(w http.ResponseWriter, model string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	if model == "" {
		model = "mock-model"
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	tokens := []string{"Hello", " from", " mock", "!"}

	// Role chunk.
	writeChunk(w, model, "", true)
	flusher.Flush()

	// Content chunks.
	for _, token := range tokens {
		writeChunk(w, model, token, false)
		flusher.Flush()
	}

	// Finish chunk.
	finishData, _ := json.Marshal(map[string]any{
		"id": "chatcmpl-mock-stream", "object": "chat.completion.chunk", "model": model,
		"choices": []map[string]any{
			{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"},
		},
		"usage": map[string]any{
			"prompt_tokens": 10, "completion_tokens": len(tokens), "total_tokens": 10 + len(tokens),
		},
	})
	fmt.Fprintf(w, "data: %s\n\n", finishData)
	flusher.Flush()

	// Done sentinel.
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

// handleMockReasoningResponse returns a non-streaming response with reasoning_content.
// handleMockTruncatedResponse returns a non-streaming response with finish_reason=length.
// handleMockStreamingToolCall sends SSE chunks containing a tool call.
func handleMockStreamingToolCall(w http.ResponseWriter, model string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	if model == "" {
		model = "mock-model"
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Role chunk.
	writeChunk(w, model, "", true)
	flusher.Flush()

	// Tool call chunk.
	toolCallData, _ := json.Marshal(map[string]any{
		"id": "chatcmpl-mock-tc", "object": "chat.completion.chunk", "model": model,
		"choices": []map[string]any{
			{
				"index": 0,
				"delta": map[string]any{
					"tool_calls": []map[string]any{
						{
							"index": 0,
							"id":    "call_mock_1",
							"type":  "function",
							"function": map[string]any{
								"name":      "get_weather",
								"arguments": "",
							},
						},
					},
				},
				"finish_reason": nil,
			},
		},
	})
	fmt.Fprintf(w, "data: %s\n\n", toolCallData)
	flusher.Flush()

	// Tool call arguments chunk.
	argsData, _ := json.Marshal(map[string]any{
		"id": "chatcmpl-mock-tc", "object": "chat.completion.chunk", "model": model,
		"choices": []map[string]any{
			{
				"index": 0,
				"delta": map[string]any{
					"tool_calls": []map[string]any{
						{
							"index": 0,
							"function": map[string]any{
								"arguments": `{"location":"SF"}`,
							},
						},
					},
				},
				"finish_reason": nil,
			},
		},
	})
	fmt.Fprintf(w, "data: %s\n\n", argsData)
	flusher.Flush()

	// Finish with tool_calls.
	finishData, _ := json.Marshal(map[string]any{
		"id": "chatcmpl-mock-tc", "object": "chat.completion.chunk", "model": model,
		"choices": []map[string]any{
			{"index": 0, "delta": map[string]any{}, "finish_reason": "tool_calls"},
		},
		"usage": map[string]any{
			"prompt_tokens": 15, "completion_tokens": 10, "total_tokens": 25,
		},
	})
	fmt.Fprintf(w, "data: %s\n\n", finishData)
	flusher.Flush()

	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

// handleMockStreamingToolResult sends SSE chunks with a text answer (after tool execution).
func handleMockStreamingToolResult(w http.ResponseWriter, model string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	if model == "" {
		model = "mock-model"
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	writeChunk(w, model, "", true)
	flusher.Flush()

	writeChunk(w, model, "The weather is sunny, 22°C.", false)
	flusher.Flush()

	finishData, _ := json.Marshal(map[string]any{
		"id": "chatcmpl-mock-result", "object": "chat.completion.chunk", "model": model,
		"choices": []map[string]any{
			{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"},
		},
		"usage": map[string]any{
			"prompt_tokens": 25, "completion_tokens": 8, "total_tokens": 33,
		},
	})
	fmt.Fprintf(w, "data: %s\n\n", finishData)
	flusher.Flush()

	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func handleMockTruncatedResponse(w http.ResponseWriter, model string) {
	if model == "" {
		model = "mock-model"
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"id": "chatcmpl-mock-truncated", "object": "chat.completion", "model": model,
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": "This is a truncated resp",
				},
				"finish_reason": "length",
			},
		},
		"usage": map[string]any{
			"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15,
		},
	})
}

// handleMockStreamingTruncated sends SSE chunks with finish_reason=length.
func handleMockStreamingTruncated(w http.ResponseWriter, model string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	if model == "" {
		model = "mock-model"
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	writeChunk(w, model, "", true)
	flusher.Flush()

	tokens := []string{"This is", " truncated"}
	for _, token := range tokens {
		writeChunk(w, model, token, false)
		flusher.Flush()
	}

	// Finish with length (truncated).
	finishData, _ := json.Marshal(map[string]any{
		"id": "chatcmpl-mock-truncated", "object": "chat.completion.chunk", "model": model,
		"choices": []map[string]any{
			{"index": 0, "delta": map[string]any{}, "finish_reason": "length"},
		},
		"usage": map[string]any{
			"prompt_tokens": 10, "completion_tokens": 2, "total_tokens": 12,
		},
	})
	fmt.Fprintf(w, "data: %s\n\n", finishData)
	flusher.Flush()

	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func handleMockReasoningResponse(w http.ResponseWriter, model string) {
	if model == "" {
		model = "mock-model"
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"id": "chatcmpl-mock-reason", "object": "chat.completion", "model": model,
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]any{
					"role":              "assistant",
					"content":           "The answer is 42.",
					"reasoning_content": "Let me think step by step about this problem.",
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]any{
			"prompt_tokens": 10, "completion_tokens": 15, "total_tokens": 25,
		},
	})
}

// handleMockStreamingWithReasoning sends SSE chunks with reasoning_content then text content.
func handleMockStreamingWithReasoning(w http.ResponseWriter, model string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	if model == "" {
		model = "mock-model"
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Role chunk.
	writeChunk(w, model, "", true)
	flusher.Flush()

	// Reasoning chunks.
	reasoningTokens := []string{"Let me", " think", " about this."}
	for _, token := range reasoningTokens {
		writeReasoningChunk(w, model, token)
		flusher.Flush()
	}

	// Text content chunks (after reasoning).
	textTokens := []string{"The answer", " is 42."}
	for _, token := range textTokens {
		writeChunk(w, model, token, false)
		flusher.Flush()
	}

	// Finish chunk.
	finishData, _ := json.Marshal(map[string]any{
		"id": "chatcmpl-mock-reason-stream", "object": "chat.completion.chunk", "model": model,
		"choices": []map[string]any{
			{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"},
		},
		"usage": map[string]any{
			"prompt_tokens": 10, "completion_tokens": 8, "total_tokens": 18,
		},
	})
	fmt.Fprintf(w, "data: %s\n\n", finishData)
	flusher.Flush()

	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

// writeReasoningChunk writes a streaming chunk with reasoning_content.
func writeReasoningChunk(w http.ResponseWriter, model, reasoning string) {
	delta := map[string]any{
		"reasoning_content": reasoning,
	}
	data, _ := json.Marshal(map[string]any{
		"id": "chatcmpl-mock-reason-stream", "object": "chat.completion.chunk", "model": model,
		"choices": []map[string]any{
			{"index": 0, "delta": delta, "finish_reason": nil},
		},
	})
	fmt.Fprintf(w, "data: %s\n\n", data)
}

func writeChunk(w http.ResponseWriter, model, content string, isRole bool) {
	delta := map[string]any{}
	if isRole {
		delta["role"] = "assistant"
	}
	if content != "" {
		delta["content"] = content
	}

	data, _ := json.Marshal(map[string]any{
		"id": "chatcmpl-mock-stream", "object": "chat.completion.chunk", "model": model,
		"choices": []map[string]any{
			{"index": 0, "delta": delta, "finish_reason": nil},
		},
	})
	fmt.Fprintf(w, "data: %s\n\n", data)
}

// --- Mock tool executor ---

// mockToolExecutor handles get_weather tool calls for testing.
type mockToolExecutor struct{}

func (m *mockToolExecutor) Kind() tools.ToolKind {
	return tools.ToolKindBuiltin
}

func (m *mockToolExecutor) CanExecute(toolName string) bool {
	return toolName == "get_weather"
}

func (m *mockToolExecutor) Execute(_ context.Context, call tools.ToolCall) (*tools.ToolResult, error) {
	return &tools.ToolResult{
		CallID: call.ID,
		Output: `{"temperature": "22°C", "condition": "sunny"}`,
	}, nil
}
