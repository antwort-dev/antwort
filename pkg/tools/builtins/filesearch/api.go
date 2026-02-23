package filesearch

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/rhuss/antwort/pkg/storage"
)

// createStoreRequest is the JSON request body for creating a vector store.
type createStoreRequest struct {
	Name string `json:"name"`
}

// vectorStoreResponse is the OpenAI-compatible JSON response for a vector store.
type vectorStoreResponse struct {
	ID        string `json:"id"`
	Object    string `json:"object"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"created_at"`
}

// vectorStoreListResponse is the OpenAI-compatible JSON response for listing vector stores.
type vectorStoreListResponse struct {
	Object string                `json:"object"`
	Data   []vectorStoreResponse `json:"data"`
}

func toResponse(vs *VectorStore) vectorStoreResponse {
	return vectorStoreResponse{
		ID:        vs.ID,
		Object:    "vector_store",
		Name:      vs.Name,
		CreatedAt: vs.CreatedAt,
	}
}

// handleCreateStore handles POST requests to create a new vector store.
func (p *FileSearchProvider) handleCreateStore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req createStoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		writeJSONError(w, "name is required", http.StatusBadRequest)
		return
	}

	tenantID := storage.GetTenant(r.Context())

	// Generate a collection name from the store name.
	collName, err := generateID("col_")
	if err != nil {
		slog.Error("failed to generate collection name", "error", err)
		writeJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}

	vs := &VectorStore{
		Name:           req.Name,
		TenantID:       tenantID,
		CollectionName: collName,
		CreatedAt:      time.Now().Unix(),
	}

	// Create metadata record first (generates the ID).
	if err := p.metadata.Create(vs); err != nil {
		slog.Error("failed to create vector store metadata", "error", err)
		writeJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Create the backend collection.
	dims := p.embedding.Dimensions()
	if dims == 0 {
		// Default dimension if embedding has not been called yet.
		dims = 1536
	}
	if err := p.backend.CreateCollection(r.Context(), collName, dims); err != nil {
		slog.Error("failed to create backend collection", "error", err, "collection", collName)
		// Clean up metadata on backend failure.
		_ = p.metadata.Delete(vs.ID)
		writeJSONError(w, "failed to create vector store", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toResponse(vs))
}

// handleListStores handles GET requests to list vector stores for the current tenant.
func (p *FileSearchProvider) handleListStores(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tenantID := storage.GetTenant(r.Context())
	stores := p.metadata.List(tenantID)

	data := make([]vectorStoreResponse, 0, len(stores))
	for _, vs := range stores {
		data = append(data, toResponse(vs))
	}

	resp := vectorStoreListResponse{
		Object: "list",
		Data:   data,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleGetStore handles GET requests to retrieve a single vector store by ID.
func (p *FileSearchProvider) handleGetStore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	storeID := r.PathValue("store_id")
	if storeID == "" {
		writeJSONError(w, "store_id is required", http.StatusBadRequest)
		return
	}

	vs, err := p.metadata.Get(storeID)
	if err != nil {
		writeJSONError(w, "vector store not found", http.StatusNotFound)
		return
	}

	// Tenant isolation check.
	tenantID := storage.GetTenant(r.Context())
	if tenantID != "" && vs.TenantID != tenantID {
		writeJSONError(w, "vector store not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toResponse(vs))
}

// handleDeleteStore handles DELETE requests to remove a vector store.
func (p *FileSearchProvider) handleDeleteStore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	storeID := r.PathValue("store_id")
	if storeID == "" {
		writeJSONError(w, "store_id is required", http.StatusBadRequest)
		return
	}

	vs, err := p.metadata.Get(storeID)
	if err != nil {
		writeJSONError(w, "vector store not found", http.StatusNotFound)
		return
	}

	// Tenant isolation check.
	tenantID := storage.GetTenant(r.Context())
	if tenantID != "" && vs.TenantID != tenantID {
		writeJSONError(w, "vector store not found", http.StatusNotFound)
		return
	}

	// Delete backend collection.
	if err := p.backend.DeleteCollection(r.Context(), vs.CollectionName); err != nil {
		slog.Error("failed to delete backend collection", "error", err, "collection", vs.CollectionName)
		writeJSONError(w, "failed to delete vector store", http.StatusInternalServerError)
		return
	}

	// Delete metadata.
	if err := p.metadata.Delete(storeID); err != nil {
		slog.Error("failed to delete vector store metadata", "error", err, "store_id", storeID)
		writeJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":      storeID,
		"object":  "vector_store.deleted",
		"deleted": true,
	})
}

// writeJSONError writes a JSON error response.
func writeJSONError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"message": message,
			"type":    "invalid_request_error",
		},
	})
}
