package observability

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// TestMetricsRegistered verifies that all metrics are registered in the
// default registry without panicking.
func TestMetricsRegistered(t *testing.T) {
	// Gather all metrics from the default registry. If registration failed
	// in init(), this test would never run (MustRegister panics), but we
	// verify gathering works cleanly.
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("unexpected gather error: %v", err)
	}

	expected := map[string]bool{
		"antwort_requests_total":                     false,
		"antwort_request_duration_seconds":           false,
		"antwort_streaming_connections_active":        false,
		"antwort_provider_requests_total":             false,
		"antwort_provider_latency_seconds":            false,
		"antwort_provider_tokens_total":               false,
		"antwort_tool_executions_total":               false,
		"antwort_ratelimit_rejected_total":            false,
		"gen_ai_client_token_usage":                   false,
		"gen_ai_client_operation_duration_seconds":    false,
		"gen_ai_server_time_to_first_token_seconds":   false,
		"gen_ai_server_time_per_output_token_seconds": false,
	}

	for _, mf := range families {
		if _, ok := expected[mf.GetName()]; ok {
			expected[mf.GetName()] = true
		}
	}

	// Some counters/histograms only appear after first observation.
	// The gauge (streaming_connections_active) should always appear.
	// We seed all metrics to make them visible.
	RequestsTotal.WithLabelValues("GET", "2xx", "test").Inc()
	RequestDuration.WithLabelValues("GET", "test").Observe(0.1)
	ProviderRequestsTotal.WithLabelValues("vllm", "test", "ok").Inc()
	ProviderLatency.WithLabelValues("vllm", "test").Observe(0.1)
	ProviderTokensTotal.WithLabelValues("vllm", "test", "input").Add(10)
	ToolExecutionsTotal.WithLabelValues("test_tool", "ok").Inc()
	RateLimitRejectedTotal.WithLabelValues("default").Inc()

	// Seed GenAI metrics.
	RecordGenAIMetrics("vllm", "test-model", 1*time.Second, 100, 50, nil)
	ttft := 200 * time.Millisecond
	RecordGenAIMetrics("vllm", "test-model", 2*time.Second, 200, 100, &ttft)

	families, err = prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("unexpected gather error after seeding: %v", err)
	}

	for _, mf := range families {
		if _, ok := expected[mf.GetName()]; ok {
			expected[mf.GetName()] = true
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("metric %q not found in default registry", name)
		}
	}
}

// TestMiddlewareRecordsRequestCount verifies that the middleware increments
// the request counter for each served request.
func TestMiddlewareRecordsRequestCount(t *testing.T) {
	// Get baseline count.
	before := counterValue(t, RequestsTotal, "GET", "2xx", "unknown")

	handler := MetricsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/responses", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	after := counterValue(t, RequestsTotal, "GET", "2xx", "unknown")
	if after-before != 1 {
		t.Errorf("expected request count to increase by 1, got delta=%f", after-before)
	}
}

// TestMiddlewareRecordsDuration verifies that the middleware records
// a positive request duration observation.
func TestMiddlewareRecordsDuration(t *testing.T) {
	before := histogramCount(t, RequestDuration, "POST", "unknown")

	handler := MetricsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/v1/responses", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	after := histogramCount(t, RequestDuration, "POST", "unknown")
	if after-before != 1 {
		t.Errorf("expected histogram sample count to increase by 1, got delta=%d", after-before)
	}
}

// TestMiddlewareStreamingGauge verifies that the streaming connections gauge
// increments during a streaming request and decrements after completion.
func TestMiddlewareStreamingGauge(t *testing.T) {
	baseline := gaugeValue(t, StreamingConnections)

	inHandler := make(chan float64, 1)
	handler := MetricsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture gauge value while inside the handler.
		inHandler <- gaugeValue(t, StreamingConnections)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/responses/resp_123", nil)
	req.Header.Set("Accept", "text/event-stream")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	duringRequest := <-inHandler
	afterRequest := gaugeValue(t, StreamingConnections)

	if duringRequest != baseline+1 {
		t.Errorf("expected streaming gauge=%f during request, got %f", baseline+1, duringRequest)
	}
	if afterRequest != baseline {
		t.Errorf("expected streaming gauge=%f after request, got %f", baseline, afterRequest)
	}
}

// TestMiddlewareCapturesStatusCode verifies that non-200 status codes are
// captured correctly in the status label.
func TestMiddlewareCapturesStatusCode(t *testing.T) {
	before := counterValue(t, RequestsTotal, "POST", "4xx", "unknown")

	handler := MetricsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))

	req := httptest.NewRequest("POST", "/v1/responses", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	after := counterValue(t, RequestsTotal, "POST", "4xx", "unknown")
	if after-before != 1 {
		t.Errorf("expected 4xx count to increase by 1, got delta=%f", after-before)
	}
}

// TestStatusWriterFlush verifies that the statusWriter Flush method
// delegates to the underlying writer when it implements http.Flusher.
func TestStatusWriterFlush(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec, status: http.StatusOK}

	// Should not panic even though it delegates to a Flusher.
	sw.Flush()

	if !rec.Flushed {
		t.Error("expected underlying writer to be flushed")
	}
}

// TestRecordGenAIMetrics verifies that the RecordGenAIMetrics helper records
// all expected OTel GenAI metrics in a single call.
func TestRecordGenAIMetrics(t *testing.T) {
	// Capture baselines.
	durationBefore := histogramCount(t, GenAIClientOperationDuration, "chat", "test-prov", "mdl", "mdl", "")
	inputBefore := histogramCount(t, GenAIClientTokenUsage, "chat", "test-prov", "input", "mdl", "mdl")
	outputBefore := histogramCount(t, GenAIClientTokenUsage, "chat", "test-prov", "output", "mdl", "mdl")
	ttftBefore := histogramCount(t, GenAIServerTimeToFirstToken, "chat", "test-prov", "mdl")
	perTokenBefore := histogramCount(t, GenAIServerTimePerOutputToken, "chat", "test-prov", "mdl")

	ttft := 150 * time.Millisecond
	RecordGenAIMetrics("test-prov", "mdl", 2*time.Second, 500, 200, &ttft)

	durationAfter := histogramCount(t, GenAIClientOperationDuration, "chat", "test-prov", "mdl", "mdl", "")
	inputAfter := histogramCount(t, GenAIClientTokenUsage, "chat", "test-prov", "input", "mdl", "mdl")
	outputAfter := histogramCount(t, GenAIClientTokenUsage, "chat", "test-prov", "output", "mdl", "mdl")
	ttftAfter := histogramCount(t, GenAIServerTimeToFirstToken, "chat", "test-prov", "mdl")
	perTokenAfter := histogramCount(t, GenAIServerTimePerOutputToken, "chat", "test-prov", "mdl")

	if durationAfter-durationBefore != 1 {
		t.Errorf("expected 1 duration observation, got delta=%d", durationAfter-durationBefore)
	}
	if inputAfter-inputBefore != 1 {
		t.Errorf("expected 1 input token observation, got delta=%d", inputAfter-inputBefore)
	}
	if outputAfter-outputBefore != 1 {
		t.Errorf("expected 1 output token observation, got delta=%d", outputAfter-outputBefore)
	}
	if ttftAfter-ttftBefore != 1 {
		t.Errorf("expected 1 TTFT observation, got delta=%d", ttftAfter-ttftBefore)
	}
	if perTokenAfter-perTokenBefore != 1 {
		t.Errorf("expected 1 per-token observation, got delta=%d", perTokenAfter-perTokenBefore)
	}
}

// TestRecordGenAIMetrics_NoTTFT verifies that RecordGenAIMetrics skips TTFT
// recording when ttft is nil (non-streaming case).
func TestRecordGenAIMetrics_NoTTFT(t *testing.T) {
	ttftBefore := histogramCount(t, GenAIServerTimeToFirstToken, "chat", "no-ttft-prov", "mdl2")

	RecordGenAIMetrics("no-ttft-prov", "mdl2", 1*time.Second, 100, 50, nil)

	ttftAfter := histogramCount(t, GenAIServerTimeToFirstToken, "chat", "no-ttft-prov", "mdl2")
	if ttftAfter-ttftBefore != 0 {
		t.Errorf("expected 0 TTFT observations without ttft, got delta=%d", ttftAfter-ttftBefore)
	}
}

// counterValue reads the current value of a CounterVec for the given labels.
func counterValue(t *testing.T, cv *prometheus.CounterVec, labels ...string) float64 {
	t.Helper()
	m := &dto.Metric{}
	c, err := cv.GetMetricWithLabelValues(labels...)
	if err != nil {
		t.Fatalf("getting counter metric: %v", err)
	}
	if err := c.(prometheus.Metric).Write(m); err != nil {
		t.Fatalf("writing counter metric: %v", err)
	}
	return m.GetCounter().GetValue()
}

// histogramCount reads the observation count from a HistogramVec.
func histogramCount(t *testing.T, hv *prometheus.HistogramVec, labels ...string) uint64 {
	t.Helper()
	m := &dto.Metric{}
	obs, err := hv.GetMetricWithLabelValues(labels...)
	if err != nil {
		t.Fatalf("getting histogram metric: %v", err)
	}
	if err := obs.(prometheus.Metric).Write(m); err != nil {
		t.Fatalf("writing histogram metric: %v", err)
	}
	return m.GetHistogram().GetSampleCount()
}

// gaugeValue reads the current value of a Gauge.
func gaugeValue(t *testing.T, g prometheus.Gauge) float64 {
	t.Helper()
	m := &dto.Metric{}
	if err := g.Write(m); err != nil {
		t.Fatalf("writing gauge metric: %v", err)
	}
	return m.GetGauge().GetValue()
}
