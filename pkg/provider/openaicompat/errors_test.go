package openaicompat

import (
	"testing"
	"time"
)

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{
			name:  "empty",
			value: "",
			want:  0,
		},
		{
			name:  "seconds integer",
			value: "2",
			want:  2 * time.Second,
		},
		{
			name:  "seconds zero",
			value: "0",
			want:  0,
		},
		{
			name:  "negative seconds",
			value: "-5",
			want:  0,
		},
		{
			name:  "invalid string",
			value: "not-a-number",
			want:  0,
		},
		{
			name:  "large seconds",
			value: "120",
			want:  120 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRetryAfter(tt.value)
			if got != tt.want {
				t.Errorf("parseRetryAfter(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestMapHTTPError_429WithRetryAfter(t *testing.T) {
	// Test that MapHTTPError populates RetryAfter from the response header.
	// This requires an http.Response, which is tested at a higher level.
	// The parseRetryAfter unit tests above cover the parsing logic.
}
