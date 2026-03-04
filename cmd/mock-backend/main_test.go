package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "sorts keys",
			input:    `{"b":2,"a":1}`,
			expected: `{"a":1,"b":2}`,
		},
		{
			name:     "removes stream_options",
			input:    `{"model":"m","stream_options":{"include_usage":true},"messages":[]}`,
			expected: `{"messages":[],"model":"m"}`,
		},
		{
			name:     "nested sorting",
			input:    `{"z":{"b":2,"a":1},"a":0}`,
			expected: `{"a":0,"z":{"a":1,"b":2}}`,
		},
		{
			name:     "arrays preserved",
			input:    `{"items":[3,1,2]}`,
			expected: `{"items":[3,1,2]}`,
		},
		{
			name:     "deterministic output",
			input:    `{"c":3,"b":2,"a":1}`,
			expected: `{"a":1,"b":2,"c":3}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeJSON([]byte(tc.input))
			if err != nil {
				t.Fatalf("normalizeJSON error: %v", err)
			}
			if string(got) != tc.expected {
				t.Errorf("got %s, want %s", got, tc.expected)
			}
		})
	}
}

func TestNormalizeJSONInvalid(t *testing.T) {
	_, err := normalizeJSON([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestComputeRequestHash(t *testing.T) {
	body := []byte(`{"model":"m","messages":[]}`)

	h1 := computeRequestHash("POST", "/v1/chat/completions", body)
	h2 := computeRequestHash("POST", "/v1/chat/completions", body)
	if h1 != h2 {
		t.Errorf("same input produced different hashes: %s vs %s", h1, h2)
	}

	// Different body should produce different hash.
	h3 := computeRequestHash("POST", "/v1/chat/completions", []byte(`{"model":"other"}`))
	if h1 == h3 {
		t.Error("different input produced same hash")
	}

	// Different method should produce different hash.
	h4 := computeRequestHash("GET", "/v1/chat/completions", body)
	if h1 == h4 {
		t.Error("different method produced same hash")
	}

	// Key order should not matter.
	bodyReordered := []byte(`{"messages":[],"model":"m"}`)
	h5 := computeRequestHash("POST", "/v1/chat/completions", bodyReordered)
	if h1 != h5 {
		t.Errorf("reordered keys produced different hash: %s vs %s", h1, h5)
	}
}

func TestLoadRecordings(t *testing.T) {
	dir := t.TempDir()

	rec := Recording{
		Request: RecordingRequest{
			Method: "POST",
			Path:   "/v1/chat/completions",
			Body:   json.RawMessage(`{"model":"test","messages":[]}`),
		},
		Response: RecordingResponse{
			Status:  200,
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    json.RawMessage(`{"id":"test"}`),
		},
		Streaming: false,
	}

	data, _ := json.Marshal(rec)
	if err := os.WriteFile(filepath.Join(dir, "test.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	recordings, err := loadRecordings(dir)
	if err != nil {
		t.Fatalf("loadRecordings error: %v", err)
	}
	if len(recordings) != 1 {
		t.Fatalf("expected 1 recording, got %d", len(recordings))
	}

	hash := computeRequestHash("POST", "/v1/chat/completions", []byte(`{"model":"test","messages":[]}`))
	if _, ok := recordings[hash]; !ok {
		t.Errorf("recording not indexed by expected hash %s", hash[:12])
	}
}

func TestLoadRecordingsCorruptFile(t *testing.T) {
	dir := t.TempDir()

	// Write a corrupt JSON file.
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("not json{"), 0644); err != nil {
		t.Fatal(err)
	}

	// Write a valid recording too.
	rec := Recording{
		Request: RecordingRequest{
			Method: "POST",
			Path:   "/v1/test",
			Body:   json.RawMessage(`{}`),
		},
		Response: RecordingResponse{Status: 200, Body: json.RawMessage(`{}`)},
	}
	data, _ := json.Marshal(rec)
	if err := os.WriteFile(filepath.Join(dir, "good.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	recordings, err := loadRecordings(dir)
	if err != nil {
		t.Fatalf("loadRecordings should not fail on corrupt files: %v", err)
	}
	if len(recordings) != 1 {
		t.Errorf("expected 1 valid recording (corrupt skipped), got %d", len(recordings))
	}
}

func TestReplayHandler(t *testing.T) {
	body := json.RawMessage(`{"model":"mock-model","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	respBody := json.RawMessage(`{"id":"chatcmpl-mock","object":"chat.completion","model":"mock-model","choices":[{"index":0,"message":{"role":"assistant","content":"Hello from mock!"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`)

	recordings := map[string]*Recording{}
	hash := computeRequestHash("POST", "/v1/chat/completions", body)
	recordings[hash] = &Recording{
		Request: RecordingRequest{
			Method: "POST",
			Path:   "/v1/chat/completions",
			Body:   body,
		},
		Response: RecordingResponse{
			Status:  200,
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    respBody,
		},
	}

	handler := makeReplayHandler(recordings, t.TempDir())
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/chat/completions", "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	got, _ := io.ReadAll(resp.Body)
	var result map[string]any
	if err := json.Unmarshal(got, &result); err != nil {
		t.Fatalf("invalid response JSON: %v", err)
	}
	if result["id"] != "chatcmpl-mock" {
		t.Errorf("unexpected response id: %v", result["id"])
	}
}

func TestReplayHandlerMiss(t *testing.T) {
	recordings := map[string]*Recording{}
	handler := makeReplayHandler(recordings, t.TempDir())
	srv := httptest.NewServer(handler)
	defer srv.Close()

	body := `{"model":"unknown","messages":[]}`
	resp, err := http.Post(srv.URL+"/v1/chat/completions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 500 {
		t.Errorf("expected 500 on miss, got %d", resp.StatusCode)
	}

	var errResp map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp["error"] != "no recording found" {
		t.Errorf("unexpected error message: %v", errResp["error"])
	}
	if _, ok := errResp["hash"]; !ok {
		t.Error("error response should include hash")
	}
}

func TestReplayStreaming(t *testing.T) {
	body := json.RawMessage(`{"model":"mock-model","messages":[{"role":"user","content":"hello"}],"stream":true}`)

	recordings := map[string]*Recording{}
	hash := computeRequestHash("POST", "/v1/chat/completions", body)
	recordings[hash] = &Recording{
		Request: RecordingRequest{
			Method: "POST",
			Path:   "/v1/chat/completions",
			Body:   body,
		},
		Response: RecordingResponse{
			Status:  200,
			Headers: map[string]string{"Content-Type": "text/event-stream"},
		},
		Streaming: true,
		Chunks: []string{
			"data: {\"id\":\"chatcmpl-mock-stream\",\"object\":\"chat.completion.chunk\",\"model\":\"mock-model\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\"},\"finish_reason\":null}]}\n\n",
			"data: {\"id\":\"chatcmpl-mock-stream\",\"object\":\"chat.completion.chunk\",\"model\":\"mock-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hello\"},\"finish_reason\":null}]}\n\n",
			"data: [DONE]\n\n",
		},
	}

	handler := makeReplayHandler(recordings, t.TempDir())
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/chat/completions", "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Errorf("expected text/event-stream, got %s", ct)
	}

	got, _ := io.ReadAll(resp.Body)
	body_str := string(got)
	if !strings.Contains(body_str, "data: [DONE]") {
		t.Error("streaming response should contain DONE sentinel")
	}
	if !strings.Contains(body_str, `"role":"assistant"`) {
		t.Error("streaming response should contain role chunk")
	}
}

func TestRecordMode(t *testing.T) {
	// Start a simple httptest.Server as the "real backend" that returns canned responses.
	cannedResponse := map[string]any{
		"id":     "chatcmpl-recorded",
		"object": "chat.completion",
		"model":  "mock-model",
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": "Recorded response from real backend",
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     10,
			"completion_tokens": 6,
			"total_tokens":      16,
		},
	}
	realBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cannedResponse)
	}))
	defer realBackend.Close()

	// Create a temp directory for recordings.
	tempDir := t.TempDir()

	// Set up record mode flags via package-level vars.
	oldMode := *mode
	oldTarget := *recordTarget
	*mode = "record"
	*recordTarget = realBackend.URL
	defer func() {
		*mode = oldMode
		*recordTarget = oldTarget
	}()

	// Create the replay handler with an empty recordings map.
	recordings := map[string]*Recording{}
	handler := makeReplayHandler(recordings, tempDir)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	// Send a request through the recording handler.
	body := `{"model":"mock-model","messages":[{"role":"user","content":"record me"}],"stream":false}`
	resp, err := http.Post(srv.URL+"/v1/chat/completions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		got, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, got)
	}

	// Verify the response came from the real backend.
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("invalid response JSON: %v", err)
	}
	if result["id"] != "chatcmpl-recorded" {
		t.Errorf("expected id chatcmpl-recorded, got %v", result["id"])
	}

	// Verify a recording JSON file was created in the temp directory.
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("failed to read temp dir: %v", err)
	}
	var jsonFiles []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			jsonFiles = append(jsonFiles, entry.Name())
		}
	}
	if len(jsonFiles) != 1 {
		t.Fatalf("expected 1 recording file, got %d", len(jsonFiles))
	}

	// Verify the recording file has correct format.
	data, err := os.ReadFile(filepath.Join(tempDir, jsonFiles[0]))
	if err != nil {
		t.Fatalf("failed to read recording file: %v", err)
	}
	var rec Recording
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("invalid recording JSON: %v", err)
	}
	if rec.Request.Method != "POST" {
		t.Errorf("expected request method POST, got %s", rec.Request.Method)
	}
	if rec.Request.Path != "/v1/chat/completions" {
		t.Errorf("expected request path /v1/chat/completions, got %s", rec.Request.Path)
	}
	if rec.Response.Status != 200 {
		t.Errorf("expected response status 200, got %d", rec.Response.Status)
	}
	if rec.Streaming {
		t.Error("expected non-streaming recording")
	}
}

func TestRecordIfMissingMode(t *testing.T) {
	// Start a real backend.
	realBackendHits := 0
	cannedResponse := map[string]any{
		"id":     "chatcmpl-from-real",
		"object": "chat.completion",
		"model":  "mock-model",
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": "From real backend",
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]any{
			"prompt_tokens": 10, "completion_tokens": 4, "total_tokens": 14,
		},
	}
	realBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		realBackendHits++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cannedResponse)
	}))
	defer realBackend.Close()

	// Create temp dir and copy an existing recording.
	tempDir := t.TempDir()
	existingBody := json.RawMessage(`{"model":"mock-model","messages":[{"role":"user","content":"existing"}],"stream":false}`)
	existingRec := Recording{
		Request: RecordingRequest{
			Method: "POST",
			Path:   "/v1/chat/completions",
			Body:   existingBody,
		},
		Response: RecordingResponse{
			Status:  200,
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    json.RawMessage(`{"id":"chatcmpl-existing","object":"chat.completion","model":"mock-model","choices":[{"index":0,"message":{"role":"assistant","content":"Existing response"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":3,"total_tokens":13}}`),
		},
	}
	existingData, _ := json.MarshalIndent(existingRec, "", "  ")
	if err := os.WriteFile(filepath.Join(tempDir, "existing.json"), existingData, 0644); err != nil {
		t.Fatal(err)
	}

	// Load recordings and set up handler.
	recordings, err := loadRecordings(tempDir)
	if err != nil {
		t.Fatalf("loadRecordings: %v", err)
	}
	if len(recordings) != 1 {
		t.Fatalf("expected 1 loaded recording, got %d", len(recordings))
	}

	oldMode := *mode
	oldTarget := *recordTarget
	*mode = "record-if-missing"
	*recordTarget = realBackend.URL
	defer func() {
		*mode = oldMode
		*recordTarget = oldTarget
	}()

	handler := makeReplayHandler(recordings, tempDir)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	// Send the matching request. It should be replayed (not forwarded).
	resp, err := http.Post(srv.URL+"/v1/chat/completions", "application/json", strings.NewReader(string(existingBody)))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var replayResult map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&replayResult); err != nil {
		t.Fatalf("invalid replay response: %v", err)
	}
	if replayResult["id"] != "chatcmpl-existing" {
		t.Errorf("expected replayed id chatcmpl-existing, got %v (was forwarded instead)", replayResult["id"])
	}
	if realBackendHits != 0 {
		t.Errorf("expected 0 real backend hits for existing recording, got %d", realBackendHits)
	}

	// Send a new request. It should be forwarded and recorded.
	newBody := `{"model":"mock-model","messages":[{"role":"user","content":"new request"}],"stream":false}`
	resp2, err := http.Post(srv.URL+"/v1/chat/completions", "application/json", strings.NewReader(newBody))
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	defer resp2.Body.Close()

	var forwardResult map[string]any
	if err := json.NewDecoder(resp2.Body).Decode(&forwardResult); err != nil {
		t.Fatalf("invalid forward response: %v", err)
	}
	if forwardResult["id"] != "chatcmpl-from-real" {
		t.Errorf("expected forwarded id chatcmpl-from-real, got %v", forwardResult["id"])
	}
	if realBackendHits != 1 {
		t.Errorf("expected 1 real backend hit for new request, got %d", realBackendHits)
	}

	// Verify a new recording file was saved.
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("failed to read temp dir: %v", err)
	}
	jsonCount := 0
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			jsonCount++
		}
	}
	if jsonCount < 2 {
		t.Errorf("expected at least 2 recording files (original + new), got %d", jsonCount)
	}
}

func TestBackwardCompatibility(t *testing.T) {
	// Without recordings-dir, the deterministic handler should work.
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/chat/completions", handleChatCompletions)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	body := `{"model":"mock-model","messages":[{"role":"user","content":"hello"}],"stream":false}`
	resp, err := http.Post(srv.URL+"/v1/chat/completions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result.Choices[0].Message.Content == nil || *result.Choices[0].Message.Content != "Hello, nice day!" {
		t.Errorf("unexpected response content: %v", result.Choices[0].Message.Content)
	}
}
