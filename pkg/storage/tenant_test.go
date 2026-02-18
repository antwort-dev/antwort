package storage

import (
	"context"
	"testing"
)

func TestSetGetTenant(t *testing.T) {
	ctx := context.Background()

	// No tenant set: empty string.
	if got := GetTenant(ctx); got != "" {
		t.Errorf("GetTenant(empty ctx) = %q, want %q", got, "")
	}

	// Set tenant.
	ctx = SetTenant(ctx, "tenant-abc")
	if got := GetTenant(ctx); got != "tenant-abc" {
		t.Errorf("GetTenant = %q, want %q", got, "tenant-abc")
	}

	// Override tenant.
	ctx = SetTenant(ctx, "tenant-xyz")
	if got := GetTenant(ctx); got != "tenant-xyz" {
		t.Errorf("GetTenant = %q, want %q", got, "tenant-xyz")
	}
}

func TestGetTenant_NoCollision(t *testing.T) {
	// Ensure the private key type prevents collisions.
	ctx := context.WithValue(context.Background(), "tenant", "wrong")
	if got := GetTenant(ctx); got != "" {
		t.Errorf("GetTenant should not match string key, got %q", got)
	}
}
