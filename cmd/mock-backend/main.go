// Command mock-backend runs a deterministic Chat Completions server
// for conformance testing. It returns predictable responses based on
// request content analysis, matching the 6 official OpenResponses
// compliance test scenarios.
//
// In replay mode (--recordings-dir), it serves pre-recorded responses
// matched by request hash. In record mode, it proxies to a real backend
// and saves the responses for later replay.
//
// Configuration:
//
//	MOCK_PORT - Listen port (default: 9090)
package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

var (
	recordingsDir = flag.String("recordings-dir", "", "Directory with recording JSON files (enables replay mode)")
	mode          = flag.String("mode", "replay", "Operating mode: replay, record, record-if-missing")
	recordTarget  = flag.String("record-target", "", "Backend URL for record mode")
)

func main() {
	flag.Parse()

	port := os.Getenv("MOCK_PORT")
	if port == "" {
		port = "9090"
	}

	mux := http.NewServeMux()

	if *recordingsDir != "" {
		recordings, err := loadRecordings(*recordingsDir)
		if err != nil {
			slog.Error("failed to load recordings", "dir", *recordingsDir, "error", err)
			os.Exit(1)
		}
		slog.Info("loaded recordings", "count", len(recordings), "dir", *recordingsDir, "mode", *mode)

		handler := makeReplayHandler(recordings, *recordingsDir)
		mux.HandleFunc("POST /v1/chat/completions", handler)
		mux.HandleFunc("POST /v1/responses", handler)
	} else {
		mux.HandleFunc("POST /v1/chat/completions", handleChatCompletions)
	}

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

// --- Recording types ---

// Recording represents a single captured request/response pair.
type Recording struct {
	Request   RecordingRequest  `json:"request"`
	Response  RecordingResponse `json:"response"`
	Streaming bool              `json:"streaming"`
	Chunks    []string          `json:"chunks,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// RecordingRequest captures the essentials of an HTTP request.
type RecordingRequest struct {
	Method string          `json:"method"`
	Path   string          `json:"path"`
	Body   json.RawMessage `json:"body"`
}

// RecordingResponse captures the essentials of an HTTP response.
type RecordingResponse struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    json.RawMessage   `json:"body,omitempty"`
}

// --- Normalization and hashing ---

// normalizeJSON recursively sorts JSON object keys and returns compact JSON.
// It removes the "stream_options" key to ensure consistent hashing.
func normalizeJSON(data []byte) ([]byte, error) {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	normalized := normalizeValue(raw)
	return json.Marshal(normalized)
}

func normalizeValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		sorted := make(map[string]any, len(val))
		keys := make([]string, 0, len(val))
		for k := range val {
			if k == "stream_options" {
				continue
			}
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sorted[k] = normalizeValue(val[k])
		}
		return sorted
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = normalizeValue(item)
		}
		return result
	default:
		return v
	}
}

// computeRequestHash returns SHA256 hex of "method\npath\nnormalized_body".
func computeRequestHash(method, path string, body []byte) string {
	normalized, err := normalizeJSON(body)
	if err != nil {
		normalized = body
	}
	input := method + "\n" + path + "\n" + string(normalized)
	sum := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", sum)
}

// --- Recording loader ---

// loadRecordings reads all .json files from dir and indexes by request hash.
func loadRecordings(dir string) (map[string]*Recording, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading recordings dir: %w", err)
	}

	recordings := make(map[string]*Recording)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("skipping recording file", "path", path, "error", err)
			continue
		}
		var rec Recording
		if err := json.Unmarshal(data, &rec); err != nil {
			slog.Warn("skipping corrupt recording", "path", path, "error", err)
			continue
		}
		hash := computeRequestHash(rec.Request.Method, rec.Request.Path, rec.Request.Body)
		recordings[hash] = &rec
		slog.Debug("loaded recording", "file", entry.Name(), "hash", hash[:12])
	}
	return recordings, nil
}

// --- Replay handler ---

func makeReplayHandler(recordings map[string]*Recording, recDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, `{"error":"failed to read request body"}`, http.StatusInternalServerError)
			return
		}

		hash := computeRequestHash(r.Method, r.URL.Path, body)

		rec, found := recordings[hash]

		// In record or record-if-missing mode, forward to backend if needed.
		if *mode == "record" || (*mode == "record-if-missing" && !found) {
			if *recordTarget == "" {
				http.Error(w, `{"error":"record-target not configured"}`, http.StatusInternalServerError)
				return
			}
			newRec, err := recordRequest(r.Method, r.URL.Path, body)
			if err != nil {
				slog.Error("recording failed", "error", err)
				http.Error(w, fmt.Sprintf(`{"error":"recording failed: %s"}`, err), http.StatusBadGateway)
				return
			}
			recordings[hash] = newRec
			rec = newRec
			found = true

			// Save to disk.
			if err := saveRecording(recDir, hash, newRec); err != nil {
				slog.Error("failed to save recording", "error", err)
			}
		}

		if !found {
			available := make([]string, 0, len(recordings))
			for h := range recordings {
				available = append(available, h[:12])
			}
			sort.Strings(available)
			errResp := map[string]any{
				"error":     "no recording found",
				"hash":      hash,
				"available": available,
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(errResp)
			return
		}

		// Serve the recording.
		if rec.Streaming {
			serveStreamingRecording(w, rec)
		} else {
			serveRecording(w, rec)
		}
	}
}

func serveRecording(w http.ResponseWriter, rec *Recording) {
	for k, v := range rec.Response.Headers {
		w.Header().Set(k, v)
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}
	status := rec.Response.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	w.Write(rec.Response.Body)
}

func serveStreamingRecording(w http.ResponseWriter, rec *Recording) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	for k, v := range rec.Response.Headers {
		w.Header().Set(k, v)
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "text/event-stream")
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	status := rec.Response.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)

	for _, chunk := range rec.Chunks {
		w.Write([]byte(chunk))
		flusher.Flush()
		time.Sleep(1 * time.Millisecond)
	}
}

// --- Record mode ---

func recordRequest(method, path string, body []byte) (*Recording, error) {
	targetURL := strings.TrimRight(*recordTarget, "/") + path

	req, err := http.NewRequest(method, targetURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("forwarding request: %w", err)
	}
	defer resp.Body.Close()

	headers := make(map[string]string)
	for k := range resp.Header {
		headers[k] = resp.Header.Get(k)
	}

	isStreaming := strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream")

	rec := &Recording{
		Request: RecordingRequest{
			Method: method,
			Path:   path,
			Body:   json.RawMessage(body),
		},
		Response: RecordingResponse{
			Status:  resp.StatusCode,
			Headers: headers,
		},
		Streaming: isStreaming,
		Metadata: map[string]string{
			"recorded_at": time.Now().UTC().Format(time.RFC3339),
		},
	}

	if isStreaming {
		scanner := bufio.NewScanner(resp.Body)
		var chunks []string
		var currentChunk string
		for scanner.Scan() {
			line := scanner.Text()
			currentChunk += line + "\n"
			if line == "" && currentChunk != "\n" {
				chunks = append(chunks, currentChunk)
				currentChunk = ""
			}
		}
		if currentChunk != "" {
			chunks = append(chunks, currentChunk)
		}
		rec.Chunks = chunks
	} else {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("reading response: %w", err)
		}
		rec.Response.Body = json.RawMessage(respBody)
	}

	return rec, nil
}

func saveRecording(dir, hash string, rec *Recording) error {
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, hash+".json")
	return os.WriteFile(path, data, 0644)
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
	Index        int     `json:"index"`
	Message      chatMsg `json:"message"`
	FinishReason string  `json:"finish_reason"`
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
