//go:build cluster

package cluster

import (
	"testing"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/responses"
)

func TestToolCallSimple(t *testing.T) {
	client := newAntwortClient()
	ctx := testContext(t)

	start := time.Now()
	resp, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Model: model,
		Input: userInput("What is the weather in San Francisco?"),
		Temperature: openai.Float(0),
		Tools: []responses.ToolUnionParam{
			{
				OfFunction: &responses.FunctionToolParam{
					Name:        "get_weather",
					Description: openai.String("Get the current weather for a location"),
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"location": map[string]any{
								"type":        "string",
								"description": "City name",
							},
						},
						"required": []string{"location"},
					},
				},
			},
		},
	})
	duration := time.Since(start)

	passed := true
	var errMsg string

	if err != nil {
		passed = false
		errMsg = err.Error()
		t.Fatalf("tool call request failed: %v", err)
	}

	foundToolCall := false
	for _, item := range resp.Output {
		if item.Type == "function_call" {
			foundToolCall = true
			if item.Name != "get_weather" {
				t.Errorf("expected function name get_weather, got %s", item.Name)
			}
			if item.Arguments == "" {
				t.Error("expected non-empty arguments")
			}
			t.Logf("Tool call: %s(%s)", item.Name, item.Arguments)
		}
	}

	if !foundToolCall {
		passed = false
		errMsg = "no function_call output item found"
		t.Error("expected a function_call in response output")
	}

	collector.Record(TestResult{
		Name:     "TestToolCallSimple",
		Category: "tools",
		Passed:   passed,
		Duration: duration,
		Error:    errMsg,
	})
}

func TestToolCallNoCall(t *testing.T) {
	client := newAntwortClient()
	ctx := testContext(t)

	start := time.Now()
	resp, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Model: model,
		Input: userInput("What is 2+2? Just give me the number."),
		Temperature: openai.Float(0),
		Tools: []responses.ToolUnionParam{
			{
				OfFunction: &responses.FunctionToolParam{
					Name:        "get_weather",
					Description: openai.String("Get the current weather for a location"),
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"location": map[string]any{
								"type":        "string",
								"description": "City name",
							},
						},
						"required": []string{"location"},
					},
				},
			},
		},
	})
	duration := time.Since(start)

	passed := true
	var errMsg string

	if err != nil {
		passed = false
		errMsg = err.Error()
		t.Fatalf("request failed: %v", err)
	}

	for _, item := range resp.Output {
		if item.Type == "function_call" {
			passed = false
			errMsg = "unexpected function_call: " + item.Name
			t.Errorf("expected no tool calls, got function_call: %s", item.Name)
		}
	}

	text := extractOutputText(resp)
	if text == "" {
		passed = false
		errMsg = "expected text output but got empty"
		t.Error("expected non-empty text response")
	}

	collector.Record(TestResult{
		Name:     "TestToolCallNoCall",
		Category: "tools",
		Passed:   passed,
		Duration: duration,
		Error:    errMsg,
	})
}

func TestToolCallStreaming(t *testing.T) {
	client := newAntwortClient()
	ctx := testContext(t)

	start := time.Now()
	stream := client.Responses.NewStreaming(ctx, responses.ResponseNewParams{
		Model: model,
		Input: userInput("What is the weather in Paris?"),
		Temperature: openai.Float(0),
		Tools: []responses.ToolUnionParam{
			{
				OfFunction: &responses.FunctionToolParam{
					Name:        "get_weather",
					Description: openai.String("Get the current weather for a location"),
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"location": map[string]any{
								"type":        "string",
								"description": "City name",
							},
						},
						"required": []string{"location"},
					},
				},
			},
		},
	})

	var events []string
	for stream.Next() {
		events = append(events, stream.Current().Type)
	}
	duration := time.Since(start)

	passed := true
	var errMsg string

	if err := stream.Err(); err != nil {
		passed = false
		errMsg = err.Error()
		t.Fatalf("streaming tool call failed: %v", err)
	}

	// Check for function call argument events
	hasArgDelta := containsEvent(events, "response.function_call_arguments.delta")
	hasArgDone := containsEvent(events, "response.function_call_arguments.done")

	if !hasArgDelta && !hasArgDone {
		// Model may choose not to call the tool, which is acceptable
		// but log it for visibility
		t.Log("No function_call_arguments events received (model may have responded with text)")
		hasTextDelta := containsEvent(events, "response.output_text.delta")
		if !hasTextDelta {
			passed = false
			errMsg = "no function call or text output events"
			t.Error("expected either function call or text output events")
		}
	} else {
		t.Logf("Streaming tool call events: arg_delta=%v, arg_done=%v", hasArgDelta, hasArgDone)
	}

	collector.Record(TestResult{
		Name:     "TestToolCallStreaming",
		Category: "tools",
		Passed:   passed,
		Duration: duration,
		Error:    errMsg,
	})
}
