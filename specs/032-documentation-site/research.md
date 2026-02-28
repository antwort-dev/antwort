# Research: Documentation Site

## Decision: Antora Module Layout

- **Decision**: Six modules (ROOT, tutorial, reference, extensions, quickstarts, operations) under `docs/modules/`
- **Rationale**: Maps to distinct audiences (newcomers, developers, operators, contributors). Each module gets its own nav.adoc for clean navigation. Standard Antora pattern.
- **Alternatives**: Single ROOT module (too flat at this scale), multiple Antora components (overkill for single repo)

## Decision: Antora Playbook Location

- **Decision**: Place `antora-playbook.yml` in the `docs/` directory, not repo root
- **Rationale**: Keeps doc build config near the content. The `make docs` target runs `npx antora docs/antora-playbook.yml`.
- **Alternatives**: Repo root (clutters root with doc-specific files)

## Decision: Search Extension

- **Decision**: Use `@antora/lunr-extension` for offline search
- **Rationale**: Works without a server, generates search index at build time. Standard for Antora sites.
- **Alternatives**: Algolia DocSearch (requires external service and approval)

## Decision: Quickstart Format Conversion

- **Decision**: Convert quickstart READMEs from Markdown to AsciiDoc. AsciiDoc files in `docs/modules/quickstarts/pages/` become the source of truth.
- **Rationale**: Enables cross-references (xrefs), admonitions, callouts, and consistent rendering in the Antora site. One format for all docs.
- **Alternatives**: Markdown includes (partial support in Antora), wrapper pages (dual maintenance)

## Decision: Configuration Reference Format

- **Decision**: Two complementary pages: annotated YAML examples (configuration.adoc) and a complete reference table (config-reference.adoc)
- **Rationale**: Annotated examples are better for learning, tables are better for lookup. Both audiences served.
- **Alternatives**: Single page with both (too long), auto-generated from Go tags (fragile, loses prose)

## Decision: Voice Profile Application

- **Decision**: Use `kubernetes-patterns` voice profile at `~/.claude/style/voices/kubernetes-patterns.yaml` via prose plugin for all content creation
- **Rationale**: Authoritative O'Reilly-style technical writing with OOP analogies, no humor, "we" pronouns, semantic line breaks. Matches the project's technical depth.
- **Alternatives**: Default prose style (less consistent), custom voice (unnecessary when existing profile fits)

## Interface Inventory (for Extensions Guide)

### Provider Interface (6 methods)
- `Name() string`
- `Capabilities() ProviderCapabilities`
- `Complete(ctx, *ProviderRequest) (*ProviderResponse, error)`
- `Stream(ctx, *ProviderRequest) (<-chan ProviderEvent, error)`
- `ListModels(ctx) ([]ModelInfo, error)`
- `Close() error`

### ResponseStore Interface (8 methods)
- `SaveResponse(ctx, *Response) error`
- `GetResponse(ctx, id) (*Response, error)`
- `GetResponseForChain(ctx, id) (*Response, error)`
- `DeleteResponse(ctx, id) error`
- `ListResponses(ctx, ListOptions) (*ResponseList, error)`
- `GetInputItems(ctx, responseID, ListOptions) (*ItemList, error)`
- `HealthCheck(ctx) error`
- `Close() error`

### Authenticator Interface (1 method)
- `Authenticate(ctx, *http.Request) AuthResult`
- Three-outcome voting: Yes/No/Abstain
- AuthChain composes multiple authenticators

### FunctionProvider Interface (7 methods)
- `Name() string`
- `Tools() []ToolDefinition`
- `CanExecute(name) bool`
- `Execute(ctx, ToolCall) (*ToolResult, error)`
- `Routes() []Route`
- `Collectors() []prometheus.Collector`
- `Close() error`

## Prometheus Metrics Inventory (for Operations Guide)

16 metrics total across gateway, built-in tools, and OpenTelemetry GenAI conventions.
See research agent output for complete list with labels and descriptions.

## Config Fields Inventory (for Reference Manual)

8 top-level sections, 40+ config keys with types, defaults, and env var mappings.
See research agent output for complete struct listing.
