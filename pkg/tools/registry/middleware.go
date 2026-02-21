package registry

import (
	"net/http"
	"strconv"
	"time"
)

// wrapRoute wraps a Route's handler with metrics recording.
// It captures request count and duration per provider/method/path.
func wrapRoute(providerName string, route Route) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusCapture{ResponseWriter: w, status: http.StatusOK}

		route.Handler.ServeHTTP(sw, r)

		duration := time.Since(start).Seconds()
		statusStr := strconv.Itoa(sw.status)

		builtinAPIRequests.WithLabelValues(providerName, r.Method, route.Pattern, statusStr).Inc()
		builtinAPIDuration.WithLabelValues(providerName, r.Method, route.Pattern).Observe(duration)
	}
}

// statusCapture wraps http.ResponseWriter to capture the status code.
type statusCapture struct {
	http.ResponseWriter
	status  int
	written bool
}

// WriteHeader captures the status code and delegates to the underlying writer.
func (w *statusCapture) WriteHeader(status int) {
	if !w.written {
		w.status = status
		w.written = true
	}
	w.ResponseWriter.WriteHeader(status)
}

// Write delegates to the underlying writer and marks as written.
func (w *statusCapture) Write(b []byte) (int, error) {
	if !w.written {
		w.written = true
	}
	return w.ResponseWriter.Write(b)
}

// Flush delegates to the underlying writer if it implements http.Flusher.
func (w *statusCapture) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap returns the underlying ResponseWriter.
func (w *statusCapture) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
