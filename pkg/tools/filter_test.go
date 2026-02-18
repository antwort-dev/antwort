package tools

import (
	"testing"
)

func TestFilterAllowedTools(t *testing.T) {
	tests := []struct {
		name         string
		calls        []ToolCall
		allowedTools []string
		wantAllowed  int
		wantRejected int
	}{
		{
			name: "all allowed when no filter",
			calls: []ToolCall{
				{ID: "c1", Name: "get_weather"},
				{ID: "c2", Name: "search"},
			},
			allowedTools: nil,
			wantAllowed:  2,
			wantRejected: 0,
		},
		{
			name: "all allowed when empty filter",
			calls: []ToolCall{
				{ID: "c1", Name: "get_weather"},
			},
			allowedTools: []string{},
			wantAllowed:  1,
			wantRejected: 0,
		},
		{
			name: "some rejected",
			calls: []ToolCall{
				{ID: "c1", Name: "get_weather"},
				{ID: "c2", Name: "delete_account"},
				{ID: "c3", Name: "search"},
			},
			allowedTools: []string{"get_weather", "search"},
			wantAllowed:  2,
			wantRejected: 1,
		},
		{
			name: "all rejected",
			calls: []ToolCall{
				{ID: "c1", Name: "delete_account"},
				{ID: "c2", Name: "drop_table"},
			},
			allowedTools: []string{"get_weather"},
			wantAllowed:  0,
			wantRejected: 2,
		},
		{
			name:         "empty calls",
			calls:        []ToolCall{},
			allowedTools: []string{"get_weather"},
			wantAllowed:  0,
			wantRejected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterAllowedTools(tt.calls, tt.allowedTools)

			if len(result.Allowed) != tt.wantAllowed {
				t.Errorf("allowed count = %d, want %d", len(result.Allowed), tt.wantAllowed)
			}
			if len(result.Rejected) != tt.wantRejected {
				t.Errorf("rejected count = %d, want %d", len(result.Rejected), tt.wantRejected)
			}

			// Verify rejected results have IsError=true and descriptive messages.
			for _, r := range result.Rejected {
				if !r.IsError {
					t.Errorf("rejected result for %q should have IsError=true", r.CallID)
				}
				if r.Output == "" {
					t.Errorf("rejected result for %q should have non-empty output", r.CallID)
				}
			}
		})
	}
}
