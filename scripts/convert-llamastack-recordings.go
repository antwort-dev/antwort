// Command convert-llamastack-recordings converts llama-stack format recordings
// to the antwort recording format used by mock-backend's replay mode.
//
// Usage:
//
//	go run scripts/convert-llamastack-recordings.go <input-dir> <output-dir>
//
// The script reads each .json file from the input directory, parses the
// llama-stack format (request.body, request.method, request.endpoint, and
// response.body with __type__/__data__ wrappers), and outputs antwort-format
// recording files. Only recordings whose endpoint contains "/chat/completions"
// are converted; others (e.g. Ollama-native endpoints) are skipped.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// llamaStackRecording represents the llama-stack recording format.
type llamaStackRecording struct {
	Request struct {
		Method   string          `json:"method"`
		Endpoint string          `json:"endpoint"`
		Body     json.RawMessage `json:"body"`
	} `json:"request"`
	Response struct {
		StatusCode int             `json:"status_code"`
		Body       json.RawMessage `json:"body"`
	} `json:"response"`
}

// antwortRecording is the output format matching mock-backend's Recording type.
type antwortRecording struct {
	Request struct {
		Method string          `json:"method"`
		Path   string          `json:"path"`
		Body   json.RawMessage `json:"body"`
	} `json:"request"`
	Response struct {
		Status  int               `json:"status"`
		Headers map[string]string `json:"headers"`
		Body    json.RawMessage   `json:"body,omitempty"`
	} `json:"response"`
	Streaming bool              `json:"streaming"`
	Chunks    []string          `json:"chunks,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <input-dir> <output-dir>\n", os.Args[0])
		os.Exit(1)
	}

	inputDir := os.Args[1]
	outputDir := os.Args[2]

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	entries, err := os.ReadDir(inputDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input directory: %v\n", err)
		os.Exit(1)
	}

	converted := 0
	skipped := 0

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(inputDir, entry.Name()))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping %s: %v\n", entry.Name(), err)
			skipped++
			continue
		}

		var lsRec llamaStackRecording
		if err := json.Unmarshal(data, &lsRec); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping %s (invalid JSON): %v\n", entry.Name(), err)
			skipped++
			continue
		}

		// Filter: only convert recordings with endpoint containing /chat/completions.
		if !strings.Contains(lsRec.Request.Endpoint, "/chat/completions") {
			skipped++
			continue
		}

		out, isStreaming, err := convertRecording(&lsRec)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping %s (conversion error): %v\n", entry.Name(), err)
			skipped++
			continue
		}
		_ = isStreaming

		// Compute hash for filename.
		hash := computeHash(out.Request.Method, out.Request.Path, out.Request.Body)
		outData, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping %s (marshal error): %v\n", entry.Name(), err)
			skipped++
			continue
		}

		outPath := filepath.Join(outputDir, hash+".json")
		if err := os.WriteFile(outPath, outData, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to write %s: %v\n", outPath, err)
			skipped++
			continue
		}

		converted++
	}

	fmt.Printf("Converted %d recordings, skipped %d (non-Chat Completions or errors)\n", converted, skipped)
}

// convertRecording transforms a llama-stack recording into the antwort format.
func convertRecording(lsRec *llamaStackRecording) (*antwortRecording, bool, error) {
	out := &antwortRecording{}
	out.Request.Method = lsRec.Request.Method
	if out.Request.Method == "" {
		out.Request.Method = "POST"
	}
	out.Request.Path = lsRec.Request.Endpoint
	out.Request.Body = lsRec.Request.Body

	status := lsRec.Response.StatusCode
	if status == 0 {
		status = 200
	}
	out.Response.Status = status
	out.Response.Headers = map[string]string{"Content-Type": "application/json"}

	out.Metadata = map[string]string{
		"source": "llamastack-conversion",
	}

	// Unwrap response body: handle __type__/__data__ wrapper.
	responseBody, isStreaming, err := unwrapResponseBody(lsRec.Response.Body)
	if err != nil {
		return nil, false, fmt.Errorf("unwrapping response body: %w", err)
	}

	out.Streaming = isStreaming

	if isStreaming {
		// For streaming responses, responseBody is an array of data objects.
		// Reconstruct SSE chunks.
		var items []json.RawMessage
		if err := json.Unmarshal(responseBody, &items); err != nil {
			return nil, false, fmt.Errorf("parsing streaming items: %w", err)
		}

		out.Response.Headers["Content-Type"] = "text/event-stream"
		chunks := make([]string, 0, len(items)+1)
		for _, item := range items {
			// Each item may itself have __type__/__data__ wrapper.
			unwrapped := unwrapDataWrapper(item)
			chunks = append(chunks, fmt.Sprintf("data: %s\n\n", string(unwrapped)))
		}
		chunks = append(chunks, "data: [DONE]\n\n")
		out.Chunks = chunks
	} else {
		out.Response.Body = responseBody
	}

	return out, isStreaming, nil
}

// unwrapResponseBody handles the __type__/__data__ wrapper in llama-stack responses.
// Returns the unwrapped body and whether the response is streaming (array of chunks).
func unwrapResponseBody(raw json.RawMessage) (json.RawMessage, bool, error) {
	if len(raw) == 0 {
		return raw, false, nil
	}

	// Try to parse as an object with __type__ and __data__.
	var wrapper struct {
		Type string          `json:"__type__"`
		Data json.RawMessage `json:"__data__"`
	}
	if err := json.Unmarshal(raw, &wrapper); err == nil && wrapper.Type != "" && wrapper.Data != nil {
		// Check if __data__ is an array (streaming).
		trimmed := strings.TrimSpace(string(wrapper.Data))
		if strings.HasPrefix(trimmed, "[") {
			return wrapper.Data, true, nil
		}
		// Single object, unwrap further.
		return unwrapDataWrapper(wrapper.Data), false, nil
	}

	// No wrapper, check if it is an array (streaming without wrapper).
	trimmed := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trimmed, "[") {
		// Could be a streaming response array.
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err == nil && len(items) > 0 {
			// Check if items look like SSE chunks (have choices with delta).
			var firstItem map[string]any
			if json.Unmarshal(items[0], &firstItem) == nil {
				if _, hasDelta := firstItem["choices"]; hasDelta {
					return raw, true, nil
				}
			}
		}
	}

	return raw, false, nil
}

// unwrapDataWrapper recursively removes __type__/__data__ wrappers from a JSON value.
func unwrapDataWrapper(raw json.RawMessage) json.RawMessage {
	var wrapper struct {
		Type string          `json:"__type__"`
		Data json.RawMessage `json:"__data__"`
	}
	if err := json.Unmarshal(raw, &wrapper); err == nil && wrapper.Type != "" && wrapper.Data != nil {
		return unwrapDataWrapper(wrapper.Data)
	}
	return raw
}

// computeHash returns SHA256 hex matching mock-backend's computeRequestHash.
func computeHash(method, path string, body []byte) string {
	normalized, err := normalizeJSON(body)
	if err != nil {
		normalized = body
	}
	input := method + "\n" + path + "\n" + string(normalized)
	sum := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", sum)
}

// normalizeJSON sorts object keys and returns compact JSON (matching mock-backend).
func normalizeJSON(data []byte) ([]byte, error) {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	normalized := normalizeValue(raw)
	return json.Marshal(normalized)
}

func normalizeValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		sorted := make(map[string]any, len(val))
		keys := make([]string, 0, len(val))
		for k := range val {
			if k == "stream_options" {
				continue
			}
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sorted[k] = normalizeValue(val[k])
		}
		return sorted
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = normalizeValue(item)
		}
		return result
	default:
		return v
	}
}
