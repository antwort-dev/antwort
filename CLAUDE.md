# antwort Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-02-16

## Active Technologies
- Go 1.22+ (required for `http.ServeMux` method+pattern routing) + Go standard library only (`net/http`, `log/slog`, `encoding/json`, `context`, `sync`, `os/signal`) (002-transport-layer)
- N/A (transport layer has no persistence; ResponseStore is an interface for future specs) (002-transport-layer)
- Go 1.22+ (required for `http.ServeMux` method+pattern routing, consistent with Spec 002) + None (Go standard library only: `net/http`, `log/slog`, `encoding/json`, `context`, `sync`, `io`, `bufio`, `bytes`, `fmt`, `strings`, `time`, `net/url`) (003-core-engine)
- N/A (engine uses `transport.ResponseStore` interface; no storage implementation in this spec) (003-core-engine)

- Go 1.22+ + None (Go standard library only: `encoding/json`, `crypto/rand`, `errors`, `fmt`, `strings`, `regexp`) (001-core-protocol)

## Project Structure

```text
src/
tests/
```

## Commands

# Add commands for Go 1.22+

## Code Style

Go 1.22+: Follow standard conventions

## Recent Changes
- 003-core-engine: Added Go 1.22+ (required for `http.ServeMux` method+pattern routing, consistent with Spec 002) + None (Go standard library only: `net/http`, `log/slog`, `encoding/json`, `context`, `sync`, `io`, `bufio`, `bytes`, `fmt`, `strings`, `time`, `net/url`)
- 002-transport-layer: Added Go 1.22+ (required for `http.ServeMux` method+pattern routing) + Go standard library only (`net/http`, `log/slog`, `encoding/json`, `context`, `sync`, `os/signal`)

- 001-core-protocol: Added Go 1.22+ + None (Go standard library only: `encoding/json`, `crypto/rand`, `errors`, `fmt`, `strings`, `regexp`)

<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
