# antwort Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-02-16

## Active Technologies
- Go 1.22+ (required for `http.ServeMux` method+pattern routing) + Go standard library only (`net/http`, `log/slog`, `encoding/json`, `context`, `sync`, `os/signal`) (002-transport-layer)
- N/A (transport layer has no persistence; ResponseStore is an interface for future specs) (002-transport-layer)
- Go 1.22+ (required for `http.ServeMux` method+pattern routing, consistent with Spec 002) + None (Go standard library only: `net/http`, `log/slog`, `encoding/json`, `context`, `sync`, `io`, `bufio`, `bytes`, `fmt`, `strings`, `time`, `net/url`) (003-core-engine)
- N/A (engine uses `transport.ResponseStore` interface; no storage implementation in this spec) (003-core-engine)
- Go 1.22+ (consistent with Specs 001-003) + None (Go standard library only: `context`, `sync`, `log/slog`, `fmt`, `strings`, `time`) (004-agentic-loop)
- N/A (uses existing `transport.ResponseStore` interface for conversation chaining) (004-agentic-loop)
- Go 1.22+ (consistent with Specs 001-004) + `pgx/v5` (PostgreSQL driver, adapter package only). Go standard library for core interface and in-memory store. (005-storage)
- PostgreSQL 14+ for production. In-memory for testing/development. (005-storage)
- Go 1.22+ for server and mock binaries. TypeScript/bun for the official compliance suite (run via container). + Existing antwort packages (api, transport, engine, provider, storage, tools). No new Go dependencies. (006-conformance)
- Go 1.22+ (consistent with Specs 001-006) + Go stdlib for core + API key. `golang.org/x/crypto` for constant-time comparison (optional). JWT validation needs a JWKS library (adapter package only). (007-auth)
- Go 1.22+ (consistent with Specs 001-007) + None new. Shared base uses existing types from pkg/api and pkg/provider. (008-provider-litellm)
- HTML5, CSS3, minimal vanilla JavaScript (progressive enhancement). AsciiDoc for documentation. + Antora (documentation generator), @antora/lunr-extension (search), Google Fonts CDN (Inter, Inter Tight, JetBrains Mono) (018-landing-page)
- N/A (static site, no server-side storage) (018-landing-page)
- Astro 5.x, TypeScript/JavaScript, AsciiDoc + Astro, AstroWind template, Tailwind CSS, Antora, @antora/lunr-extension (018-landing-page)
- N/A (static site) (018-landing-page)

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
- 018-landing-page: Added Astro 5.x, TypeScript/JavaScript, AsciiDoc + Astro, AstroWind template, Tailwind CSS, Antora, @antora/lunr-extension
- 018-landing-page: Added HTML5, CSS3, minimal vanilla JavaScript (progressive enhancement). AsciiDoc for documentation. + Antora (documentation generator), @antora/lunr-extension (search), Google Fonts CDN (Inter, Inter Tight, JetBrains Mono)
- 008-provider-litellm: Added Go 1.22+ (consistent with Specs 001-007) + None new. Shared base uses existing types from pkg/api and pkg/provider.


<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
