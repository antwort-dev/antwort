//go:build cluster

package cluster

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/responses"
)

func bfclTestDataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "testdata", "bfcl")
}

func TestBFCLSimple(t *testing.T) {
	skipIfBFCLAll(t)
	runBFCLCategory(t, "simple_python")
}

func TestBFCLIrrelevance(t *testing.T) {
	skipIfBFCLAll(t)
	runBFCLCategory(t, "irrelevance")
}

func TestBFCLMultiple(t *testing.T) {
	skipIfBFCLAll(t)
	runBFCLCategory(t, "multiple")
}

func TestBFCLParallel(t *testing.T) {
	skipIfBFCLAll(t)
	runBFCLCategory(t, "parallel")
}

func TestBFCLParallelMultiple(t *testing.T) {
	skipIfBFCLAll(t)
	runBFCLCategory(t, "parallel_multiple")
}

func skipIfBFCLAll(t *testing.T) {
	t.Helper()
	if bfclAll {
		t.Skip("skipping individual category test, CLUSTER_BFCL_ALL runs all categories via TestBFCLAll")
	}
}

func TestBFCLAll(t *testing.T) {
	if !bfclAll {
		t.Skip("CLUSTER_BFCL_ALL not set, skipping full BFCL suite")
	}
	t.Log("Running full BFCL suite (this may take a long time)")
	// Full suite would load all categories from a downloaded dataset
	// For now, run all available categories
	for _, cat := range []string{"simple_python", "multiple", "parallel", "parallel_multiple", "irrelevance"} {
		t.Run(cat, func(t *testing.T) {
			runBFCLCategory(t, cat)
		})
	}
}

func runBFCLCategory(t *testing.T, category string) {
	t.Helper()
	dir := bfclTestDataDir()
	cases, err := LoadBFCLCases(dir, category)
	if err != nil {
		t.Skipf("%s test data not available: %v", category, err)
		return
	}
	runBFCLCases(t, cases, category)
}

func runBFCLCases(t *testing.T, cases []BFCLCase, category string) {
	t.Helper()
	dir := bfclTestDataDir()

	answers, err := LoadBFCLAnswers(dir, category)
	if err != nil {
		t.Fatalf("loading answers for %s: %v", category, err)
	}

	client := newAntwortClient()

	for _, tc := range cases {
		tc := tc
		t.Run(tc.ID, func(t *testing.T) {
			ctx := testContext(t)

			// Get ground truth
			gtRaw, ok := answers[tc.ID]
			if !ok {
				t.Fatalf("no ground truth for %s", tc.ID)
			}

			// Parse expected function calls
			var expected []BFCLFunctionCall
			for _, raw := range gtRaw {
				calls, err := ParseGroundTruth(raw)
				if err != nil {
					t.Fatalf("parsing ground truth: %v", err)
				}
				expected = append(expected, calls...)
			}

			// Convert tools to OpenAPI format
			tools, err := ConvertGorillaTools(tc.Functions)
			if err != nil {
				t.Fatalf("converting tools: %v", err)
			}

			// Build tool params
			var toolParams []responses.ToolUnionParam
			for _, tool := range tools {
				toolParams = append(toolParams, responses.ToolUnionParam{
					OfFunction: &responses.FunctionToolParam{
						Name:        tool.Name,
						Description: openai.String(tool.Description),
						Parameters:  tool.Parameters,
					},
				})
			}

			// Get prompt from first turn, first message
			prompt := ""
			if len(tc.Question) > 0 && len(tc.Question[0]) > 0 {
				prompt = tc.Question[0][0].Content
			}
			if prompt == "" {
				t.Skip("empty prompt")
			}

			// Send request
			start := time.Now()
			resp, err := client.Responses.New(ctx, responses.ResponseNewParams{
				Model: model,
				Input: userInput(prompt),
				Temperature: openai.Float(0),
				Tools:       toolParams,
			})
			duration := time.Since(start)

			if err != nil {
				collector.Record(TestResult{
					Name:     tc.ID,
					Category: "bfcl_" + category,
					Passed:   false,
					Duration: duration,
					Error:    err.Error(),
				})
				t.Fatalf("request failed: %v", err)
			}

			// Extract function calls from response
			var outputItems []map[string]interface{}
			for _, item := range resp.Output {
				m := map[string]interface{}{
					"type":      item.Type,
					"name":      item.Name,
					"arguments": item.Arguments,
				}
				outputItems = append(outputItems, m)
			}
			got := ParseFunctionCallOutput(outputItems)

			// Score
			passed, reason := ScoreBFCL(category, expected, got)

			if !passed {
				expectedJSON, _ := json.Marshal(expected)
				gotJSON, _ := json.Marshal(got)
				t.Errorf("BFCL %s failed: %s\n  expected: %s\n  got: %s",
					tc.ID, reason, expectedJSON, gotJSON)
			}

			collector.Record(TestResult{
				Name:     tc.ID,
				Category: "bfcl_" + category,
				Passed:   passed,
				Duration: duration,
				Error:    reason,
			})
		})
	}
}
