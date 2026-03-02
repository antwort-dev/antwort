package files

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// VectorStoreFileRecord tracks the relationship between a file and a vector store.
type VectorStoreFileRecord struct {
	VectorStoreID string     `json:"vector_store_id"`
	FileID        string     `json:"id"`
	Object        string     `json:"object"`
	Status        FileStatus `json:"status"`
	ChunkCount    int        `json:"chunk_count,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
	BatchID       string     `json:"batch_id,omitempty"`
	CreatedAt     int64      `json:"created_at"`
}

// NewVectorStoreFileRecord creates a record with in_progress status.
func NewVectorStoreFileRecord(vsID, fileID string) *VectorStoreFileRecord {
	return &VectorStoreFileRecord{
		VectorStoreID: vsID,
		FileID:        fileID,
		Object:        "vector_store.file",
		Status:        FileStatusProcessing,
		CreatedAt:     time.Now().Unix(),
	}
}

// VectorStoreFileStore tracks file-to-vector-store relationships.
type VectorStoreFileStore interface {
	// Save stores a new file-to-store record.
	Save(ctx context.Context, rec *VectorStoreFileRecord) error

	// Get retrieves a record for the given vector store and file.
	Get(ctx context.Context, vsID, fileID string) (*VectorStoreFileRecord, error)

	// List returns all file records for the given vector store.
	List(ctx context.Context, vsID string) ([]*VectorStoreFileRecord, error)

	// Delete removes a file-to-store record.
	Delete(ctx context.Context, vsID, fileID string) error

	// ListByFile returns all store records for the given file ID.
	ListByFile(ctx context.Context, fileID string) ([]*VectorStoreFileRecord, error)

	// ListByBatch returns all records associated with the given batch ID.
	ListByBatch(ctx context.Context, batchID string) ([]*VectorStoreFileRecord, error)
}

// key builds a composite key for the in-memory map.
func vsFileKey(vsID, fileID string) string {
	return vsID + "|" + fileID
}

// MemoryVectorStoreFileStore is a thread-safe in-memory VectorStoreFileStore.
type MemoryVectorStoreFileStore struct {
	mu      sync.RWMutex
	records map[string]*VectorStoreFileRecord // "vsID|fileID" -> record
}

// NewMemoryVectorStoreFileStore creates a new empty in-memory store.
func NewMemoryVectorStoreFileStore() *MemoryVectorStoreFileStore {
	return &MemoryVectorStoreFileStore{
		records: make(map[string]*VectorStoreFileRecord),
	}
}

func (m *MemoryVectorStoreFileStore) Save(_ context.Context, rec *VectorStoreFileRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := vsFileKey(rec.VectorStoreID, rec.FileID)
	if _, exists := m.records[key]; exists {
		// Update existing record.
		m.records[key] = rec
		return nil
	}
	m.records[key] = rec
	return nil
}

func (m *MemoryVectorStoreFileStore) Get(_ context.Context, vsID, fileID string) (*VectorStoreFileRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rec, ok := m.records[vsFileKey(vsID, fileID)]
	if !ok {
		return nil, fmt.Errorf("file %q not found in vector store %q", fileID, vsID)
	}
	return rec, nil
}

func (m *MemoryVectorStoreFileStore) List(_ context.Context, vsID string) ([]*VectorStoreFileRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*VectorStoreFileRecord
	for _, rec := range m.records {
		if rec.VectorStoreID == vsID {
			result = append(result, rec)
		}
	}
	return result, nil
}

func (m *MemoryVectorStoreFileStore) Delete(_ context.Context, vsID, fileID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.records, vsFileKey(vsID, fileID))
	return nil
}

func (m *MemoryVectorStoreFileStore) ListByFile(_ context.Context, fileID string) ([]*VectorStoreFileRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*VectorStoreFileRecord
	for _, rec := range m.records {
		if rec.FileID == fileID {
			result = append(result, rec)
		}
	}
	return result, nil
}

func (m *MemoryVectorStoreFileStore) ListByBatch(_ context.Context, batchID string) ([]*VectorStoreFileRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*VectorStoreFileRecord
	for _, rec := range m.records {
		if rec.BatchID == batchID {
			result = append(result, rec)
		}
	}
	return result, nil
}
