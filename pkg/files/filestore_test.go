package files

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

// ---------- MemoryFileStore ----------

func TestMemoryFileStore_StoreAndRetrieve(t *testing.T) {
	tests := []struct {
		name    string
		fileID  string
		content string
	}{
		{name: "simple text", fileID: "f-1", content: "hello world"},
		{name: "empty content", fileID: "f-2", content: ""},
		{name: "binary-like content", fileID: "f-3", content: "\x00\x01\x02\xff"},
		{name: "large content", fileID: "f-4", content: strings.Repeat("x", 100_000)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewMemoryFileStore()
			ctx := context.Background()

			err := store.Store(ctx, tt.fileID, strings.NewReader(tt.content))
			if err != nil {
				t.Fatalf("Store: unexpected error: %v", err)
			}

			rc, err := store.Retrieve(ctx, tt.fileID)
			if err != nil {
				t.Fatalf("Retrieve: unexpected error: %v", err)
			}
			defer rc.Close()

			got, err := io.ReadAll(rc)
			if err != nil {
				t.Fatalf("ReadAll: unexpected error: %v", err)
			}
			if string(got) != tt.content {
				t.Errorf("content mismatch: got %d bytes, want %d bytes", len(got), len(tt.content))
			}
		})
	}
}

func TestMemoryFileStore_RetrieveMissing(t *testing.T) {
	store := NewMemoryFileStore()
	_, err := store.Retrieve(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

func TestMemoryFileStore_Delete(t *testing.T) {
	store := NewMemoryFileStore()
	ctx := context.Background()

	_ = store.Store(ctx, "f-del", strings.NewReader("data"))

	err := store.Delete(ctx, "f-del")
	if err != nil {
		t.Fatalf("Delete: unexpected error: %v", err)
	}

	_, err = store.Retrieve(ctx, "f-del")
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}

func TestMemoryFileStore_DeleteNonexistent(t *testing.T) {
	store := NewMemoryFileStore()
	// Deleting a non-existent file should not error.
	err := store.Delete(context.Background(), "nope")
	if err != nil {
		t.Fatalf("Delete non-existent: unexpected error: %v", err)
	}
}

func TestMemoryFileStore_Overwrite(t *testing.T) {
	store := NewMemoryFileStore()
	ctx := context.Background()

	_ = store.Store(ctx, "f-ow", strings.NewReader("first"))
	_ = store.Store(ctx, "f-ow", strings.NewReader("second"))

	rc, _ := store.Retrieve(ctx, "f-ow")
	defer rc.Close()
	got, _ := io.ReadAll(rc)
	if string(got) != "second" {
		t.Errorf("expected overwrite: got %q, want %q", string(got), "second")
	}
}

// ---------- FilesystemStore ----------

func TestFilesystemStore_StoreAndRetrieve(t *testing.T) {
	tests := []struct {
		name    string
		fileID  string
		content string
	}{
		{name: "simple text", fileID: "f-1", content: "hello filesystem"},
		{name: "empty content", fileID: "f-2", content: ""},
		{name: "large content", fileID: "f-3", content: strings.Repeat("abc", 50_000)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			store, err := NewFilesystemStore(dir)
			if err != nil {
				t.Fatalf("NewFilesystemStore: %v", err)
			}
			ctx := context.Background()

			err = store.Store(ctx, tt.fileID, strings.NewReader(tt.content))
			if err != nil {
				t.Fatalf("Store: %v", err)
			}

			rc, err := store.Retrieve(ctx, tt.fileID)
			if err != nil {
				t.Fatalf("Retrieve: %v", err)
			}
			defer rc.Close()

			got, _ := io.ReadAll(rc)
			if string(got) != tt.content {
				t.Errorf("content mismatch: got %d bytes, want %d bytes", len(got), len(tt.content))
			}
		})
	}
}

func TestFilesystemStore_RetrieveMissing(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFilesystemStore(dir)

	_, err := store.Retrieve(context.Background(), "missing-file")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

func TestFilesystemStore_Delete(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFilesystemStore(dir)
	ctx := context.Background()

	_ = store.Store(ctx, "f-del", strings.NewReader("data"))

	err := store.Delete(ctx, "f-del")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = store.Retrieve(ctx, "f-del")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestFilesystemStore_DeleteNonexistent(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFilesystemStore(dir)

	err := store.Delete(context.Background(), "nope")
	if err != nil {
		t.Fatalf("Delete non-existent: unexpected error: %v", err)
	}
}

func TestFilesystemStore_StoreWithSubdirectory(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFilesystemStore(dir)
	ctx := context.Background()

	// File ID containing a path separator to test MkdirAll.
	fileID := "user1/file-abc"
	content := "nested directory file"

	err := store.Store(ctx, fileID, strings.NewReader(content))
	if err != nil {
		t.Fatalf("Store: %v", err)
	}

	rc, err := store.Retrieve(ctx, fileID)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	defer rc.Close()

	got, _ := io.ReadAll(rc)
	if string(got) != content {
		t.Errorf("got %q, want %q", string(got), content)
	}
}

// ---------- bytesReader ----------

func TestBytesReader(t *testing.T) {
	data := []byte("hello reader")
	r := newBytesReader(data)

	// Read in small chunks.
	buf := make([]byte, 5)
	n, err := r.Read(buf)
	if err != nil {
		t.Fatalf("first read: %v", err)
	}
	if n != 5 || string(buf[:n]) != "hello" {
		t.Errorf("first read: got %q, want %q", string(buf[:n]), "hello")
	}

	// Read the rest.
	rest, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(rest) != " reader" {
		t.Errorf("remainder: got %q, want %q", string(rest), " reader")
	}
}

func TestBytesReader_Isolation(t *testing.T) {
	original := []byte("abc")
	r := newBytesReader(original)

	// Mutate original to verify data was copied.
	original[0] = 'x'

	var buf bytes.Buffer
	io.Copy(&buf, r)
	if buf.String() != "abc" {
		t.Errorf("got %q, want %q (copy isolation failed)", buf.String(), "abc")
	}
}
