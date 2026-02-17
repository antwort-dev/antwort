# Quickstart: Core Engine & Provider Abstraction

**Feature**: 003-core-engine
**Date**: 2026-02-17

## Overview

This guide shows how to wire the core engine, a vLLM provider, and the HTTP transport layer into a working Antwort server that can process OpenResponses API requests against a Chat Completions backend.

## Minimal Server Setup

```go
package main

import (
    "log/slog"
    "os"

    "github.com/rhuss/antwort/pkg/engine"
    "github.com/rhuss/antwort/pkg/provider/vllm"
    "github.com/rhuss/antwort/pkg/transport"
    httpAdapter "github.com/rhuss/antwort/pkg/transport/http"
)

func main() {
    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

    // 1. Create the vLLM provider (Chat Completions adapter)
    provider, err := vllm.New(vllm.Config{
        BaseURL: "http://localhost:8000",  // vLLM server
    })
    if err != nil {
        logger.Error("failed to create provider", "error", err)
        os.Exit(1)
    }
    defer provider.Close()

    // 2. Create the engine (no store = stateless only)
    eng, err := engine.New(provider, nil, engine.Config{})
    if err != nil {
        logger.Error("failed to create engine", "error", err)
        os.Exit(1)
    }

    // 3. Apply middleware
    creator := transport.Chain(
        transport.Recovery(),
        transport.RequestID(),
        transport.Logging(logger),
    )(eng)

    // 4. Create HTTP adapter and start server
    adapter := httpAdapter.NewAdapter(
        creator,
        nil,  // no store = GET/DELETE unavailable
        httpAdapter.Config{Addr: ":8080"},
    )

    server := httpAdapter.NewServer(adapter, httpAdapter.Config{Addr: ":8080"})
    if err := server.ListenAndServe(); err != nil {
        logger.Error("server stopped", "error", err)
    }
}
```

## Testing with Mock Provider

```go
package mytest

import (
    "context"
    "testing"

    "github.com/rhuss/antwort/pkg/api"
    "github.com/rhuss/antwort/pkg/engine"
    "github.com/rhuss/antwort/pkg/provider"
)

// MockProvider implements provider.Provider for testing
type MockProvider struct {
    response *provider.ProviderResponse
}

func (m *MockProvider) Name() string { return "mock" }
func (m *MockProvider) Capabilities() provider.ProviderCapabilities {
    return provider.ProviderCapabilities{Streaming: true, ToolCalling: true}
}
func (m *MockProvider) Complete(ctx context.Context, req *provider.ProviderRequest) (*provider.ProviderResponse, error) {
    return m.response, nil
}
func (m *MockProvider) Stream(ctx context.Context, req *provider.ProviderRequest) (<-chan provider.ProviderEvent, error) {
    ch := make(chan provider.ProviderEvent, 3)
    go func() {
        defer close(ch)
        ch <- provider.ProviderEvent{Type: provider.ProviderEventTextDelta, Delta: "Hello"}
        ch <- provider.ProviderEvent{Type: provider.ProviderEventTextDone}
        ch <- provider.ProviderEvent{
            Type:  provider.ProviderEventDone,
            Usage: &api.Usage{InputTokens: 5, OutputTokens: 1, TotalTokens: 6},
        }
    }()
    return ch, nil
}
func (m *MockProvider) ListModels(ctx context.Context) ([]provider.ModelInfo, error) {
    return []provider.ModelInfo{{ID: "test-model"}}, nil
}
func (m *MockProvider) Close() error { return nil }

func TestEngineStreaming(t *testing.T) {
    mock := &MockProvider{}
    eng, _ := engine.New(mock, nil, engine.Config{})

    // eng implements transport.ResponseCreator
    // Use it with a mock ResponseWriter to verify event sequence
}
```

## Key Wiring Patterns

### Stateless mode (no store)

```go
eng, _ := engine.New(provider, nil, config)
// previous_response_id will return an error
// store: true requests will skip storage silently
```

### With response store (stateful mode)

```go
eng, _ := engine.New(provider, myStore, config)
// previous_response_id chains work
// store: true responses are persisted
```

### Default model

```go
eng, _ := engine.New(provider, nil, engine.Config{
    DefaultModel: "meta-llama/Llama-3.1-8B-Instruct",
})
// Requests without a model field use this default
```

## Dependency Graph

```
HTTP Request
    |
    v
transport.HTTPAdapter (Spec 002)
    |
    v
transport.Middleware chain (recovery -> requestID -> logging)
    |
    v
engine.Engine (implements ResponseCreator)  <-- THIS SPEC
    |
    v
provider.Provider (interface)  <-- THIS SPEC
    |
    v
provider/vllm.VLLMProvider (Chat Completions)  <-- THIS SPEC
    |
    v
vLLM / Ollama / any Chat Completions backend
```
