package files

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/audit"
	"github.com/rhuss/antwort/pkg/storage"
)

// maxDefaultUploadSize is 50 MB.
const maxDefaultUploadSize int64 = 50 * 1024 * 1024

// FilesAPI provides HTTP handlers for file operations.
type FilesAPI struct {
	fileStore          FileStore
	metadata           FileMetadataStore
	vsFileStore        VectorStoreFileStore
	indexer            VectorIndexer
	vsCollectionLookup func(vsID string) (string, error)
	maxUploadSize      int64
	logger             *slog.Logger
	auditLogger        *audit.Logger
}

func (a *FilesAPI) handleUpload(w http.ResponseWriter, r *http.Request) {
	// Limit request body size.
	r.Body = http.MaxBytesReader(w, r.Body, a.maxUploadSize+1024) // extra for form fields

	contentType := r.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "multipart/form-data" {
		writeAPIError(w, api.NewInvalidRequestError("Content-Type", "expected multipart/form-data"))
		return
	}

	reader := multipart.NewReader(r.Body, params["boundary"])

	var (
		fileData       []byte
		filename       string
		fileMIMEType   string
		purpose        string
		permissionsRaw string
	)

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			writeAPIError(w, api.NewInvalidRequestError("file", "error reading multipart form"))
			return
		}

		switch part.FormName() {
		case "file":
			filename = part.FileName()
			fileMIMEType = detectMIME(filename, part.Header.Get("Content-Type"))
			fileData, err = io.ReadAll(part)
			if err != nil {
				writeAPIError(w, api.NewInvalidRequestError("file", "error reading file data"))
				return
			}
		case "purpose":
			data, _ := io.ReadAll(part)
			purpose = string(data)
		case "permissions":
			data, _ := io.ReadAll(part)
			permissionsRaw = string(data)
		}
		part.Close()
	}

	if len(fileData) == 0 || filename == "" {
		writeAPIError(w, api.NewInvalidRequestError("file", "file is required"))
		return
	}
	if purpose == "" {
		writeAPIError(w, api.NewInvalidRequestError("purpose", "purpose is required"))
		return
	}
	if !ValidPurpose(purpose) {
		writeAPIError(w, api.NewInvalidRequestError("purpose", fmt.Sprintf("invalid purpose %q", purpose)))
		return
	}
	if int64(len(fileData)) > a.maxUploadSize {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"message": fmt.Sprintf("file exceeds maximum upload size of %d bytes", a.maxUploadSize),
				"type":    "invalid_request_error",
				"param":   "file",
			},
		})
		return
	}

	userID := userFromCtx(r.Context())
	tenantID := storage.GetTenant(r.Context())
	fileID := api.NewFileID()
	file := NewFile(fileID, filename, fileMIMEType, purpose, userID, int64(len(fileData)))
	file.TenantID = tenantID

	// Parse permissions if provided.
	if permissionsRaw != "" {
		file.Permissions = parseFilePermissions(permissionsRaw)
	}

	// Store file content.
	if err := a.fileStore.Store(r.Context(), fileID, newBytesReader(fileData)); err != nil {
		a.logger.Error("failed to store file", "error", err)
		writeAPIError(w, api.NewServerError("failed to store file"))
		return
	}

	// Save metadata.
	if err := a.metadata.Save(r.Context(), file); err != nil {
		a.logger.Error("failed to save file metadata", "error", err)
		writeAPIError(w, api.NewServerError("failed to save file metadata"))
		return
	}

	// Record upload metric.
	filesUploadedTotal.WithLabelValues(fileMIMEType).Inc()

	a.auditLogger.Log(r.Context(), "resource.created", "resource_type", "file", "resource_id", fileID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(file)
}

func (a *FilesAPI) handleListFiles(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	opts := ListOptions{
		After:   q.Get("after"),
		Limit:   limit,
		Order:   q.Get("order"),
		Purpose: q.Get("purpose"),
	}

	list, err := a.metadata.List(r.Context(), opts)
	if err != nil {
		writeAPIError(w, api.NewServerError("failed to list files"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (a *FilesAPI) handleGetFile(w http.ResponseWriter, r *http.Request) {
	fileID := r.PathValue("file_id")
	file, err := a.metadata.Get(r.Context(), fileID)
	if err != nil {
		writeAPIError(w, api.NewNotFoundError("file not found"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(file)
}

func (a *FilesAPI) handleGetContent(w http.ResponseWriter, r *http.Request) {
	fileID := r.PathValue("file_id")
	file, err := a.metadata.Get(r.Context(), fileID)
	if err != nil {
		writeAPIError(w, api.NewNotFoundError("file not found"))
		return
	}

	reader, err := a.fileStore.Retrieve(r.Context(), fileID)
	if err != nil {
		writeAPIError(w, api.NewServerError("failed to retrieve file content"))
		return
	}
	defer reader.Close()

	if file.MIMEType != "" {
		w.Header().Set("Content-Type", file.MIMEType)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	io.Copy(w, reader)
}

func (a *FilesAPI) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	fileID := r.PathValue("file_id")

	// Verify file exists and belongs to user.
	_, err := a.metadata.Get(r.Context(), fileID)
	if err != nil {
		writeAPIError(w, api.NewNotFoundError("file not found"))
		return
	}

	// Cascade: remove from all vector stores.
	records, _ := a.vsFileStore.ListByFile(r.Context(), fileID)
	for _, rec := range records {
		if a.indexer != nil {
			vs, err := a.getCollectionName(rec.VectorStoreID)
			if err == nil {
				_ = a.indexer.DeletePointsByFile(r.Context(), vs, fileID)
			}
		}
		_ = a.vsFileStore.Delete(r.Context(), rec.VectorStoreID, fileID)
	}

	// Delete content and metadata.
	_ = a.fileStore.Delete(r.Context(), fileID)
	_ = a.metadata.Delete(r.Context(), fileID)

	a.auditLogger.Log(r.Context(), "resource.deleted", "resource_type", "file", "resource_id", fileID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":      fileID,
		"object":  "file",
		"deleted": true,
	})
}

// getCollectionName looks up the collection name for a vector store ID.
// This is a helper that reaches into the filesearch MetadataStore via the provider.
func (a *FilesAPI) getCollectionName(vsID string) (string, error) {
	// This will be set from the provider during construction.
	if a.vsCollectionLookup != nil {
		return a.vsCollectionLookup(vsID)
	}
	return "", fmt.Errorf("no collection lookup configured")
}

// detectMIME determines the MIME type from filename extension or Content-Type header.
func detectMIME(filename, headerType string) string {
	if headerType != "" && headerType != "application/octet-stream" {
		return headerType
	}
	ext := ""
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '.' {
			ext = filename[i:]
			break
		}
	}
	switch ext {
	case ".pdf":
		return "application/pdf"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".txt":
		return "text/plain"
	case ".md", ".markdown":
		return "text/markdown"
	case ".csv":
		return "text/csv"
	case ".json":
		return "application/json"
	case ".html", ".htm":
		return "text/html"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

// filePermissionsInput represents the JSON permissions object for files.
type filePermissionsInput struct {
	Group  string `json:"group"`
	Others string `json:"others"`
}

// parseFilePermissions parses a JSON permissions string into compact format.
func parseFilePermissions(raw string) string {
	var input filePermissionsInput
	if err := json.Unmarshal([]byte(raw), &input); err != nil {
		return DefaultFilePermissions
	}
	g := normalizeFilePermSegment(input.Group)
	o := normalizeFilePermSegment(input.Others)
	return "rwd|" + g + "|" + o
}

// normalizeFilePermSegment ensures a permission segment only contains valid chars.
func normalizeFilePermSegment(s string) string {
	if s == "" {
		return "---"
	}
	result := [3]byte{'-', '-', '-'}
	for _, c := range s {
		switch c {
		case 'r':
			result[0] = 'r'
		case 'w':
			result[1] = 'w'
		case 'd':
			result[2] = 'd'
		}
	}
	return string(result[:])
}

// canAccessFile checks if a caller can read a file based on its permissions string.
func canAccessFile(permissions, callerOwner, resourceOwner, callerTenant, resourceTenant string) bool {
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

func writeAPIError(w http.ResponseWriter, apiErr *api.APIError) {
	w.Header().Set("Content-Type", "application/json")
	status := http.StatusBadRequest
	switch apiErr.Type {
	case api.ErrorTypeNotFound:
		status = http.StatusNotFound
	case api.ErrorTypeServerError:
		status = http.StatusInternalServerError
	case api.ErrorTypeTooManyRequests:
		status = http.StatusTooManyRequests
	}
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"message": apiErr.Message,
			"type":    string(apiErr.Type),
			"param":   apiErr.Param,
		},
	})
}
