package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/transport"
)

// mockCreator is a configurable mock ResponseCreator for testing.
type mockCreator struct {
	response *api.Response
	err      error
	events   []api.StreamEvent
}

func (m *mockCreator) CreateResponse(ctx context.Context, req *api.CreateResponseRequest, w transport.ResponseWriter) error {
	if m.err != nil {
		return m.err
	}
	if len(m.events) > 0 {
		for _, event := range m.events {
			if err := w.WriteEvent(ctx, event); err != nil {
				return err
			}
		}
		return nil
	}
	if m.response != nil {
		return w.WriteResponse(ctx, m.response)
	}
	return nil
}

// mockStore is a configurable mock ResponseStore for testing.
type mockStore struct {
	responses map[string]*api.Response
}

func (m *mockStore) SaveResponse(_ context.Context, resp *api.Response) error {
	if m.responses == nil {
		m.responses = make(map[string]*api.Response)
	}
	m.responses[resp.ID] = resp
	return nil
}

func (m *mockStore) GetResponse(_ context.Context, id string) (*api.Response, error) {
	resp, ok := m.responses[id]
	if !ok {
		return nil, api.NewNotFoundError("response not found: " + id)
	}
	return resp, nil
}

func (m *mockStore) GetResponseForChain(_ context.Context, id string) (*api.Response, error) {
	resp, ok := m.responses[id]
	if !ok {
		return nil, api.NewNotFoundError("response not found: " + id)
	}
	return resp, nil
}

func (m *mockStore) DeleteResponse(_ context.Context, id string) error {
	if _, ok := m.responses[id]; !ok {
		return api.NewNotFoundError("response not found: " + id)
	}
	delete(m.responses, id)
	return nil
}

func (m *mockStore) HealthCheck(_ context.Context) error { return nil }
func (m *mockStore) Close() error                        { return nil }

func newTestAdapter(creator transport.ResponseCreator, store transport.ResponseStore) *Adapter {
	return NewAdapter(creator, store, DefaultConfig())
}

func postJSON(t *testing.T, srv *httptest.Server, body any) *http.Response {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	resp, err := http.Post(srv.URL+"/v1/responses", "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("POST error: %v", err)
	}
	return resp
}

// --- Non-streaming tests (US1) ---

func TestNonStreamingPostReturnsJSON(t *testing.T) {
	creator := &mockCreator{
		response: &api.Response{
			ID:     "resp_testABC12345678901234567",
			Object: "response",
			Status: api.ResponseStatusCompleted,
			Model:  "test-model",
		},
	}

	adapter := newTestAdapter(creator, nil)
	srv := httptest.NewServer(adapter.Handler())
	defer srv.Close()

	req := api.CreateResponseRequest{
		Model: "test-model",
		Input: []api.Item{{Type: api.ItemTypeMessage}},
	}
	resp := postJSON(t, srv, req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	var got api.Response
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if got.ID != "resp_testABC12345678901234567" {
		t.Errorf("response ID = %q, want %q", got.ID, "resp_testABC12345678901234567")
	}
}

func TestInvalidJSONBodyReturns400(t *testing.T) {
	adapter := newTestAdapter(&mockCreator{}, nil)
	srv := httptest.NewServer(adapter.Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/responses", "application/json", strings.NewReader("{invalid"))
	if err != nil {
		t.Fatalf("POST error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var errResp api.ErrorResponse
	json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp.Error.Type != api.ErrorTypeInvalidRequest {
		t.Errorf("error type = %q, want %q", errResp.Error.Type, api.ErrorTypeInvalidRequest)
	}
}

func TestOversizedBodyReturns413(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxBodySize = 10 // 10 bytes max
	adapter := NewAdapter(&mockCreator{}, nil, cfg)
	srv := httptest.NewServer(adapter.Handler())
	defer srv.Close()

	bigBody := strings.NewReader(`{"model":"test","input":[{"type":"message"}]}`)
	resp, err := http.Post(srv.URL+"/v1/responses", "application/json", bigBody)
	if err != nil {
		t.Fatalf("POST error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusRequestEntityTooLarge)
	}
}

func TestWrongContentTypeReturns415(t *testing.T) {
	adapter := newTestAdapter(&mockCreator{}, nil)
	srv := httptest.NewServer(adapter.Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/responses", "text/plain", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnsupportedMediaType)
	}
}

func TestUnknownPathReturns404(t *testing.T) {
	adapter := newTestAdapter(&mockCreator{}, nil)
	srv := httptest.NewServer(adapter.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/nonexistent")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestHandlerErrorMapping(t *testing.T) {
	tests := []struct {
		name       string
		err        *api.APIError
		wantStatus int
	}{
		{"invalid_request -> 400", api.NewInvalidRequestError("model", "required"), http.StatusBadRequest},
		{"not_found -> 404", api.NewNotFoundError("not found"), http.StatusNotFound},
		{"too_many_requests -> 429", api.NewTooManyRequestsError("rate limit"), http.StatusTooManyRequests},
		{"server_error -> 500", api.NewServerError("internal"), http.StatusInternalServerError},
		{"model_error -> 500", api.NewModelError("overloaded"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creator := &mockCreator{err: tt.err}
			adapter := newTestAdapter(creator, nil)
			srv := httptest.NewServer(adapter.Handler())
			defer srv.Close()

			req := api.CreateResponseRequest{Model: "test"}
			resp := postJSON(t, srv, req)
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}

			var errResp api.ErrorResponse
			json.NewDecoder(resp.Body).Decode(&errResp)
			if errResp.Error.Type != tt.err.Type {
				t.Errorf("error type = %q, want %q", errResp.Error.Type, tt.err.Type)
			}
		})
	}
}

func TestGetWithoutStoreReturnsError(t *testing.T) {
	adapter := newTestAdapter(&mockCreator{}, nil) // no store
	srv := httptest.NewServer(adapter.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/responses/resp_abc123456789012345678901")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotImplemented {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotImplemented)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	adapter := newTestAdapter(&mockCreator{}, nil)
	srv := httptest.NewServer(adapter.Handler())
	defer srv.Close()

	req, _ := http.NewRequest("PUT", srv.URL+"/v1/responses", strings.NewReader("{}"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}
}

// --- Streaming tests (US2) ---

func TestStreamingPostReturnsSSE(t *testing.T) {
	creator := &mockCreator{
		events: []api.StreamEvent{
			{Type: api.EventResponseCreated, SequenceNumber: 0, Response: &api.Response{ID: "resp_streamABCDE2345678901230", Status: api.ResponseStatusInProgress}},
			{Type: api.EventOutputTextDelta, SequenceNumber: 1, Delta: "Hello"},
			{Type: api.EventOutputTextDelta, SequenceNumber: 2, Delta: " world"},
			{Type: api.EventResponseCompleted, SequenceNumber: 3, Response: &api.Response{ID: "resp_streamABCDE2345678901230", Status: api.ResponseStatusCompleted}},
		},
	}

	adapter := newTestAdapter(creator, nil)
	srv := httptest.NewServer(adapter.Handler())
	defer srv.Close()

	reqBody := api.CreateResponseRequest{Model: "test", Input: []api.Item{{Type: api.ItemTypeMessage}}, Stream: true}
	resp := postJSON(t, srv, reqBody)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want %q", ct, "text/event-stream")
	}

	// Read full body and check SSE format.
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	body := buf.String()

	if !strings.Contains(body, "event: response.created\n") {
		t.Error("missing response.created event")
	}
	if !strings.Contains(body, "event: response.output_text.delta\n") {
		t.Error("missing output_text.delta event")
	}
	if !strings.Contains(body, "event: response.completed\n") {
		t.Error("missing response.completed event")
	}
	if !strings.Contains(body, "data: [DONE]\n") {
		t.Error("missing [DONE] sentinel")
	}
}

func TestStreamingErrorBeforeEventsReturnsJSON(t *testing.T) {
	creator := &mockCreator{
		err: api.NewInvalidRequestError("model", "required"),
	}

	adapter := newTestAdapter(creator, nil)
	srv := httptest.NewServer(adapter.Handler())
	defer srv.Close()

	reqBody := api.CreateResponseRequest{Model: "", Stream: true, Input: []api.Item{{Type: api.ItemTypeMessage}}}
	resp := postJSON(t, srv, reqBody)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	// Should be JSON, not SSE.
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestStreamingInFlightRegistration(t *testing.T) {
	// Verify that the in-flight registry is populated during streaming
	// and cleaned up after completion.
	creator := &mockCreator{
		events: []api.StreamEvent{
			{Type: api.EventResponseCreated, SequenceNumber: 0, Response: &api.Response{ID: "resp_inflightABCD567890123450", Status: api.ResponseStatusInProgress, Output: []api.Item{}, Tools: []api.ToolDefinition{}, Metadata: map[string]any{}}},
			{Type: api.EventResponseCompleted, SequenceNumber: 1, Response: &api.Response{ID: "resp_inflightABCD567890123450", Status: api.ResponseStatusCompleted, Output: []api.Item{}, Tools: []api.ToolDefinition{}, Metadata: map[string]any{}}},
		},
	}

	adapter := newTestAdapter(creator, nil)
	srv := httptest.NewServer(adapter.Handler())
	defer srv.Close()

	reqBody := api.CreateResponseRequest{Model: "test", Stream: true, Input: []api.Item{{Type: api.ItemTypeMessage}}}
	resp := postJSON(t, srv, reqBody)
	defer resp.Body.Close()

	// After streaming completes, the in-flight entry should be cleaned up.
	// We verify this by checking that Cancel returns false.
	ok := adapter.inflight.Cancel("resp_inflightABCD567890123450")
	if ok {
		t.Error("in-flight entry should have been cleaned up after streaming completed")
	}
}

func TestStreamingExplicitCancellation(t *testing.T) {
	// Test that DELETE cancels an in-flight streaming response via the registry.
	handlerStarted := make(chan struct{})
	handlerDone := make(chan struct{})

	creator := transport.ResponseCreatorFunc(func(ctx context.Context, req *api.CreateResponseRequest, w transport.ResponseWriter) error {
		w.WriteEvent(ctx, api.StreamEvent{
			Type:     api.EventResponseCreated,
			Response: &api.Response{ID: "resp_canceltestABC34567890100", Status: api.ResponseStatusInProgress},
		})
		close(handlerStarted)

		// Wait for cancellation or timeout.
		select {
		case <-ctx.Done():
			// Cancelled. Send cancelled event.
			w.WriteEvent(context.Background(), api.StreamEvent{
				Type:     api.EventResponseCancelled,
				Response: &api.Response{ID: "resp_canceltestABC34567890100", Status: api.ResponseStatusCancelled},
			})
		case <-time.After(10 * time.Second):
			t.Error("handler was not cancelled within timeout")
		}
		close(handlerDone)
		return nil
	})

	adapter := newTestAdapter(creator, nil)
	srv := httptest.NewServer(adapter.Handler())
	defer srv.Close()

	// Start streaming request in background.
	go func() {
		reqBody, _ := json.Marshal(api.CreateResponseRequest{Model: "test", Stream: true, Input: []api.Item{{Type: api.ItemTypeMessage}}})
		resp, err := http.Post(srv.URL+"/v1/responses", "application/json", bytes.NewReader(reqBody))
		if err != nil {
			return
		}
		defer resp.Body.Close()
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
	}()

	// Wait for handler to start.
	<-handlerStarted

	// Send DELETE to cancel.
	req, _ := http.NewRequest("DELETE", srv.URL+"/v1/responses/resp_canceltestABC34567890100", nil)
	deleteResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE error: %v", err)
	}
	defer deleteResp.Body.Close()

	if deleteResp.StatusCode != http.StatusNoContent {
		t.Errorf("DELETE status = %d, want %d", deleteResp.StatusCode, http.StatusNoContent)
	}

	// Handler should complete after cancellation.
	select {
	case <-handlerDone:
		// Success.
	case <-time.After(5 * time.Second):
		t.Error("handler did not complete after cancellation")
	}
}

// --- GET/DELETE tests (US3) ---

func TestGetReturnsStoredResponse(t *testing.T) {
	store := &mockStore{
		responses: map[string]*api.Response{
			"resp_abc123456789012345678901": {
				ID:     "resp_abc123456789012345678901",
				Object: "response",
				Status: api.ResponseStatusCompleted,
				Model:  "test-model",
			},
		},
	}

	adapter := newTestAdapter(&mockCreator{}, store)
	srv := httptest.NewServer(adapter.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/responses/resp_abc123456789012345678901")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var got api.Response
	json.NewDecoder(resp.Body).Decode(&got)
	if got.ID != "resp_abc123456789012345678901" {
		t.Errorf("response ID = %q, want %q", got.ID, "resp_abc123456789012345678901")
	}
}

func TestGetUnknownIDReturns404(t *testing.T) {
	store := &mockStore{responses: map[string]*api.Response{}}
	adapter := newTestAdapter(&mockCreator{}, store)
	srv := httptest.NewServer(adapter.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/responses/resp_unknown12345678901234567")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestGetMalformedIDReturns400(t *testing.T) {
	store := &mockStore{responses: map[string]*api.Response{}}
	adapter := newTestAdapter(&mockCreator{}, store)
	srv := httptest.NewServer(adapter.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/responses/bad-id")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestDeleteReturns204(t *testing.T) {
	store := &mockStore{
		responses: map[string]*api.Response{
			"resp_abc123456789012345678901": {ID: "resp_abc123456789012345678901"},
		},
	}

	adapter := newTestAdapter(&mockCreator{}, store)
	srv := httptest.NewServer(adapter.Handler())
	defer srv.Close()

	req, _ := http.NewRequest("DELETE", srv.URL+"/v1/responses/resp_abc123456789012345678901", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}

func TestDeleteUnknownIDReturns404(t *testing.T) {
	store := &mockStore{responses: map[string]*api.Response{}}
	adapter := newTestAdapter(&mockCreator{}, store)
	srv := httptest.NewServer(adapter.Handler())
	defer srv.Close()

	req, _ := http.NewRequest("DELETE", srv.URL+"/v1/responses/resp_unknown12345678901234567", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestDeleteMalformedIDReturns400(t *testing.T) {
	store := &mockStore{responses: map[string]*api.Response{}}
	adapter := newTestAdapter(&mockCreator{}, store)
	srv := httptest.NewServer(adapter.Handler())
	defer srv.Close()

	req, _ := http.NewRequest("DELETE", srv.URL+"/v1/responses/bad-id", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestDeleteChecksInFlightBeforeStore(t *testing.T) {
	store := &mockStore{
		responses: map[string]*api.Response{
			"resp_abc123456789012345678901": {ID: "resp_abc123456789012345678901"},
		},
	}

	adapter := newTestAdapter(&mockCreator{}, store)
	srv := httptest.NewServer(adapter.Handler())
	defer srv.Close()

	// Register an in-flight entry manually.
	cancelled := false
	adapter.inflight.Register("resp_abc123456789012345678901", func() { cancelled = true })

	req, _ := http.NewRequest("DELETE", srv.URL+"/v1/responses/resp_abc123456789012345678901", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE error: %v", err)
	}
	defer resp.Body.Close()

	// Should return 204 from in-flight cancel, not from store.
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	if !cancelled {
		t.Error("expected in-flight cancel to be called")
	}

	// Store should still have the entry (it was not deleted from store).
	if _, ok := store.responses["resp_abc123456789012345678901"]; !ok {
		t.Error("store entry should not have been deleted (in-flight cancel takes priority)")
	}
}
