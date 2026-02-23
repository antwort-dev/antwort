package filesearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rhuss/antwort/pkg/storage"
	"github.com/rhuss/antwort/pkg/tools"
)

// mockBackend is a test implementation of VectorStoreBackend.
type mockBackend struct {
	collections map[string]int // name -> dimensions
	searchFn    func(collection string, vector []float32, maxResults int) ([]SearchMatch, error)
}

func newMockBackend() *mockBackend {
	return &mockBackend{
		collections: make(map[string]int),
	}
}

func (m *mockBackend) CreateCollection(_ context.Context, name string, dimensions int) error {
	m.collections[name] = dimensions
	return nil
}

func (m *mockBackend) DeleteCollection(_ context.Context, name string) error {
	delete(m.collections, name)
	return nil
}

func (m *mockBackend) Search(_ context.Context, collection string, vector []float32, maxResults int) ([]SearchMatch, error) {
	if m.searchFn != nil {
		return m.searchFn(collection, vector, maxResults)
	}
	return nil, nil
}

// mockEmbedding is a test implementation of EmbeddingClient.
type mockEmbedding struct {
	embedFn func(texts []string) ([][]float32, error)
	dims    int
}

func newMockEmbedding(dims int) *mockEmbedding {
	return &mockEmbedding{
		dims: dims,
		embedFn: func(texts []string) ([][]float32, error) {
			result := make([][]float32, len(texts))
			for i := range texts {
				vec := make([]float32, dims)
				for j := range vec {
					vec[j] = 0.1 * float32(j+1)
				}
				result[i] = vec
			}
			return result, nil
		},
	}
}

func (m *mockEmbedding) Embed(_ context.Context, texts []string) ([][]float32, error) {
	return m.embedFn(texts)
}

func (m *mockEmbedding) Dimensions() int {
	return m.dims
}

// helper to create a provider with a store already set up.
func setupProvider(t *testing.T) (*FileSearchProvider, *mockBackend) {
	t.Helper()
	backend := newMockBackend()
	embedding := newMockEmbedding(384)
	p := newWithDeps(backend, embedding, 10)

	// Create a test store.
	vs := &VectorStore{
		Name:           "test-docs",
		TenantID:       "tenant-1",
		CollectionName: "col_test",
		CreatedAt:      1700000000,
	}
	if err := p.metadata.Create(vs); err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}

	return p, backend
}

func TestFileSearch_Execute(t *testing.T) {
	p, _ := setupProvider(t)

	// Configure search to return results.
	p.backend.(*mockBackend).searchFn = func(collection string, vector []float32, maxResults int) ([]SearchMatch, error) {
		return []SearchMatch{
			{DocumentID: "doc-1", Score: 0.95, Content: "Go is great", Metadata: map[string]string{"file": "intro.md"}},
			{DocumentID: "doc-2", Score: 0.80, Content: "Go is fast", Metadata: map[string]string{"file": "perf.md"}},
		}, nil
	}

	ctx := storage.SetTenant(context.Background(), "tenant-1")
	result, err := p.Execute(ctx, tools.ToolCall{
		ID:        "call_1",
		Name:      "file_search",
		Arguments: `{"query":"golang"}`,
	})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Output)
	}
	if result.CallID != "call_1" {
		t.Errorf("CallID = %q, want %q", result.CallID, "call_1")
	}

	if !strings.Contains(result.Output, `Search results for "golang"`) {
		t.Errorf("output missing header, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "Go is great") {
		t.Errorf("output missing result 1, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "Go is fast") {
		t.Errorf("output missing result 2, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "0.9500") {
		t.Errorf("output missing score, got: %s", result.Output)
	}
}

func TestFileSearch_EmptyResults(t *testing.T) {
	p, _ := setupProvider(t)

	// Search returns no matches.
	p.backend.(*mockBackend).searchFn = func(collection string, vector []float32, maxResults int) ([]SearchMatch, error) {
		return nil, nil
	}

	ctx := storage.SetTenant(context.Background(), "tenant-1")
	result, err := p.Execute(ctx, tools.ToolCall{
		ID:        "call_empty",
		Name:      "file_search",
		Arguments: `{"query":"nonexistent topic"}`,
	})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result.IsError {
		t.Errorf("expected IsError = false for empty results, got error: %s", result.Output)
	}
	if !strings.Contains(result.Output, "No results found") {
		t.Errorf("expected 'No results found' message, got: %s", result.Output)
	}
}

func TestFileSearch_EmptyQuery(t *testing.T) {
	p, _ := setupProvider(t)

	result, err := p.Execute(context.Background(), tools.ToolCall{
		ID:        "call_empty_q",
		Name:      "file_search",
		Arguments: `{"query":""}`,
	})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError = true for empty query")
	}
	if !strings.Contains(result.Output, "must not be empty") {
		t.Errorf("expected empty query error, got: %s", result.Output)
	}
}

func TestFileSearch_InvalidArguments(t *testing.T) {
	p, _ := setupProvider(t)

	result, err := p.Execute(context.Background(), tools.ToolCall{
		ID:        "call_invalid",
		Name:      "file_search",
		Arguments: `not valid json`,
	})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError = true for invalid arguments")
	}
	if !strings.Contains(result.Output, "invalid arguments") {
		t.Errorf("expected invalid arguments error, got: %s", result.Output)
	}
}

func TestFileSearch_APICreateStore(t *testing.T) {
	backend := newMockBackend()
	embedding := newMockEmbedding(384)
	p := newWithDeps(backend, embedding, 10)

	ctx := storage.SetTenant(context.Background(), "tenant-1")

	// Create a store.
	createBody := strings.NewReader(`{"name":"my-docs"}`)
	req := httptest.NewRequest(http.MethodPost, "/vector_stores", createBody)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	p.handleCreateStore(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create returned status %d, want %d: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var createResp vectorStoreResponse
	if err := json.NewDecoder(w.Body).Decode(&createResp); err != nil {
		t.Fatalf("failed to decode create response: %v", err)
	}
	if createResp.Object != "vector_store" {
		t.Errorf("object = %q, want %q", createResp.Object, "vector_store")
	}
	if createResp.Name != "my-docs" {
		t.Errorf("name = %q, want %q", createResp.Name, "my-docs")
	}
	if !strings.HasPrefix(createResp.ID, "vs_") {
		t.Errorf("ID = %q, expected vs_ prefix", createResp.ID)
	}

	// Verify backend collection was created.
	if len(backend.collections) != 1 {
		t.Errorf("expected 1 backend collection, got %d", len(backend.collections))
	}

	// List stores.
	req = httptest.NewRequest(http.MethodGet, "/vector_stores", nil)
	req = req.WithContext(ctx)
	w = httptest.NewRecorder()
	p.handleListStores(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("list returned status %d", w.Code)
	}

	var listResp vectorStoreListResponse
	if err := json.NewDecoder(w.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}
	if listResp.Object != "list" {
		t.Errorf("object = %q, want %q", listResp.Object, "list")
	}
	if len(listResp.Data) != 1 {
		t.Fatalf("expected 1 store, got %d", len(listResp.Data))
	}
	if listResp.Data[0].Name != "my-docs" {
		t.Errorf("store name = %q, want %q", listResp.Data[0].Name, "my-docs")
	}

	// Get store.
	req = httptest.NewRequest(http.MethodGet, "/vector_stores/"+createResp.ID, nil)
	req.SetPathValue("store_id", createResp.ID)
	req = req.WithContext(ctx)
	w = httptest.NewRecorder()
	p.handleGetStore(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("get returned status %d: %s", w.Code, w.Body.String())
	}

	var getResp vectorStoreResponse
	if err := json.NewDecoder(w.Body).Decode(&getResp); err != nil {
		t.Fatalf("failed to decode get response: %v", err)
	}
	if getResp.ID != createResp.ID {
		t.Errorf("get ID = %q, want %q", getResp.ID, createResp.ID)
	}

	// Delete store.
	req = httptest.NewRequest(http.MethodDelete, "/vector_stores/"+createResp.ID, nil)
	req.SetPathValue("store_id", createResp.ID)
	req = req.WithContext(ctx)
	w = httptest.NewRecorder()
	p.handleDeleteStore(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("delete returned status %d: %s", w.Code, w.Body.String())
	}

	// Verify store is gone.
	if _, err := p.metadata.Get(createResp.ID); err == nil {
		t.Error("expected store to be deleted")
	}

	// Verify backend collection is gone.
	if len(backend.collections) != 0 {
		t.Errorf("expected 0 backend collections after delete, got %d", len(backend.collections))
	}
}

func TestFileSearch_TenantIsolation(t *testing.T) {
	backend := newMockBackend()
	embedding := newMockEmbedding(384)
	p := newWithDeps(backend, embedding, 10)

	// Create store for tenant-1.
	ctx1 := storage.SetTenant(context.Background(), "tenant-1")
	createBody := strings.NewReader(`{"name":"tenant1-docs"}`)
	req := httptest.NewRequest(http.MethodPost, "/vector_stores", createBody)
	req = req.WithContext(ctx1)
	w := httptest.NewRecorder()
	p.handleCreateStore(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create for tenant-1 returned status %d", w.Code)
	}

	var createResp vectorStoreResponse
	json.NewDecoder(w.Body).Decode(&createResp)

	// Create store for tenant-2.
	ctx2 := storage.SetTenant(context.Background(), "tenant-2")
	createBody2 := strings.NewReader(`{"name":"tenant2-docs"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/vector_stores", createBody2)
	req2 = req2.WithContext(ctx2)
	w2 := httptest.NewRecorder()
	p.handleCreateStore(w2, req2)

	if w2.Code != http.StatusCreated {
		t.Fatalf("create for tenant-2 returned status %d", w2.Code)
	}

	// List for tenant-1 should see only their store.
	req = httptest.NewRequest(http.MethodGet, "/vector_stores", nil)
	req = req.WithContext(ctx1)
	w = httptest.NewRecorder()
	p.handleListStores(w, req)

	var listResp1 vectorStoreListResponse
	json.NewDecoder(w.Body).Decode(&listResp1)
	if len(listResp1.Data) != 1 {
		t.Fatalf("tenant-1 should see 1 store, got %d", len(listResp1.Data))
	}
	if listResp1.Data[0].Name != "tenant1-docs" {
		t.Errorf("tenant-1 store name = %q, want %q", listResp1.Data[0].Name, "tenant1-docs")
	}

	// List for tenant-2 should see only their store.
	req = httptest.NewRequest(http.MethodGet, "/vector_stores", nil)
	req = req.WithContext(ctx2)
	w = httptest.NewRecorder()
	p.handleListStores(w, req)

	var listResp2 vectorStoreListResponse
	json.NewDecoder(w.Body).Decode(&listResp2)
	if len(listResp2.Data) != 1 {
		t.Fatalf("tenant-2 should see 1 store, got %d", len(listResp2.Data))
	}
	if listResp2.Data[0].Name != "tenant2-docs" {
		t.Errorf("tenant-2 store name = %q, want %q", listResp2.Data[0].Name, "tenant2-docs")
	}

	// Tenant-2 should not be able to get tenant-1's store.
	req = httptest.NewRequest(http.MethodGet, "/vector_stores/"+createResp.ID, nil)
	req.SetPathValue("store_id", createResp.ID)
	req = req.WithContext(ctx2)
	w = httptest.NewRecorder()
	p.handleGetStore(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("cross-tenant get returned status %d, want %d", w.Code, http.StatusNotFound)
	}

	// Tenant-2 should not be able to delete tenant-1's store.
	req = httptest.NewRequest(http.MethodDelete, "/vector_stores/"+createResp.ID, nil)
	req.SetPathValue("store_id", createResp.ID)
	req = req.WithContext(ctx2)
	w = httptest.NewRecorder()
	p.handleDeleteStore(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("cross-tenant delete returned status %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestFileSearch_CanExecute(t *testing.T) {
	p, _ := setupProvider(t)

	if !p.CanExecute("file_search") {
		t.Error("expected CanExecute('file_search') = true")
	}
	if p.CanExecute("web_search") {
		t.Error("expected CanExecute('web_search') = false")
	}
	if p.CanExecute("other_tool") {
		t.Error("expected CanExecute('other_tool') = false")
	}
	if p.CanExecute("") {
		t.Error("expected CanExecute('') = false")
	}
}

func TestFileSearch_Tools(t *testing.T) {
	p, _ := setupProvider(t)

	defs := p.Tools()
	if len(defs) != 1 {
		t.Fatalf("Tools() returned %d definitions, want 1", len(defs))
	}

	td := defs[0]
	if td.Type != "function" {
		t.Errorf("Type = %q, want %q", td.Type, "function")
	}
	if td.Name != "file_search" {
		t.Errorf("Name = %q, want %q", td.Name, "file_search")
	}
	if td.Description == "" {
		t.Error("Description should not be empty")
	}
	if len(td.Parameters) == 0 {
		t.Error("Parameters should not be empty")
	}

	// Verify parameters schema has the expected structure.
	var params map[string]interface{}
	if err := json.Unmarshal(td.Parameters, &params); err != nil {
		t.Fatalf("failed to parse parameters JSON: %v", err)
	}
	if params["type"] != "object" {
		t.Errorf("parameters type = %v, want 'object'", params["type"])
	}
	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("parameters.properties is not an object")
	}
	if _, ok := props["query"]; !ok {
		t.Error("parameters.properties missing 'query'")
	}
	if _, ok := props["vector_store_ids"]; !ok {
		t.Error("parameters.properties missing 'vector_store_ids'")
	}
}

func TestFileSearch_EmbedQuery(t *testing.T) {
	backend := newMockBackend()
	var embeddedTexts []string
	embedding := newMockEmbedding(384)
	embedding.embedFn = func(texts []string) ([][]float32, error) {
		embeddedTexts = texts
		result := make([][]float32, len(texts))
		for i := range texts {
			result[i] = make([]float32, 384)
		}
		return result, nil
	}

	p := newWithDeps(backend, embedding, 10)

	// Create a store for the tenant.
	vs := &VectorStore{
		Name:           "docs",
		TenantID:       "t1",
		CollectionName: "col_docs",
		CreatedAt:      1700000000,
	}
	p.metadata.Create(vs)

	ctx := storage.SetTenant(context.Background(), "t1")
	_, err := p.Execute(ctx, tools.ToolCall{
		ID:        "call_emb",
		Name:      "file_search",
		Arguments: `{"query":"how does Go handle errors?"}`,
	})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if len(embeddedTexts) != 1 {
		t.Fatalf("expected 1 text embedded, got %d", len(embeddedTexts))
	}
	if embeddedTexts[0] != "how does Go handle errors?" {
		t.Errorf("embedded text = %q, want %q", embeddedTexts[0], "how does Go handle errors?")
	}
}

func TestFileSearch_Name(t *testing.T) {
	p, _ := setupProvider(t)
	if name := p.Name(); name != "file_search" {
		t.Errorf("Name() = %q, want %q", name, "file_search")
	}
}

func TestFileSearch_Routes(t *testing.T) {
	p, _ := setupProvider(t)
	routes := p.Routes()

	if len(routes) != 4 {
		t.Fatalf("Routes() returned %d routes, want 4", len(routes))
	}

	// Verify the expected route patterns.
	patterns := map[string]bool{
		"POST /vector_stores":              false,
		"GET /vector_stores":               false,
		"GET /vector_stores/{store_id}":     false,
		"DELETE /vector_stores/{store_id}":  false,
	}
	for _, route := range routes {
		key := route.Method + " " + route.Pattern
		if _, ok := patterns[key]; ok {
			patterns[key] = true
		} else {
			t.Errorf("unexpected route: %s %s", route.Method, route.Pattern)
		}
	}
	for pattern, found := range patterns {
		if !found {
			t.Errorf("missing expected route: %s", pattern)
		}
	}
}

func TestFileSearch_Collectors(t *testing.T) {
	p, _ := setupProvider(t)
	collectors := p.Collectors()
	if len(collectors) != 3 {
		t.Errorf("Collectors() returned %d collectors, want 3", len(collectors))
	}
}

func TestFileSearch_Close(t *testing.T) {
	p, _ := setupProvider(t)
	if err := p.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

func TestFileSearch_NoStoresAvailable(t *testing.T) {
	backend := newMockBackend()
	embedding := newMockEmbedding(384)
	p := newWithDeps(backend, embedding, 10)

	// No stores created, tenant has nothing.
	ctx := storage.SetTenant(context.Background(), "tenant-1")
	result, err := p.Execute(ctx, tools.ToolCall{
		ID:        "call_no_stores",
		Name:      "file_search",
		Arguments: `{"query":"test"}`,
	})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result.IsError {
		t.Errorf("expected IsError = false, got error: %s", result.Output)
	}
	if !strings.Contains(result.Output, "No vector stores available") {
		t.Errorf("expected 'No vector stores available' message, got: %s", result.Output)
	}
}

func TestFileSearch_SearchMergesAndRanks(t *testing.T) {
	backend := newMockBackend()
	embedding := newMockEmbedding(384)
	p := newWithDeps(backend, embedding, 5)

	// Create two stores.
	vs1 := &VectorStore{Name: "store1", TenantID: "", CollectionName: "col1", CreatedAt: 1}
	vs2 := &VectorStore{Name: "store2", TenantID: "", CollectionName: "col2", CreatedAt: 2}
	p.metadata.Create(vs1)
	p.metadata.Create(vs2)

	callCount := 0
	backend.searchFn = func(collection string, vector []float32, maxResults int) ([]SearchMatch, error) {
		callCount++
		if collection == "col1" {
			return []SearchMatch{
				{DocumentID: "a", Score: 0.90, Content: "result from store1"},
			}, nil
		}
		return []SearchMatch{
			{DocumentID: "b", Score: 0.95, Content: "result from store2"},
			{DocumentID: "c", Score: 0.70, Content: "low score from store2"},
		}, nil
	}

	result, err := p.Execute(context.Background(), tools.ToolCall{
		ID:        "call_merge",
		Name:      "file_search",
		Arguments: `{"query":"test"}`,
	})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	// Both stores should be searched.
	if callCount != 2 {
		t.Errorf("expected 2 backend search calls, got %d", callCount)
	}

	// Results should be sorted by score: store2(0.95), store1(0.90), store2(0.70).
	if !strings.Contains(result.Output, "result from store2") {
		t.Errorf("output missing store2 result, got: %s", result.Output)
	}

	// The highest-score result should be first (1.).
	idx1 := strings.Index(result.Output, "result from store2")
	idx2 := strings.Index(result.Output, "result from store1")
	if idx1 > idx2 {
		t.Errorf("store2 result (score 0.95) should appear before store1 (0.90)")
	}
}

func TestFileSearch_CreateStoreEmptyName(t *testing.T) {
	p, _ := setupProvider(t)

	req := httptest.NewRequest(http.MethodPost, "/vector_stores", strings.NewReader(`{"name":""}`))
	w := httptest.NewRecorder()
	p.handleCreateStore(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("create with empty name returned status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestFileSearch_GetStoreNotFound(t *testing.T) {
	p, _ := setupProvider(t)

	req := httptest.NewRequest(http.MethodGet, "/vector_stores/vs_nonexistent", nil)
	req.SetPathValue("store_id", "vs_nonexistent")
	w := httptest.NewRecorder()
	p.handleGetStore(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("get non-existent store returned status %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestFileSearch_DeleteStoreNotFound(t *testing.T) {
	p, _ := setupProvider(t)

	req := httptest.NewRequest(http.MethodDelete, "/vector_stores/vs_nonexistent", nil)
	req.SetPathValue("store_id", "vs_nonexistent")
	w := httptest.NewRecorder()
	p.handleDeleteStore(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("delete non-existent store returned status %d, want %d", w.Code, http.StatusNotFound)
	}
}
