package files

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/rhuss/antwort/pkg/api"
)

// IngestionPipeline orchestrates the extract, chunk, embed, index workflow.
type IngestionPipeline struct {
	fileStore   FileStore
	metadata    FileMetadataStore
	vsFileStore VectorStoreFileStore
	passthrough ContentExtractor
	docling     ContentExtractor // nil when docling-serve not configured
	chunker     Chunker
	embedding   Embedder
	indexer     VectorIndexer
	vsLookup    VectorStoreLookup
	sem         chan struct{} // concurrency limiter
	logger      *slog.Logger
}

// PipelineConfig holds pipeline construction parameters.
type PipelineConfig struct {
	FileStore   FileStore
	Metadata    FileMetadataStore
	VSFileStore VectorStoreFileStore
	Passthrough ContentExtractor
	Docling     ContentExtractor
	Chunker     Chunker
	Embedding   Embedder
	Indexer     VectorIndexer
	VSLookup    VectorStoreLookup
	Workers     int
	Logger      *slog.Logger
}

// NewIngestionPipeline creates a pipeline with the given dependencies.
func NewIngestionPipeline(cfg PipelineConfig) *IngestionPipeline {
	workers := cfg.Workers
	if workers <= 0 {
		workers = 4
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &IngestionPipeline{
		fileStore:   cfg.FileStore,
		metadata:    cfg.Metadata,
		vsFileStore: cfg.VSFileStore,
		passthrough: cfg.Passthrough,
		docling:     cfg.Docling,
		chunker:     cfg.Chunker,
		embedding:   cfg.Embedding,
		indexer:     cfg.Indexer,
		vsLookup:    cfg.VSLookup,
		sem:         make(chan struct{}, workers),
		logger:      logger,
	}
}

// Ingest starts asynchronous ingestion of a file into a vector store.
// It returns immediately; the actual work runs in a background goroutine.
func (p *IngestionPipeline) Ingest(file *File, vectorStoreID string) {
	go func() {
		p.sem <- struct{}{}        // acquire worker slot
		defer func() { <-p.sem }() // release worker slot

		ctx := context.Background()
		p.logger.Info("starting ingestion", "file_id", file.ID, "vector_store_id", vectorStoreID)

		if err := p.ingest(ctx, file, vectorStoreID); err != nil {
			p.logger.Error("ingestion failed", "file_id", file.ID, "error", err)
			_ = p.updateStatus(ctx, file.ID, vectorStoreID, FileStatusFailed, 0, err.Error())
			return
		}

		p.logger.Info("ingestion completed", "file_id", file.ID, "vector_store_id", vectorStoreID)
	}()
}

func (p *IngestionPipeline) ingest(ctx context.Context, file *File, vectorStoreID string) error {
	// Stage 1: Update status to processing.
	if err := p.updateStatus(ctx, file.ID, vectorStoreID, FileStatusProcessing, 0, ""); err != nil {
		return fmt.Errorf("updating status to processing: %w", err)
	}

	// Stage 2: Read file content.
	reader, err := p.fileStore.Retrieve(ctx, file.ID)
	if err != nil {
		return fmt.Errorf("retrieving file content: %w", err)
	}
	defer reader.Close()

	// Stage 3: Extract text.
	extractor := p.selectExtractor(file.MIMEType)
	if extractor == nil {
		return fmt.Errorf("%s extraction requires an external extraction service (docling-serve)", file.MIMEType)
	}

	result, err := extractor.Extract(ctx, file.Filename, file.MIMEType, reader)
	if err != nil {
		return fmt.Errorf("extracting content: %w", err)
	}
	if result.Text == "" {
		return fmt.Errorf("no extractable content found")
	}

	// Stage 4: Chunk text.
	chunks := p.chunker.Chunk(result.Text)
	if len(chunks) == 0 {
		return fmt.Errorf("chunking produced no output")
	}

	// Stage 5: Embed chunks.
	texts := make([]string, len(chunks))
	for i, c := range chunks {
		texts[i] = c.Text
	}
	vectors, err := p.embedding.Embed(ctx, texts)
	if err != nil {
		return fmt.Errorf("embedding chunks: %w", err)
	}

	// Stage 6: Build vector points and index.
	collectionName, err := p.vsLookup(vectorStoreID)
	if err != nil {
		return fmt.Errorf("looking up vector store collection: %w", err)
	}

	points := make([]VectorPoint, len(chunks))
	for i, chunk := range chunks {
		points[i] = VectorPoint{
			ID:     api.NewFileID(), // unique point ID
			Vector: vectors[i],
			Metadata: map[string]string{
				"file_id":  file.ID,
				"filename": file.Filename,
				"content":  chunk.Text,
			},
		}
	}

	if err := p.indexer.UpsertPoints(ctx, collectionName, points); err != nil {
		return fmt.Errorf("indexing vectors: %w", err)
	}

	// Stage 7: Mark completed.
	return p.updateStatus(ctx, file.ID, vectorStoreID, FileStatusCompleted, len(chunks), "")
}

// selectExtractor returns the appropriate extractor for the MIME type, or nil if
// the format requires docling and docling is not configured.
func (p *IngestionPipeline) selectExtractor(mimeType string) ContentExtractor {
	if IsPassthroughFormat(mimeType) {
		return p.passthrough
	}
	if IsComplexFormat(mimeType) {
		return p.docling // may be nil (graceful degradation)
	}
	// Unknown format: try passthrough.
	return p.passthrough
}

func (p *IngestionPipeline) updateStatus(ctx context.Context, fileID, vectorStoreID string, status FileStatus, chunkCount int, errMsg string) error {
	// Update file metadata.
	if err := p.metadata.Update(ctx, fileID, status, errMsg); err != nil {
		p.logger.Error("failed to update file metadata", "file_id", fileID, "error", err)
	}

	// Update vector store file record.
	rec, err := p.vsFileStore.Get(ctx, vectorStoreID, fileID)
	if err != nil {
		return err
	}
	rec.Status = status
	rec.ChunkCount = chunkCount
	rec.LastError = errMsg
	return p.vsFileStore.Save(ctx, rec)
}
