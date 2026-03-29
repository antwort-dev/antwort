package resilience

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/config"
	"github.com/rhuss/antwort/pkg/debug"
	"github.com/rhuss/antwort/pkg/observability"
	"github.com/rhuss/antwort/pkg/provider"
)

// ResilientProvider wraps a provider.Provider with circuit breaker and retry logic.
// It is transparent to the engine: the agentic loop is unaware of retries or
// circuit breaker state.
type ResilientProvider struct {
	inner  provider.Provider
	cb     *CircuitBreaker
	policy RetryPolicy
}

// Wrap creates a ResilientProvider wrapping the given provider. If resilience
// is not enabled in the config, the original provider is returned unchanged
// (zero overhead).
func Wrap(inner provider.Provider, cfg config.ResilienceConfig) provider.Provider {
	if !cfg.Enabled {
		return inner
	}
	slog.Info("resilience enabled",
		"provider", inner.Name(),
		"failure_threshold", cfg.FailureThreshold,
		"reset_timeout", cfg.ResetTimeout,
		"max_attempts", cfg.MaxAttempts,
		"backoff_base", cfg.BackoffBase,
		"backoff_max", cfg.BackoffMax,
	)
	return &ResilientProvider{
		inner: inner,
		cb:    NewCircuitBreaker(int64(cfg.FailureThreshold), cfg.ResetTimeout),
		policy: RetryPolicy{
			MaxAttempts:   cfg.MaxAttempts,
			BackoffBase:   cfg.BackoffBase,
			BackoffMax:    cfg.BackoffMax,
			RetryAfterMax: cfg.RetryAfterMax,
		},
	}
}

// Name delegates to the wrapped provider.
func (r *ResilientProvider) Name() string {
	return r.inner.Name()
}

// Capabilities delegates to the wrapped provider.
func (r *ResilientProvider) Capabilities() provider.ProviderCapabilities {
	return r.inner.Capabilities()
}

// ListModels delegates to the wrapped provider without resilience.
func (r *ResilientProvider) ListModels(ctx context.Context) ([]provider.ModelInfo, error) {
	return r.inner.ListModels(ctx)
}

// Close delegates to the wrapped provider.
func (r *ResilientProvider) Close() error {
	return r.inner.Close()
}

// Complete performs a non-streaming inference call with retry and circuit breaker
// protection. Each retry attempt counts as a separate attempt for circuit breaker
// failure tracking. Rate-limited (429) responses are retried without affecting
// the circuit breaker.
func (r *ResilientProvider) Complete(ctx context.Context, req *provider.ProviderRequest) (*provider.ProviderResponse, error) {
	r.recordCircuitState()

	for attempt := 1; attempt <= r.policy.MaxAttempts; attempt++ {
		if !r.cb.Allow() {
			debug.Log("providers", "circuit breaker open, fast-fail",
				"provider", r.inner.Name(),
				"attempt", attempt,
			)
			return nil, r.circuitOpenError()
		}

		resp, err := r.inner.Complete(ctx, req)
		if err == nil {
			r.cb.RecordSuccess()
			r.recordCircuitState()
			if attempt > 1 {
				observability.ResilienceRetryAttemptsTotal.WithLabelValues(r.inner.Name(), "success").Inc()
			}
			return resp, nil
		}

		classification := Classify(err)
		if err := r.handleError(ctx, attempt, classification, err); err != nil {
			return nil, err
		}
	}

	// All retry attempts exhausted (e.g., rate-limited on every attempt).
	observability.ResilienceRetryExhaustedTotal.WithLabelValues(r.inner.Name()).Inc()
	return nil, api.NewServerError(fmt.Sprintf("all %d retry attempts exhausted for provider %q", r.policy.MaxAttempts, r.inner.Name()))
}

// Stream performs a streaming inference call with retry on connection failures.
// Only the connection phase (Stream() returning an error) is retried. Once the
// event channel is returned successfully, no further retry is attempted.
func (r *ResilientProvider) Stream(ctx context.Context, req *provider.ProviderRequest) (<-chan provider.ProviderEvent, error) {
	r.recordCircuitState()

	for attempt := 1; attempt <= r.policy.MaxAttempts; attempt++ {
		if !r.cb.Allow() {
			debug.Log("providers", "circuit breaker open, fast-fail (streaming)",
				"provider", r.inner.Name(),
				"attempt", attempt,
			)
			return nil, r.circuitOpenError()
		}

		ch, err := r.inner.Stream(ctx, req)
		if err == nil {
			r.cb.RecordSuccess()
			r.recordCircuitState()
			if attempt > 1 {
				observability.ResilienceRetryAttemptsTotal.WithLabelValues(r.inner.Name(), "success").Inc()
			}
			return ch, nil
		}

		classification := Classify(err)
		if err := r.handleError(ctx, attempt, classification, err); err != nil {
			return nil, err
		}
	}

	observability.ResilienceRetryExhaustedTotal.WithLabelValues(r.inner.Name()).Inc()
	return nil, api.NewServerError(fmt.Sprintf("all %d retry attempts exhausted for provider %q (streaming)", r.policy.MaxAttempts, r.inner.Name()))
}

// handleError processes an error from a provider call. It classifies the error,
// records metrics, and either waits for a retry or returns the error.
// Returns nil if the caller should retry, or an error if it should stop.
func (r *ResilientProvider) handleError(ctx context.Context, attempt int, classification Classification, originalErr error) error {
	switch classification {
	case RateLimited:
		observability.ResilienceRetryAttemptsTotal.WithLabelValues(r.inner.Name(), "rate_limited").Inc()
		// 429 does NOT affect circuit breaker (FR-011).
		wait := r.retryAfterWait(originalErr, attempt)
		debug.Log("providers", "rate limited, waiting",
			"provider", r.inner.Name(),
			"attempt", attempt,
			"wait", wait,
		)
		// If wait exceeds remaining context deadline, skip retry (FR-012).
		if remaining, ok := contextRemainingTime(ctx); ok && wait > remaining {
			return originalErr
		}
		if err := sleepWithContext(ctx, wait); err != nil {
			return originalErr
		}
		return nil // retry

	case Retryable:
		observability.ResilienceRetryAttemptsTotal.WithLabelValues(r.inner.Name(), "failure").Inc()
		prevState := r.cb.State()
		r.cb.RecordFailure()
		r.recordCircuitTransitionFrom(prevState)
		if attempt >= r.policy.MaxAttempts {
			observability.ResilienceRetryExhaustedTotal.WithLabelValues(r.inner.Name()).Inc()
			debug.Log("providers", "all retries exhausted",
				"provider", r.inner.Name(),
				"attempt", attempt,
				"error", originalErr.Error(),
			)
			return originalErr
		}
		wait := computeBackoff(attempt, r.policy.BackoffBase, r.policy.BackoffMax)
		debug.Log("providers", "retryable error, backing off",
			"provider", r.inner.Name(),
			"attempt", attempt,
			"wait", wait,
			"error", originalErr.Error(),
		)
		if err := sleepWithContext(ctx, wait); err != nil {
			return originalErr
		}
		return nil // retry

	default:
		// NonRetryable: return immediately, no circuit breaker impact.
		return originalErr
	}
}

// retryAfterWait extracts the Retry-After duration from a 429 error, falling
// back to standard exponential backoff if not available. The duration is capped
// at RetryAfterMax.
func (r *ResilientProvider) retryAfterWait(err error, attempt int) time.Duration {
	var apiErr *api.APIError
	if errors.As(err, &apiErr) && apiErr.RetryAfter > 0 {
		wait := apiErr.RetryAfter
		if wait > r.policy.RetryAfterMax {
			wait = r.policy.RetryAfterMax
		}
		return wait
	}
	return computeBackoff(attempt, r.policy.BackoffBase, r.policy.BackoffMax)
}

func (r *ResilientProvider) circuitOpenError() *api.APIError {
	return &api.APIError{
		Type:    api.ErrorTypeServerError,
		Message: fmt.Sprintf("circuit breaker open for provider %q: backend unavailable, will probe in %s", r.inner.Name(), r.cb.resetTimeout),
	}
}

// recordCircuitState updates the circuit breaker state gauge metric.
func (r *ResilientProvider) recordCircuitState() {
	observability.ResilienceCircuitBreakerState.WithLabelValues(r.inner.Name()).Set(float64(r.cb.State()))
}

// recordCircuitTransitionFrom records a circuit breaker state transition
// by comparing the previous state to the current state and incrementing
// the transitions counter if a change occurred.
func (r *ResilientProvider) recordCircuitTransitionFrom(prevState int32) {
	newState := r.cb.State()
	observability.ResilienceCircuitBreakerState.WithLabelValues(r.inner.Name()).Set(float64(newState))
	if prevState != newState {
		observability.ResilienceCircuitBreakerTransitionsTotal.WithLabelValues(
			r.inner.Name(), StateName(prevState), StateName(newState),
		).Inc()
		debug.Log("providers", "circuit breaker state changed",
			"provider", r.inner.Name(),
			"from", StateName(prevState),
			"to", StateName(newState),
		)
	}
}
