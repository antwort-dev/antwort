// Command mock-backend runs a deterministic Chat Completions server
// for conformance testing. It returns predictable responses based on
// request content analysis, matching the 6 official OpenResponses
// compliance test scenarios.
//
// Configuration:
//
//	MOCK_PORT - Listen port (default: 9090)
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	port := os.Getenv("MOCK_PORT")
	if port == "" {
		port = "9090"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/chat/completions", handleChatCompletions)
	mux.HandleFunc("GET /v1/models", handleModels)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok\n"))
	})

	srv := &http.Server{Addr: ":" + port, Handler: mux}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("mock backend starting", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("mock backend failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("mock backend shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
}

// --- Request types ---

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Tools    []any         `json:"tools,omitempty"`
	Stream   bool          `json:"stream"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// --- Response types ---

type chatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Model   string       `json:"model"`
	Choices []chatChoice `json:"choices"`
	Usage   chatUsage    `json:"usage"`
}

type chatChoice struct {
	Index        int         `json:"index"`
	Message      chatMsg     `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type chatMsg struct {
	Role      string     `json:"role"`
	Content   *string    `json:"content"`
	ToolCalls []toolCall `json:"tool_calls,omitempty"`
}

type toolCall struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Function funcCall `json:"function"`
}

type funcCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// --- Handler ---

func handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":{"message":"invalid request","type":"invalid_request_error"}}`, http.StatusBadRequest)
		return
	}

	if req.Stream {
		handleStreaming(w, &req)
		return
	}

	// Classify the request and generate appropriate response.
	resp := classifyAndRespond(&req)
	resp.Model = req.Model
	if resp.Model == "" {
		resp.Model = "mock-model"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func classifyAndRespond(req *chatRequest) chatResponse {
	// Check for tool calls first.
	if len(req.Tools) > 0 {
		return toolCallResponse(req)
	}

	// Check for image content.
	if hasImageContent(req) {
		return imageResponse(req)
	}

	// Check for system prompt (pirate).
	if hasSystemPrompt(req) {
		return systemPromptResponse(req)
	}

	// Default: basic text response.
	return basicTextResponse(req)
}

func basicTextResponse(req *chatRequest) chatResponse {
	text := "Hello, nice day!"

	// Check for specific prompts.
	lastMsg := getLastUserMessage(req)
	if strings.Contains(strings.ToLower(lastMsg), "count from 1 to 5") {
		text = "1, 2, 3, 4, 5"
	}

	return makeTextResponse(text)
}

func systemPromptResponse(req *chatRequest) chatResponse {
	return makeTextResponse("Ahoy there, matey! Welcome aboard!")
}

func toolCallResponse(req *chatRequest) chatResponse {
	content := (*string)(nil)
	return chatResponse{
		ID:     "chatcmpl-mock-tool",
		Object: "chat.completion",
		Choices: []chatChoice{
			{
				Index: 0,
				Message: chatMsg{
					Role:    "assistant",
					Content: content,
					ToolCalls: []toolCall{
						{
							ID:   "call_mock_1",
							Type: "function",
							Function: funcCall{
								Name:      "get_weather",
								Arguments: `{"location":"San Francisco","unit":"celsius"}`,
							},
						},
					},
				},
				FinishReason: "tool_calls",
			},
		},
		Usage: chatUsage{PromptTokens: 20, CompletionTokens: 15, TotalTokens: 35},
	}
}

func imageResponse(req *chatRequest) chatResponse {
	return makeTextResponse("I can see the image you shared. It appears to be a small red icon or symbol.")
}

func makeTextResponse(text string) chatResponse {
	return chatResponse{
		ID:     "chatcmpl-mock-text",
		Object: "chat.completion",
		Choices: []chatChoice{
			{
				Index: 0,
				Message: chatMsg{
					Role:    "assistant",
					Content: &text,
				},
				FinishReason: "stop",
			},
		},
		Usage: chatUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}
}

// --- Streaming ---

func handleStreaming(w http.ResponseWriter, req *chatRequest) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	model := req.Model
	if model == "" {
		model = "mock-model"
	}

	// Determine streaming content.
	tokens := []string{"Hello", ", ", "nice", " ", "day", "!"}
	lastMsg := getLastUserMessage(req)
	if strings.Contains(strings.ToLower(lastMsg), "count from 1 to 5") {
		tokens = []string{"1", ", ", "2", ", ", "3", ", ", "4", ", ", "5"}
	}

	// Send role chunk.
	writeSSEChunk(w, model, "", true, false)
	flusher.Flush()

	// Send content chunks.
	for _, token := range tokens {
		writeSSEChunk(w, model, token, false, false)
		flusher.Flush()
	}

	// Send finish chunk with usage.
	writeFinishChunk(w, model, len(tokens))
	flusher.Flush()

	// Send [DONE].
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func writeSSEChunk(w http.ResponseWriter, model, content string, isRole, isFinish bool) {
	chunk := map[string]any{
		"id":     "chatcmpl-mock-stream",
		"object": "chat.completion.chunk",
		"model":  model,
	}

	delta := map[string]any{}
	if isRole {
		delta["role"] = "assistant"
	}
	if content != "" {
		delta["content"] = content
	}

	choice := map[string]any{
		"index":         0,
		"delta":         delta,
		"finish_reason": nil,
	}

	chunk["choices"] = []any{choice}

	data, _ := json.Marshal(chunk)
	fmt.Fprintf(w, "data: %s\n\n", data)
}

func writeFinishChunk(w http.ResponseWriter, model string, tokenCount int) {
	chunk := map[string]any{
		"id":     "chatcmpl-mock-stream",
		"object": "chat.completion.chunk",
		"model":  model,
		"choices": []any{
			map[string]any{
				"index":         0,
				"delta":         map[string]any{},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     10,
			"completion_tokens": tokenCount,
			"total_tokens":      10 + tokenCount,
		},
	}

	data, _ := json.Marshal(chunk)
	fmt.Fprintf(w, "data: %s\n\n", data)
}

// --- Models endpoint ---

func handleModels(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{
		"object": "list",
		"data": []map[string]any{
			{"id": "mock-model", "object": "model", "owned_by": "antwort-mock"},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// --- Helpers ---

func getLastUserMessage(req *chatRequest) string {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			switch v := req.Messages[i].Content.(type) {
			case string:
				return v
			case []any:
				// Multimodal content array: find text part.
				for _, part := range v {
					if m, ok := part.(map[string]any); ok {
						if t, ok := m["type"].(string); ok && t == "input_text" {
							if text, ok := m["text"].(string); ok {
								return text
							}
						}
					}
				}
			}
		}
	}
	return ""
}

func hasImageContent(req *chatRequest) bool {
	for _, msg := range req.Messages {
		if msg.Role == "user" {
			if parts, ok := msg.Content.([]any); ok {
				for _, part := range parts {
					if m, ok := part.(map[string]any); ok {
						if t, ok := m["type"].(string); ok && (t == "input_image" || t == "image_url") {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

func hasSystemPrompt(req *chatRequest) bool {
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			return true
		}
	}
	return false
}
