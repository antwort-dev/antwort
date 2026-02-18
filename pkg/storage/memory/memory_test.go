package memory

import (
	"context"
	"errors"
	"testing"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/storage"
)

func makeResponse(id string) *api.Response {
	return &api.Response{
		ID:     id,
		Object: "response",
		Status: api.ResponseStatusCompleted,
		Model:  "test-model",
		Input: []api.Item{
			{ID: "item_in", Type: api.ItemTypeMessage, Status: api.ItemStatusCompleted,
				Message: &api.MessageData{Role: api.RoleUser, Content: []api.ContentPart{{Type: "input_text", Text: "hello"}}}},
		},
		Output: []api.Item{
			{ID: "item_out", Type: api.ItemTypeMessage, Status: api.ItemStatusCompleted,
				Message: &api.MessageData{Role: api.RoleAssistant, Output: []api.OutputContentPart{{Type: "output_text", Text: "hi"}}}},
		},
		Usage:     &api.Usage{InputTokens: 5, OutputTokens: 2, TotalTokens: 7},
		CreatedAt: 1000,
	}
}

func TestSaveAndGet(t *testing.T) {
	s := New(0)
	ctx := context.Background()

	resp := makeResponse("resp_test1")
	if err := s.SaveResponse(ctx, resp); err != nil {
		t.Fatalf("SaveResponse failed: %v", err)
	}

	got, err := s.GetResponse(ctx, "resp_test1")
	if err != nil {
		t.Fatalf("GetResponse failed: %v", err)
	}

	if got.ID != "resp_test1" {
		t.Errorf("ID = %q, want %q", got.ID, "resp_test1")
	}
	if got.Model != "test-model" {
		t.Errorf("Model = %q, want %q", got.Model, "test-model")
	}
	if len(got.Input) != 1 {
		t.Errorf("len(Input) = %d, want 1", len(got.Input))
	}
	if len(got.Output) != 1 {
		t.Errorf("len(Output) = %d, want 1", len(got.Output))
	}
}

func TestGetNotFound(t *testing.T) {
	s := New(0)
	ctx := context.Background()

	_, err := s.GetResponse(ctx, "resp_missing")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSoftDelete(t *testing.T) {
	s := New(0)
	ctx := context.Background()

	resp := makeResponse("resp_del")
	s.SaveResponse(ctx, resp)

	// Delete.
	if err := s.DeleteResponse(ctx, "resp_del"); err != nil {
		t.Fatalf("DeleteResponse failed: %v", err)
	}

	// GetResponse should return not-found.
	_, err := s.GetResponse(ctx, "resp_del")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}

	// GetResponseForChain should still return the response.
	got, err := s.GetResponseForChain(ctx, "resp_del")
	if err != nil {
		t.Fatalf("GetResponseForChain should return deleted response: %v", err)
	}
	if got.ID != "resp_del" {
		t.Errorf("chain response ID = %q, want %q", got.ID, "resp_del")
	}
}

func TestDuplicateSave(t *testing.T) {
	s := New(0)
	ctx := context.Background()

	resp := makeResponse("resp_dup")
	s.SaveResponse(ctx, resp)

	err := s.SaveResponse(ctx, resp)
	if !errors.Is(err, storage.ErrConflict) {
		t.Errorf("expected ErrConflict for duplicate, got %v", err)
	}
}

func TestDeleteNotFound(t *testing.T) {
	s := New(0)
	ctx := context.Background()

	err := s.DeleteResponse(ctx, "resp_missing")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestHealthCheck(t *testing.T) {
	s := New(0)
	if err := s.HealthCheck(context.Background()); err != nil {
		t.Errorf("HealthCheck failed: %v", err)
	}
}

func TestLRUEviction(t *testing.T) {
	s := New(3) // max 3 entries
	ctx := context.Background()

	s.SaveResponse(ctx, makeResponse("resp_a"))
	s.SaveResponse(ctx, makeResponse("resp_b"))
	s.SaveResponse(ctx, makeResponse("resp_c"))

	// All three should be accessible.
	for _, id := range []string{"resp_a", "resp_b", "resp_c"} {
		if _, err := s.GetResponse(ctx, id); err != nil {
			t.Fatalf("expected %s to exist, got %v", id, err)
		}
	}

	// Save a 4th: oldest (resp_a) should be evicted.
	s.SaveResponse(ctx, makeResponse("resp_d"))

	if _, err := s.GetResponse(ctx, "resp_a"); !errors.Is(err, storage.ErrNotFound) {
		t.Error("expected resp_a to be evicted")
	}

	// resp_b, resp_c, resp_d should still exist.
	for _, id := range []string{"resp_b", "resp_c", "resp_d"} {
		if _, err := s.GetResponse(ctx, id); err != nil {
			t.Errorf("expected %s to exist after eviction, got %v", id, err)
		}
	}
}

func TestLRUEviction_Unlimited(t *testing.T) {
	s := New(0) // unlimited
	ctx := context.Background()

	for i := 0; i < 100; i++ {
		s.SaveResponse(ctx, makeResponse("resp_"+string(rune('a'+i))))
	}

	// All should exist (no eviction).
	s.mu.RLock()
	count := len(s.entries)
	s.mu.RUnlock()

	if count != 100 {
		t.Errorf("expected 100 entries, got %d", count)
	}
}

func TestTenantIsolation(t *testing.T) {
	s := New(0)

	ctxA := storage.SetTenant(context.Background(), "tenant-a")
	ctxB := storage.SetTenant(context.Background(), "tenant-b")
	ctxNone := context.Background()

	// Save for tenant A.
	s.SaveResponse(ctxA, makeResponse("resp_a1"))

	// Tenant A can retrieve.
	if _, err := s.GetResponse(ctxA, "resp_a1"); err != nil {
		t.Fatalf("tenant A should retrieve own response: %v", err)
	}

	// Tenant B cannot retrieve.
	if _, err := s.GetResponse(ctxB, "resp_a1"); !errors.Is(err, storage.ErrNotFound) {
		t.Error("tenant B should not see tenant A's response")
	}

	// No tenant (single-tenant mode) can retrieve.
	if _, err := s.GetResponse(ctxNone, "resp_a1"); err != nil {
		t.Fatalf("no-tenant context should see all responses: %v", err)
	}
}

func TestTenantIsolation_Delete(t *testing.T) {
	s := New(0)

	ctxA := storage.SetTenant(context.Background(), "tenant-a")
	ctxB := storage.SetTenant(context.Background(), "tenant-b")

	s.SaveResponse(ctxA, makeResponse("resp_a2"))

	// Tenant B cannot delete tenant A's response.
	if err := s.DeleteResponse(ctxB, "resp_a2"); !errors.Is(err, storage.ErrNotFound) {
		t.Error("tenant B should not delete tenant A's response")
	}

	// Tenant A can delete.
	if err := s.DeleteResponse(ctxA, "resp_a2"); err != nil {
		t.Fatalf("tenant A should delete own response: %v", err)
	}
}

func TestChainWithSoftDelete(t *testing.T) {
	s := New(0)
	ctx := context.Background()

	// Save chain: A -> B -> C
	respA := makeResponse("resp_chain_a")
	respB := makeResponse("resp_chain_b")
	respB.PreviousResponseID = "resp_chain_a"
	respC := makeResponse("resp_chain_c")
	respC.PreviousResponseID = "resp_chain_b"

	s.SaveResponse(ctx, respA)
	s.SaveResponse(ctx, respB)
	s.SaveResponse(ctx, respC)

	// Delete the middle one.
	s.DeleteResponse(ctx, "resp_chain_b")

	// GetResponse for B should return not-found.
	if _, err := s.GetResponse(ctx, "resp_chain_b"); !errors.Is(err, storage.ErrNotFound) {
		t.Error("expected GetResponse for deleted B to return not-found")
	}

	// GetResponseForChain for B should return the response (chain intact).
	got, err := s.GetResponseForChain(ctx, "resp_chain_b")
	if err != nil {
		t.Fatalf("GetResponseForChain for deleted B should work: %v", err)
	}
	if got.PreviousResponseID != "resp_chain_a" {
		t.Errorf("chain link broken: previous = %q, want %q", got.PreviousResponseID, "resp_chain_a")
	}
}
