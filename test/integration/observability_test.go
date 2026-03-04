package integration

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func TestMetricsEndpoint(t *testing.T) {
	// Create a standalone server with the metrics handler.
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	server := httptest.NewServer(mux)
	defer server.Close()

	resp := getURL(t, server.URL+"/metrics")
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	body := readBody(t, resp)

	// The metrics endpoint should contain at least Go runtime metrics.
	if !strings.Contains(body, "go_") {
		t.Error("metrics response does not contain go_ metrics")
	}
}

func TestMetricsAfterRequest(t *testing.T) {
	// Create a server with both the antwort handler and a metrics endpoint.
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/", testEnv.AntwortServer.Config.Handler)

	server := httptest.NewServer(mux)
	defer server.Close()

	// Make a request to the shared antwort server (triggers metric recording).
	reqBody := map[string]any{
		"model": "mock-model",
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
	resp := postJSON(t, testEnv.BaseURL()+"/v1/responses", reqBody)
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("response request: expected 200, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Now check that the metrics endpoint returns antwort-specific metrics.
	metricsResp := getURL(t, server.URL+"/metrics")
	if metricsResp.StatusCode != http.StatusOK {
		body := readBody(t, metricsResp)
		t.Fatalf("metrics: expected 200, got %d: %s", metricsResp.StatusCode, body)
	}

	body := readBody(t, metricsResp)

	// After making a request, we should see antwort metrics registered via the
	// observability package (they are registered in init()).
	if !strings.Contains(body, "antwort_") {
		t.Error("metrics response does not contain antwort_ metrics after request")
	}
}
