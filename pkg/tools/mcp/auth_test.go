package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockTokenServer creates an httptest.Server that serves as an OAuth token endpoint.
// It returns the token, tracks call count, and can be configured to fail.
func mockTokenServer(t *testing.T, token string, expiresIn int, failAfter int) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	callCount := &atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := callCount.Add(1)

		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			http.Error(w, "bad content type", http.StatusBadRequest)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}

		if r.FormValue("grant_type") != "client_credentials" {
			http.Error(w, "bad grant_type", http.StatusBadRequest)
			return
		}

		// Fail after the configured number of successful calls.
		if failAfter > 0 && int(count) > failAfter {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}

		resp := tokenResponse{
			AccessToken: token,
			TokenType:   "bearer",
			ExpiresIn:   expiresIn,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	return srv, callCount
}

func TestOAuthClientCredentials_AcquireToken(t *testing.T) {
	srv, callCount := mockTokenServer(t, "test-token-123", 3600, 0)
	defer srv.Close()

	auth := NewOAuthClientCredentials(srv.URL, "my-client", "my-secret", []string{"read", "write"})

	headers, err := auth.GetHeaders(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "Bearer test-token-123"
	if got := headers["Authorization"]; got != expected {
		t.Errorf("Authorization header = %q, want %q", got, expected)
	}

	if got := callCount.Load(); got != 1 {
		t.Errorf("token endpoint called %d times, want 1", got)
	}
}

func TestOAuthClientCredentials_CacheToken(t *testing.T) {
	srv, callCount := mockTokenServer(t, "cached-token", 3600, 0)
	defer srv.Close()

	auth := NewOAuthClientCredentials(srv.URL, "my-client", "my-secret", nil)

	// First call acquires the token.
	_, err := auth.GetHeaders(context.Background())
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	// Second call should use cached token.
	headers, err := auth.GetHeaders(context.Background())
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if got := headers["Authorization"]; got != "Bearer cached-token" {
		t.Errorf("Authorization = %q, want %q", got, "Bearer cached-token")
	}

	if got := callCount.Load(); got != 1 {
		t.Errorf("token endpoint called %d times, want 1 (caching failed)", got)
	}
}

func TestOAuthClientCredentials_ProactiveRefresh(t *testing.T) {
	// Use a very short expiry to test proactive refresh.
	// Token expires in 10 seconds, refresh at 80% = 8 seconds.
	srv, callCount := mockTokenServer(t, "refreshed-token", 10, 0)
	defer srv.Close()

	auth := NewOAuthClientCredentials(srv.URL, "my-client", "my-secret", nil)

	// Acquire initial token.
	now := time.Now()
	auth.nowFunc = func() time.Time { return now }

	_, err := auth.GetHeaders(context.Background())
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if callCount.Load() != 1 {
		t.Fatal("expected 1 call after first request")
	}

	// Advance time past the 80% mark (8s) but before expiry (10s).
	auth.nowFunc = func() time.Time { return now.Add(9 * time.Second) }

	_, err = auth.GetHeaders(context.Background())
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	// Should have refreshed proactively.
	if got := callCount.Load(); got != 2 {
		t.Errorf("token endpoint called %d times, want 2 (proactive refresh)", got)
	}
}

func TestOAuthClientCredentials_RefreshFailure_UseExisting(t *testing.T) {
	// Token endpoint succeeds once then fails.
	srv, _ := mockTokenServer(t, "still-valid-token", 10, 1)
	defer srv.Close()

	auth := NewOAuthClientCredentials(srv.URL, "my-client", "my-secret", nil)

	now := time.Now()
	auth.nowFunc = func() time.Time { return now }

	// Acquire initial token.
	_, err := auth.GetHeaders(context.Background())
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	// Advance past refresh point (80% of 10s = 8s) but before expiry (10s).
	auth.nowFunc = func() time.Time { return now.Add(9 * time.Second) }

	// Should use cached token since it's still valid despite refresh failure.
	headers, err := auth.GetHeaders(context.Background())
	if err != nil {
		t.Fatalf("expected cached token on refresh failure, got error: %v", err)
	}

	if got := headers["Authorization"]; got != "Bearer still-valid-token" {
		t.Errorf("Authorization = %q, want %q", got, "Bearer still-valid-token")
	}
}

func TestOAuthClientCredentials_ExpiredAndFailure(t *testing.T) {
	// Token endpoint succeeds once then fails.
	srv, _ := mockTokenServer(t, "expired-token", 10, 1)
	defer srv.Close()

	auth := NewOAuthClientCredentials(srv.URL, "my-client", "my-secret", nil)

	now := time.Now()
	auth.nowFunc = func() time.Time { return now }

	// Acquire initial token.
	_, err := auth.GetHeaders(context.Background())
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	// Advance past expiry (10s).
	auth.nowFunc = func() time.Time { return now.Add(11 * time.Second) }

	// Token is expired AND refresh fails: should return error.
	_, err = auth.GetHeaders(context.Background())
	if err == nil {
		t.Fatal("expected error when token is expired and refresh fails")
	}
}

func TestOAuthClientCredentials_InvalidCredentials(t *testing.T) {
	// Mock server that always returns 401.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"invalid_client"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	auth := NewOAuthClientCredentials(srv.URL, "bad-client", "bad-secret", nil)

	_, err := auth.GetHeaders(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid credentials")
	}

	// Verify the error includes the status code.
	expected := "401"
	if got := fmt.Sprintf("%v", err); !contains(got, expected) {
		t.Errorf("error %q should contain %q", got, expected)
	}
}

func TestOAuthClientCredentials_ConcurrentRefresh(t *testing.T) {
	srv, callCount := mockTokenServer(t, "concurrent-token", 3600, 0)
	defer srv.Close()

	auth := NewOAuthClientCredentials(srv.URL, "my-client", "my-secret", nil)

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)
	errCh := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			headers, err := auth.GetHeaders(context.Background())
			if err != nil {
				errCh <- err
				return
			}
			if got := headers["Authorization"]; got != "Bearer concurrent-token" {
				errCh <- fmt.Errorf("got %q, want %q", got, "Bearer concurrent-token")
			}
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("goroutine error: %v", err)
	}

	// Token endpoint should be called exactly once due to mutex serialization.
	if got := callCount.Load(); got != 1 {
		t.Errorf("token endpoint called %d times, want 1 (concurrent refresh)", got)
	}
}

func TestOAuthClientCredentials_ScopesIncluded(t *testing.T) {
	var receivedScope string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		receivedScope = r.FormValue("scope")
		resp := tokenResponse{AccessToken: "scoped-token", TokenType: "bearer", ExpiresIn: 3600}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	auth := NewOAuthClientCredentials(srv.URL, "client", "secret", []string{"read", "write", "admin"})
	_, err := auth.GetHeaders(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedScope != "read write admin" {
		t.Errorf("scope = %q, want %q", receivedScope, "read write admin")
	}
}

func TestOAuthClientCredentials_NoScopesOmitsParam(t *testing.T) {
	var hasScope bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		_, hasScope = r.Form["scope"]
		resp := tokenResponse{AccessToken: "no-scope-token", TokenType: "bearer", ExpiresIn: 3600}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	auth := NewOAuthClientCredentials(srv.URL, "client", "secret", nil)
	_, err := auth.GetHeaders(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if hasScope {
		t.Error("scope parameter should not be sent when scopes is nil")
	}
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
