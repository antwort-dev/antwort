package agent

import (
	"context"
	"testing"
)

func TestVectorStoreIDsContext(t *testing.T) {
	ctx := context.Background()

	// No IDs set.
	if ids := GetVectorStoreIDs(ctx); ids != nil {
		t.Errorf("expected nil, got %v", ids)
	}

	// Set IDs.
	ctx = SetVectorStoreIDs(ctx, []string{"vs-1", "vs-2"})
	ids := GetVectorStoreIDs(ctx)
	if len(ids) != 2 || ids[0] != "vs-1" || ids[1] != "vs-2" {
		t.Errorf("expected [vs-1 vs-2], got %v", ids)
	}
}
