package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/config"
	"github.com/rhuss/antwort/pkg/engine"
	"github.com/rhuss/antwort/pkg/provider/vllm"
	"github.com/rhuss/antwort/pkg/storage/memory"
	"github.com/rhuss/antwort/pkg/tools"
	transporthttp "github.com/rhuss/antwort/pkg/transport/http"
)

// backgroundTestEnv creates a test environment with an integrated-mode
// background worker for testing async request processing.
type backgroundTestEnv struct {
	server  *httptest.Server
	backend *httptest.Server
	worker  *engine.Worker
	store   *memory.Store
}

func newBackgroundTestEnv(t *testing.T) *backgroundTestEnv {
	t.Helper()

	backend := startMockBackend()

	prov, err := vllm.New(vllm.Config{BaseURL: backend.URL})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	store := memory.New(100)

	eng, err := engine.New(prov, store, engine.Config{
		DefaultModel:    "mock-model",
		MaxAgenticTurns: 10,
		Executors:       []tools.ToolExecutor{&mockToolExecutor{}},
	})
	if err != nil {
		t.Fatalf("creating engine: %v", err)
	}

	adapter := transporthttp.NewAdapter(eng, store, transporthttp.DefaultConfig())

	// Create and register background worker (integrated mode).
	bgWorker := engine.NewWorker(eng, config.BackgroundConfig{
		PollInterval:      200 * time.Millisecond, // fast polling for tests
		DrainTimeout:      5 * time.Second,
		StalenessTimeout:  10 * time.Minute,
		HeartbeatInterval: 1 * time.Second,
		TTL:               1 * time.Hour,
		CleanupBatchSize:  10,
	})
	adapter.SetBackgroundCanceller(bgWorker)

	mux := http.NewServeMux()
	mux.Handle("/", adapter.Handler())

	server := httptest.NewServer(mux)

	env := &backgroundTestEnv{
		server:  server,
		backend: backend,
		worker:  bgWorker,
		store:   store,
	}

	// Start worker in background.
	go bgWorker.Start(t.Context())

	return env
}

func (e *backgroundTestEnv) close() {
	e.worker.Stop()
	e.server.Close()
	e.backend.Close()
}

func (e *backgroundTestEnv) url(path string) string {
	return e.server.URL + path
}

// waitForStatus polls until the response reaches the expected status or times out.
func (e *backgroundTestEnv) waitForStatus(t *testing.T, responseID string, want api.ResponseStatus, timeout time.Duration) api.Response {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp := getURL(t, e.url("/v1/responses/"+responseID))
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			time.Sleep(100 * time.Millisecond)
			continue
		}
		var r api.Response
		decodeJSON(t, resp, &r)
		if r.Status == want {
			return r
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("response %s did not reach status %q within %s", responseID, want, timeout)
	return api.Response{} // unreachable
}

// --- US1: Fire-and-Forget ---

func TestBackgroundSubmitAndPoll(t *testing.T) {
	env := newBackgroundTestEnv(t)
	defer env.close()

	// Submit a background request.
	reqBody := map[string]any{
		"model":      "mock-model",
		"background": true,
		"input": []map[string]any{
			{
				"type": "message",
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "Hello"},
				},
			},
		},
	}

	resp := postJSON(t, env.url("/v1/responses"), reqBody)
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var queued api.Response
	decodeJSON(t, resp, &queued)

	// Verify immediate response.
	if queued.Status != api.ResponseStatusQueued {
		t.Errorf("status = %q, want %q", queued.Status, api.ResponseStatusQueued)
	}
	if !queued.Background {
		t.Error("background = false, want true")
	}
	if len(queued.Output) != 0 {
		t.Errorf("output should be empty for queued response, got %d items", len(queued.Output))
	}

	// Poll until completed.
	completed := env.waitForStatus(t, queued.ID, api.ResponseStatusCompleted, 10*time.Second)

	// Verify completed response has output (functional equivalence).
	if len(completed.Output) == 0 {
		t.Fatal("completed response has no output")
	}
	if completed.Output[0].Type != api.ItemTypeMessage {
		t.Errorf("output[0].type = %q, want %q", completed.Output[0].Type, api.ItemTypeMessage)
	}
	if completed.Usage == nil {
		t.Error("completed response has no usage")
	}
	if !completed.Background {
		t.Error("completed background = false, want true")
	}
}

func TestBackgroundValidation(t *testing.T) {
	env := newBackgroundTestEnv(t)
	defer env.close()

	tests := []struct {
		name    string
		body    map[string]any
		wantMsg string
	}{
		{
			name: "background with store false",
			body: map[string]any{
				"model":      "mock-model",
				"background": true,
				"store":      false,
				"input":      []map[string]any{{"type": "message", "role": "user", "content": []map[string]any{{"type": "input_text", "text": "hi"}}}},
			},
			wantMsg: "background mode requires store",
		},
		{
			name: "background with stream true",
			body: map[string]any{
				"model":      "mock-model",
				"background": true,
				"stream":     true,
				"input":      []map[string]any{{"type": "message", "role": "user", "content": []map[string]any{{"type": "input_text", "text": "hi"}}}},
			},
			wantMsg: "background mode cannot be used with streaming",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := postJSON(t, env.url("/v1/responses"), tt.body)
			if resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				t.Fatal("expected error, got 200")
			}
			var errResp struct {
				Error api.APIError `json:"error"`
			}
			decodeJSON(t, resp, &errResp)
			if errResp.Error.Type != api.ErrorTypeInvalidRequest {
				t.Errorf("error type = %q, want %q", errResp.Error.Type, api.ErrorTypeInvalidRequest)
			}
		})
	}
}

// --- US3: Cancellation ---

func TestBackgroundCancelQueued(t *testing.T) {
	env := newBackgroundTestEnv(t)
	defer env.close()

	// Stop the worker so the request stays queued.
	env.worker.Stop()

	reqBody := map[string]any{
		"model":      "mock-model",
		"background": true,
		"input":      []map[string]any{{"type": "message", "role": "user", "content": []map[string]any{{"type": "input_text", "text": "Hello"}}}},
	}

	resp := postJSON(t, env.url("/v1/responses"), reqBody)
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var queued api.Response
	decodeJSON(t, resp, &queued)

	if queued.Status != api.ResponseStatusQueued {
		t.Fatalf("status = %q, want queued", queued.Status)
	}

	// Cancel it.
	delResp := deleteURL(t, env.url("/v1/responses/"+queued.ID))
	if delResp.StatusCode != http.StatusNoContent {
		body := readBody(t, delResp)
		t.Fatalf("DELETE: expected 204, got %d: %s", delResp.StatusCode, body)
	}
	delResp.Body.Close()

	// Verify it's cancelled.
	getResp := getURL(t, env.url("/v1/responses/"+queued.ID))
	var cancelled api.Response
	decodeJSON(t, getResp, &cancelled)

	if cancelled.Status != api.ResponseStatusCancelled {
		t.Errorf("status after cancel = %q, want %q", cancelled.Status, api.ResponseStatusCancelled)
	}
}

// --- US4: List Filtering ---

func TestBackgroundListFiltering(t *testing.T) {
	env := newBackgroundTestEnv(t)
	defer env.close()

	// Stop worker to keep background requests in queued state.
	env.worker.Stop()

	// Create a synchronous response.
	syncBody := map[string]any{
		"model": "mock-model",
		"input": []map[string]any{{"type": "message", "role": "user", "content": []map[string]any{{"type": "input_text", "text": "sync"}}}},
	}
	syncResp := postJSON(t, env.url("/v1/responses"), syncBody)
	syncResp.Body.Close()

	// Create a background response.
	bgBody := map[string]any{
		"model":      "mock-model",
		"background": true,
		"input":      []map[string]any{{"type": "message", "role": "user", "content": []map[string]any{{"type": "input_text", "text": "bg"}}}},
	}
	bgResp := postJSON(t, env.url("/v1/responses"), bgBody)
	bgResp.Body.Close()

	// Filter by background=true.
	listResp := getURL(t, env.url("/v1/responses?background=true"))
	var listResult struct {
		Data []*api.Response `json:"data"`
	}
	decodeJSON(t, listResp, &listResult)

	for _, r := range listResult.Data {
		if !r.Background {
			t.Errorf("response %s has background=false in background=true filter", r.ID)
		}
	}
	if len(listResult.Data) == 0 {
		t.Error("expected at least one background response")
	}

	// Filter by status=queued.
	queuedResp := getURL(t, env.url("/v1/responses?status=queued"))
	var queuedResult struct {
		Data []*api.Response `json:"data"`
	}
	decodeJSON(t, queuedResp, &queuedResult)

	for _, r := range queuedResult.Data {
		if r.Status != api.ResponseStatusQueued {
			t.Errorf("response %s has status=%q in status=queued filter", r.ID, r.Status)
		}
	}

	// Filter by background=false should not include the background response.
	nonBgResp := getURL(t, env.url("/v1/responses?background=false"))
	var nonBgResult struct {
		Data []*api.Response `json:"data"`
	}
	decodeJSON(t, nonBgResp, &nonBgResult)

	for _, r := range nonBgResult.Data {
		if r.Background {
			t.Errorf("response %s has background=true in background=false filter", r.ID)
		}
	}
}

// --- US5: Graceful Shutdown ---

func TestBackgroundGracefulDrain(t *testing.T) {
	env := newBackgroundTestEnv(t)
	// Don't defer env.close() since we manually stop the worker.

	// Submit a background request.
	reqBody := map[string]any{
		"model":      "mock-model",
		"background": true,
		"input":      []map[string]any{{"type": "message", "role": "user", "content": []map[string]any{{"type": "input_text", "text": "Hello"}}}},
	}

	resp := postJSON(t, env.url("/v1/responses"), reqBody)
	var queued api.Response
	decodeJSON(t, resp, &queued)

	// Wait for it to complete (worker should process it).
	env.waitForStatus(t, queued.ID, api.ResponseStatusCompleted, 10*time.Second)

	// Now stop the worker gracefully.
	env.worker.Stop()

	// The response should still be completed (not marked as failed by drain).
	getResp := getURL(t, env.url("/v1/responses/"+queued.ID))
	var final api.Response
	decodeJSON(t, getResp, &final)

	if final.Status != api.ResponseStatusCompleted {
		t.Errorf("status after drain = %q, want completed", final.Status)
	}

	env.server.Close()
	env.backend.Close()
}

// Ensure json import is used.
var _ = json.Marshal
var _ = fmt.Sprintf
