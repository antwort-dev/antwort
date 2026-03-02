package files

import "time"

// FileStatus represents the processing state of a file.
type FileStatus string

const (
	FileStatusUploaded   FileStatus = "uploaded"
	FileStatusProcessing FileStatus = "processing"
	FileStatusCompleted  FileStatus = "completed"
	FileStatusFailed     FileStatus = "failed"
)

// FilePurpose represents the intended use of an uploaded file.
type FilePurpose string

const (
	FilePurposeAssistants FilePurpose = "assistants"
	FilePurposeBatch      FilePurpose = "batch"
	FilePurposeFineTune   FilePurpose = "fine-tune"
	FilePurposeVision     FilePurpose = "vision"
)

// ValidPurpose checks whether the given string is a recognized file purpose.
func ValidPurpose(s string) bool {
	switch FilePurpose(s) {
	case FilePurposeAssistants, FilePurposeBatch, FilePurposeFineTune, FilePurposeVision:
		return true
	}
	return false
}

// File represents an uploaded file with metadata and status tracking.
type File struct {
	ID          string     `json:"id"`
	Object      string     `json:"object"`
	Filename    string     `json:"filename"`
	Bytes       int64      `json:"bytes"`
	MIMEType    string     `json:"mime_type,omitempty"`
	Purpose     string     `json:"purpose"`
	Status      FileStatus `json:"status"`
	StatusError string     `json:"status_details,omitempty"`
	UserID      string     `json:"-"`
	CreatedAt   int64      `json:"created_at"`
	UpdatedAt   int64      `json:"updated_at,omitempty"`
}

// NewFile creates a File with initial uploaded status and current timestamps.
func NewFile(id, filename, mimeType, purpose, userID string, size int64) *File {
	now := time.Now().Unix()
	return &File{
		ID:        id,
		Object:    "file",
		Filename:  filename,
		Bytes:     size,
		MIMEType:  mimeType,
		Purpose:   purpose,
		Status:    FileStatusUploaded,
		UserID:    userID,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Chunk represents a segment of extracted text with positional metadata.
type Chunk struct {
	Index     int    `json:"index"`
	Text      string `json:"text"`
	StartChar int    `json:"start_char"`
	EndChar   int    `json:"end_char"`
}

// ExtractionResult holds the output of content extraction.
type ExtractionResult struct {
	Text      string `json:"text"`
	PageCount int    `json:"page_count"`
	Method    string `json:"method"`
}

// VectorPoint represents a chunk prepared for vector store insertion.
type VectorPoint struct {
	ID       string
	Vector   []float32
	Metadata map[string]string
}

// FileBatch tracks a batch of files being added to a vector store.
type FileBatch struct {
	ID            string          `json:"id"`
	Object        string          `json:"object"`
	VectorStoreID string          `json:"vector_store_id"`
	Status        string          `json:"status"`
	FileCounts    FileBatchCounts `json:"file_counts"`
	CreatedAt     int64           `json:"created_at"`
}

// FileBatchCounts holds per-status counts for a file batch.
type FileBatchCounts struct {
	InProgress int `json:"in_progress"`
	Completed  int `json:"completed"`
	Failed     int `json:"failed"`
	Cancelled  int `json:"cancelled"`
	Total      int `json:"total"`
}
