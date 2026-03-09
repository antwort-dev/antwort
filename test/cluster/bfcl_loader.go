//go:build cluster

package cluster

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BFCLCase represents a single BFCL benchmark test case.
type BFCLCase struct {
	ID          string            `json:"id"`
	Category    string            `json:"-"`
	Question    [][]BFCLMessage   `json:"question"`
	Functions   []json.RawMessage `json:"function"`
	GroundTruth []json.RawMessage `json:"ground_truth"`
}

// BFCLMessage is a chat message in a BFCL test case.
type BFCLMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// BFCLToolDef is a function definition in Gorilla format (before conversion).
type BFCLToolDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// BFCLFunctionCall represents an expected function call from ground truth.
type BFCLFunctionCall struct {
	Name      string
	Arguments map[string][]interface{} // param -> list of acceptable values
}

// ConvertedTool is an OpenAPI-format tool ready for the Responses API.
type ConvertedTool struct {
	Type       string             `json:"type"`
	Name       string             `json:"name"`
	Parameters map[string]any     `json:"parameters"`
	Strict     bool               `json:"strict,omitempty"`
	Description string            `json:"description,omitempty"`
}

// LoadBFCLCases loads test cases from a JSONL file.
func LoadBFCLCases(dir, category string) ([]BFCLCase, error) {
	path := filepath.Join(dir, category+".jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	var cases []BFCLCase
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		var c BFCLCase
		if err := json.Unmarshal(scanner.Bytes(), &c); err != nil {
			return nil, fmt.Errorf("parsing case in %s: %w", path, err)
		}
		c.Category = category
		cases = append(cases, c)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	return cases, nil
}

// LoadBFCLAnswers loads ground truth answers from a JSONL file.
func LoadBFCLAnswers(dir, category string) (map[string][]json.RawMessage, error) {
	path := filepath.Join(dir, "answers", category+".jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening answers %s: %w", path, err)
	}
	defer f.Close()

	answers := make(map[string][]json.RawMessage)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		var entry struct {
			ID          string            `json:"id"`
			GroundTruth []json.RawMessage `json:"ground_truth"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return nil, fmt.Errorf("parsing answer in %s: %w", path, err)
		}
		answers[entry.ID] = entry.GroundTruth
	}

	return answers, scanner.Err()
}

// ConvertGorillaTools converts BFCL Gorilla-format functions to OpenAPI tool definitions.
func ConvertGorillaTools(rawFunctions []json.RawMessage) ([]ConvertedTool, error) {
	var tools []ConvertedTool
	for _, raw := range rawFunctions {
		var def BFCLToolDef
		if err := json.Unmarshal(raw, &def); err != nil {
			return nil, fmt.Errorf("parsing function def: %w", err)
		}

		// Convert name: dots to underscores
		name := strings.ReplaceAll(def.Name, ".", "_")

		// Convert parameter types from Gorilla to OpenAPI
		params := convertParameters(def.Parameters)

		tools = append(tools, ConvertedTool{
			Type:        "function",
			Name:        name,
			Description: def.Description,
			Parameters:  params,
		})
	}
	return tools, nil
}

// convertParameters converts Gorilla parameter types to OpenAPI.
func convertParameters(params map[string]interface{}) map[string]any {
	result := make(map[string]any)
	for k, v := range params {
		switch k {
		case "type":
			if s, ok := v.(string); ok {
				result["type"] = convertType(s)
			} else {
				result["type"] = v
			}
		case "properties":
			if props, ok := v.(map[string]interface{}); ok {
				converted := make(map[string]any)
				for pk, pv := range props {
					if pm, ok := pv.(map[string]interface{}); ok {
						converted[pk] = convertParameters(pm)
					} else {
						converted[pk] = pv
					}
				}
				result["properties"] = converted
			}
		case "items":
			if items, ok := v.(map[string]interface{}); ok {
				result["items"] = convertParameters(items)
			} else {
				result["items"] = v
			}
		default:
			result[k] = v
		}
	}
	return result
}

// convertType maps Gorilla types to OpenAPI types.
func convertType(gorillaType string) string {
	switch gorillaType {
	case "dict":
		return "object"
	case "float":
		return "number"
	case "tuple":
		return "array"
	case "any":
		return "string"
	default:
		return gorillaType
	}
}

// ParseGroundTruth parses a BFCL ground truth entry into function calls.
func ParseGroundTruth(raw json.RawMessage) ([]BFCLFunctionCall, error) {
	// Ground truth is an array of objects, each {func_name: {param: [acceptable_values]}}
	var entries []map[string]map[string][]interface{}
	if err := json.Unmarshal(raw, &entries); err != nil {
		// Try single object format
		var single map[string]map[string][]interface{}
		if err2 := json.Unmarshal(raw, &single); err2 != nil {
			return nil, fmt.Errorf("parsing ground truth: %w (also tried single: %v)", err, err2)
		}
		entries = []map[string]map[string][]interface{}{single}
	}

	var calls []BFCLFunctionCall
	for _, entry := range entries {
		for funcName, args := range entry {
			calls = append(calls, BFCLFunctionCall{
				Name:      strings.ReplaceAll(funcName, ".", "_"),
				Arguments: args,
			})
		}
	}
	return calls, nil
}

// ParseModelOutput extracts function calls from a Responses API response output.
type ParsedCall struct {
	Name      string
	Arguments map[string]interface{}
}

// ParseFunctionCallOutput extracts function call name and arguments from response output items.
func ParseFunctionCallOutput(output []map[string]interface{}) []ParsedCall {
	var calls []ParsedCall
	for _, item := range output {
		itemType, _ := item["type"].(string)
		if itemType == "function_call" {
			name, _ := item["name"].(string)
			argsStr, _ := item["arguments"].(string)
			var args map[string]interface{}
			json.Unmarshal([]byte(argsStr), &args)
			calls = append(calls, ParsedCall{
				Name:      name,
				Arguments: args,
			})
		}
	}
	return calls
}
