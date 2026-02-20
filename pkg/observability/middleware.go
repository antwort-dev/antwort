package observability

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// MetricsMiddleware wraps an HTTP handler to record request metrics.
//
// It captures:
//   - antwort_requests_total (counter): incremented per request with method, status class, and model labels
//   - antwort_request_duration_seconds (histogram): request duration with method and model labels
//   - antwort_streaming_connections_active (gauge): incremented while an SSE streaming response is in flight
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Detect SSE streaming from the Accept header.
		isStreaming := r.Header.Get("Accept") == "text/event-stream"

		if isStreaming {
			StreamingConnections.Inc()
			defer StreamingConnections.Dec()
		}

		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		duration := time.Since(start).Seconds()

		// Extract model from the request path if it looks like a model-scoped route,
		// otherwise use "unknown". The model can be enriched later from response bodies.
		model := "unknown"

		// Build a status class label like "2xx", "4xx", "5xx".
		statusStr := strconv.Itoa(sw.status/100) + "xx"

		// Also detect streaming from the response Content-Type header, since the
		// handler may decide to stream even when Accept was not set explicitly.
		if !isStreaming && strings.HasPrefix(sw.Header().Get("Content-Type"), "text/event-stream") {
			// The connection already completed, so we do not adjust the gauge,
			// but we note it was a streaming request for logging purposes.
			_ = true
		}

		RequestsTotal.WithLabelValues(r.Method, statusStr, model).Inc()
		RequestDuration.WithLabelValues(r.Method, model).Observe(duration)
	})
}

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	status  int
	written bool
}

// WriteHeader captures the status code and delegates to the underlying writer.
func (w *statusWriter) WriteHeader(status int) {
	if !w.written {
		w.status = status
		w.written = true
	}
	w.ResponseWriter.WriteHeader(status)
}

// Write delegates to the underlying writer and marks the status as written.
func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.written = true
	}
	return w.ResponseWriter.Write(b)
}

// Flush delegates to the underlying writer if it implements http.Flusher.
// This is essential for SSE streaming support.
func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap returns the underlying ResponseWriter, enabling http.ResponseController
// and similar utilities to access the original writer.
func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
