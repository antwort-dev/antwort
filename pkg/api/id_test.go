package api

import (
	"testing"
)

func TestNewResponseID(t *testing.T) {
	id := NewResponseID()
	if !ValidateResponseID(id) {
		t.Errorf("NewResponseID() = %q, want valid response ID", id)
	}
}

func TestNewItemID(t *testing.T) {
	id := NewItemID()
	if !ValidateItemID(id) {
		t.Errorf("NewItemID() = %q, want valid item ID", id)
	}
}

func TestValidateResponseID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want bool
	}{
		{"valid", "resp_abcdefghijklmnopqrstuvwx", true},
		{"valid mixed case", "resp_AbCdEfGhIjKlMnOpQrStUvWx", true},
		{"valid digits", "resp_123456789012345678901234", true},
		{"wrong prefix", "item_abcdefghijklmnopqrstuvwx", false},
		{"no prefix", "abcdefghijklmnopqrstuvwxyz1234", false},
		{"too short", "resp_abc", false},
		{"too long", "resp_abcdefghijklmnopqrstuvwxy", false},
		{"special chars", "resp_abcdefghijklmnopqrstuv!@", false},
		{"empty", "", false},
		{"prefix only", "resp_", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateResponseID(tt.id); got != tt.want {
				t.Errorf("ValidateResponseID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestValidateItemID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want bool
	}{
		{"valid", "item_abcdefghijklmnopqrstuvwx", true},
		{"valid mixed case", "item_AbCdEfGhIjKlMnOpQrStUvWx", true},
		{"valid digits", "item_123456789012345678901234", true},
		{"wrong prefix", "resp_abcdefghijklmnopqrstuvwx", false},
		{"no prefix", "abcdefghijklmnopqrstuvwxyz1234", false},
		{"too short", "item_abc", false},
		{"too long", "item_abcdefghijklmnopqrstuvwxy", false},
		{"special chars", "item_abcdefghijklmnopqrstuv!@", false},
		{"empty", "", false},
		{"prefix only", "item_", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateItemID(tt.id); got != tt.want {
				t.Errorf("ValidateItemID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestIDUniqueness(t *testing.T) {
	const count = 1000
	seen := make(map[string]bool, count)

	for i := 0; i < count; i++ {
		id := NewResponseID()
		if seen[id] {
			t.Fatalf("duplicate response ID after %d generations: %s", i, id)
		}
		seen[id] = true
	}

	seen = make(map[string]bool, count)
	for i := 0; i < count; i++ {
		id := NewItemID()
		if seen[id] {
			t.Fatalf("duplicate item ID after %d generations: %s", i, id)
		}
		seen[id] = true
	}
}
