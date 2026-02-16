package api

import (
	"strings"
	"testing"
)

func TestValidateResponseTransition(t *testing.T) {
	tests := []struct {
		name    string
		from    ResponseStatus
		to      ResponseStatus
		wantErr bool
	}{
		// Valid transitions
		{name: "initial to queued", from: "", to: ResponseStatusQueued, wantErr: false},
		{name: "initial to in_progress (skip queued)", from: "", to: ResponseStatusInProgress, wantErr: false},
		{name: "queued to in_progress", from: ResponseStatusQueued, to: ResponseStatusInProgress, wantErr: false},
		{name: "in_progress to completed", from: ResponseStatusInProgress, to: ResponseStatusCompleted, wantErr: false},
		{name: "in_progress to failed", from: ResponseStatusInProgress, to: ResponseStatusFailed, wantErr: false},
		{name: "in_progress to cancelled", from: ResponseStatusInProgress, to: ResponseStatusCancelled, wantErr: false},

		// Invalid transitions from terminal states
		{name: "completed to in_progress", from: ResponseStatusCompleted, to: ResponseStatusInProgress, wantErr: true},
		{name: "completed to failed", from: ResponseStatusCompleted, to: ResponseStatusFailed, wantErr: true},
		{name: "completed to queued", from: ResponseStatusCompleted, to: ResponseStatusQueued, wantErr: true},
		{name: "failed to in_progress", from: ResponseStatusFailed, to: ResponseStatusInProgress, wantErr: true},
		{name: "failed to completed", from: ResponseStatusFailed, to: ResponseStatusCompleted, wantErr: true},
		{name: "cancelled to in_progress", from: ResponseStatusCancelled, to: ResponseStatusInProgress, wantErr: true},
		{name: "cancelled to completed", from: ResponseStatusCancelled, to: ResponseStatusCompleted, wantErr: true},

		// Invalid transitions skipping required states or going backward
		{name: "queued to completed (skip in_progress)", from: ResponseStatusQueued, to: ResponseStatusCompleted, wantErr: true},
		{name: "queued to failed", from: ResponseStatusQueued, to: ResponseStatusFailed, wantErr: true},
		{name: "queued to cancelled", from: ResponseStatusQueued, to: ResponseStatusCancelled, wantErr: true},
		{name: "in_progress to queued (backward)", from: ResponseStatusInProgress, to: ResponseStatusQueued, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateResponseTransition(tt.from, tt.to)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateResponseTransition(%q, %q) = nil, want error", tt.from, tt.to)
				} else if !strings.Contains(err.Message, "invalid transition") {
					t.Errorf("error message %q does not contain \"invalid transition\"", err.Message)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateResponseTransition(%q, %q) = %v, want nil", tt.from, tt.to, err)
				}
			}
		})
	}
}

func TestValidateItemTransition(t *testing.T) {
	tests := []struct {
		name    string
		from    ItemStatus
		to      ItemStatus
		wantErr bool
	}{
		// Valid transitions
		{name: "initial to in_progress", from: "", to: ItemStatusInProgress, wantErr: false},
		{name: "in_progress to completed", from: ItemStatusInProgress, to: ItemStatusCompleted, wantErr: false},
		{name: "in_progress to incomplete", from: ItemStatusInProgress, to: ItemStatusIncomplete, wantErr: false},
		{name: "in_progress to failed", from: ItemStatusInProgress, to: ItemStatusFailed, wantErr: false},

		// Invalid transitions from terminal states
		{name: "completed to in_progress", from: ItemStatusCompleted, to: ItemStatusInProgress, wantErr: true},
		{name: "completed to failed", from: ItemStatusCompleted, to: ItemStatusFailed, wantErr: true},
		{name: "incomplete to in_progress", from: ItemStatusIncomplete, to: ItemStatusInProgress, wantErr: true},
		{name: "incomplete to completed", from: ItemStatusIncomplete, to: ItemStatusCompleted, wantErr: true},
		{name: "failed to in_progress", from: ItemStatusFailed, to: ItemStatusInProgress, wantErr: true},
		{name: "failed to completed", from: ItemStatusFailed, to: ItemStatusCompleted, wantErr: true},

		// Invalid self-transition
		{name: "in_progress to in_progress (self-transition)", from: ItemStatusInProgress, to: ItemStatusInProgress, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateItemTransition(tt.from, tt.to)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateItemTransition(%q, %q) = nil, want error", tt.from, tt.to)
				} else if !strings.Contains(err.Message, "invalid transition") {
					t.Errorf("error message %q does not contain \"invalid transition\"", err.Message)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateItemTransition(%q, %q) = %v, want nil", tt.from, tt.to, err)
				}
			}
		})
	}
}
