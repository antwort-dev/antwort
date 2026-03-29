//go:build cluster

package cluster

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

// ScoreBFCL evaluates model output against BFCL ground truth.
// Returns true if the output matches the expected function calls.
func ScoreBFCL(category string, expected []BFCLFunctionCall, got []ParsedCall) (bool, string) {
	switch {
	case strings.Contains(category, "irrelevance"):
		return irrelevanceChecker(got)
	case strings.Contains(category, "parallel"):
		return parallelChecker(expected, got)
	case strings.Contains(category, "multiple"):
		return multipleChecker(expected, got)
	default:
		return simpleChecker(expected, got)
	}
}

// simpleChecker verifies exactly 1 function call matches expected.
func simpleChecker(expected []BFCLFunctionCall, got []ParsedCall) (bool, string) {
	if len(got) == 0 {
		return false, "no function calls produced"
	}
	if len(expected) == 0 {
		return false, "no expected function calls"
	}

	// Match first expected against first got
	exp := expected[0]
	actual := got[0]

	if exp.Name != actual.Name {
		return false, fmt.Sprintf("function name mismatch: expected %q, got %q", exp.Name, actual.Name)
	}

	return checkArguments(exp, actual)
}

// parallelChecker verifies all expected calls are present (order-independent).
func parallelChecker(expected []BFCLFunctionCall, got []ParsedCall) (bool, string) {
	if len(got) != len(expected) {
		return false, fmt.Sprintf("function count mismatch: expected %d, got %d", len(expected), len(got))
	}

	matched := make([]bool, len(got))
	for _, exp := range expected {
		found := false
		for i, actual := range got {
			if matched[i] {
				continue
			}
			if exp.Name == actual.Name {
				ok, _ := checkArguments(exp, actual)
				if ok {
					matched[i] = true
					found = true
					break
				}
			}
		}
		if !found {
			return false, fmt.Sprintf("no match found for expected call %q", exp.Name)
		}
	}
	return true, ""
}

// multipleChecker verifies exactly 1 call is made (from multiple options).
func multipleChecker(expected []BFCLFunctionCall, got []ParsedCall) (bool, string) {
	if len(got) != 1 {
		return false, fmt.Sprintf("expected 1 function call, got %d", len(got))
	}
	if len(expected) != 1 {
		return false, fmt.Sprintf("expected 1 ground truth entry, got %d", len(expected))
	}
	return simpleChecker(expected, got)
}

// irrelevanceChecker verifies NO function calls are made.
func irrelevanceChecker(got []ParsedCall) (bool, string) {
	if len(got) > 0 {
		names := make([]string, len(got))
		for i, c := range got {
			names[i] = c.Name
		}
		return false, fmt.Sprintf("expected no function calls, got: %s", strings.Join(names, ", "))
	}
	return true, ""
}

// checkArguments verifies function arguments match expected values.
func checkArguments(exp BFCLFunctionCall, actual ParsedCall) (bool, string) {
	// Check all required params are present
	for paramName, acceptableValues := range exp.Arguments {
		actualVal, exists := actual.Arguments[paramName]

		// Check if parameter is optional (empty string in acceptable values)
		isOptional := false
		for _, av := range acceptableValues {
			if s, ok := av.(string); ok && s == "" {
				isOptional = true
				break
			}
		}

		if !exists {
			if isOptional {
				continue
			}
			return false, fmt.Sprintf("missing required parameter %q", paramName)
		}

		// Check value is in acceptable list
		if !valueInAcceptable(actualVal, acceptableValues) {
			return false, fmt.Sprintf("parameter %q: value %v not in acceptable values %v",
				paramName, actualVal, acceptableValues)
		}
	}

	// Note: we do NOT reject unexpected parameters. The model may include
	// optional parameters with default values that the ground truth omits.

	return true, ""
}

// valueInAcceptable checks if a value matches any of the acceptable values.
func valueInAcceptable(actual interface{}, acceptable []interface{}) bool {
	for _, acc := range acceptable {
		if acc == nil && actual == nil {
			return true
		}
		if acc == nil || actual == nil {
			continue
		}
		// Skip empty string (marks optional)
		if s, ok := acc.(string); ok && s == "" {
			continue
		}
		if valuesMatch(actual, acc) {
			return true
		}
	}
	return false
}

// valuesMatch compares two values with type-aware comparison.
func valuesMatch(actual, expected interface{}) bool {
	// String comparison (case-insensitive, normalized)
	if aStr, ok := actual.(string); ok {
		if eStr, ok := expected.(string); ok {
			return standardizeString(aStr) == standardizeString(eStr)
		}
	}

	// Numeric comparison (int/float flexibility)
	if aNum, ok := toFloat64(actual); ok {
		if eNum, ok := toFloat64(expected); ok {
			return aNum == eNum
		}
	}

	// Bool comparison
	if aBool, ok := actual.(bool); ok {
		if eBool, ok := expected.(bool); ok {
			return aBool == eBool
		}
	}

	// Map comparison: ground truth maps have values wrapped in acceptable-value
	// arrays (e.g. {"key": [val1, val2]}), model output has plain values
	// (e.g. {"key": val}). Recursively check each key.
	if aMap, ok := actual.(map[string]interface{}); ok {
		if eMap, ok := expected.(map[string]interface{}); ok {
			return mapsMatch(aMap, eMap)
		}
	}

	// Array comparison: element-wise, order-sensitive
	if aArr, ok := actual.([]interface{}); ok {
		if eArr, ok := expected.([]interface{}); ok {
			return arraysMatch(aArr, eArr)
		}
	}

	// Fallback: string representation
	return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", expected)
}

// mapsMatch compares two maps. Expected map values may be acceptable-value
// arrays ([]interface{}) where the actual value must match any element.
func mapsMatch(actual, expected map[string]interface{}) bool {
	if len(actual) != len(expected) {
		return false
	}
	for key, eVal := range expected {
		aVal, exists := actual[key]
		if !exists {
			return false
		}
		// If expected value is an array, treat as acceptable-values list
		if eArr, ok := eVal.([]interface{}); ok {
			if !valueInAcceptable(aVal, eArr) {
				return false
			}
		} else if !valuesMatch(aVal, eVal) {
			return false
		}
	}
	return true
}

// arraysMatch compares two arrays element-wise.
func arraysMatch(actual, expected []interface{}) bool {
	if len(actual) != len(expected) {
		return false
	}
	for i := range actual {
		if !valuesMatch(actual[i], expected[i]) {
			return false
		}
	}
	return true
}

// standardizeString normalizes a string for comparison.
// Removes punctuation, spaces, and lowercases.
func standardizeString(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "'", "\"")
	var result []rune
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '"' {
			result = append(result, r)
		}
	}
	return string(result)
}

// toFloat64 converts numeric types to float64.
func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}
