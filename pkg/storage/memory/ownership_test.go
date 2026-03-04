package memory

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/storage"
	"github.com/rhuss/antwort/pkg/transport"
)

// --- helpers ---

func ctxWithOwner(owner string) context.Context {
	return storage.SetOwner(context.Background(), owner)
}

func ctxWithTenantOwner(tenant, owner string) context.Context {
	ctx := storage.SetTenant(context.Background(), tenant)
	return storage.SetOwner(ctx, owner)
}

func ctxWithAdmin(tenant, owner string) context.Context {
	ctx := ctxWithTenantOwner(tenant, owner)
	return storage.SetAdmin(ctx, true)
}

func makeResp(id string, createdAt int64) *api.Response {
	return &api.Response{
		ID:        id,
		Object:    "response",
		Status:    api.ResponseStatusCompleted,
		Model:     "test-model",
		CreatedAt: createdAt,
	}
}

func makeConv(id string, createdAt int64) *api.Conversation {
	return &api.Conversation{
		ID:        id,
		Object:    "conversation",
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
}

// --- Phase 4: Admin Override Tests (T029-T032) ---

func TestAdminCanListAllResponsesInTenant(t *testing.T) {
	// T029: Admin can list all responses in their tenant.
	s := New(0)

	ctxUser1 := ctxWithTenantOwner("tenant-a", "user-1")
	ctxUser2 := ctxWithTenantOwner("tenant-a", "user-2")
	ctxAdmin := ctxWithAdmin("tenant-a", "admin-user")

	s.SaveResponse(ctxUser1, makeResp("resp_u1", 1000))
	s.SaveResponse(ctxUser2, makeResp("resp_u2", 2000))

	// Admin sees both.
	list, err := s.ListResponses(ctxAdmin, transport.ListOptions{})
	if err != nil {
		t.Fatalf("ListResponses failed: %v", err)
	}
	if len(list.Data) != 2 {
		t.Errorf("admin should see 2 responses, got %d", len(list.Data))
	}

	// Regular user-1 only sees their own.
	list, err = s.ListResponses(ctxUser1, transport.ListOptions{})
	if err != nil {
		t.Fatalf("ListResponses failed: %v", err)
	}
	if len(list.Data) != 1 {
		t.Errorf("user-1 should see 1 response, got %d", len(list.Data))
	}
	if list.Data[0].ID != "resp_u1" {
		t.Errorf("user-1 should see resp_u1, got %s", list.Data[0].ID)
	}
}

func TestAdminCanDeleteAnotherUsersResponse(t *testing.T) {
	// T030: Admin can delete another user's response.
	s := New(0)

	ctxUser := ctxWithTenantOwner("tenant-a", "user-1")
	ctxAdmin := ctxWithAdmin("tenant-a", "admin-user")

	s.SaveResponse(ctxUser, makeResp("resp_owned", 1000))

	// Admin deletes user's response.
	if err := s.DeleteResponse(ctxAdmin, "resp_owned"); err != nil {
		t.Fatalf("admin should delete another user's response: %v", err)
	}

	// Verify it's deleted.
	_, err := s.GetResponse(ctxUser, "resp_owned")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("expected ErrNotFound after admin delete, got %v", err)
	}
}

func TestAdminCannotAddItemsToAnotherUsersConversation(t *testing.T) {
	// T031: Admin cannot add items to another user's conversation (write op blocked by FR-007).
	cs := NewConversationStore()

	ctxUser := ctxWithTenantOwner("tenant-a", "user-1")
	ctxAdmin := ctxWithAdmin("tenant-a", "admin-user")

	conv := makeConv("conv_owned", 1000)
	cs.SaveConversation(ctxUser, conv)

	// Admin tries to add items (write op).
	items := []api.ConversationItem{
		{Item: api.Item{ID: "item_1", Type: api.ItemTypeMessage}},
	}
	err := cs.AddItems(ctxAdmin, "conv_owned", items)
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("admin write to another user's conversation should return ErrNotFound, got %v", err)
	}
}

func TestAdminCrossTenantIsolation(t *testing.T) {
	// T032: Admin in tenant-a cannot see resources from tenant-b.
	s := New(0)
	cs := NewConversationStore()

	ctxUserB := ctxWithTenantOwner("tenant-b", "user-b")
	ctxAdminA := ctxWithAdmin("tenant-a", "admin-a")

	// Create resources in tenant-b.
	s.SaveResponse(ctxUserB, makeResp("resp_b", 1000))
	cs.SaveConversation(ctxUserB, makeConv("conv_b", 1000))

	// Admin in tenant-a cannot get response from tenant-b.
	_, err := s.GetResponse(ctxAdminA, "resp_b")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Error("admin-a should not see tenant-b response")
	}

	// Admin in tenant-a cannot list responses from tenant-b.
	list, err := s.ListResponses(ctxAdminA, transport.ListOptions{})
	if err != nil {
		t.Fatalf("ListResponses failed: %v", err)
	}
	if len(list.Data) != 0 {
		t.Errorf("admin-a should see 0 responses from tenant-b, got %d", len(list.Data))
	}

	// Admin in tenant-a cannot get conversation from tenant-b.
	_, err = cs.GetConversation(ctxAdminA, "conv_b")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Error("admin-a should not see tenant-b conversation")
	}
}

// --- Phase 5: Backward Compatibility Tests (T033-T036) ---

func TestNilIdentitySkipsOwnerChecks(t *testing.T) {
	// T033 + T035: NoOp auth (no identity in context) skips ownership checks.
	s := New(0)
	cs := NewConversationStore()
	ctxNone := context.Background()

	// Save with no identity.
	s.SaveResponse(ctxNone, makeResp("resp_noauth", 1000))
	cs.SaveConversation(ctxNone, makeConv("conv_noauth", 1000))

	// Retrieve with no identity.
	if _, err := s.GetResponse(ctxNone, "resp_noauth"); err != nil {
		t.Errorf("no-auth should access response: %v", err)
	}

	if _, err := cs.GetConversation(ctxNone, "conv_noauth"); err != nil {
		t.Errorf("no-auth should access conversation: %v", err)
	}

	// List with no identity.
	list, err := s.ListResponses(ctxNone, transport.ListOptions{})
	if err != nil {
		t.Fatalf("ListResponses failed: %v", err)
	}
	if len(list.Data) != 1 {
		t.Errorf("no-auth should see 1 response, got %d", len(list.Data))
	}

	// Delete with no identity.
	if err := s.DeleteResponse(ctxNone, "resp_noauth"); err != nil {
		t.Errorf("no-auth should delete response: %v", err)
	}
}

func TestEmptyOwnerMatchesAllAuthenticatedUsers(t *testing.T) {
	// T034 + T036: Resources with empty owner field are accessible to all authenticated users.
	s := New(0)
	cs := NewConversationStore()

	// Save with no identity (simulating legacy/pre-ownership data with empty owner).
	ctxNone := context.Background()
	s.SaveResponse(ctxNone, makeResp("resp_legacy", 1000))
	cs.SaveConversation(ctxNone, makeConv("conv_legacy", 1000))

	// Any authenticated user can access these.
	ctxAlice := ctxWithOwner("alice")
	ctxBob := ctxWithOwner("bob")

	for _, name := range []string{"alice", "bob"} {
		ctx := ctxAlice
		if name == "bob" {
			ctx = ctxBob
		}
		t.Run(fmt.Sprintf("%s_reads_legacy_response", name), func(t *testing.T) {
			if _, err := s.GetResponse(ctx, "resp_legacy"); err != nil {
				t.Errorf("%s should access empty-owner response: %v", name, err)
			}
		})
		t.Run(fmt.Sprintf("%s_reads_legacy_conversation", name), func(t *testing.T) {
			if _, err := cs.GetConversation(ctx, "conv_legacy"); err != nil {
				t.Errorf("%s should access empty-owner conversation: %v", name, err)
			}
		})
	}

	// Authenticated user sees legacy data in list.
	list, err := s.ListResponses(ctxAlice, transport.ListOptions{})
	if err != nil {
		t.Fatalf("ListResponses failed: %v", err)
	}
	if len(list.Data) != 1 {
		t.Errorf("authenticated user should see 1 legacy response, got %d", len(list.Data))
	}
}

// --- Phase 6: Owner Auto-Assignment Tests (T037-T039) ---

func TestOwnerSetFromIdentitySubject(t *testing.T) {
	// T037 + T038 + T039: Owner is always set from Identity.Subject (via storage.GetOwner),
	// not from any request body field. The owner field is immutable.
	s := New(0)

	// Save with owner from context (simulating Identity.Subject = "real-user").
	ctxUser := ctxWithOwner("real-user")
	resp := makeResp("resp_owned_test", 1000)
	s.SaveResponse(ctxUser, resp)

	// Verify that a different user cannot access this response.
	ctxOther := ctxWithOwner("other-user")
	_, err := s.GetResponse(ctxOther, "resp_owned_test")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Error("other-user should not access real-user's response")
	}

	// Verify the actual owner can access it.
	got, err := s.GetResponse(ctxUser, "resp_owned_test")
	if err != nil {
		t.Fatalf("owner should access own response: %v", err)
	}
	if got.ID != "resp_owned_test" {
		t.Errorf("wrong response ID: %s", got.ID)
	}

	// Verify owner immutability: the stored entry has the original owner.
	s.mu.RLock()
	e := s.entries["resp_owned_test"]
	s.mu.RUnlock()
	if e.owner != "real-user" {
		t.Errorf("stored owner should be 'real-user', got %q", e.owner)
	}
}

// --- Phase 7: Consistent 404 Tests (T040-T041) ---

func TestConsistent404ForOwnershipDenialVsNotFound(t *testing.T) {
	// T040 + T041: Ownership-denied responses use the same ErrNotFound as genuinely missing resources.
	s := New(0)
	cs := NewConversationStore()

	ctxUser := ctxWithOwner("user-1")
	ctxOther := ctxWithOwner("user-2")

	s.SaveResponse(ctxUser, makeResp("resp_owned404", 1000))
	cs.SaveConversation(ctxUser, makeConv("conv_owned404", 1000))

	// Ownership denial on response.
	_, errDenied := s.GetResponse(ctxOther, "resp_owned404")
	_, errMissing := s.GetResponse(ctxOther, "resp_truly_missing")
	if !errors.Is(errDenied, storage.ErrNotFound) {
		t.Errorf("ownership denial should return ErrNotFound, got %v", errDenied)
	}
	if !errors.Is(errMissing, storage.ErrNotFound) {
		t.Errorf("missing resource should return ErrNotFound, got %v", errMissing)
	}
	if errDenied.Error() != errMissing.Error() {
		t.Errorf("error messages should match: denied=%q, missing=%q", errDenied.Error(), errMissing.Error())
	}

	// Ownership denial on delete.
	errDeniedDel := s.DeleteResponse(ctxOther, "resp_owned404")
	errMissingDel := s.DeleteResponse(ctxOther, "resp_truly_missing_del")
	if !errors.Is(errDeniedDel, storage.ErrNotFound) {
		t.Errorf("ownership denial on delete should return ErrNotFound, got %v", errDeniedDel)
	}
	if errDeniedDel.Error() != errMissingDel.Error() {
		t.Errorf("delete error messages should match: denied=%q, missing=%q", errDeniedDel.Error(), errMissingDel.Error())
	}

	// Ownership denial on conversation.
	_, errConvDenied := cs.GetConversation(ctxOther, "conv_owned404")
	_, errConvMissing := cs.GetConversation(ctxOther, "conv_truly_missing")
	if !errors.Is(errConvDenied, storage.ErrNotFound) {
		t.Errorf("conv ownership denial should return ErrNotFound, got %v", errConvDenied)
	}
	if errConvDenied.Error() != errConvMissing.Error() {
		t.Errorf("conv error messages should match: denied=%q, missing=%q", errConvDenied.Error(), errConvMissing.Error())
	}
}

// --- Additional edge case tests ---

func TestAdminCanGetAnotherUsersResponse(t *testing.T) {
	// Admin can read (GET) another user's response (read, not write).
	s := New(0)

	ctxUser := ctxWithTenantOwner("tenant-a", "user-1")
	ctxAdmin := ctxWithAdmin("tenant-a", "admin-user")

	s.SaveResponse(ctxUser, makeResp("resp_adminread", 1000))

	got, err := s.GetResponse(ctxAdmin, "resp_adminread")
	if err != nil {
		t.Fatalf("admin should read another user's response: %v", err)
	}
	if got.ID != "resp_adminread" {
		t.Errorf("wrong response ID: %s", got.ID)
	}
}

func TestAdminCanGetAnotherUsersConversation(t *testing.T) {
	// Admin can read another user's conversation (read, not write).
	cs := NewConversationStore()

	ctxUser := ctxWithTenantOwner("tenant-a", "user-1")
	ctxAdmin := ctxWithAdmin("tenant-a", "admin-user")

	cs.SaveConversation(ctxUser, makeConv("conv_adminread", 1000))

	got, err := cs.GetConversation(ctxAdmin, "conv_adminread")
	if err != nil {
		t.Fatalf("admin should read another user's conversation: %v", err)
	}
	if got.ID != "conv_adminread" {
		t.Errorf("wrong conversation ID: %s", got.ID)
	}
}

func TestAdminCanDeleteAnotherUsersConversation(t *testing.T) {
	// Admin can delete another user's conversation (delete, not write).
	cs := NewConversationStore()

	ctxUser := ctxWithTenantOwner("tenant-a", "user-1")
	ctxAdmin := ctxWithAdmin("tenant-a", "admin-user")

	cs.SaveConversation(ctxUser, makeConv("conv_admindel", 1000))

	if err := cs.DeleteConversation(ctxAdmin, "conv_admindel"); err != nil {
		t.Fatalf("admin should delete another user's conversation: %v", err)
	}
}

func TestAdminCannotSaveToAnotherUsersConversation(t *testing.T) {
	// Admin cannot update (SaveConversation on existing) another user's conversation.
	cs := NewConversationStore()

	ctxUser := ctxWithTenantOwner("tenant-a", "user-1")
	ctxAdmin := ctxWithAdmin("tenant-a", "admin-user")

	cs.SaveConversation(ctxUser, makeConv("conv_adminwrite", 1000))

	// Admin tries to overwrite.
	updated := makeConv("conv_adminwrite", time.Now().Unix())
	updated.Name = "admin-modified"
	err := cs.SaveConversation(ctxAdmin, updated)
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("admin write to another user's conversation should return ErrNotFound, got %v", err)
	}
}

func TestOwnerAllowedFunction(t *testing.T) {
	// Direct test of ownerAllowed helper.
	tests := []struct {
		name        string
		owner       string // caller owner in context
		isAdmin     bool
		storedOwner string
		writeOp     bool
		want        bool
	}{
		{"no identity", "", false, "user-1", false, true},
		{"owner match", "user-1", false, "user-1", false, true},
		{"owner mismatch", "user-2", false, "user-1", false, false},
		{"admin read", "admin", true, "user-1", false, true},
		{"admin write blocked", "admin", true, "user-1", true, false},
		{"empty stored owner", "user-1", false, "", false, true},
		{"admin empty stored owner", "admin", true, "", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.owner != "" {
				ctx = storage.SetOwner(ctx, tt.owner)
			}
			if tt.isAdmin {
				ctx = storage.SetAdmin(ctx, true)
			}

			got := ownerAllowed(ctx, tt.storedOwner, "test-id", "test-op", tt.writeOp)
			if got != tt.want {
				t.Errorf("ownerAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}
