# Quickstart: Core Protocol & Data Model

## Prerequisites

- Go 1.22+
- No external dependencies (stdlib only)

## Package Location

```
pkg/api/
├── types.go           # All data types (Item, Request, Response, etc.)
├── events.go          # StreamEvent types and constructors
├── errors.go          # APIError constructors and helpers
├── state.go           # Response and Item state machine transitions
├── validation.go      # Request and item validation
├── id.go              # ID generation (resp_, item_ prefixed)
├── types_test.go      # JSON round-trip tests
├── events_test.go     # Event serialization tests
├── state_test.go      # State machine transition tests
├── validation_test.go # Validation tests
└── id_test.go         # ID format tests
```

## Quick Verification

After implementation, verify the core protocol works:

```bash
# Run all tests
go test ./pkg/api/...

# Run with verbose output
go test -v ./pkg/api/...

# Run specific test
go test -v -run TestItemRoundTrip ./pkg/api/
```

## Key Usage Patterns

### Creating a Request

```go
req := &api.CreateResponseRequest{
    Model: "meta-llama/Llama-3-8B",
    Input: []api.Item{
        {
            ID:     api.NewItemID(),
            Type:   api.ItemTypeMessage,
            Status: api.ItemStatusCompleted,
            Message: &api.MessageData{
                Role: api.RoleUser,
                Content: []api.ContentPart{
                    {Type: "input_text", Text: "Hello, world!"},
                },
            },
        },
    },
}

if err := api.ValidateRequest(req, api.DefaultValidationConfig()); err != nil {
    // err is *api.APIError with Type, Code, Param, Message
}
```

### Validating State Transitions

```go
// Valid transition
err := api.ValidateResponseTransition(api.ResponseStatusInProgress, api.ResponseStatusCompleted)
// err == nil

// Invalid transition (terminal -> any)
err = api.ValidateResponseTransition(api.ResponseStatusCompleted, api.ResponseStatusInProgress)
// err != nil: "invalid transition from completed to in_progress"
```

### Creating Streaming Events

```go
event := api.StreamEvent{
    Type:           api.EventOutputTextDelta,
    SequenceNumber: 1,
    Delta:          "Hello",
    ItemID:         "item_abc123",
    OutputIndex:    0,
    ContentIndex:   0,
}
```

### Working with ToolChoice

```go
// String form
tc := api.ToolChoiceAuto // "auto"

// Structured form (force a specific function)
tc := api.NewToolChoiceFunction("get_weather")
```

## What This Package Does NOT Do

- **No HTTP handling**: See Spec 02 (Transport Layer)
- **No provider calls**: See Spec 03 (Provider Abstraction)
- **No persistence**: See Spec 05 (Storage)
- **No authentication**: See Spec 06 (Auth)

This package is pure data types, validation, and state machine logic. It has zero I/O and zero external dependencies.
