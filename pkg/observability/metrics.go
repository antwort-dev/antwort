// Package observability provides Prometheus metrics and HTTP middleware
// for monitoring the antwort gateway.
package observability

import "github.com/prometheus/client_golang/prometheus"

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
	)
}
