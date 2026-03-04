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
