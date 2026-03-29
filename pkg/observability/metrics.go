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

// ConversationDepthBuckets defines histogram buckets for conversation item
// counts, ranging from 1 to 50 items.
var ConversationDepthBuckets = []float64{1, 2, 5, 10, 20, 50}

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

// Responses Layer metrics (spec 046).
var (
	// ResponsesTotal counts completed responses by model, status, and mode.
	ResponsesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "antwort_responses_total",
			Help: "Total responses by model, status, and mode",
		},
		[]string{"model", "status", "mode"},
	)

	// ResponsesDuration records response duration in seconds by model and mode.
	ResponsesDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "antwort_responses_duration_seconds",
			Help:    "Response duration by model and mode",
			Buckets: LLMBuckets,
		},
		[]string{"model", "mode"},
	)

	// ResponsesActive tracks the number of in-flight responses by mode.
	ResponsesActive = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "antwort_responses_active",
			Help: "Active in-flight responses by mode",
		},
		[]string{"mode"},
	)

	// ResponsesChainedTotal counts responses using previous_response_id.
	ResponsesChainedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "antwort_responses_chained_total",
			Help: "Responses using previous_response_id",
		},
		[]string{"model"},
	)

	// ResponsesTokensTotal counts tokens by model and direction.
	ResponsesTokensTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "antwort_responses_tokens_total",
			Help: "Token usage by model and direction",
		},
		[]string{"model", "type"},
	)
)

// Engine Layer metrics (spec 046).
var (
	// EngineIterationsTotal counts agentic loop iterations by model.
	EngineIterationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "antwort_engine_iterations_total",
			Help: "Total agentic loop iterations by model",
		},
		[]string{"model"},
	)

	// EngineIterationDuration records per-iteration duration by model.
	EngineIterationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "antwort_engine_iteration_duration_seconds",
			Help:    "Iteration duration by model",
			Buckets: LLMBuckets,
		},
		[]string{"model"},
	)

	// EngineMaxIterationsHit counts responses that hit the max iteration limit.
	EngineMaxIterationsHit = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "antwort_engine_max_iterations_hit_total",
			Help: "Responses hitting max iterations limit",
		},
		[]string{"model"},
	)

	// EngineToolDuration records tool execution duration by tool name.
	EngineToolDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "antwort_engine_tool_duration_seconds",
			Help:    "Tool execution duration by tool name",
			Buckets: LLMBuckets,
		},
		[]string{"tool_name"},
	)

	// EngineConversationDepth records conversation item count by model.
	EngineConversationDepth = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "antwort_engine_conversation_depth",
			Help:    "Conversation item count by model",
			Buckets: ConversationDepthBuckets,
		},
		[]string{"model"},
	)
)

// Storage Layer metrics (spec 046).
var (
	// StorageOperationsTotal counts storage operations by backend, type, and result.
	StorageOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "antwort_storage_operations_total",
			Help: "Storage operations by backend, operation, and result",
		},
		[]string{"backend", "operation", "result"},
	)

	// StorageOperationDuration records storage operation duration by backend and type.
	StorageOperationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "antwort_storage_operation_duration_seconds",
			Help:    "Storage operation duration by backend and operation",
			Buckets: LLMBuckets,
		},
		[]string{"backend", "operation"},
	)

	// StorageResponsesStored tracks the current response count by backend.
	StorageResponsesStored = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "antwort_storage_responses_stored",
			Help: "Current response count in storage by backend",
		},
		[]string{"backend"},
	)

	// StorageConnectionsActive tracks active PostgreSQL connections.
	StorageConnectionsActive = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "antwort_storage_connections_active",
			Help: "Active PostgreSQL connection pool connections",
		},
	)
)

// Files and Vector Store Layer metrics (spec 046).
var (
	// FilesUploadedTotal counts uploaded files by content type.
	FilesUploadedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "antwort_files_uploaded_total",
			Help: "Files uploaded by content type",
		},
		[]string{"content_type"},
	)

	// FilesIngestionDuration records file ingestion pipeline duration.
	FilesIngestionDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "antwort_files_ingestion_duration_seconds",
			Help:    "File ingestion pipeline duration",
			Buckets: LLMBuckets,
		},
	)

	// VectorstoreSearchesTotal counts vector store searches by store and result.
	VectorstoreSearchesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "antwort_vectorstore_searches_total",
			Help: "Vector store searches by store and result",
		},
		[]string{"store_id", "result"},
	)

	// VectorstoreSearchDuration records vector store search latency.
	VectorstoreSearchDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "antwort_vectorstore_search_duration_seconds",
			Help:    "Vector store search latency",
			Buckets: LLMBuckets,
		},
	)

	// VectorstoreItemsStored tracks item count per vector store.
	VectorstoreItemsStored = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "antwort_vectorstore_items_stored",
			Help: "Item count per vector store",
		},
		[]string{"store_id"},
	)
)

// Background Worker Layer metrics (spec 046).
var (
	// BackgroundQueued tracks the background response queue depth.
	BackgroundQueued = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "antwort_background_queued",
			Help: "Background response queue depth",
		},
	)

	// BackgroundClaimedTotal counts responses claimed by worker.
	BackgroundClaimedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "antwort_background_claimed_total",
			Help: "Responses claimed by worker",
		},
		[]string{"worker_id"},
	)

	// BackgroundStaleTotal counts stale responses detected and reclaimed.
	BackgroundStaleTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "antwort_background_stale_total",
			Help: "Stale responses detected and reclaimed",
		},
	)

	// BackgroundWorkerHeartbeatAge tracks time since last worker heartbeat.
	BackgroundWorkerHeartbeatAge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "antwort_background_worker_heartbeat_age_seconds",
			Help: "Time since last worker heartbeat",
		},
		[]string{"worker_id"},
	)
)

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
		// Spec 013 metrics.
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

		// Spec 046: Responses Layer.
		ResponsesTotal,
		ResponsesDuration,
		ResponsesActive,
		ResponsesChainedTotal,
		ResponsesTokensTotal,

		// Spec 046: Engine Layer.
		EngineIterationsTotal,
		EngineIterationDuration,
		EngineMaxIterationsHit,
		EngineToolDuration,
		EngineConversationDepth,

		// Spec 046: Storage Layer.
		StorageOperationsTotal,
		StorageOperationDuration,
		StorageResponsesStored,
		StorageConnectionsActive,

		// Spec 046: Files/Vector Store Layer.
		FilesUploadedTotal,
		FilesIngestionDuration,
		VectorstoreSearchesTotal,
		VectorstoreSearchDuration,
		VectorstoreItemsStored,

		// Spec 046: Background Worker Layer.
		BackgroundQueued,
		BackgroundClaimedTotal,
		BackgroundStaleTotal,
		BackgroundWorkerHeartbeatAge,
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
