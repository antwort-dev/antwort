//go:build cluster

package cluster

import (
	"strings"
	"testing"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/responses"
)

const comparisonPrompt = "What is the capital of France? Answer in one word."

func TestMultiProviderNonStreaming(t *testing.T) {
	client := newAntwortClient()
	ctx := testContext(t)

	// Path 1: Antwort
	start := time.Now()
	resp, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Model: model,
		Input: userInput(comparisonPrompt),
		Temperature: openai.Float(0),
	})
	antwortDuration := time.Since(start)

	passed := true
	var errMsg string

	if err != nil {
		passed = false
		errMsg = "antwort request failed: " + err.Error()
		t.Fatalf("antwort non-streaming failed: %v", err)
	}

	antwortText := extractOutputText(resp)
	if antwortText == "" {
		passed = false
		errMsg = "antwort returned empty text"
		t.Error("antwort returned empty output text")
	}

	collector.Record(TestResult{
		Name:         "TestMultiProviderNonStreaming",
		Category:     "provider_comparison",
		ProviderPath: "antwort",
		Passed:       passed,
		Duration:     antwortDuration,
		Error:        errMsg,
	})

	t.Logf("Antwort: %v, text=%q", antwortDuration, truncate(antwortText, 50))

	// Path 2: Direct vLLM (if available)
	vllm := newVLLMClient()
	if vllm == nil {
		t.Log("CLUSTER_VLLM_URL not set, skipping direct vLLM comparison")
		return
	}

	start = time.Now()
	vllmResp, err := vllm.Responses.New(ctx, responses.ResponseNewParams{
		Model: model,
		Input: userInput(comparisonPrompt),
		Temperature: openai.Float(0),
	})
	vllmDuration := time.Since(start)

	vllmPassed := true
	var vllmErr string

	if err != nil {
		vllmPassed = false
		vllmErr = "vllm request failed: " + err.Error()
		t.Errorf("direct vLLM non-streaming failed: %v", err)
	} else {
		vllmText := extractOutputText(vllmResp)
		if vllmText == "" {
			vllmPassed = false
			vllmErr = "vllm returned empty text"
			t.Error("direct vLLM returned empty output text")
		}
		t.Logf("vLLM direct: %v, text=%q", vllmDuration, truncate(vllmText, 50))
	}

	collector.Record(TestResult{
		Name:         "TestMultiProviderNonStreaming",
		Category:     "provider_comparison",
		ProviderPath: "vllm_direct",
		Passed:       vllmPassed,
		Duration:     vllmDuration,
		Error:        vllmErr,
	})

	overhead := antwortDuration - vllmDuration
	t.Logf("Gateway overhead: %v", overhead)
}

func TestMultiProviderStreaming(t *testing.T) {
	client := newAntwortClient()
	ctx := testContext(t)

	// Path 1: Antwort streaming
	start := time.Now()
	stream := client.Responses.NewStreaming(ctx, responses.ResponseNewParams{
		Model: model,
		Input: userInput(comparisonPrompt),
		Temperature: openai.Float(0),
	})

	var antwortTTFT time.Duration
	firstToken := false
	for stream.Next() {
		if !firstToken && stream.Current().Type == "response.output_text.delta" {
			antwortTTFT = time.Since(start)
			firstToken = true
		}
	}
	antwortDuration := time.Since(start)

	passed := stream.Err() == nil
	var errMsg string
	if !passed {
		errMsg = stream.Err().Error()
		t.Errorf("antwort streaming failed: %v", stream.Err())
	}

	collector.Record(TestResult{
		Name:         "TestMultiProviderStreaming",
		Category:     "provider_comparison",
		ProviderPath: "antwort",
		Passed:       passed,
		Duration:     antwortDuration,
		TTFT:         antwortTTFT,
		Error:        errMsg,
	})

	t.Logf("Antwort streaming: TTFT=%v, Total=%v", antwortTTFT, antwortDuration)

	// Path 2: Direct vLLM streaming
	vllm := newVLLMClient()
	if vllm == nil {
		t.Log("CLUSTER_VLLM_URL not set, skipping direct vLLM streaming comparison")
		return
	}

	start = time.Now()
	vllmStream := vllm.Responses.NewStreaming(ctx, responses.ResponseNewParams{
		Model: model,
		Input: userInput(comparisonPrompt),
		Temperature: openai.Float(0),
	})

	var vllmTTFT time.Duration
	firstToken = false
	for vllmStream.Next() {
		if !firstToken && vllmStream.Current().Type == "response.output_text.delta" {
			vllmTTFT = time.Since(start)
			firstToken = true
		}
	}
	vllmDuration := time.Since(start)

	vllmPassed := vllmStream.Err() == nil
	var vllmErr string
	if !vllmPassed {
		vllmErr = vllmStream.Err().Error()
		t.Errorf("direct vLLM streaming failed: %v", vllmStream.Err())
	}

	collector.Record(TestResult{
		Name:         "TestMultiProviderStreaming",
		Category:     "provider_comparison",
		ProviderPath: "vllm_direct",
		Passed:       vllmPassed,
		Duration:     vllmDuration,
		TTFT:         vllmTTFT,
		Error:        vllmErr,
	})

	t.Logf("vLLM streaming: TTFT=%v, Total=%v", vllmTTFT, vllmDuration)
	t.Logf("TTFT overhead: %v", antwortTTFT-vllmTTFT)
}

func TestMultiProviderOverhead(t *testing.T) {
	skipIfNoVLLM(t)

	client := newAntwortClient()
	vllm := newVLLMClient()
	const numRequests = 5

	var antwortTotal, vllmTotal time.Duration

	for i := 0; i < numRequests; i++ {
		ctx := testContext(t)

		start := time.Now()
		_, err := client.Responses.New(ctx, responses.ResponseNewParams{
			Model: model,
			Input: userInput("What is 1+1? Answer with just the number."),
			Temperature: openai.Float(0),
		})
		if err != nil {
			t.Fatalf("antwort request %d failed: %v", i, err)
		}
		antwortTotal += time.Since(start)

		start = time.Now()
		_, err = vllm.Responses.New(ctx, responses.ResponseNewParams{
			Model: model,
			Input: userInput("What is 1+1? Answer with just the number."),
			Temperature: openai.Float(0),
		})
		if err != nil {
			t.Fatalf("vllm request %d failed: %v", i, err)
		}
		vllmTotal += time.Since(start)
	}

	antwortAvg := antwortTotal / time.Duration(numRequests)
	vllmAvg := vllmTotal / time.Duration(numRequests)
	overhead := antwortAvg - vllmAvg

	t.Logf("Antwort avg: %v", antwortAvg)
	t.Logf("vLLM avg: %v", vllmAvg)
	t.Logf("Gateway overhead: %v", overhead)

	collector.Record(TestResult{
		Name:     "TestMultiProviderOverhead",
		Category: "provider_comparison",
		Passed:   true,
		Duration: overhead,
		Details: map[string]any{
			"antwort_avg_ms": float64(antwortAvg.Milliseconds()),
			"vllm_avg_ms":    float64(vllmAvg.Milliseconds()),
			"overhead_ms":    float64(overhead.Milliseconds()),
			"num_requests":   numRequests,
		},
	})
}

func truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
