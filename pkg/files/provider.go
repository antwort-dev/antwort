package files

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/tools"
	"github.com/rhuss/antwort/pkg/tools/registry"
)

// FilesProvider implements registry.FunctionProvider for the Files API.
// It contributes HTTP routes for file management but no tool definitions.
type FilesProvider struct {
	filesAPI   *FilesAPI
	vsFilesAPI *VSFilesAPI
	pipeline   *IngestionPipeline
	logger     *slog.Logger
}

// Compile-time check.
var _ registry.FunctionProvider = (*FilesProvider)(nil)

// Settings keys for provider configuration.
const (
	settingMaxUploadSize  = "max_upload_size"
	settingStoreType      = "store_type"
	settingStorePath      = "store_path"
	settingDoclingURL     = "docling_url"
	settingDoclingAPIKey  = "docling_api_key"
	settingDoclingOCR     = "docling_ocr"
	settingChunkSize      = "chunk_size"
	settingChunkOverlap   = "chunk_overlap"
	settingWorkers        = "pipeline_workers"
	settingDoclingTimeout = "docling_timeout"
)

// ProviderDeps holds external dependencies that must be passed from the server.
type ProviderDeps struct {
	Embedding Embedder
	Indexer   VectorIndexer
	VSLookup  VectorStoreLookup
}

// New creates a FilesProvider from the provider settings map and external dependencies.
func New(settings map[string]interface{}, deps ProviderDeps) (*FilesProvider, error) {
	logger := slog.Default().With("provider", "files")

	// Parse configuration with defaults.
	maxUpload := getInt64(settings, settingMaxUploadSize, maxDefaultUploadSize)
	storeType := getString(settings, settingStoreType, "filesystem")
	storePath := getString(settings, settingStorePath, "/data/files")
	doclingURL := getString(settings, settingDoclingURL, "")
	doclingAPIKey := getString(settings, settingDoclingAPIKey, "")
	doclingOCR := getBool(settings, settingDoclingOCR, true)
	doclingTimeout := getDuration(settings, settingDoclingTimeout, 300*time.Second)
	chunkSize := getInt(settings, settingChunkSize, 800)
	chunkOverlap := getInt(settings, settingChunkOverlap, 200)
	workers := getInt(settings, settingWorkers, 4)

	// Create file store.
	var fileStore FileStore
	switch storeType {
	case "memory":
		fileStore = NewMemoryFileStore()
	case "filesystem":
		var err error
		fileStore, err = NewFilesystemStore(storePath)
		if err != nil {
			return nil, fmt.Errorf("creating filesystem store: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported file store type: %s", storeType)
	}

	// Create metadata and VS file stores.
	metadataStore := NewMemoryMetadataStore()
	vsFileStore := NewMemoryVectorStoreFileStore()

	// Create extractors.
	passthrough := NewPassthroughExtractor()
	var docling ContentExtractor
	if doclingURL != "" {
		docling = NewDoclingExtractor(doclingURL, doclingAPIKey, doclingOCR, doclingTimeout)
	}

	// Create chunker.
	chunker := NewFixedSizeChunker(chunkSize, chunkOverlap)

	// Create pipeline.
	pipeline := NewIngestionPipeline(PipelineConfig{
		FileStore:   fileStore,
		Metadata:    metadataStore,
		VSFileStore: vsFileStore,
		Passthrough: passthrough,
		Docling:     docling,
		Chunker:     chunker,
		Embedding:   deps.Embedding,
		Indexer:     deps.Indexer,
		VSLookup:    deps.VSLookup,
		Workers:     workers,
		Logger:      logger,
	})

	// Create API handlers.
	filesAPI := &FilesAPI{
		fileStore:     fileStore,
		metadata:      metadataStore,
		vsFileStore:   vsFileStore,
		indexer:       deps.Indexer,
		maxUploadSize: maxUpload,
		logger:        logger,
		vsCollectionLookup: func(vsID string) (string, error) {
			return deps.VSLookup(vsID)
		},
	}

	vsFilesAPI := &VSFilesAPI{
		metadata:    metadataStore,
		vsFileStore: vsFileStore,
		vsLookup:    deps.VSLookup,
		indexer:     deps.Indexer,
		pipeline:    pipeline,
		batches:     NewBatchStore(),
	}

	return &FilesProvider{
		filesAPI:   filesAPI,
		vsFilesAPI: vsFilesAPI,
		pipeline:   pipeline,
		logger:     logger,
	}, nil
}

func (p *FilesProvider) Name() string { return "files" }

func (p *FilesProvider) Tools() []api.ToolDefinition { return nil }

func (p *FilesProvider) CanExecute(_ string) bool { return false }

func (p *FilesProvider) Execute(_ context.Context, _ tools.ToolCall) (*tools.ToolResult, error) {
	return nil, fmt.Errorf("files provider does not execute tools")
}

func (p *FilesProvider) Routes() []registry.Route {
	return []registry.Route{
		// Files API
		{Method: "POST", Pattern: "/files", Handler: p.filesAPI.handleUpload},
		{Method: "GET", Pattern: "/files", Handler: p.filesAPI.handleListFiles},
		{Method: "GET", Pattern: "/files/{file_id}", Handler: p.filesAPI.handleGetFile},
		{Method: "GET", Pattern: "/files/{file_id}/content", Handler: p.filesAPI.handleGetContent},
		{Method: "DELETE", Pattern: "/files/{file_id}", Handler: p.filesAPI.handleDeleteFile},
		// Vector store file management
		{Method: "POST", Pattern: "/vector_stores/{store_id}/files", Handler: p.vsFilesAPI.handleAddFile},
		{Method: "GET", Pattern: "/vector_stores/{store_id}/files", Handler: p.vsFilesAPI.handleListFiles},
		{Method: "DELETE", Pattern: "/vector_stores/{store_id}/files/{file_id}", Handler: p.vsFilesAPI.handleRemoveFile},
		// Batch file operations
		{Method: "POST", Pattern: "/vector_stores/{store_id}/file_batches", Handler: p.vsFilesAPI.handleCreateBatch},
		{Method: "GET", Pattern: "/vector_stores/{store_id}/file_batches/{batch_id}", Handler: p.vsFilesAPI.handleGetBatch},
	}
}

func (p *FilesProvider) Collectors() []prometheus.Collector { return nil }

func (p *FilesProvider) Close() error { return nil }

// Settings helpers.

func getString(m map[string]interface{}, key, def string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return def
}

func getInt(m map[string]interface{}, key string, def int) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return def
}

func getInt64(m map[string]interface{}, key string, def int64) int64 {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return int64(n)
		case int64:
			return n
		case int:
			return int64(n)
		}
	}
	return def
}

func getBool(m map[string]interface{}, key string, def bool) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return def
}

func getDuration(m map[string]interface{}, key string, def time.Duration) time.Duration {
	if v, ok := m[key]; ok {
		switch d := v.(type) {
		case string:
			if parsed, err := time.ParseDuration(d); err == nil {
				return parsed
			}
		case float64:
			return time.Duration(d) * time.Second
		}
	}
	return def
}
