package filesearch

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/rhuss/antwort/pkg/storage"
)

// vsOwnerAllowed checks if the caller is allowed to access a vector store.
// Admin bypass only applies to read/delete operations (writeOp=false).
// Per FR-007, admin users must NOT modify resources owned by other users.
func vsOwnerAllowed(r *http.Request, storedOwner, resourceID, operation string, writeOp bool) bool {
	callerOwner := storage.GetOwner(r.Context())
	if callerOwner == "" {
		return true
	}
	if !writeOp && storage.GetAdmin(r.Context()) {
		return true
	}
	if storedOwner == "" {
		return true
	}
	if callerOwner == storedOwner {
		return true
	}
	slog.Debug("ownership denied",
		"subject", callerOwner,
		"resource_id", resourceID,
		"operation", operation,
	)
	return false
}

// permissionsRequest represents the JSON permissions object on create/update.
type permissionsRequest struct {
	Group  string `json:"group"`
	Others string `json:"others"`
}

// toCompactPermissions converts a permissionsRequest to the compact string format.
// Owner is always "rwd". Group and others default to "---" if not set.
func toCompactPermissions(pr *permissionsRequest) string {
	if pr == nil {
		return DefaultPermissions
	}
	g := normalizePermSegment(pr.Group)
	o := normalizePermSegment(pr.Others)
	return "rwd|" + g + "|" + o
}

// normalizePermSegment ensures a permission segment only contains valid chars (r, w, d).
func normalizePermSegment(s string) string {
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

// canAccessResource checks if a caller can read a resource based on its permissions string.
// permissions format: "owner|group|others" where each segment is a subset of "rwd".
func canAccessResource(permissions, callerOwner, resourceOwner, callerTenant, resourceTenant string) bool {
	if callerOwner == resourceOwner {
		return true
	}
	if permissions == "" {
		permissions = DefaultPermissions
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

// createStoreRequest is the JSON request body for creating a vector store.
type createStoreRequest struct {
	Name        string              `json:"name"`
	Permissions *permissionsRequest `json:"permissions,omitempty"`
}

// vectorStoreResponse is the OpenAI-compatible JSON response for a vector store.
type vectorStoreResponse struct {
	ID          string `json:"id"`
	Object      string `json:"object"`
	Name        string `json:"name"`
	Permissions string `json:"permissions"`
	CreatedAt   int64  `json:"created_at"`
}

// vectorStoreListResponse is the OpenAI-compatible JSON response for listing vector stores.
type vectorStoreListResponse struct {
	Object string                `json:"object"`
	Data   []vectorStoreResponse `json:"data"`
}

func toResponse(vs *VectorStore) vectorStoreResponse {
	perms := vs.Permissions
	if perms == "" {
		perms = DefaultPermissions
	}
	return vectorStoreResponse{
		ID:          vs.ID,
		Object:      "vector_store",
		Name:        vs.Name,
		Permissions: perms,
		CreatedAt:   vs.CreatedAt,
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
	owner := storage.GetOwner(r.Context())

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
		Owner:          owner,
		Permissions:    toCompactPermissions(req.Permissions),
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

	callerTenant := storage.GetTenant(r.Context())
	callerOwner := storage.GetOwner(r.Context())
	isAdminCaller := storage.GetAdmin(r.Context())

	// Get same-tenant stores plus cross-tenant stores with "others" read permission.
	var stores []*VectorStore
	if callerTenant != "" {
		// Start with same-tenant stores.
		stores = p.metadata.List(callerTenant)
		// Add cross-tenant stores that grant "others" read permission.
		for _, vs := range p.metadata.ListAll() {
			if vs.TenantID != callerTenant && canAccessResource(vs.Permissions, callerOwner, vs.Owner, callerTenant, vs.TenantID) {
				stores = append(stores, vs)
			}
		}
	} else {
		stores = p.metadata.List("")
	}

	data := make([]vectorStoreResponse, 0, len(stores))
	for _, vs := range stores {
		// Admin bypass: full read access for same-tenant admins.
		if callerOwner != "" && !isAdminCaller {
			if vs.Owner != "" && vs.Owner != callerOwner {
				// Check group/others permissions for non-owners.
				if !canAccessResource(vs.Permissions, callerOwner, vs.Owner, callerTenant, vs.TenantID) {
					continue
				}
			}
		}
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

	// Tenant isolation check (hard boundary unless others permissions allow).
	tenantID := storage.GetTenant(r.Context())
	callerOwner := storage.GetOwner(r.Context())

	if tenantID != "" && vs.TenantID != tenantID {
		// Cross-tenant: only allow if others permissions include read.
		if !canAccessResource(vs.Permissions, callerOwner, vs.Owner, tenantID, vs.TenantID) {
			writeJSONError(w, "vector store not found", http.StatusNotFound)
			return
		}
	} else {
		// Same tenant: check owner or admin bypass or group permissions.
		if !vsOwnerAllowed(r, vs.Owner, storeID, "GetStore", false) {
			if !canAccessResource(vs.Permissions, callerOwner, vs.Owner, tenantID, vs.TenantID) {
				writeJSONError(w, "vector store not found", http.StatusNotFound)
				return
			}
		}
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

	// Owner isolation check.
	if !vsOwnerAllowed(r, vs.Owner, storeID, "DeleteStore", false) {
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
