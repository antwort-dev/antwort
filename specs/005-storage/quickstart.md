# Quickstart: State Management & Storage

**Feature**: 005-storage
**Date**: 2026-02-18

## In-Memory Store (Testing/Development)

```go
// Create an in-memory store with max 1000 responses.
store := memory.New(1000)
defer store.Close()

// Create engine with store.
eng, err := engine.New(myProvider, store, engine.Config{
    DefaultModel: "llama-3-8b",
})

// Responses are now automatically saved after inference.
// store: true is the default.
req := &api.CreateResponseRequest{
    Model: "llama-3-8b",
    Input: []api.Item{...},
}
eng.CreateResponse(ctx, req, writer)

// Retrieve the response later.
resp, err := store.GetResponse(ctx, "resp_abc123")

// Delete the response (soft delete).
err = store.DeleteResponse(ctx, "resp_abc123")
```

## PostgreSQL Store (Production)

```go
// Create PostgreSQL store with connection pool.
pgStore, err := postgres.New(ctx, postgres.Config{
    DSN:            "postgres://user:pass@host:5432/antwort?sslmode=require",
    MaxConns:       25,
    MigrateOnStart: true,
})
defer pgStore.Close()

// Use with engine.
eng, err := engine.New(myProvider, pgStore, engine.Config{})

// Health check (for Kubernetes readiness probe).
if err := pgStore.HealthCheck(ctx); err != nil {
    log.Fatal("database not ready")
}
```

## Conversation Chaining

```go
// First request: stored automatically.
req1 := &api.CreateResponseRequest{
    Model: "llama-3-8b",
    Input: []api.Item{...},
}
eng.CreateResponse(ctx, req1, writer1)
// Response stored as resp_001

// Follow-up: references the first response.
req2 := &api.CreateResponseRequest{
    Model:              "llama-3-8b",
    PreviousResponseID: "resp_001",
    Input:              []api.Item{...},
}
eng.CreateResponse(ctx, req2, writer2)
// Engine loads resp_001 from store, reconstructs conversation,
// sends full context to model. New response stored as resp_002.
```

## Skipping Storage

```go
// Explicitly skip storage for stateless requests.
storeFalse := false
req := &api.CreateResponseRequest{
    Model: "llama-3-8b",
    Input: []api.Item{...},
    Store: &storeFalse,
}
// Response is returned to client but NOT saved.
```

## Multi-Tenant Usage

```go
// Auth middleware injects tenant into context.
ctx = storage.SetTenant(ctx, "tenant-abc")

// All storage operations are now scoped to tenant-abc.
store.SaveResponse(ctx, resp)  // saved with tenant_id = "tenant-abc"
store.GetResponse(ctx, id)     // only returns if tenant matches

// Without tenant context (single-tenant mode):
store.GetResponse(context.Background(), id)  // no tenant filtering
```
