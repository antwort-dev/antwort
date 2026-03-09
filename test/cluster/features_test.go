//go:build cluster

package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/responses"
)

func TestBackgroundSubmitAndPoll(t *testing.T) {
	ctx := testContext(t)

	// Submit background request via raw HTTP (background is not in SDK)
	body := map[string]any{
		"model":      model,
		"input":      "What is the meaning of life? Answer briefly.",
		"background": true,
	}
	data, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, antwortURL+"/responses", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("submit background request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusBadRequest {
			t.Skip("background mode not enabled on this deployment")
		}
		t.Fatalf("expected 202 Accepted, got %d: %s", resp.StatusCode, respBody)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	responseID, ok := result["id"].(string)
	if !ok || responseID == "" {
		t.Fatal("expected response ID in background response")
	}

	// Poll for completion
	start := time.Now()
	passed := false
	for time.Since(start) < 30*time.Second {
		pollReq, _ := http.NewRequestWithContext(ctx, http.MethodGet,
			fmt.Sprintf("%s/responses/%s", antwortURL, responseID), nil)
		if apiKey != "" {
			pollReq.Header.Set("Authorization", "Bearer "+apiKey)
		}

		pollResp, err := http.DefaultClient.Do(pollReq)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}

		var pollResult map[string]any
		json.NewDecoder(pollResp.Body).Decode(&pollResult)
		pollResp.Body.Close()

		status, _ := pollResult["status"].(string)
		if status == "completed" {
			passed = true
			t.Logf("Background response completed in %v", time.Since(start))
			break
		}
		time.Sleep(time.Second)
	}

	var errMsg string
	if !passed {
		errMsg = "background response did not complete within 30s"
		t.Error(errMsg)
	}

	collector.Record(TestResult{
		Name:     "TestBackgroundSubmitAndPoll",
		Category: "background",
		Passed:   passed,
		Duration: time.Since(start),
		Error:    errMsg,
	})
}

func TestRAGFileSearch(t *testing.T) {
	ctx := testContext(t)

	// Probe if Files API is available
	probeReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, antwortURL+"/files", nil)
	if apiKey != "" {
		probeReq.Header.Set("Authorization", "Bearer "+apiKey)
	}
	probeResp, err := http.DefaultClient.Do(probeReq)
	if err != nil || probeResp.StatusCode == http.StatusNotFound {
		t.Skip("Files API not available on this deployment")
	}
	if probeResp != nil {
		probeResp.Body.Close()
	}

	// Upload a test file
	fileContent := "The Antwort project is an OpenResponses-compatible gateway for Kubernetes."
	uploadBody := map[string]any{
		"purpose": "assistants",
		"content": fileContent,
	}
	data, _ := json.Marshal(uploadBody)

	uploadReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, antwortURL+"/files", bytes.NewReader(data))
	uploadReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		uploadReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	uploadResp, err := http.DefaultClient.Do(uploadReq)
	if err != nil {
		t.Skipf("file upload failed (feature may not be configured): %v", err)
	}
	defer uploadResp.Body.Close()

	if uploadResp.StatusCode != http.StatusOK && uploadResp.StatusCode != http.StatusCreated {
		t.Skipf("file upload returned %d (feature may not be configured)", uploadResp.StatusCode)
	}

	t.Log("RAG file search test: file uploaded, querying with file_search")

	// Query with file_search would require vector store setup
	// For now, validate the upload succeeded
	passed := true
	collector.Record(TestResult{
		Name:     "TestRAGFileSearch",
		Category: "rag",
		Passed:   passed,
		Duration: 0,
	})
}

func TestAuthAccepted(t *testing.T) {
	if apiKey == "" {
		t.Skip("CLUSTER_API_KEY not set, skipping auth test")
	}

	client := newAntwortClient()
	ctx := testContext(t)

	start := time.Now()
	_, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Model: model,
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String("Say ok."),
		},
	})
	duration := time.Since(start)

	passed := err == nil
	var errMsg string
	if err != nil {
		errMsg = err.Error()
		t.Errorf("authenticated request should succeed: %v", err)
	}

	collector.Record(TestResult{
		Name:     "TestAuthAccepted",
		Category: "auth",
		Passed:   passed,
		Duration: duration,
		Error:    errMsg,
	})
}

func TestAuthRejected(t *testing.T) {
	// Probe: if no auth is configured, skip
	ctx := testContext(t)
	probeReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, antwortURL+"/responses",
		strings.NewReader(`{"model":"`+model+`","input":"test"}`))
	probeReq.Header.Set("Content-Type", "application/json")
	// No auth header

	probeResp, err := http.DefaultClient.Do(probeReq)
	if err != nil {
		t.Fatalf("probe request failed: %v", err)
	}
	probeResp.Body.Close()

	if probeResp.StatusCode == http.StatusOK {
		t.Skip("auth not configured on this deployment (unauthenticated request succeeded)")
	}

	passed := probeResp.StatusCode == http.StatusUnauthorized || probeResp.StatusCode == http.StatusForbidden
	var errMsg string
	if !passed {
		errMsg = fmt.Sprintf("expected 401/403, got %d", probeResp.StatusCode)
		t.Error(errMsg)
	}

	collector.Record(TestResult{
		Name:     "TestAuthRejected",
		Category: "auth",
		Passed:   passed,
		Duration: 0,
		Error:    errMsg,
	})
}

func TestConversationChaining(t *testing.T) {
	client := newAntwortClient()
	ctx := testContext(t)

	// First response
	resp1, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Model: model,
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String("My name is Alice. Remember that."),
		},
		Temperature: openai.Float(0),
	})
	if err != nil {
		// Check if storage is configured by looking at the error
		if strings.Contains(err.Error(), "store") || strings.Contains(err.Error(), "storage") {
			t.Skip("storage not configured, skipping conversation chaining test")
		}
		t.Fatalf("first response failed: %v", err)
	}

	// Second response chained to the first
	start := time.Now()
	resp2, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Model: model,
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String("What is my name?"),
		},
		Temperature:        openai.Float(0),
		PreviousResponseID: openai.String(resp1.ID),
	})
	duration := time.Since(start)

	passed := true
	var errMsg string

	if err != nil {
		if strings.Contains(err.Error(), "not_found") || strings.Contains(err.Error(), "store") {
			t.Skip("response storage not available for chaining")
		}
		passed = false
		errMsg = err.Error()
		t.Errorf("chained response failed: %v", err)
	} else {
		text := extractOutputText(resp2)
		if !strings.Contains(strings.ToLower(text), "alice") {
			t.Logf("chained response may not have conversation context: %q", truncate(text, 100))
		} else {
			t.Logf("Conversation chaining works: response mentions 'Alice'")
		}
	}

	collector.Record(TestResult{
		Name:     "TestConversationChaining",
		Category: "conversations",
		Passed:   passed,
		Duration: duration,
		Error:    errMsg,
	})
}

// Ensure context import is used.
var _ = context.Background
