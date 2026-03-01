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
- Go 1.22+ + Go standard library only (consistent with constitution) (020-api-compliance)
- N/A (no new persistence, fields echo through existing request/response flow) (020-api-compliance)
- N/A (no new persistence) (021-reasoning-streaming)
- Go 1.22+ for the server binary + Go standard library (`net/http`, `os/exec`, `context`, `encoding/json`, `encoding/base64`, `sync/atomic`) (024-sandbox-server)
- Go 1.22+ + Go standard library only (`log/slog`, `os`, `strings`, `sync`) (026-debug-logging)
- Go 1.22+ for the server binary + Go standard library (`net/http`, `os/exec`, `encoding/json`) (027-sandbox-modes)
- Go 1.22+ (consistent with Specs 001-028) + Go standard library only (`encoding/json`) (029-structured-output)
- N/A (passthrough, no persistence changes) (029-structured-output)
- Astro 5.x, TypeScript/JavaScript, AsciiDoc + Astro, AstroWind template, Tailwind CSS, Antora, @antora/lunr-extension (018-landing-page)
- N/A (static site) (018-landing-page)
- Go 1.22+ + Go standard library for core. `sigs.k8s.io/controller-runtime` + `sigs.k8s.io/agent-sandbox` for SandboxClaim adapter (adapter package only). (025-code-interpreter)
- N/A (no persistence in this feature) (025-code-interpreter)
- Go 1.22+ + Go standard library only for the new provider (consistent with existing providers). No new external dependencies. (030-responses-api-provider)
- N/A (provider does not manage state) (030-responses-api-provider)
- YAML (Kubernetes manifests), Markdown (READMEs), Bash (test commands) + Kustomize, kubectl/oc CLI, existing antwort container images (031-quickstart-updates)
- AsciiDoc (content), YAML (Antora config), JavaScript (Antora build via npx) + Antora 3.x, @antora/lunr-extension, npx (Node.js 18+) (032-documentation-site)
- N/A (static site generator) (032-documentation-site)
- Go 1.25 (server, mock-backend), Python 3.x (SDK tests), TypeScript/Bun (SDK tests + conformance) + `openai` Python/TypeScript SDK, `kind` (K8s in Docker), `oasdiff`, `bun`, `pytest` (033-ci-pipeline)
- N/A (CI pipeline, no persistent storage) (033-ci-pipeline)

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
- 033-ci-pipeline: Added Go 1.25 (server, mock-backend), Python 3.x (SDK tests), TypeScript/Bun (SDK tests + conformance) + `openai` Python/TypeScript SDK, `kind` (K8s in Docker), `oasdiff`, `bun`, `pytest`
- 032-documentation-site: Added AsciiDoc (content), YAML (Antora config), JavaScript (Antora build via npx) + Antora 3.x, @antora/lunr-extension, npx (Node.js 18+)
- 031-quickstart-updates: Added YAML (Kubernetes manifests), Markdown (READMEs), Bash (test commands) + Kustomize, kubectl/oc CLI, existing antwort container images


<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
