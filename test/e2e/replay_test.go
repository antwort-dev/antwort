//go:build e2e

package e2e

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// mockBackendBinary builds and returns the path to the mock-backend binary.
// The binary is cached in os.TempDir so it is only built once per test run.
func mockBackendBinary(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(os.TempDir(), "antwort-mock-backend-test")

	// Build the binary.
	cmd := exec.Command("go", "build", "-o", binary, "./cmd/mock-backend")
	cmd.Dir = projectRoot()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build mock-backend: %v\n%s", err, out)
	}
	return binary
}

// projectRoot returns the root of the antwort project.
func projectRoot() string {
	// test/e2e -> project root
	dir, _ := filepath.Abs(filepath.Join("..", ".."))
	return dir
}

// recordingsDir returns the path to the test recordings directory.
func recordingsDir() string {
	return filepath.Join(projectRoot(), "test", "e2e", "recordings")
}

// startMockBackend starts the mock-backend binary with the given flags and
// returns the base URL and a cleanup function. It waits for the /healthz
// endpoint to become available before returning.
func startMockBackend(t *testing.T, binary string, args ...string) (string, func()) {
	t.Helper()

	// Find a free port by binding to :0 temporarily.
	port := findFreePort(t)

	env := append(os.Environ(), fmt.Sprintf("MOCK_PORT=%d", port))
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("failed to start mock-backend: %v", err)
	}

	baseURL := fmt.Sprintf("http://localhost:%d", port)

	// Wait for the server to be ready.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/healthz")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}

	cleanup := func() {
		cancel()
		_ = cmd.Wait()
	}

	return baseURL, cleanup
}

// findFreePort returns an available TCP port.
func findFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

// --- T011: Replay backend verification tests ---

func TestReplayBackendNonStreaming(t *testing.T) {
	binary := mockBackendBinary(t)
	baseURL, cleanup := startMockBackend(t, binary, "--recordings-dir", recordingsDir())
	defer cleanup()

	// Send a request matching chat-basic.json.
	body := `{"model":"mock-model","messages":[{"role":"user","content":"hello"}],"stream":false}`
	resp, err := http.Post(baseURL+"/v1/chat/completions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, respBody)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("invalid response JSON: %v", err)
	}
	if result["id"] != "chatcmpl-mock" {
		t.Errorf("expected id chatcmpl-mock, got %v", result["id"])
	}
}

func TestReplayBackendStreaming(t *testing.T) {
	binary := mockBackendBinary(t)
	baseURL, cleanup := startMockBackend(t, binary, "--recordings-dir", recordingsDir())
	defer cleanup()

	// Send a streaming request matching chat-streaming.json.
	body := `{"model":"mock-model","messages":[{"role":"user","content":"hello"}],"stream":true}`
	resp, err := http.Post(baseURL+"/v1/chat/completions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, respBody)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Errorf("expected text/event-stream content type, got %s", ct)
	}

	// Read and verify SSE chunks.
	var lines []string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			lines = append(lines, line)
		}
	}

	if len(lines) == 0 {
		t.Fatal("expected SSE data lines, got none")
	}

	// Should contain role chunk and DONE sentinel.
	allData := strings.Join(lines, "\n")
	if !strings.Contains(allData, `"role":"assistant"`) {
		t.Error("streaming response should contain role chunk")
	}
	if !strings.Contains(allData, "data: [DONE]") {
		t.Error("streaming response should contain DONE sentinel")
	}
}

func TestReplayBackendMiss(t *testing.T) {
	binary := mockBackendBinary(t)
	baseURL, cleanup := startMockBackend(t, binary, "--recordings-dir", recordingsDir())
	defer cleanup()

	// Send a request that does not match any recording.
	body := `{"model":"unknown-model","messages":[{"role":"user","content":"this will not match"}],"stream":false}`
	resp, err := http.Post(baseURL+"/v1/chat/completions", "application/json", strings.NewReader(body))
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
		t.Errorf("expected 'no recording found' error, got %v", errResp["error"])
	}
	if _, ok := errResp["hash"]; !ok {
		t.Error("error response should include the request hash")
	}
	if _, ok := errResp["available"]; !ok {
		t.Error("error response should include available hashes for debugging")
	}
}

// --- T012: Responses API recording test ---

func TestReplayResponsesAPI(t *testing.T) {
	binary := mockBackendBinary(t)
	baseURL, cleanup := startMockBackend(t, binary, "--recordings-dir", recordingsDir())
	defer cleanup()

	// Send a Responses API request matching responses-api-basic.json.
	body := `{"model":"mock-model","input":[{"role":"user","content":"hello"}]}`
	resp, err := http.Post(baseURL+"/v1/responses", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, respBody)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("invalid response JSON: %v", err)
	}
	if result["object"] != "response" {
		t.Errorf("expected object 'response', got %v", result["object"])
	}
	if result["id"] != "resp_mock123456789012345678" {
		t.Errorf("expected id resp_mock123456789012345678, got %v", result["id"])
	}
	if result["status"] != "completed" {
		t.Errorf("expected status 'completed', got %v", result["status"])
	}
}

// --- T013: Record-if-missing mode test ---

func TestReplayRecordIfMissing(t *testing.T) {
	// 1. Create a temp directory for recordings.
	tempDir := t.TempDir()

	// 2. Copy one existing recording to the temp directory.
	srcPath := filepath.Join(recordingsDir(), "chat-basic.json")
	srcData, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("failed to read source recording: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "chat-basic.json"), srcData, 0644); err != nil {
		t.Fatalf("failed to copy recording: %v", err)
	}

	// 3. Start a simple test HTTP server that returns canned responses (the "real backend").
	cannedResponse := map[string]any{
		"id":     "chatcmpl-from-real-backend",
		"object": "chat.completion",
		"model":  "mock-model",
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": "Response from the real backend",
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     10,
			"completion_tokens": 8,
			"total_tokens":      18,
		},
	}
	realBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cannedResponse)
	}))
	defer realBackend.Close()

	// 4. Start mock-backend in record-if-missing mode.
	binary := mockBackendBinary(t)
	baseURL, cleanup := startMockBackend(t, binary,
		"--recordings-dir", tempDir,
		"--mode", "record-if-missing",
		"--record-target", realBackend.URL,
	)
	defer cleanup()

	// 5. Send the request matching the existing recording. It should be replayed (not forwarded).
	existingBody := `{"model":"mock-model","messages":[{"role":"user","content":"hello"}],"stream":false}`
	resp, err := http.Post(baseURL+"/v1/chat/completions", "application/json", strings.NewReader(existingBody))
	if err != nil {
		t.Fatalf("replay request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 for replayed request, got %d: %s", resp.StatusCode, respBody)
	}

	var replayResult map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&replayResult); err != nil {
		t.Fatalf("invalid replay response JSON: %v", err)
	}
	// The replayed response should have the recorded ID, not the real backend's.
	if replayResult["id"] != "chatcmpl-mock" {
		t.Errorf("expected replayed id chatcmpl-mock, got %v (request may have been forwarded instead)", replayResult["id"])
	}

	// 6. Send a new request that does not match any recording. It should be forwarded and recorded.
	newBody := `{"model":"mock-model","messages":[{"role":"user","content":"this is a new request"}],"stream":false}`
	resp2, err := http.Post(baseURL+"/v1/chat/completions", "application/json", strings.NewReader(newBody))
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp2.Body)
		t.Fatalf("expected 200 for forwarded request, got %d: %s", resp2.StatusCode, respBody)
	}

	var forwardResult map[string]any
	if err := json.NewDecoder(resp2.Body).Decode(&forwardResult); err != nil {
		t.Fatalf("invalid forwarded response JSON: %v", err)
	}
	// The forwarded response should come from the real backend.
	if forwardResult["id"] != "chatcmpl-from-real-backend" {
		t.Errorf("expected forwarded id chatcmpl-from-real-backend, got %v", forwardResult["id"])
	}

	// 7. Verify a new recording file was created in the temp directory.
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("failed to read temp dir: %v", err)
	}

	jsonFiles := 0
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			jsonFiles++
		}
	}
	// Should have the original (chat-basic.json) plus the newly recorded file.
	if jsonFiles < 2 {
		t.Errorf("expected at least 2 recording files (original + new), got %d", jsonFiles)
	}
}
