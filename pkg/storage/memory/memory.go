// Package memory provides an in-memory implementation of transport.ResponseStore
// for testing and lightweight deployments. Responses are stored in memory and
// lost when the process restarts. Optional LRU eviction limits memory usage.
package memory

import (
	"context"
	"container/list"
	"sync"
	"time"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/storage"
	"github.com/rhuss/antwort/pkg/transport"
)

// entry holds a stored response and its metadata.
type entry struct {
	resp      *api.Response
	tenantID  string
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

	// Evict if at capacity.
	if s.maxSize > 0 && len(s.entries) >= s.maxSize {
		s.evictOldest()
	}

	elem := s.lruList.PushFront(resp.ID)
	s.entries[resp.ID] = &entry{
		resp:     resp,
		tenantID: tenantID,
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
