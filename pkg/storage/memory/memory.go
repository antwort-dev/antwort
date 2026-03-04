// Package memory provides an in-memory implementation of transport.ResponseStore
// for testing and lightweight deployments. Responses are stored in memory and
// lost when the process restarts. Optional LRU eviction limits memory usage.
package memory

import (
	"context"
	"container/list"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/storage"
	"github.com/rhuss/antwort/pkg/transport"
)

// ownerAllowed checks if the caller in ctx is allowed to access a resource with the given owner.
// Returns true if: no identity is present (NoOp auth), stored owner is empty (legacy data),
// or owner matches. Admin bypass only applies to read/delete operations (writeOp=false).
// Per FR-007, admin users must NOT modify resources owned by other users.
func ownerAllowed(ctx context.Context, storedOwner, resourceID, operation string, writeOp bool) bool {
	callerOwner := storage.GetOwner(ctx)
	// No identity in context (NoOp auth): skip owner checks.
	if callerOwner == "" {
		return true
	}
	// Admin bypasses owner checks for read/delete only (FR-007).
	if !writeOp && storage.GetAdmin(ctx) {
		return true
	}
	// Legacy data with empty owner matches all authenticated users.
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

// entry holds a stored response and its metadata.
type entry struct {
	resp      *api.Response
	tenantID  string
	owner     string
	deletedAt *time.Time
	lruElem   *list.Element // position in LRU list
}

// Store is an in-memory ResponseStore with optional LRU eviction.
type Store struct {
	mu       sync.RWMutex
	entries  map[string]*entry
	lruList  *list.List // front = most recently used, back = least recently used
	maxSize  int        // 0 = unlimited
}

// Ensure Store implements transport.ResponseStore at compile time.
var _ transport.ResponseStore = (*Store)(nil)

// New creates a new in-memory store. If maxSize is 0, the store grows
// without limit. If maxSize > 0, the oldest entry is evicted when the
// limit is reached.
func New(maxSize int) *Store {
	return &Store{
		entries: make(map[string]*entry),
		lruList: list.New(),
		maxSize: maxSize,
	}
}

// SaveResponse persists a response in memory.
func (s *Store) SaveResponse(ctx context.Context, resp *api.Response) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.entries[resp.ID]; exists {
		return storage.ErrConflict
	}

	tenantID := storage.GetTenant(ctx)
	owner := storage.GetOwner(ctx)

	// Evict if at capacity.
	if s.maxSize > 0 && len(s.entries) >= s.maxSize {
		s.evictOldest()
	}

	elem := s.lruList.PushFront(resp.ID)
	s.entries[resp.ID] = &entry{
		resp:     resp,
		tenantID: tenantID,
		owner:    owner,
		lruElem:  elem,
	}

	return nil
}

// GetResponse retrieves a response by ID. Returns ErrNotFound if the
// response does not exist or has been soft-deleted. Scoped by tenant
// when a tenant is present in the context.
func (s *Store) GetResponse(ctx context.Context, id string) (*api.Response, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, ok := s.entries[id]
	if !ok || e.deletedAt != nil {
		return nil, storage.ErrNotFound
	}

	// Tenant scoping.
	tenantID := storage.GetTenant(ctx)
	if tenantID != "" && e.tenantID != tenantID {
		return nil, storage.ErrNotFound
	}

	// Owner scoping.
	if !ownerAllowed(ctx, e.owner, id, "GetResponse", false) {
		return nil, storage.ErrNotFound
	}

	return e.resp, nil
}

// GetResponseForChain retrieves a response by ID for chain reconstruction.
// Includes soft-deleted responses so chains remain intact.
func (s *Store) GetResponseForChain(ctx context.Context, id string) (*api.Response, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, ok := s.entries[id]
	if !ok {
		return nil, storage.ErrNotFound
	}

	// Tenant scoping (even for chain, respect tenant boundaries).
	tenantID := storage.GetTenant(ctx)
	if tenantID != "" && e.tenantID != tenantID {
		return nil, storage.ErrNotFound
	}

	// Owner scoping (prevent chaining to another user's response).
	if !ownerAllowed(ctx, e.owner, id, "GetResponseForChain", false) {
		return nil, storage.ErrNotFound
	}

	return e.resp, nil
}

// DeleteResponse soft-deletes a response. The response data remains
// available for chain reconstruction via GetResponseForChain.
func (s *Store) DeleteResponse(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.entries[id]
	if !ok {
		return storage.ErrNotFound
	}

	// Tenant scoping.
	tenantID := storage.GetTenant(ctx)
	if tenantID != "" && e.tenantID != tenantID {
		return storage.ErrNotFound
	}

	// Owner scoping.
	if !ownerAllowed(ctx, e.owner, id, "DeleteResponse", false) {
		return storage.ErrNotFound
	}

	now := time.Now()
	e.deletedAt = &now
	return nil
}

// HealthCheck always returns nil for the in-memory store.
func (s *Store) HealthCheck(_ context.Context) error {
	return nil
}

// Close is a no-op for the in-memory store.
func (s *Store) Close() error {
	return nil
}

// ListResponses returns a paginated list of stored responses filtered by
// tenant and optionally by model, with cursor-based pagination.
func (s *Store) ListResponses(ctx context.Context, opts transport.ListOptions) (*transport.ResponseList, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tenantID := storage.GetTenant(ctx)
	callerOwner := storage.GetOwner(ctx)
	isAdminCaller := storage.GetAdmin(ctx)

	// Collect matching entries.
	var matches []*api.Response
	for _, e := range s.entries {
		if e.deletedAt != nil {
			continue
		}
		if tenantID != "" && e.tenantID != tenantID {
			continue
		}
		// Owner filtering for list: skip entries not owned by caller.
		if callerOwner != "" && !isAdminCaller && e.owner != "" && e.owner != callerOwner {
			continue
		}
		if opts.Model != "" && e.resp.Model != opts.Model {
			continue
		}
		matches = append(matches, e.resp)
	}

	// Sort by created_at. Default is desc (newest first).
	asc := opts.Order == "asc"
	sort.Slice(matches, func(i, j int) bool {
		if asc {
			if matches[i].CreatedAt != matches[j].CreatedAt {
				return matches[i].CreatedAt < matches[j].CreatedAt
			}
			return matches[i].ID < matches[j].ID
		}
		if matches[i].CreatedAt != matches[j].CreatedAt {
			return matches[i].CreatedAt > matches[j].CreatedAt
		}
		return matches[i].ID > matches[j].ID
	})

	// Apply cursor-based pagination.
	if opts.After != "" {
		idx := -1
		for i, r := range matches {
			if r.ID == opts.After {
				idx = i
				break
			}
		}
		if idx >= 0 {
			matches = matches[idx+1:]
		} else {
			matches = nil
		}
	} else if opts.Before != "" {
		idx := -1
		for i, r := range matches {
			if r.ID == opts.Before {
				idx = i
				break
			}
		}
		if idx > 0 {
			matches = matches[:idx]
		} else {
			matches = nil
		}
	}

	// Apply limit.
	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	hasMore := len(matches) > limit
	if hasMore {
		matches = matches[:limit]
	}

	result := &transport.ResponseList{
		Object:  "list",
		Data:    matches,
		HasMore: hasMore,
	}
	if len(matches) > 0 {
		result.FirstID = matches[0].ID
		result.LastID = matches[len(matches)-1].ID
	}
	if result.Data == nil {
		result.Data = []*api.Response{}
	}

	return result, nil
}

// GetInputItems returns a paginated list of input items for a stored response.
func (s *Store) GetInputItems(ctx context.Context, responseID string, opts transport.ListOptions) (*transport.ItemList, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, ok := s.entries[responseID]
	if !ok || e.deletedAt != nil {
		return nil, storage.ErrNotFound
	}

	tenantID := storage.GetTenant(ctx)
	if tenantID != "" && e.tenantID != tenantID {
		return nil, storage.ErrNotFound
	}

	// Owner scoping.
	if !ownerAllowed(ctx, e.owner, responseID, "GetInputItems", false) {
		return nil, storage.ErrNotFound
	}

	items := e.resp.Input

	// Apply cursor-based pagination using item IDs.
	if opts.After != "" {
		idx := -1
		for i, item := range items {
			if item.ID == opts.After {
				idx = i
				break
			}
		}
		if idx >= 0 {
			items = items[idx+1:]
		} else {
			items = nil
		}
	} else if opts.Before != "" {
		idx := -1
		for i, item := range items {
			if item.ID == opts.Before {
				idx = i
				break
			}
		}
		if idx > 0 {
			items = items[:idx]
		} else {
			items = nil
		}
	}

	// Apply limit.
	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	result := &transport.ItemList{
		Object:  "list",
		Data:    items,
		HasMore: hasMore,
	}
	if len(items) > 0 {
		result.FirstID = items[0].ID
		result.LastID = items[len(items)-1].ID
	}
	if result.Data == nil {
		result.Data = []api.Item{}
	}

	return result, nil
}

// evictOldest removes the least recently used entry.
// Must be called with s.mu held.
func (s *Store) evictOldest() {
	back := s.lruList.Back()
	if back == nil {
		return
	}

	id := back.Value.(string)
	s.lruList.Remove(back)
	delete(s.entries, id)
}
