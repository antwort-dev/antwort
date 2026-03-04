package memory

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/storage"
	"github.com/rhuss/antwort/pkg/transport"
)

// convOwnerAllowed checks if the caller is allowed to access a conversation with the given owner.
// Admin bypass only applies to read/delete operations (writeOp=false).
// Per FR-007, admin users must NOT modify resources owned by other users.
func convOwnerAllowed(ctx context.Context, storedOwner, resourceID, operation string, writeOp bool) bool {
	callerOwner := storage.GetOwner(ctx)
	if callerOwner == "" {
		return true
	}
	if !writeOp && storage.GetAdmin(ctx) {
		return true
	}
	if storedOwner == "" {
		return true
	}
	if callerOwner == storedOwner {
		return true
	}
	slog.Debug("ownership denied",
		"subject", callerOwner,
		"resource_id", resourceID,
		"operation", operation,
	)
	return false
}

// Compile-time check.
var _ transport.ConversationStore = (*ConversationStore)(nil)

type convEntry struct {
	conv      *api.Conversation
	tenantID  string
	owner     string
	items     []api.ConversationItem
	deletedAt *time.Time
}

// ConversationStore is an in-memory implementation of transport.ConversationStore.
type ConversationStore struct {
	mu      sync.RWMutex
	entries map[string]*convEntry
}

// NewConversationStore creates an empty in-memory conversation store.
func NewConversationStore() *ConversationStore {
	return &ConversationStore{
		entries: make(map[string]*convEntry),
	}
}

func (s *ConversationStore) SaveConversation(ctx context.Context, conv *api.Conversation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tenantID := storage.GetTenant(ctx)
	owner := storage.GetOwner(ctx)

	if existing, ok := s.entries[conv.ID]; ok {
		if existing.deletedAt != nil {
			return storage.ErrNotFound
		}
		if tenantID != "" && existing.tenantID != tenantID {
			return storage.ErrNotFound
		}
		if !convOwnerAllowed(ctx, existing.owner, conv.ID, "SaveConversation", true) {
			return storage.ErrNotFound
		}
		existing.conv = conv
		return nil
	}

	s.entries[conv.ID] = &convEntry{
		conv:     conv,
		tenantID: tenantID,
		owner:    owner,
	}
	return nil
}

func (s *ConversationStore) GetConversation(ctx context.Context, id string) (*api.Conversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, ok := s.entries[id]
	if !ok || e.deletedAt != nil {
		return nil, storage.ErrNotFound
	}

	tenantID := storage.GetTenant(ctx)
	if tenantID != "" && e.tenantID != tenantID {
		return nil, storage.ErrNotFound
	}

	if !convOwnerAllowed(ctx, e.owner, id, "GetConversation", false) {
		return nil, storage.ErrNotFound
	}

	return e.conv, nil
}

func (s *ConversationStore) DeleteConversation(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.entries[id]
	if !ok || e.deletedAt != nil {
		return storage.ErrNotFound
	}

	tenantID := storage.GetTenant(ctx)
	if tenantID != "" && e.tenantID != tenantID {
		return storage.ErrNotFound
	}

	if !convOwnerAllowed(ctx, e.owner, id, "DeleteConversation", false) {
		return storage.ErrNotFound
	}

	now := time.Now()
	e.deletedAt = &now
	return nil
}

func (s *ConversationStore) ListConversations(ctx context.Context, opts transport.ListOptions) (*transport.ConversationList, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tenantID := storage.GetTenant(ctx)
	callerOwner := storage.GetOwner(ctx)
	isAdminCaller := storage.GetAdmin(ctx)

	var all []*api.Conversation
	for _, e := range s.entries {
		if e.deletedAt != nil {
			continue
		}
		if tenantID != "" && e.tenantID != tenantID {
			continue
		}
		// Owner filtering for list.
		if callerOwner != "" && !isAdminCaller && e.owner != "" && e.owner != callerOwner {
			continue
		}
		all = append(all, e.conv)
	}

	// Sort by created_at.
	sort.Slice(all, func(i, j int) bool {
		if opts.Order == "asc" {
			return all[i].CreatedAt < all[j].CreatedAt
		}
		return all[i].CreatedAt > all[j].CreatedAt
	})

	// Pagination.
	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	start := 0
	if opts.After != "" {
		for i, c := range all {
			if c.ID == opts.After {
				start = i + 1
				break
			}
		}
	}

	end := start + limit
	hasMore := end < len(all)
	if end > len(all) {
		end = len(all)
	}

	page := all[start:end]

	result := &transport.ConversationList{
		Object:  "list",
		Data:    page,
		HasMore: hasMore,
	}
	if len(page) > 0 {
		result.FirstID = page[0].ID
		result.LastID = page[len(page)-1].ID
	}
	return result, nil
}

func (s *ConversationStore) AddItems(ctx context.Context, conversationID string, items []api.ConversationItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.entries[conversationID]
	if !ok || e.deletedAt != nil {
		return storage.ErrNotFound
	}

	tenantID := storage.GetTenant(ctx)
	if tenantID != "" && e.tenantID != tenantID {
		return storage.ErrNotFound
	}

	if !convOwnerAllowed(ctx, e.owner, conversationID, "AddItems", true) {
		return storage.ErrNotFound
	}

	// Assign positions starting from current length.
	basePos := len(e.items)
	now := time.Now().Unix()
	for i := range items {
		items[i].Position = basePos + i
		items[i].ConversationID = conversationID
		if items[i].CreatedAt == 0 {
			items[i].CreatedAt = now
		}
	}

	e.items = append(e.items, items...)
	e.conv.UpdatedAt = now

	return nil
}

func (s *ConversationStore) ListItems(ctx context.Context, conversationID string, opts transport.ListOptions) (*transport.ItemList, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, ok := s.entries[conversationID]
	if !ok || e.deletedAt != nil {
		return nil, storage.ErrNotFound
	}

	tenantID := storage.GetTenant(ctx)
	if tenantID != "" && e.tenantID != tenantID {
		return nil, storage.ErrNotFound
	}

	if !convOwnerAllowed(ctx, e.owner, conversationID, "ListItems", false) {
		return nil, storage.ErrNotFound
	}

	// Copy items for sorting.
	items := make([]api.Item, len(e.items))
	for i, ci := range e.items {
		items[i] = ci.Item
	}

	// Sort by position.
	if opts.Order == "desc" {
		sort.Slice(items, func(i, j int) bool {
			return i > j // reverse
		})
	}

	// Pagination.
	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	start := 0
	if opts.After != "" {
		for i, item := range items {
			if item.ID == opts.After {
				start = i + 1
				break
			}
		}
	}

	end := start + limit
	hasMore := end < len(items)
	if end > len(items) {
		end = len(items)
	}
	page := items[start:end]

	result := &transport.ItemList{
		Object:  "list",
		Data:    page,
		HasMore: hasMore,
	}
	if len(page) > 0 {
		result.FirstID = page[0].ID
		result.LastID = page[len(page)-1].ID
	}
	return result, nil
}

// ItemCount returns the number of items in a conversation.
func (s *ConversationStore) ItemCount(id string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.entries[id]
	if !ok {
		return 0
	}
	return len(e.items)
}

// AllItems returns all items in a conversation (for engine history reconstruction).
func (s *ConversationStore) AllItems(ctx context.Context, conversationID string) ([]api.ConversationItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, ok := s.entries[conversationID]
	if !ok || e.deletedAt != nil {
		return nil, fmt.Errorf("conversation %q not found", conversationID)
	}

	tenantID := storage.GetTenant(ctx)
	if tenantID != "" && e.tenantID != tenantID {
		return nil, fmt.Errorf("conversation %q not found", conversationID)
	}

	if !convOwnerAllowed(ctx, e.owner, conversationID, "AllItems", false) {
		return nil, fmt.Errorf("conversation %q not found", conversationID)
	}

	cp := make([]api.ConversationItem, len(e.items))
	copy(cp, e.items)
	return cp, nil
}
