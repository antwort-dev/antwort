package files

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"strings"

	"github.com/rhuss/antwort/pkg/auth"
	"github.com/rhuss/antwort/pkg/storage"
)

// FileList is a paginated list of files.
type FileList struct {
	Object  string  `json:"object"`
	Data    []*File `json:"data"`
	HasMore bool    `json:"has_more"`
	FirstID string  `json:"first_id"`
	LastID  string  `json:"last_id"`
}

// ListOptions controls pagination and filtering for list operations.
type ListOptions struct {
	After   string
	Limit   int
	Order   string
	Purpose string
}

// FileMetadataStore provides CRUD operations for file metadata records.
// All operations are user-scoped via the identity in context.
type FileMetadataStore interface {
	// Save stores a new file record.
	Save(ctx context.Context, file *File) error

	// Get retrieves a file by ID, scoped to the authenticated user.
	Get(ctx context.Context, id string) (*File, error)

	// List returns files for the authenticated user with pagination.
	List(ctx context.Context, opts ListOptions) (*FileList, error)

	// Delete removes a file record by ID.
	Delete(ctx context.Context, id string) error

	// Update changes the status and optional error message of a file.
	Update(ctx context.Context, id string, status FileStatus, errMsg string) error
}

// MemoryMetadataStore is a thread-safe in-memory FileMetadataStore.
type MemoryMetadataStore struct {
	mu    sync.RWMutex
	files map[string]*File // id -> file
}

// NewMemoryMetadataStore creates a new empty in-memory metadata store.
func NewMemoryMetadataStore() *MemoryMetadataStore {
	return &MemoryMetadataStore{
		files: make(map[string]*File),
	}
}

func (m *MemoryMetadataStore) Save(_ context.Context, file *File) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.files[file.ID]; exists {
		return fmt.Errorf("file %q already exists", file.ID)
	}
	m.files[file.ID] = file
	return nil
}

func (m *MemoryMetadataStore) Get(ctx context.Context, id string) (*File, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	file, ok := m.files[id]
	if !ok {
		return nil, fmt.Errorf("file %q not found", id)
	}
	userID := userFromCtx(ctx)
	if userID != "" && file.UserID != userID {
		// Check group/others permissions before denying.
		callerTenant := storage.GetTenant(ctx)
		if !fileCanAccess(file.Permissions, userID, file.UserID, callerTenant, file.TenantID) {
			return nil, fmt.Errorf("file %q not found", id)
		}
	}
	return file, nil
}

func (m *MemoryMetadataStore) List(ctx context.Context, opts ListOptions) (*FileList, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	userID := userFromCtx(ctx)
	callerTenant := storage.GetTenant(ctx)

	// Collect matching files.
	var all []*File
	for _, f := range m.files {
		if userID != "" && f.UserID != userID {
			// Check group/others permissions for non-owner files.
			if !fileCanAccess(f.Permissions, userID, f.UserID, callerTenant, f.TenantID) {
				continue
			}
		}
		if opts.Purpose != "" && f.Purpose != opts.Purpose {
			continue
		}
		all = append(all, f)
	}

	// Sort by created_at descending (default) or ascending.
	sort.Slice(all, func(i, j int) bool {
		if opts.Order == "asc" {
			return all[i].CreatedAt < all[j].CreatedAt
		}
		return all[i].CreatedAt > all[j].CreatedAt
	})

	// Apply cursor-based pagination.
	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	start := 0
	if opts.After != "" {
		for i, f := range all {
			if f.ID == opts.After {
				start = i + 1
				break
			}
		}
	}

	end := start + limit
	hasMore := false
	if end < len(all) {
		hasMore = true
	}
	if end > len(all) {
		end = len(all)
	}

	page := all[start:end]

	result := &FileList{
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

func (m *MemoryMetadataStore) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.files, id)
	return nil
}

func (m *MemoryMetadataStore) Update(_ context.Context, id string, status FileStatus, errMsg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	file, ok := m.files[id]
	if !ok {
		return fmt.Errorf("file %q not found", id)
	}
	file.Status = status
	file.StatusError = errMsg
	file.UpdatedAt = time.Now().Unix()
	return nil
}

// userFromCtx extracts the user subject from context via auth identity.
func userFromCtx(ctx context.Context) string {
	id := auth.IdentityFromContext(ctx)
	if id == nil {
		return ""
	}
	return id.Subject
}

// fileCanAccess checks if a caller can access a file based on its permissions string.
// permissions format: "owner|group|others" where each segment is a subset of "rwd".
func fileCanAccess(permissions, callerOwner, resourceOwner, callerTenant, resourceTenant string) bool {
	if callerOwner != "" && callerOwner == resourceOwner {
		return true
	}
	if permissions == "" {
		permissions = DefaultFilePermissions
	}
	parts := strings.Split(permissions, "|")
	if len(parts) != 3 {
		return false
	}
	// Same tenant: check group permissions.
	if callerTenant != "" && callerTenant == resourceTenant {
		return strings.Contains(parts[1], "r")
	}
	// Different tenant: check others permissions.
	return strings.Contains(parts[2], "r")
}
