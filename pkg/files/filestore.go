package files

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// FileStore persists and retrieves file binary content.
type FileStore interface {
	// Store saves the content for the given file ID.
	Store(ctx context.Context, fileID string, content io.Reader) error

	// Retrieve returns a reader for the content of the given file ID.
	Retrieve(ctx context.Context, fileID string) (io.ReadCloser, error)

	// Delete removes the content for the given file ID.
	Delete(ctx context.Context, fileID string) error
}

// MemoryFileStore is an in-memory FileStore for testing.
type MemoryFileStore struct {
	mu    sync.RWMutex
	files map[string][]byte
}

// NewMemoryFileStore creates a new empty in-memory file store.
func NewMemoryFileStore() *MemoryFileStore {
	return &MemoryFileStore{
		files: make(map[string][]byte),
	}
}

func (m *MemoryFileStore) Store(_ context.Context, fileID string, content io.Reader) error {
	data, err := io.ReadAll(content)
	if err != nil {
		return fmt.Errorf("reading content: %w", err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[fileID] = data
	return nil
}

func (m *MemoryFileStore) Retrieve(_ context.Context, fileID string) (io.ReadCloser, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.files[fileID]
	if !ok {
		return nil, fmt.Errorf("file %q not found", fileID)
	}
	return io.NopCloser(newBytesReader(data)), nil
}

func (m *MemoryFileStore) Delete(_ context.Context, fileID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.files, fileID)
	return nil
}

// bytesReader wraps a byte slice as an io.Reader (avoids importing bytes in the interface file).
type bytesReader struct {
	data []byte
	pos  int
}

func newBytesReader(data []byte) *bytesReader {
	cp := make([]byte, len(data))
	copy(cp, data)
	return &bytesReader{data: cp}
}

func (r *bytesReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// FilesystemStore stores files on the local filesystem with user-scoped subdirectories.
type FilesystemStore struct {
	baseDir string
}

// NewFilesystemStore creates a filesystem-backed file store rooted at baseDir.
func NewFilesystemStore(baseDir string) (*FilesystemStore, error) {
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating base directory: %w", err)
	}
	return &FilesystemStore{baseDir: baseDir}, nil
}

func (fs *FilesystemStore) path(fileID string) string {
	return filepath.Join(fs.baseDir, fileID)
}

func (fs *FilesystemStore) Store(_ context.Context, fileID string, content io.Reader) error {
	p := fs.path(fileID)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	f, err := os.Create(p)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, content); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}
	return nil
}

func (fs *FilesystemStore) Retrieve(_ context.Context, fileID string) (io.ReadCloser, error) {
	f, err := os.Open(fs.path(fileID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file %q not found", fileID)
		}
		return nil, fmt.Errorf("opening file: %w", err)
	}
	return f, nil
}

func (fs *FilesystemStore) Delete(_ context.Context, fileID string) error {
	err := os.Remove(fs.path(fileID))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing file: %w", err)
	}
	return nil
}
