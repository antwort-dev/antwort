//go:build cluster

package cluster

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// TestResult captures the outcome of a single test case.
type TestResult struct {
	Name         string         `json:"name"`
	Category     string         `json:"category"`
	ProviderPath string         `json:"provider_path,omitempty"`
	Passed       bool           `json:"passed"`
	Duration     time.Duration  `json:"duration_ns"`
	DurationMS   float64        `json:"duration_ms"`
	TTFT         time.Duration  `json:"ttft_ns,omitempty"`
	TTFTMS       float64        `json:"ttft_ms,omitempty"`
	Error        string         `json:"error,omitempty"`
	Details      map[string]any `json:"details,omitempty"`
}

// CategoryScore summarizes results for a test category.
type CategoryScore struct {
	Passed int     `json:"passed"`
	Total  int     `json:"total"`
	Score  float64 `json:"score"`
}

// LatencyStats holds percentile latency data.
type LatencyStats struct {
	NonStreamingP50 float64 `json:"non_streaming_p50_ms"`
	NonStreamingP95 float64 `json:"non_streaming_p95_ms"`
	NonStreamingP99 float64 `json:"non_streaming_p99_ms"`
	StreamingTTFTP50 float64 `json:"streaming_ttft_p50_ms,omitempty"`
	StreamingTTFTP95 float64 `json:"streaming_ttft_p95_ms,omitempty"`
	StreamingTTFTP99 float64 `json:"streaming_ttft_p99_ms,omitempty"`
}

// FailureDetail records information about a failed test.
type FailureDetail struct {
	TestName string `json:"test_name"`
	Category string `json:"category"`
	Error    string `json:"error"`
	Expected string `json:"expected,omitempty"`
	Got      string `json:"got,omitempty"`
}

// ResultSummary is the JSON output written after a validation run.
type ResultSummary struct {
	Model          string                   `json:"model"`
	AntwortVersion string                   `json:"antwort_version"`
	Cluster        string                   `json:"cluster"`
	Timestamp      string                   `json:"timestamp"`
	Categories     map[string]CategoryScore `json:"categories"`
	Latency        LatencyStats             `json:"latency"`
	Failures       []FailureDetail          `json:"failures"`
	TotalPassed    int                      `json:"total_passed"`
	TotalTests     int                      `json:"total_tests"`
}

// ResultCollector aggregates test results in a thread-safe manner.
type ResultCollector struct {
	mu             sync.Mutex
	model          string
	antwortVersion string
	cluster        string
	startTime      time.Time
	results        []TestResult
}

// NewResultCollector creates a new collector.
func NewResultCollector(model, version, cluster string) *ResultCollector {
	return &ResultCollector{
		model:          model,
		antwortVersion: version,
		cluster:        cluster,
		startTime:      time.Now(),
	}
}

// Record adds a test result to the collector.
func (rc *ResultCollector) Record(r TestResult) {
	r.DurationMS = float64(r.Duration.Milliseconds())
	if r.TTFT > 0 {
		r.TTFTMS = float64(r.TTFT.Milliseconds())
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.results = append(rc.results, r)
}

// Summary generates a ResultSummary from collected results.
func (rc *ResultCollector) Summary() ResultSummary {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	categories := make(map[string]CategoryScore)
	var failures []FailureDetail
	var nonStreamingLatencies []float64
	var streamingTTFTs []float64

	for _, r := range rc.results {
		cat := categories[r.Category]
		cat.Total++
		if r.Passed {
			cat.Passed++
		} else {
			failures = append(failures, FailureDetail{
				TestName: r.Name,
				Category: r.Category,
				Error:    r.Error,
			})
		}
		cat.Score = float64(cat.Passed) / float64(cat.Total)
		categories[r.Category] = cat

		if r.DurationMS > 0 && r.TTFTMS == 0 {
			nonStreamingLatencies = append(nonStreamingLatencies, r.DurationMS)
		}
		if r.TTFTMS > 0 {
			streamingTTFTs = append(streamingTTFTs, r.TTFTMS)
		}
	}

	totalPassed := 0
	totalTests := 0
	for _, cat := range categories {
		totalPassed += cat.Passed
		totalTests += cat.Total
	}

	summary := ResultSummary{
		Model:          rc.model,
		AntwortVersion: rc.antwortVersion,
		Cluster:        rc.cluster,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		Categories:     categories,
		Failures:       failures,
		TotalPassed:    totalPassed,
		TotalTests:     totalTests,
	}

	if len(nonStreamingLatencies) > 0 {
		summary.Latency.NonStreamingP50 = percentile(nonStreamingLatencies, 50)
		summary.Latency.NonStreamingP95 = percentile(nonStreamingLatencies, 95)
		summary.Latency.NonStreamingP99 = percentile(nonStreamingLatencies, 99)
	}
	if len(streamingTTFTs) > 0 {
		summary.Latency.StreamingTTFTP50 = percentile(streamingTTFTs, 50)
		summary.Latency.StreamingTTFTP95 = percentile(streamingTTFTs, 95)
		summary.Latency.StreamingTTFTP99 = percentile(streamingTTFTs, 99)
	}

	return summary
}

// WriteJSON writes the summary to a timestamped JSON file in the given directory.
func (rc *ResultCollector) WriteJSON(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating results directory: %w", err)
	}

	summary := rc.Summary()
	filename := fmt.Sprintf("%s_%s.json",
		time.Now().UTC().Format("2006-01-02T15-04-05"),
		sanitizeFilename(rc.model))

	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling results: %w", err)
	}

	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing results file: %w", err)
	}

	fmt.Printf("Results written to %s\n", path)
	return nil
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	rank := (p / 100.0) * float64(len(sorted)-1)
	lower := int(math.Floor(rank))
	upper := int(math.Ceil(rank))

	if lower == upper || upper >= len(sorted) {
		return sorted[lower]
	}

	frac := rank - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}

func sanitizeFilename(s string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' {
			result = append(result, c)
		} else {
			result = append(result, '_')
		}
	}
	return string(result)
}
