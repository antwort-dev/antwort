//go:build cluster

package cluster

import (
	"strings"
	"testing"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/responses"
)

func TestBasicNonStreaming(t *testing.T) {
	client := newAntwortClient()
	ctx := testContext(t)

	start := time.Now()
	resp, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Model: model,
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String("Say hello in exactly three words."),
		},
	})
	duration := time.Since(start)

	passed := true
	var errMsg string

	if err != nil {
		passed = false
		errMsg = err.Error()
		t.Fatalf("create response failed: %v", err)
	}

	if !strings.HasPrefix(resp.ID, "resp_") {
		passed = false
		errMsg = "response ID does not have resp_ prefix: " + resp.ID
		t.Errorf("expected resp_ prefix, got %s", resp.ID)
	}

	if resp.Model != model {
		t.Logf("model mismatch: expected %s, got %s (may differ by version suffix)", model, resp.Model)
	}

	text := extractOutputText(resp)
	if text == "" {
		passed = false
		errMsg = "response output text is empty"
		t.Error("expected non-empty output text")
	}

	if resp.Usage.InputTokens == 0 || resp.Usage.OutputTokens == 0 {
		t.Logf("usage stats may be zero: input=%d, output=%d", resp.Usage.InputTokens, resp.Usage.OutputTokens)
	}

	collector.Record(TestResult{
		Name:     "TestBasicNonStreaming",
		Category: "basic",
		Passed:   passed,
		Duration: duration,
		Error:    errMsg,
	})
}

func TestBasicStreaming(t *testing.T) {
	client := newAntwortClient()
	ctx := testContext(t)

	start := time.Now()
	stream := client.Responses.NewStreaming(ctx, responses.ResponseNewParams{
		Model: model,
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String("Count from 1 to 5."),
		},
	})

	var ttft time.Duration
	var events []string
	var textParts []string
	firstToken := false
	passed := true
	var errMsg string

	for stream.Next() {
		evt := stream.Current()
		events = append(events, evt.Type)

		if !firstToken && evt.Type == "response.output_text.delta" {
			ttft = time.Since(start)
			firstToken = true
		}

		if evt.Type == "response.output_text.delta" {
			textParts = append(textParts, evt.Delta.OfString)
		}
	}
	duration := time.Since(start)

	if err := stream.Err(); err != nil {
		passed = false
		errMsg = err.Error()
		t.Fatalf("streaming failed: %v", err)
	}

	if !containsEvent(events, "response.created") {
		passed = false
		errMsg = "missing response.created event"
		t.Error("expected response.created event in stream")
	}

	if !containsEvent(events, "response.completed") {
		passed = false
		errMsg = "missing response.completed event"
		t.Error("expected response.completed event in stream")
	}

	fullText := strings.Join(textParts, "")
	if fullText == "" {
		passed = false
		errMsg = "assembled streaming text is empty"
		t.Error("expected non-empty assembled text from stream")
	}

	t.Logf("TTFT: %v, Total: %v, Text length: %d, Events: %d", ttft, duration, len(fullText), len(events))

	collector.Record(TestResult{
		Name:     "TestBasicStreaming",
		Category: "streaming",
		Passed:   passed,
		Duration: duration,
		TTFT:     ttft,
		Error:    errMsg,
	})
}

func TestBasicMultipleRequests(t *testing.T) {
	client := newAntwortClient()
	const numRequests = 10

	for i := 0; i < numRequests; i++ {
		t.Run("request", func(t *testing.T) {
			ctx := testContext(t)
			start := time.Now()

			resp, err := client.Responses.New(ctx, responses.ResponseNewParams{
				Model: model,
				Input: responses.ResponseNewParamsInputUnion{
					OfString: openai.String("What is 2+2? Answer with just the number."),
				},
				Temperature: openai.Float(0),
			})
			duration := time.Since(start)

			passed := err == nil
			var errMsg string
			if err != nil {
				errMsg = err.Error()
				t.Errorf("request %d failed: %v", i, err)
			} else {
				text := extractOutputText(resp)
				if text == "" {
					passed = false
					errMsg = "empty output"
					t.Errorf("request %d returned empty output", i)
				}
			}

			collector.Record(TestResult{
				Name:     "TestBasicMultipleRequests",
				Category: "basic",
				Passed:   passed,
				Duration: duration,
				Error:    errMsg,
			})
		})
	}
}

func extractOutputText(resp *responses.Response) string {
	var parts []string
	for _, item := range resp.Output {
		if item.Type == "message" {
			for _, content := range item.Content {
				if content.Type == "output_text" {
					parts = append(parts, content.Text)
				}
			}
		}
	}
	return strings.Join(parts, "")
}

func containsEvent(events []string, target string) bool {
	for _, e := range events {
		if e == target {
			return true
		}
	}
	return false
}
