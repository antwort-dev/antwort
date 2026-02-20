// Package observability provides Prometheus metrics and HTTP middleware
// for monitoring the antwort gateway.
package observability

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// LLMBuckets defines histogram buckets suited for LLM inference latencies,
// ranging from 100ms to 120s.
var LLMBuckets = []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120}

var (
	// RequestsTotal counts all HTTP requests by method, status class, and model.
	RequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "antwort_requests_total",
			Help: "Total requests",
		},
		[]string{"method", "status", "model"},
	)

	// RequestDuration records HTTP request duration in seconds by method and model.
	RequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "antwort_request_duration_seconds",
			Help:    "Request duration",
			Buckets: LLMBuckets,
		},
		[]string{"method", "model"},
	)

	// StreamingConnections tracks the number of active SSE streaming connections.
	StreamingConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "antwort_streaming_connections_active",
			Help: "Active streaming connections",
		},
	)

	// ProviderRequestsTotal counts requests sent to backend LLM providers.
	ProviderRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "antwort_provider_requests_total",
			Help: "Provider requests",
		},
		[]string{"provider", "model", "status"},
	)

	// ProviderLatency records backend provider latency in seconds.
	ProviderLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "antwort_provider_latency_seconds",
			Help:    "Provider latency",
			Buckets: LLMBuckets,
		},
		[]string{"provider", "model"},
	)

	// ProviderTokensTotal counts tokens processed by direction (input/output).
	ProviderTokensTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "antwort_provider_tokens_total",
			Help: "Token count",
		},
		[]string{"provider", "model", "direction"},
	)

	// ToolExecutionsTotal counts tool executions by name and outcome.
	ToolExecutionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "antwort_tool_executions_total",
			Help: "Tool executions",
		},
		[]string{"tool_name", "status"},
	)

	// RateLimitRejectedTotal counts requests rejected by the rate limiter.
	RateLimitRejectedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "antwort_ratelimit_rejected_total",
			Help: "Rate limit rejections",
		},
		[]string{"tier"},
	)
)

// TokenBuckets defines histogram buckets for token counts, using powers of 4
// from 1 to 16384.
var TokenBuckets = []float64{1, 4, 16, 64, 256, 1024, 4096, 16384}

// OTel GenAI semantic convention metrics.
var (
	// GenAIClientTokenUsage records token usage per operation following the
	// OTel gen_ai semantic conventions.
	GenAIClientTokenUsage = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gen_ai_client_token_usage",
			Help:    "GenAI client token usage",
			Buckets: TokenBuckets,
		},
		[]string{"gen_ai_operation_name", "gen_ai_provider_name", "gen_ai_token_type", "gen_ai_request_model", "gen_ai_response_model"},
	)

	// GenAIClientOperationDuration records overall operation duration for
	// GenAI client calls.
	GenAIClientOperationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gen_ai_client_operation_duration_seconds",
			Help:    "GenAI client operation duration",
			Buckets: LLMBuckets,
		},
		[]string{"gen_ai_operation_name", "gen_ai_provider_name", "gen_ai_request_model", "gen_ai_response_model", "error_type"},
	)

	// GenAIServerTimeToFirstToken records time to first token for streaming
	// responses.
	GenAIServerTimeToFirstToken = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gen_ai_server_time_to_first_token_seconds",
			Help:    "Time to first token for streaming",
			Buckets: LLMBuckets,
		},
		[]string{"gen_ai_operation_name", "gen_ai_provider_name", "gen_ai_request_model"},
	)

	// GenAIServerTimePerOutputToken records per-token decode latency.
	GenAIServerTimePerOutputToken = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gen_ai_server_time_per_output_token_seconds",
			Help:    "Time per output token (decode latency)",
			Buckets: []float64{0.001, 0.005, 0.01, 0.02, 0.05, 0.1, 0.2, 0.5, 1.0},
		},
		[]string{"gen_ai_operation_name", "gen_ai_provider_name", "gen_ai_request_model"},
	)
)

func init() {
	prometheus.MustRegister(
		RequestsTotal,
		RequestDuration,
		StreamingConnections,
		ProviderRequestsTotal,
		ProviderLatency,
		ProviderTokensTotal,
		ToolExecutionsTotal,
		RateLimitRejectedTotal,
		GenAIClientTokenUsage,
		GenAIClientOperationDuration,
		GenAIServerTimeToFirstToken,
		GenAIServerTimePerOutputToken,
	)
}

// RecordGenAIMetrics records all OTel gen_ai.* metrics for a single provider
// interaction. It records operation duration, token usage (input and output),
// and optionally time-to-first-token if ttft is non-nil.
func RecordGenAIMetrics(providerName, model string, duration time.Duration, inputTokens, outputTokens int, ttft *time.Duration) {
	op := "chat"

	// Record operation duration (no error).
	GenAIClientOperationDuration.WithLabelValues(op, providerName, model, model, "").Observe(duration.Seconds())

	// Record token usage.
	if inputTokens > 0 {
		GenAIClientTokenUsage.WithLabelValues(op, providerName, "input", model, model).Observe(float64(inputTokens))
	}
	if outputTokens > 0 {
		GenAIClientTokenUsage.WithLabelValues(op, providerName, "output", model, model).Observe(float64(outputTokens))
	}

	// Record time to first token if available (streaming only).
	if ttft != nil {
		GenAIServerTimeToFirstToken.WithLabelValues(op, providerName, model).Observe(ttft.Seconds())
	}

	// Record per-output-token latency if we have output tokens and duration.
	if outputTokens > 0 && duration > 0 {
		perToken := duration.Seconds() / float64(outputTokens)
		GenAIServerTimePerOutputToken.WithLabelValues(op, providerName, model).Observe(perToken)
	}
}
