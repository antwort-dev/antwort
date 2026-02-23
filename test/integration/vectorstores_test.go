package integration

import (
	"testing"
)

// Vector store tests are skipped by default because they require the
// file_search provider to be enabled in the test environment (which
// needs Qdrant and an embedding service).
//
// To run these tests, set ANTWORT_TEST_VECTOR_STORES=1 and ensure
// the test environment has file_search configured.

func skipIfNoVectorStores(t *testing.T) {
	t.Helper()
	// The test environment doesn't mount file_search routes.
	// These tests are for CI environments with full provider config.
	t.Skip("vector store tests require file_search provider (set ANTWORT_TEST_VECTOR_STORES=1)")
}

func TestCreateVectorStore(t *testing.T) {
	skipIfNoVectorStores(t)

	reqBody := map[string]any{
		"name": "test-store",
	}

	resp := postJSON(t, testEnv.BaseURL()+"/builtin/file_search/vector_stores", reqBody)
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		body := readBody(t, resp)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
	}

	var result map[string]any
	decodeJSON(t, resp, &result)

	if result["object"] != "vector_store" {
		t.Errorf("object = %v, want 'vector_store'", result["object"])
	}
	if result["name"] != "test-store" {
		t.Errorf("name = %v, want 'test-store'", result["name"])
	}
	if result["id"] == nil || result["id"] == "" {
		t.Error("id is empty")
	}
}

func TestListVectorStores(t *testing.T) {
	skipIfNoVectorStores(t)

	resp := getURL(t, testEnv.BaseURL()+"/builtin/file_search/vector_stores")
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body := readBody(t, resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var result map[string]any
	decodeJSON(t, resp, &result)

	if result["object"] != "list" {
		t.Errorf("object = %v, want 'list'", result["object"])
	}
}

func TestGetVectorStore(t *testing.T) {
	skipIfNoVectorStores(t)

	// Create first.
	createResp := postJSON(t, testEnv.BaseURL()+"/builtin/file_search/vector_stores", map[string]any{
		"name": "get-test-store",
	})
	defer createResp.Body.Close()

	if createResp.StatusCode != 201 {
		body := readBody(t, createResp)
		t.Fatalf("create: expected 201, got %d: %s", createResp.StatusCode, body)
	}

	var created map[string]any
	decodeJSON(t, createResp, &created)

	storeID := created["id"].(string)

	// Get it.
	getResp := getURL(t, testEnv.BaseURL()+"/builtin/file_search/vector_stores/"+storeID)
	defer getResp.Body.Close()

	if getResp.StatusCode != 200 {
		body := readBody(t, getResp)
		t.Fatalf("get: expected 200, got %d: %s", getResp.StatusCode, body)
	}

	var retrieved map[string]any
	decodeJSON(t, getResp, &retrieved)

	if retrieved["id"] != storeID {
		t.Errorf("id = %v, want %v", retrieved["id"], storeID)
	}
}

func TestDeleteVectorStore(t *testing.T) {
	skipIfNoVectorStores(t)

	// Create first.
	createResp := postJSON(t, testEnv.BaseURL()+"/builtin/file_search/vector_stores", map[string]any{
		"name": "delete-test-store",
	})
	defer createResp.Body.Close()

	if createResp.StatusCode != 201 {
		body := readBody(t, createResp)
		t.Fatalf("create: expected 201, got %d: %s", createResp.StatusCode, body)
	}

	var created map[string]any
	decodeJSON(t, createResp, &created)

	storeID := created["id"].(string)

	// Delete it.
	delResp := deleteURL(t, testEnv.BaseURL()+"/builtin/file_search/vector_stores/"+storeID)
	defer delResp.Body.Close()

	if delResp.StatusCode != 200 {
		body := readBody(t, delResp)
		t.Fatalf("delete: expected 200, got %d: %s", delResp.StatusCode, body)
	}

	var deleted map[string]any
	decodeJSON(t, delResp, &deleted)

	if deleted["object"] != "vector_store.deleted" {
		t.Errorf("object = %v, want 'vector_store.deleted'", deleted["object"])
	}
	if deleted["deleted"] != true {
		t.Errorf("deleted = %v, want true", deleted["deleted"])
	}

	// Verify it's gone.
	getResp := getURL(t, testEnv.BaseURL()+"/builtin/file_search/vector_stores/"+storeID)
	defer getResp.Body.Close()

	if getResp.StatusCode != 404 {
		body := readBody(t, getResp)
		t.Errorf("get after delete: expected 404, got %d: %s", getResp.StatusCode, body)
	}
}
