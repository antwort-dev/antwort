# Content Model: Landing Page & Documentation Site

**Feature**: 018-landing-page
**Date**: 2026-02-23

This feature has no database entities. The "data model" is a content model describing the structured content on the landing page.

## Landing Page Content Structure

### Hero Section

| Field | Type | Content |
|-------|------|---------|
| Label | Text | "OpenResponses-compliant. Kubernetes-native." |
| Headline | Text | "The server-side agentic framework." |
| Description | Text | Value proposition paragraph |
| Primary CTA | Link | "Get Started" pointing to quickstart docs |
| Secondary CTA | Link | "View on GitHub" pointing to repository |
| Code Snippet | Code block | curl example showing a Responses API request |
| Provider Logos | Image list | vLLM, LiteLLM, OpenAI, Anthropic (grayscale) |

### Value Pillars

Three cards, each with:

| Field | Type | Content |
|-------|------|---------|
| Title | Text | "OpenResponses API" / "Secure by Default" / "Kubernetes Native" |
| Description | Text | 2-3 sentences describing the value |

### Feature Cards

Each card in the feature grid:

| Field | Type | Content |
|-------|------|---------|
| Icon | SVG or emoji | Visual indicator for the feature category |
| Title | Text | Feature name (e.g., "Agentic Loop") |
| Subtitle | Text | Tagline (e.g., "Agents that act, not just answer") |
| Description | Text | 2-3 sentence explanation |
| Status | Enum | "implemented" or "coming_soon" |

**Implemented features** (9): Agentic Loop, Multi-Provider, MCP Tools, Multi-Tenant Auth, Conversation Memory, SSE Streaming, Web Search, Observability, Production Deployment

**Coming Soon features** (6): Sandbox Execution, Agent Profiles, RAG & Knowledge, Proactive Scheduling, Delivery Channels, Tool Registry

### Comparison Table

Matrix structure:

| Dimension | Type | Values |
|-----------|------|--------|
| Row header | Text | Capability name (e.g., "Responses API", "Multi-tenant") |
| Row group | Text | "API & Protocol" / "Agentic Capabilities" / "Security & Operations" |
| Cell value | Enum | full_support / partial_support / no_support / coming_soon / diy |
| Footnote | Text | Clarification text (e.g., LangGraph licensing note) |

Columns: Antwort, LlamaStack, OpenClaw, LangGraph Platform, Manual K8s

### Quickstart Section

| Field | Type | Content |
|-------|------|---------|
| Step N title | Text | Step description |
| Step N code | Code block | Shell command(s) with copy button |
| Step N note | Text | Optional clarification |

### Roadmap Section

| Field | Type | Content |
|-------|------|---------|
| Phase N | Text | Phase name (e.g., "Sandbox Executor") |
| Phase N status | Enum | "in_development" / "planned" |

### Navigation

| Field | Type | Content |
|-------|------|---------|
| Logo | SVG | A! mark + "antwort" wordmark |
| Nav items | Link list | Features (anchor), Docs, GitHub |
| CTA | Link | "Get Started" |

### Footer

| Field | Type | Content |
|-------|------|---------|
| Logo | SVG | A! mark |
| Tagline | Text | "The server-side agentic framework." |
| Links | Link list | Docs, GitHub, Quickstarts, Blog, License |
| Byline | Text | "Built with Go. Runs on Kubernetes." |

### Open Graph Metadata

| Field | Type | Content |
|-------|------|---------|
| og:title | Text | "Antwort" |
| og:description | Text | "The server-side agentic framework." |
| og:image | URL | Path to og-image.png (1200x630) |
| og:url | URL | https://antwort.github.io |
| og:type | Text | "website" |

## Documentation Structure (Antora)

### Component: antwort

| Page | Description | Priority |
|------|-------------|----------|
| index.adoc | Documentation home, overview | P1 |
| getting-started.adoc | Quickstart guide | P1 |
| architecture.adoc | System architecture overview | P2 |
| configuration.adoc | Configuration reference | P2 |
| providers.adoc | Provider setup (vLLM, LiteLLM) | P2 |
| tools.adoc | Tool execution (MCP, built-in, sandbox) | P2 |
| auth.adoc | Authentication setup (JWT, API key) | P2 |
| storage.adoc | Storage configuration (PostgreSQL, memory) | P2 |
| observability.adoc | Metrics and monitoring | P2 |
| deployment.adoc | Kubernetes deployment guide | P2 |
| api-reference.adoc | Responses API reference | P2 |

## Logo Assets

| Asset | Format | Size | Purpose |
|-------|--------|------|---------|
| logo.svg | SVG | Scalable | Primary logo mark (A! in circle) |
| logo-full.svg | SVG | Scalable | Wordmark (A! mark + "antwort") |
| favicon.svg | SVG | Scalable | Browser tab icon |
| favicon.ico | ICO | 16x16, 32x32 | Legacy browser fallback |
| apple-touch-icon.png | PNG | 180x180 | iOS home screen |
| og-image.png | PNG | 1200x630 | Social sharing preview |
