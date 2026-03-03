package files

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rhuss/antwort/pkg/vectorstore"
)

// stubExtractor is a minimal ContentExtractor for testing extractor selection.
type stubExtractor struct {
	name    string
	formats []string
}

func (s *stubExtractor) Extract(_ context.Context, _, _ string, content io.Reader) (*ExtractionResult, error) {
	data, _ := io.ReadAll(content)
	return &ExtractionResult{
		Text:   string(data),
		Method: s.name,
	}, nil
}

func (s *stubExtractor) SupportedFormats() []string {
	return s.formats
}

func TestSelectExtractor(t *testing.T) {
	passthrough := &stubExtractor{name: "passthrough"}
	docling := &stubExtractor{name: "docling"}

	pipeline := NewIngestionPipeline(PipelineConfig{
		Passthrough: passthrough,
		Docling:     docling,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	tests := []struct {
		name     string
		mimeType string
		wantName string
		wantNil  bool
	}{
		{
			name:     "application/pdf routes to docling",
			mimeType: "application/pdf",
			wantName: "docling",
		},
		{
			name:     "DOCX routes to docling",
			mimeType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			wantName: "docling",
		},
		{
			name:     "PPTX routes to docling",
			mimeType: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
			wantName: "docling",
		},
		{
			name:     "XLSX routes to docling",
			mimeType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			wantName: "docling",
		},
		{
			name:     "image/png routes to docling",
			mimeType: "image/png",
			wantName: "docling",
		},
		{
			name:     "text/plain routes to passthrough",
			mimeType: "text/plain",
			wantName: "passthrough",
		},
		{
			name:     "text/markdown routes to passthrough",
			mimeType: "text/markdown",
			wantName: "passthrough",
		},
		{
			name:     "text/csv routes to passthrough",
			mimeType: "text/csv",
			wantName: "passthrough",
		},
		{
			name:     "text/html routes to passthrough",
			mimeType: "text/html",
			wantName: "passthrough",
		},
		{
			name:     "application/json routes to passthrough",
			mimeType: "application/json",
			wantName: "passthrough",
		},
		{
			name:     "unknown MIME type falls back to passthrough",
			mimeType: "application/octet-stream",
			wantName: "passthrough",
		},
		{
			name:     "completely unknown type falls back to passthrough",
			mimeType: "application/x-custom-format",
			wantName: "passthrough",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext := pipeline.selectExtractor(tt.mimeType)
			if tt.wantNil {
				if ext != nil {
					t.Errorf("expected nil extractor, got %v", ext)
				}
				return
			}
			if ext == nil {
				t.Fatal("expected non-nil extractor, got nil")
			}
			stub, ok := ext.(*stubExtractor)
			if !ok {
				t.Fatalf("expected *stubExtractor, got %T", ext)
			}
			if stub.name != tt.wantName {
				t.Errorf("expected extractor %q, got %q", tt.wantName, stub.name)
			}
		})
	}
}

func TestSelectExtractor_NilDocling(t *testing.T) {
	passthrough := &stubExtractor{name: "passthrough"}

	pipeline := NewIngestionPipeline(PipelineConfig{
		Passthrough: passthrough,
		Docling:     nil, // docling not configured
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	tests := []struct {
		name     string
		mimeType string
		wantNil  bool
		wantName string
	}{
		{
			name:     "application/pdf with nil docling returns nil",
			mimeType: "application/pdf",
			wantNil:  true,
		},
		{
			name:     "image/jpeg with nil docling returns nil",
			mimeType: "image/jpeg",
			wantNil:  true,
		},
		{
			name:     "text/plain still routes to passthrough",
			mimeType: "text/plain",
			wantName: "passthrough",
		},
		{
			name:     "unknown type still routes to passthrough",
			mimeType: "application/octet-stream",
			wantName: "passthrough",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext := pipeline.selectExtractor(tt.mimeType)
			if tt.wantNil {
				if ext != nil {
					t.Errorf("expected nil extractor, got %v", ext)
				}
				return
			}
			if ext == nil {
				t.Fatal("expected non-nil extractor, got nil")
			}
			stub := ext.(*stubExtractor)
			if stub.name != tt.wantName {
				t.Errorf("expected extractor %q, got %q", tt.wantName, stub.name)
			}
		})
	}
}

// stubEmbedder returns fixed vectors for testing.
type stubEmbedder struct {
	dim int
}

func (s *stubEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i := range texts {
		result[i] = make([]float32, s.dim)
	}
	return result, nil
}

func (s *stubEmbedder) Dimensions() int { return s.dim }

// stubIndexer collects indexed points for verification.
type stubIndexer struct {
	mu     sync.Mutex
	points map[string][]VectorPoint
}

func newStubIndexer() *stubIndexer {
	return &stubIndexer{points: make(map[string][]VectorPoint)}
}

func (s *stubIndexer) CreateCollection(_ context.Context, _ string, _ int) error { return nil }
func (s *stubIndexer) DeleteCollection(_ context.Context, _ string) error               { return nil }
func (s *stubIndexer) Search(_ context.Context, _ string, _ []float32, _ int) ([]vectorstore.SearchMatch, error) {
	return nil, nil
}

func (s *stubIndexer) UpsertPoints(_ context.Context, collection string, points []VectorPoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.points[collection] = append(s.points[collection], points...)
	return nil
}

func (s *stubIndexer) DeletePointsByFile(_ context.Context, _ string, _ string) error {
	return nil
}

func TestIngestionPipeline_DegradationNilDoclingTextFile(t *testing.T) {
	// Nil docling + text file should succeed via passthrough.
	fileStore := NewMemoryFileStore()
	metaStore := NewMemoryMetadataStore()
	vsFileStore := NewMemoryVectorStoreFileStore()
	indexer := newStubIndexer()

	file := NewFile("file-1", "readme.txt", "text/plain", "assistants", "", 13)
	metaStore.Save(context.Background(), file)
	fileStore.Store(context.Background(), "file-1", strings.NewReader("Hello, world!"))

	vsRec := NewVectorStoreFileRecord("vs-1", "file-1")
	vsFileStore.Save(context.Background(), vsRec)

	pipeline := NewIngestionPipeline(PipelineConfig{
		FileStore:   fileStore,
		Metadata:    metaStore,
		VSFileStore: vsFileStore,
		Passthrough: NewPassthroughExtractor(),
		Docling:     nil, // not configured
		Chunker:     NewFixedSizeChunker(800, 0),
		Embedding:   &stubEmbedder{dim: 4},
		Indexer:     indexer,
		VSLookup:    func(vsID string) (string, error) { return "collection-" + vsID, nil },
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	err := pipeline.ingest(context.Background(), file, "vs-1")
	if err != nil {
		t.Fatalf("expected successful ingestion for text file without docling, got: %v", err)
	}

	// Verify the file was indexed.
	pts := indexer.points["collection-vs-1"]
	if len(pts) == 0 {
		t.Error("expected indexed points, got none")
	}
}

func TestIngestionPipeline_DegradationNilDoclingPDF(t *testing.T) {
	// Nil docling + PDF should fail with a descriptive error.
	fileStore := NewMemoryFileStore()
	metaStore := NewMemoryMetadataStore()
	vsFileStore := NewMemoryVectorStoreFileStore()

	file := NewFile("file-2", "report.pdf", "application/pdf", "assistants", "", 1024)
	metaStore.Save(context.Background(), file)
	fileStore.Store(context.Background(), "file-2", bytes.NewReader([]byte("%PDF-1.4 fake")))

	vsRec := NewVectorStoreFileRecord("vs-2", "file-2")
	vsFileStore.Save(context.Background(), vsRec)

	pipeline := NewIngestionPipeline(PipelineConfig{
		FileStore:   fileStore,
		Metadata:    metaStore,
		VSFileStore: vsFileStore,
		Passthrough: NewPassthroughExtractor(),
		Docling:     nil, // not configured
		Chunker:     NewFixedSizeChunker(800, 0),
		Embedding:   &stubEmbedder{dim: 4},
		Indexer:     newStubIndexer(),
		VSLookup:    func(vsID string) (string, error) { return "collection-" + vsID, nil },
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	err := pipeline.ingest(context.Background(), file, "vs-2")
	if err == nil {
		t.Fatal("expected error for PDF without docling, got nil")
	}
	if !strings.Contains(err.Error(), "requires an external extraction service") {
		t.Errorf("expected error mentioning external extraction service, got: %q", err.Error())
	}
}

func TestIngestionPipeline_DegradationDoclingUnreachable(t *testing.T) {
	// Docling configured but connection refused should produce an "unreachable" error.
	fileStore := NewMemoryFileStore()
	metaStore := NewMemoryMetadataStore()
	vsFileStore := NewMemoryVectorStoreFileStore()

	file := NewFile("file-3", "report.pdf", "application/pdf", "assistants", "", 1024)
	metaStore.Save(context.Background(), file)
	fileStore.Store(context.Background(), "file-3", bytes.NewReader([]byte("%PDF-1.4 fake")))

	vsRec := NewVectorStoreFileRecord("vs-3", "file-3")
	vsFileStore.Save(context.Background(), vsRec)

	// Point to a port where nothing is listening.
	docling := NewDoclingExtractor("http://127.0.0.1:1", "", false, 2*time.Second)

	pipeline := NewIngestionPipeline(PipelineConfig{
		FileStore:   fileStore,
		Metadata:    metaStore,
		VSFileStore: vsFileStore,
		Passthrough: NewPassthroughExtractor(),
		Docling:     docling,
		Chunker:     NewFixedSizeChunker(800, 0),
		Embedding:   &stubEmbedder{dim: 4},
		Indexer:     newStubIndexer(),
		VSLookup:    func(vsID string) (string, error) { return "collection-" + vsID, nil },
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	err := pipeline.ingest(context.Background(), file, "vs-3")
	if err == nil {
		t.Fatal("expected error for unreachable docling, got nil")
	}
	if !strings.Contains(err.Error(), "unreachable") {
		t.Errorf("expected error containing 'unreachable', got: %q", err.Error())
	}
}

func TestIngestionPipeline_FullPassthroughFlow(t *testing.T) {
	// Verify full pipeline flow for a text file: extract, chunk, embed, index.
	fileStore := NewMemoryFileStore()
	metaStore := NewMemoryMetadataStore()
	vsFileStore := NewMemoryVectorStoreFileStore()
	indexer := newStubIndexer()

	content := strings.Repeat("word ", 100) // ~500 chars
	file := NewFile("file-4", "notes.md", "text/markdown", "assistants", "", int64(len(content)))
	metaStore.Save(context.Background(), file)
	fileStore.Store(context.Background(), "file-4", strings.NewReader(content))

	vsRec := NewVectorStoreFileRecord("vs-4", "file-4")
	vsFileStore.Save(context.Background(), vsRec)

	pipeline := NewIngestionPipeline(PipelineConfig{
		FileStore:   fileStore,
		Metadata:    metaStore,
		VSFileStore: vsFileStore,
		Passthrough: NewPassthroughExtractor(),
		Docling:     nil,
		Chunker:     NewFixedSizeChunker(800, 0),
		Embedding:   &stubEmbedder{dim: 4},
		Indexer:     indexer,
		VSLookup:    func(vsID string) (string, error) { return "coll-" + vsID, nil },
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	err := pipeline.ingest(context.Background(), file, "vs-4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify points were indexed.
	pts := indexer.points["coll-vs-4"]
	if len(pts) == 0 {
		t.Error("expected at least one indexed point")
	}
	for _, pt := range pts {
		if pt.Metadata["file_id"] != "file-4" {
			t.Errorf("expected file_id=file-4, got %q", pt.Metadata["file_id"])
		}
		if pt.Metadata["filename"] != "notes.md" {
			t.Errorf("expected filename=notes.md, got %q", pt.Metadata["filename"])
		}
	}

	// Verify status was updated to completed.
	rec, err := vsFileStore.Get(context.Background(), "vs-4", "file-4")
	if err != nil {
		t.Fatalf("getting vsfile record: %v", err)
	}
	if rec.Status != FileStatusCompleted {
		t.Errorf("expected status %q, got %q", FileStatusCompleted, rec.Status)
	}
}

func TestIngestionPipeline_EmbeddingError(t *testing.T) {
	// Verify pipeline handles embedding errors gracefully.
	fileStore := NewMemoryFileStore()
	metaStore := NewMemoryMetadataStore()
	vsFileStore := NewMemoryVectorStoreFileStore()

	file := NewFile("file-5", "test.txt", "text/plain", "assistants", "", 5)
	metaStore.Save(context.Background(), file)
	fileStore.Store(context.Background(), "file-5", strings.NewReader("hello"))

	vsRec := NewVectorStoreFileRecord("vs-5", "file-5")
	vsFileStore.Save(context.Background(), vsRec)

	failingEmbedder := &failEmbedder{err: fmt.Errorf("embedding service unavailable")}

	pipeline := NewIngestionPipeline(PipelineConfig{
		FileStore:   fileStore,
		Metadata:    metaStore,
		VSFileStore: vsFileStore,
		Passthrough: NewPassthroughExtractor(),
		Chunker:     NewFixedSizeChunker(800, 0),
		Embedding:   failingEmbedder,
		Indexer:     newStubIndexer(),
		VSLookup:    func(vsID string) (string, error) { return "coll-" + vsID, nil },
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	err := pipeline.ingest(context.Background(), file, "vs-5")
	if err == nil {
		t.Fatal("expected error from embedding failure, got nil")
	}
	if !strings.Contains(err.Error(), "embedding") {
		t.Errorf("expected error mentioning 'embedding', got: %q", err.Error())
	}
}

type failEmbedder struct {
	err error
}

func (f *failEmbedder) Embed(_ context.Context, _ []string) ([][]float32, error) {
	return nil, f.err
}

func (f *failEmbedder) Dimensions() int { return 4 }

// ---------- Async Ingest tests ----------

// waitForFileStatus polls the metadata store until the file reaches the expected status
// or the timeout expires.
func waitForFileStatus(t *testing.T, metadata *MemoryMetadataStore, fileID string, want FileStatus, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		f, err := metadata.Get(context.Background(), fileID)
		if err == nil && f.Status == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("file %s did not reach status %q within %v", fileID, want, timeout)
}

func TestIngestionPipeline_AsyncStatusTransitions(t *testing.T) {
	var mu sync.Mutex
	var transitions []FileStatus

	fileStore := NewMemoryFileStore()
	metaStore := NewMemoryMetadataStore()
	vsFileStore := NewMemoryVectorStoreFileStore()

	// Wrap metadata Update to capture transitions.
	tracked := &statusTracker{
		FileMetadataStore: metaStore,
		onUpdate: func(status FileStatus) {
			mu.Lock()
			transitions = append(transitions, status)
			mu.Unlock()
		},
	}

	pipeline := NewIngestionPipeline(PipelineConfig{
		FileStore:   fileStore,
		Metadata:    tracked,
		VSFileStore: vsFileStore,
		Passthrough: NewPassthroughExtractor(),
		Chunker:     NewFixedSizeChunker(100, 0),
		Embedding:   &stubEmbedder{dim: 4},
		Indexer:     newStubIndexer(),
		VSLookup:    func(vsID string) (string, error) { return "coll-" + vsID, nil },
		Workers:     1,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	ctx := context.Background()
	file := NewFile("file-trans", "test.txt", "text/plain", "assistants", "", 12)
	_ = fileStore.Store(ctx, file.ID, strings.NewReader("test content"))
	_ = metaStore.Save(ctx, file)
	rec := NewVectorStoreFileRecord("vs-1", file.ID)
	_ = vsFileStore.Save(ctx, rec)

	// Trigger async ingestion.
	pipeline.Ingest(file, "vs-1")

	// Wait for completion.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		done := len(transitions) >= 2
		mu.Unlock()
		if done {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(transitions) < 2 {
		t.Fatalf("expected at least 2 status transitions, got %d: %v", len(transitions), transitions)
	}
	if transitions[0] != FileStatusProcessing {
		t.Errorf("first transition: got %q, want %q", transitions[0], FileStatusProcessing)
	}
	if transitions[len(transitions)-1] != FileStatusCompleted {
		t.Errorf("last transition: got %q, want %q", transitions[len(transitions)-1], FileStatusCompleted)
	}
}

func TestIngestionPipeline_AsyncExtractionFailureSetsStatusFailed(t *testing.T) {
	fileStore := NewMemoryFileStore()
	metaStore := NewMemoryMetadataStore()
	vsFileStore := NewMemoryVectorStoreFileStore()

	// Extraction always fails.
	failExtractor := &stubExtractorWithError{err: fmt.Errorf("extraction error")}

	pipeline := NewIngestionPipeline(PipelineConfig{
		FileStore:   fileStore,
		Metadata:    metaStore,
		VSFileStore: vsFileStore,
		Passthrough: failExtractor,
		Chunker:     NewFixedSizeChunker(100, 0),
		Embedding:   &stubEmbedder{dim: 4},
		Indexer:     newStubIndexer(),
		VSLookup:    func(vsID string) (string, error) { return "coll-" + vsID, nil },
		Workers:     1,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	ctx := context.Background()
	file := NewFile("file-async-ext-fail", "broken.txt", "text/plain", "assistants", "", 50)
	_ = fileStore.Store(ctx, file.ID, strings.NewReader("content"))
	_ = metaStore.Save(ctx, file)
	rec := NewVectorStoreFileRecord("vs-1", file.ID)
	_ = vsFileStore.Save(ctx, rec)

	pipeline.Ingest(file, "vs-1")
	waitForFileStatus(t, metaStore, file.ID, FileStatusFailed, 5*time.Second)

	f, _ := metaStore.Get(ctx, file.ID)
	if f.Status != FileStatusFailed {
		t.Errorf("file status: got %q, want %q", f.Status, FileStatusFailed)
	}
	if f.StatusError == "" {
		t.Error("expected status error to be set")
	}
}

func TestIngestionPipeline_AsyncEmbeddingFailureSetsStatusFailed(t *testing.T) {
	fileStore := NewMemoryFileStore()
	metaStore := NewMemoryMetadataStore()
	vsFileStore := NewMemoryVectorStoreFileStore()

	pipeline := NewIngestionPipeline(PipelineConfig{
		FileStore:   fileStore,
		Metadata:    metaStore,
		VSFileStore: vsFileStore,
		Passthrough: NewPassthroughExtractor(),
		Chunker:     NewFixedSizeChunker(100, 0),
		Embedding:   &failEmbedder{err: fmt.Errorf("embedding service unavailable")},
		Indexer:     newStubIndexer(),
		VSLookup:    func(vsID string) (string, error) { return "coll-" + vsID, nil },
		Workers:     1,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	ctx := context.Background()
	file := NewFile("file-async-emb-fail", "test.txt", "text/plain", "assistants", "", 18)
	_ = fileStore.Store(ctx, file.ID, strings.NewReader("some text to embed"))
	_ = metaStore.Save(ctx, file)
	rec := NewVectorStoreFileRecord("vs-1", file.ID)
	_ = vsFileStore.Save(ctx, rec)

	pipeline.Ingest(file, "vs-1")
	waitForFileStatus(t, metaStore, file.ID, FileStatusFailed, 5*time.Second)

	f, _ := metaStore.Get(ctx, file.ID)
	if f.Status != FileStatusFailed {
		t.Errorf("file status: got %q, want %q", f.Status, FileStatusFailed)
	}
}

func TestIngestionPipeline_WorkerPoolConcurrency(t *testing.T) {
	const maxWorkers = 2
	const totalFiles = 6

	var currentWorkers atomic.Int32
	var maxSeen atomic.Int32

	slowExtractor := &slowStubExtractor{
		delay:  50 * time.Millisecond,
		result: &ExtractionResult{Text: "content", Method: "mock"},
		onStart: func() {
			cur := currentWorkers.Add(1)
			for {
				seen := maxSeen.Load()
				if cur <= seen || maxSeen.CompareAndSwap(seen, cur) {
					break
				}
			}
		},
		onDone: func() {
			currentWorkers.Add(-1)
		},
	}

	fileStore := NewMemoryFileStore()
	metaStore := NewMemoryMetadataStore()
	vsFileStore := NewMemoryVectorStoreFileStore()

	pipeline := NewIngestionPipeline(PipelineConfig{
		FileStore:   fileStore,
		Metadata:    metaStore,
		VSFileStore: vsFileStore,
		Passthrough: slowExtractor,
		Chunker:     NewFixedSizeChunker(100, 0),
		Embedding:   &stubEmbedder{dim: 4},
		Indexer:     newStubIndexer(),
		VSLookup:    func(vsID string) (string, error) { return "coll-" + vsID, nil },
		Workers:     maxWorkers,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	ctx := context.Background()
	for i := 0; i < totalFiles; i++ {
		id := fmt.Sprintf("file-conc-%d", i)
		file := NewFile(id, "test.txt", "text/plain", "assistants", "", 10)
		_ = fileStore.Store(ctx, id, strings.NewReader("text"))
		_ = metaStore.Save(ctx, file)
		rec := NewVectorStoreFileRecord("vs-1", id)
		_ = vsFileStore.Save(ctx, rec)
		pipeline.Ingest(file, "vs-1")
	}

	// Wait for all files to complete.
	for i := 0; i < totalFiles; i++ {
		id := fmt.Sprintf("file-conc-%d", i)
		waitForFileStatus(t, metaStore, id, FileStatusCompleted, 10*time.Second)
	}

	if seen := maxSeen.Load(); seen > int32(maxWorkers) {
		t.Errorf("max concurrent workers: %d, limit was %d", seen, maxWorkers)
	}
}

// ---------- Additional helper types ----------

// statusTracker wraps FileMetadataStore to observe Update calls.
type statusTracker struct {
	FileMetadataStore
	onUpdate func(status FileStatus)
}

func (s *statusTracker) Update(ctx context.Context, id string, status FileStatus, errMsg string) error {
	if s.onUpdate != nil {
		s.onUpdate(status)
	}
	return s.FileMetadataStore.Update(ctx, id, status, errMsg)
}

// stubExtractorWithError always returns an error.
type stubExtractorWithError struct {
	err error
}

func (s *stubExtractorWithError) Extract(_ context.Context, _, _ string, content io.Reader) (*ExtractionResult, error) {
	io.ReadAll(content)
	return nil, s.err
}

func (s *stubExtractorWithError) SupportedFormats() []string {
	return []string{"text/plain"}
}

// slowStubExtractor adds configurable delay for concurrency testing.
type slowStubExtractor struct {
	delay   time.Duration
	result  *ExtractionResult
	onStart func()
	onDone  func()
}

func (m *slowStubExtractor) Extract(_ context.Context, _, _ string, content io.Reader) (*ExtractionResult, error) {
	io.ReadAll(content)
	if m.onStart != nil {
		m.onStart()
	}
	time.Sleep(m.delay)
	if m.onDone != nil {
		m.onDone()
	}
	return m.result, nil
}

func (m *slowStubExtractor) SupportedFormats() []string {
	return []string{"text/plain"}
}
