package filesearch

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
)

// VectorStore represents metadata about a vector store instance.
type VectorStore struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	TenantID       string `json:"tenant_id,omitempty"`
	CollectionName string `json:"collection_name"`
	CreatedAt      int64  `json:"created_at"`
}

// MetadataStore is an in-memory store for vector store metadata records.
type MetadataStore struct {
	mu     sync.RWMutex
	stores map[string]*VectorStore // id -> store
}

// NewMetadataStore creates a new empty MetadataStore.
func NewMetadataStore() *MetadataStore {
	return &MetadataStore{
		stores: make(map[string]*VectorStore),
	}
}

// Create adds a new VectorStore record. If the ID is empty, one is generated
// with a "vs_" prefix.
func (m *MetadataStore) Create(store *VectorStore) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if store.ID == "" {
		id, err := generateID("vs_")
		if err != nil {
			return fmt.Errorf("generating store ID: %w", err)
		}
		store.ID = id
	}

	if _, exists := m.stores[store.ID]; exists {
		return fmt.Errorf("vector store %q already exists", store.ID)
	}

	m.stores[store.ID] = store
	return nil
}

// Get retrieves a VectorStore by ID.
func (m *MetadataStore) Get(id string) (*VectorStore, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	store, ok := m.stores[id]
	if !ok {
		return nil, fmt.Errorf("vector store %q not found", id)
	}
	return store, nil
}

// List returns all VectorStores for the given tenant ID.
// If tenantID is empty, returns all stores (single-tenant mode).
func (m *MetadataStore) List(tenantID string) []*VectorStore {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*VectorStore
	for _, store := range m.stores {
		if tenantID == "" || store.TenantID == tenantID {
			result = append(result, store)
		}
	}
	return result
}

// Delete removes a VectorStore by ID.
func (m *MetadataStore) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.stores[id]; !ok {
		return fmt.Errorf("vector store %q not found", id)
	}
	delete(m.stores, id)
	return nil
}

// generateID creates a unique ID with the given prefix.
func generateID(prefix string) (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(b), nil
}
