package resilience

import (
	"context"
	"testing"
	"time"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/config"
	"github.com/rhuss/antwort/pkg/provider"
)

// mockProvider is a test double for provider.Provider.
type mockProvider struct {
	name         string
	completeFunc func(ctx context.Context, req *provider.ProviderRequest) (*provider.ProviderResponse, error)
	streamFunc   func(ctx context.Context, req *provider.ProviderRequest) (<-chan provider.ProviderEvent, error)
}

func (m *mockProvider) Name() string                        { return m.name }
func (m *mockProvider) Capabilities() provider.ProviderCapabilities { return provider.ProviderCapabilities{} }
func (m *mockProvider) ListModels(ctx context.Context) ([]provider.ModelInfo, error) { return nil, nil }
func (m *mockProvider) Close() error                        { return nil }

func (m *mockProvider) Complete(ctx context.Context, req *provider.ProviderRequest) (*provider.ProviderResponse, error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, req)
	}
	return &provider.ProviderResponse{}, nil
}

func (m *mockProvider) Stream(ctx context.Context, req *provider.ProviderRequest) (<-chan provider.ProviderEvent, error) {
	if m.streamFunc != nil {
		return m.streamFunc(ctx, req)
	}
	ch := make(chan provider.ProviderEvent)
	close(ch)
	return ch, nil
}

func testConfig() config.ResilienceConfig {
	return config.ResilienceConfig{
		Enabled:          true,
		FailureThreshold: 3,
		ResetTimeout:     100 * time.Millisecond,
		MaxAttempts:      3,
		BackoffBase:      10 * time.Millisecond,
		BackoffMax:       50 * time.Millisecond,
		RetryAfterMax:    100 * time.Millisecond,
	}
}

// T036: Wrap returns unwrapped provider when disabled.
func TestWrap_DisabledReturnsOriginal(t *testing.T) {
	mock := &mockProvider{name: "test"}
	cfg := config.ResilienceConfig{Enabled: false}
	wrapped := Wrap(mock, cfg)

	// Should be the exact same pointer, no wrapper.
	if wrapped != mock {
		t.Fatal("Wrap() with Enabled=false should return original provider")
	}
}

// US1: Successful passthrough.
func TestComplete_Success(t *testing.T) {
	calls := 0
	mock := &mockProvider{
		name: "test",
		completeFunc: func(_ context.Context, _ *provider.ProviderRequest) (*provider.ProviderResponse, error) {
			calls++
			return &provider.ProviderResponse{Model: "m"}, nil
		},
	}

	rp := Wrap(mock, testConfig())
	resp, err := rp.Complete(context.Background(), &provider.ProviderRequest{})

	if err != nil {
		t.Fatalf("Complete() error = %v, want nil", err)
	}
	if resp.Model != "m" {
		t.Errorf("resp.Model = %q, want %q", resp.Model, "m")
	}
	if calls != 1 {
		t.Errorf("provider called %d times, want 1", calls)
	}
}

// US1: Retry on retryable error, then succeed.
func TestComplete_RetryOnTransientError(t *testing.T) {
	calls := 0
	mock := &mockProvider{
		name: "test",
		completeFunc: func(_ context.Context, _ *provider.ProviderRequest) (*provider.ProviderResponse, error) {
			calls++
			if calls < 3 {
				return nil, api.NewServerError("backend 503")
			}
			return &provider.ProviderResponse{Model: "m"}, nil
		},
	}

	rp := Wrap(mock, testConfig())
	resp, err := rp.Complete(context.Background(), &provider.ProviderRequest{})

	if err != nil {
		t.Fatalf("Complete() error = %v, want nil (should succeed on 3rd attempt)", err)
	}
	if resp == nil {
		t.Fatal("Complete() resp = nil, want non-nil")
	}
	if calls != 3 {
		t.Errorf("provider called %d times, want 3", calls)
	}
}

// US1: No retry on non-retryable error.
func TestComplete_NoRetryOnClientError(t *testing.T) {
	calls := 0
	mock := &mockProvider{
		name: "test",
		completeFunc: func(_ context.Context, _ *provider.ProviderRequest) (*provider.ProviderResponse, error) {
			calls++
			return nil, api.NewInvalidRequestError("model", "invalid model")
		},
	}

	rp := Wrap(mock, testConfig())
	_, err := rp.Complete(context.Background(), &provider.ProviderRequest{})

	if err == nil {
		t.Fatal("Complete() expected error for invalid request")
	}
	if calls != 1 {
		t.Errorf("provider called %d times, want 1 (no retry on 4xx)", calls)
	}
}

// US1: All retries exhausted.
func TestComplete_AllRetriesExhausted(t *testing.T) {
	calls := 0
	mock := &mockProvider{
		name: "test",
		completeFunc: func(_ context.Context, _ *provider.ProviderRequest) (*provider.ProviderResponse, error) {
			calls++
			return nil, api.NewServerError("always failing")
		},
	}

	rp := Wrap(mock, testConfig())
	_, err := rp.Complete(context.Background(), &provider.ProviderRequest{})

	if err == nil {
		t.Fatal("Complete() expected error when all retries exhausted")
	}
	if calls != 3 {
		t.Errorf("provider called %d times, want 3 (max attempts)", calls)
	}
}

// US2: Circuit breaker fast-fail.
func TestComplete_CircuitBreakerOpenFastFail(t *testing.T) {
	cfg := testConfig()
	cfg.FailureThreshold = 1
	cfg.MaxAttempts = 1

	mock := &mockProvider{
		name: "test",
		completeFunc: func(_ context.Context, _ *provider.ProviderRequest) (*provider.ProviderResponse, error) {
			return nil, api.NewServerError("backend down")
		},
	}

	rp := Wrap(mock, cfg)

	// First call: fails and trips circuit.
	_, _ = rp.Complete(context.Background(), &provider.ProviderRequest{})

	// Second call: should fast-fail without calling provider.
	start := time.Now()
	_, err := rp.Complete(context.Background(), &provider.ProviderRequest{})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Complete() expected error when circuit is open")
	}
	if elapsed > 50*time.Millisecond {
		t.Errorf("fast-fail took %v, want < 50ms", elapsed)
	}
}

// US2: Circuit breaker recovery via half-open probe.
func TestComplete_CircuitBreakerRecovery(t *testing.T) {
	cfg := testConfig()
	cfg.FailureThreshold = 1
	cfg.MaxAttempts = 1
	cfg.ResetTimeout = 50 * time.Millisecond

	calls := 0
	mock := &mockProvider{
		name: "test",
		completeFunc: func(_ context.Context, _ *provider.ProviderRequest) (*provider.ProviderResponse, error) {
			calls++
			if calls <= 1 {
				return nil, api.NewServerError("backend down")
			}
			return &provider.ProviderResponse{Model: "m"}, nil
		},
	}

	rp := Wrap(mock, cfg)

	// Trip the circuit.
	_, _ = rp.Complete(context.Background(), &provider.ProviderRequest{})

	// Wait for reset timeout to elapse. The circuit transitions to half-open
	// when Allow() is called, so we just wait long enough for the timeout
	// without consuming the probe slot.
	time.Sleep(80 * time.Millisecond)

	// Probe request should succeed and close circuit.
	resp, err := rp.Complete(context.Background(), &provider.ProviderRequest{})
	if err != nil {
		t.Fatalf("Complete() error = %v after recovery, want nil", err)
	}
	if resp == nil {
		t.Fatal("Complete() resp = nil after recovery")
	}
}

// US3: 429 with RetryAfter.
func TestComplete_RateLimitedWithRetryAfter(t *testing.T) {
	calls := 0
	mock := &mockProvider{
		name: "test",
		completeFunc: func(_ context.Context, _ *provider.ProviderRequest) (*provider.ProviderResponse, error) {
			calls++
			if calls == 1 {
				err := api.NewTooManyRequestsError("rate limited")
				err.RetryAfter = 20 * time.Millisecond
				return nil, err
			}
			return &provider.ProviderResponse{Model: "m"}, nil
		},
	}

	rp := Wrap(mock, testConfig())
	resp, err := rp.Complete(context.Background(), &provider.ProviderRequest{})

	if err != nil {
		t.Fatalf("Complete() error = %v, want nil (should retry after 429)", err)
	}
	if resp == nil {
		t.Fatal("Complete() resp = nil")
	}
	if calls != 2 {
		t.Errorf("provider called %d times, want 2", calls)
	}
}

// US3: 429 does not affect circuit breaker.
func TestComplete_RateLimitedDoesNotTripCircuit(t *testing.T) {
	cfg := testConfig()
	cfg.FailureThreshold = 2

	calls := 0
	mock := &mockProvider{
		name: "test",
		completeFunc: func(_ context.Context, _ *provider.ProviderRequest) (*provider.ProviderResponse, error) {
			calls++
			err := api.NewTooManyRequestsError("rate limited")
			err.RetryAfter = 10 * time.Millisecond
			return nil, err
		},
	}

	rp := Wrap(mock, cfg).(*ResilientProvider)
	_, _ = rp.Complete(context.Background(), &provider.ProviderRequest{})

	// Circuit should still be closed (429s don't count).
	if rp.cb.State() != StateClosed {
		t.Errorf("circuit state = %s after 429s, want closed", StateName(rp.cb.State()))
	}
	if rp.cb.ConsecutiveFailures() != 0 {
		t.Errorf("consecutive failures = %d after 429s, want 0", rp.cb.ConsecutiveFailures())
	}
}

// US3: 429 with RetryAfter exceeding context deadline.
func TestComplete_RateLimitedExceedsDeadline(t *testing.T) {
	calls := 0
	mock := &mockProvider{
		name: "test",
		completeFunc: func(_ context.Context, _ *provider.ProviderRequest) (*provider.ProviderResponse, error) {
			calls++
			err := api.NewTooManyRequestsError("rate limited")
			err.RetryAfter = 5 * time.Second // way longer than context deadline
			return nil, err
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	rp := Wrap(mock, testConfig())
	_, err := rp.Complete(ctx, &provider.ProviderRequest{})

	if err == nil {
		t.Fatal("Complete() expected error when RetryAfter exceeds deadline")
	}
	if calls != 1 {
		t.Errorf("provider called %d times, want 1 (should not retry)", calls)
	}
}

// US4: Streaming connection retry.
func TestStream_RetryOnConnectionError(t *testing.T) {
	calls := 0
	mock := &mockProvider{
		name: "test",
		streamFunc: func(_ context.Context, _ *provider.ProviderRequest) (<-chan provider.ProviderEvent, error) {
			calls++
			if calls < 2 {
				return nil, api.NewServerError("connection refused")
			}
			ch := make(chan provider.ProviderEvent)
			close(ch)
			return ch, nil
		},
	}

	rp := Wrap(mock, testConfig())
	ch, err := rp.Stream(context.Background(), &provider.ProviderRequest{})

	if err != nil {
		t.Fatalf("Stream() error = %v, want nil (should succeed on retry)", err)
	}
	if ch == nil {
		t.Fatal("Stream() channel = nil")
	}
	if calls != 2 {
		t.Errorf("provider called %d times, want 2", calls)
	}
}

// US4: Streaming passthrough on success.
func TestStream_Success(t *testing.T) {
	calls := 0
	mock := &mockProvider{
		name: "test",
		streamFunc: func(_ context.Context, _ *provider.ProviderRequest) (<-chan provider.ProviderEvent, error) {
			calls++
			ch := make(chan provider.ProviderEvent)
			close(ch)
			return ch, nil
		},
	}

	rp := Wrap(mock, testConfig())
	ch, err := rp.Stream(context.Background(), &provider.ProviderRequest{})

	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	if ch == nil {
		t.Fatal("Stream() channel = nil")
	}
	if calls != 1 {
		t.Errorf("provider called %d times, want 1", calls)
	}
}

// US4: Streaming circuit breaker.
func TestStream_CircuitBreakerOpen(t *testing.T) {
	cfg := testConfig()
	cfg.FailureThreshold = 1
	cfg.MaxAttempts = 1

	mock := &mockProvider{
		name: "test",
		streamFunc: func(_ context.Context, _ *provider.ProviderRequest) (<-chan provider.ProviderEvent, error) {
			return nil, api.NewServerError("connection failed")
		},
	}

	rp := Wrap(mock, cfg)

	// Trip circuit.
	_, _ = rp.Stream(context.Background(), &provider.ProviderRequest{})

	// Second call should fast-fail.
	_, err := rp.Stream(context.Background(), &provider.ProviderRequest{})
	if err == nil {
		t.Fatal("Stream() expected error when circuit is open")
	}
}

// Delegation tests.
func TestDelegation(t *testing.T) {
	mock := &mockProvider{name: "test-provider"}
	rp := Wrap(mock, testConfig())

	if rp.Name() != "test-provider" {
		t.Errorf("Name() = %q, want %q", rp.Name(), "test-provider")
	}

	_ = rp.Capabilities() // verify no panic

	_, err := rp.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}

	err = rp.Close()
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
