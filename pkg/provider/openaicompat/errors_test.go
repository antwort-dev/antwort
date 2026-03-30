package openaicompat

import (
	"net/http"
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
		{
			name:  "whitespace around seconds",
			value: " 2 ",
			want:  2 * time.Second,
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

func TestParseRetryAfter_HTTPDate(t *testing.T) {
	// HTTP-date in RFC1123 format (future date should return positive duration).
	// http.ParseTime expects "GMT" suffix, not "UTC".
	futureDate := time.Now().Add(5 * time.Second).UTC().Format(http.TimeFormat)
	got := parseRetryAfter(futureDate)
	if got <= 0 || got > 6*time.Second {
		t.Errorf("parseRetryAfter(%q) = %v, want ~5s", futureDate, got)
	}

	// Past date should return 0.
	pastDate := time.Now().Add(-5 * time.Second).UTC().Format(http.TimeFormat)
	got = parseRetryAfter(pastDate)
	if got != 0 {
		t.Errorf("parseRetryAfter(past date %q) = %v, want 0", pastDate, got)
	}

	// Invalid date format.
	got = parseRetryAfter("Not-A-Date-At-All")
	if got != 0 {
		t.Errorf("parseRetryAfter(invalid date) = %v, want 0", got)
	}
}
