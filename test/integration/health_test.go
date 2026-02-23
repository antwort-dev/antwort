package integration

import (
	"net/http"
	"strings"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	resp := getURL(t, testEnv.BaseURL()+"/healthz")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body := readBody(t, resp)
	if !strings.Contains(body, "ok") {
		t.Errorf("body = %q, want to contain 'ok'", body)
	}
}

func TestHealthEndpointNoAuth(t *testing.T) {
	// Health endpoint should work without any auth headers.
	req, err := http.NewRequest(http.MethodGet, testEnv.BaseURL()+"/healthz", nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	// Explicitly don't set any auth headers.

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 without auth, got %d", resp.StatusCode)
	}
}
