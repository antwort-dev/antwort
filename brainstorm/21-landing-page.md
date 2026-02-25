# Brainstorm: Antwort Landing Page & Documentation Site

**Created**: 2026-02-23
**Status**: Brainstorm (pre-spec)

## Goal

Create a GitHub Pages site at `antwort.github.io` that serves two purposes:
1. **Landing page**: Marketing-focused single page that sells the vision and convinces platform engineers and AI developers to try Antwort
2. **Reference documentation**: AsciiDoc-based docs managed by Antora, aggregated from the main `antwort` repo

## Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Site URL | `antwort.github.io` | GitHub Pages, no custom domain for now |
| Doc format | AsciiDoc | Project standard, semantic line breaks, rich cross-referencing |
| Doc tooling | Antora | Standard for AsciiDoc multi-component docs, versioning, search |
| Doc source location | Main `antwort` repo | Docs stay close to code, Antora playbook in website repo aggregates |
| Landing page | Astro + AstroWind template | Production-ready design defaults (hero, features, comparison, CTA). Dark mode, responsive, Tailwind CSS. Content as component props, not raw HTML. Node.js already required for Antora. |
| Theme | Dark background + cyan/teal accents | Developer-oriented, security connotation, modern feel |
| Roadmap tone | Vision-forward, prominent | Show full platform vision with "Coming Soon" labels on unreleased features |
| Competitor comparison | Yes, direct table | Antwort vs OpenClaw vs LlamaStack vs LangServe vs manual K8s. OpenResponses compliance as lead differentiator. |
| Logo | "A!" mark in circle | Emphasizes "answer" + action. Security seal connotation. SVG for scalability. |
| Typography | Inter Tight (headlines) + Inter (body) + JetBrains Mono (code) | Sans-serif, modern, proven readability on dark backgrounds |

## Repository Structure

```
antwort.github.io/                    # Website repo (GitHub Pages)
├── index.html                        # Landing page (self-contained)
├── assets/
│   ├── css/
│   │   └── landing.css               # Landing page styles
│   ├── img/
│   │   ├── logo.svg                  # A! logo mark
│   │   ├── logo-full.svg             # A! + "antwort" wordmark
│   │   ├── architecture.svg          # Architecture diagram
│   │   └── og-image.png              # Social sharing preview
│   └── fonts/                        # Self-hosted fonts (optional, can use CDN)
├── antora-playbook.yml               # Antora config, pulls docs from main repo
├── supplemental-ui/                  # Antora UI overrides (dark theme CSS)
│   └── css/
│       └── custom.css
├── .github/
│   └── workflows/
│       └── publish.yml               # Build Antora + deploy to GitHub Pages
├── .nojekyll                         # Disable Jekyll processing
└── README.md

antwort/                              # Main repo (existing)
├── docs/                             # AsciiDoc sources (NEW)
│   └── modules/
│       └── ROOT/
│           ├── nav.adoc              # Navigation structure
│           └── pages/
│               ├── index.adoc        # Docs landing
│               ├── getting-started.adoc
│               ├── architecture.adoc
│               ├── configuration.adoc
│               ├── providers.adoc
│               ├── tools.adoc
│               ├── auth.adoc
│               ├── storage.adoc
│               ├── observability.adoc
│               ├── deployment.adoc
│               └── api-reference.adoc
├── antora.yml                        # Antora component descriptor (NEW)
└── ...existing code...
```

## Color Palette

```
Background:          #0a0e14  (deep blue-black)
Surface/Cards:       #111a27  (slightly elevated)
Card borders:        #1a2a3a  (subtle separation)
Card hover:          #152030  (interactive feedback)

Primary accent:      #00e5c0  (cyan-teal, action/highlight color)
Primary glow:        #00e5c020 (subtle glow behind accented elements)
Secondary accent:    #22d3a0  (green-teal, success/available)
Warning accent:      #f0c040  (warm amber, "coming soon" badges)

Text primary:        #e0e6ed  (warm white, not pure white)
Text secondary:      #7a8a9a  (muted blue-gray)
Text muted:          #4a5568  (deep muted, used sparingly)

Code background:     #0d1520  (slightly darker than surface)
Code text:           #00e5c0  (cyan, terminal feel)
Code comments:       #4a5568  (muted)

Link color:          #00e5c0  (same as primary accent)
Link hover:          #22d3a0  (shifts to secondary)

Comparison "yes":    #22d3a0  (green-teal)
Comparison "no":     #e05050  (muted red)
Comparison "diy":    #f0c040  (amber)
```

## Logo Design: "A!" Mark

### Concept

The logo combines the capital "A" from "Antwort" (German for "answer") with an exclamation mark "!" to convey:
- An emphatic answer (the agent acts, not just responds)
- Action and decisiveness
- The circle border suggests security, trust, and completeness

### SVG Specification

```
Shape: Circle border (stroke, not fill) with "A!" text centered inside
Circle: 2px stroke, #00e5c0 color, radius proportional to text
Text: "A!" in Inter Tight Bold or similar geometric sans-serif
Text color: #e0e6ed (warm white)
Sizing: Works at 16x16 (favicon), 32x32 (nav), 64x64 (hero), 128x128 (marketing)

Wordmark variant: Circle-A! mark + "antwort" text to the right
Wordmark font: Inter Tight, weight 600, letter-spacing -0.02em
Wordmark color: #e0e6ed
```

### Favicon

The "A!" mark scaled to 32x32 and 16x16, with the circle simplified at small sizes. Also provide as .ico and apple-touch-icon (180x180).

## Landing Page Structure

### Section 1: Navigation Bar

```
[A! antwort]   Features   Docs   GitHub ★   [Get Started →]
```
- Fixed/sticky, transparent over hero, solid on scroll
- Subtle backdrop blur when scrolled
- "Get Started" is the primary CTA (links to quickstart docs)
- GitHub link shows star count (fetched via API or static)

### Section 2: Hero

```
┌─────────────────────────────────────────────────────────┐
│                                                         │
│      [small label: OpenResponses-compliant · Kubernetes-native]  │
│                                                         │
│      The server-side                                    │
│      agentic framework.                                 │
│                                                         │
│      The open-source OpenResponses API implementation   │
│      for production. Any OpenAI SDK works. Sandboxed    │
│      code execution. Multi-tenant. Kubernetes-native.   │
│                                                         │
│      [Get Started →]    [View on GitHub]                │
│                                                         │
│      ┌─────────────────────────────────────────┐        │
│      │ $ curl -X POST .../v1/responses \       │        │
│      │   -d '{"model":"llama-4",               │        │
│      │        "tools":[{"type":"code_inter...   │        │
│      └─────────────────────────────────────────┘        │
│                                                         │
│      Works with: [vLLM] [LiteLLM] [OpenAI] [Anthropic] │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

- Subtle animated background: slow-moving grid dots or constellation pattern
- Hero headline in large Inter Tight Bold
- Code snippet with syntax highlighting and copy button
- Provider logos in muted grayscale, colored on hover

### Section 3: Three Value Pillars

Three cards side by side (responsive: stacked on mobile):

**OpenResponses API**
The only open-source server-side implementation of the OpenResponses standard. Any OpenAI SDK (Python, Node, Go, Rust) works without modification. No vendor lock-in. No proprietary API to learn. Your existing client code just works.

**Secure by Default**
Every code execution runs in a gVisor-isolated Kubernetes Pod. Network deny-all. mTLS with SPIFFE/SPIRE. Audit logging for every tool call. Security is the architecture, not an option you enable.

**Kubernetes Native**
Deploy with `kubectl apply`. Scale with HPA. Monitor with Prometheus. Multi-tenant from day one. Antwort is designed exclusively for Kubernetes, with no standalone mode.

### Section 4: Feature Grid

Responsive grid of feature cards. Each card has:
- Icon (simple SVG or emoji as placeholder)
- Title (bold, cyan accent)
- 2-3 sentence description
- Optional: small code snippet or key metric
- "Coming Soon" badge (amber) for unreleased features

**Implemented features:**
1. Agentic Loop - "Agents that act, not just answer"
2. Multi-Provider - "Bring your own model"
3. MCP Tools - "Connect to any tool server"
4. Multi-Tenant Auth - "Enterprise-ready from day one"
5. Conversation Memory - "Agents that remember"
6. SSE Streaming - "Real-time token streaming"
7. Built-in Web Search - "Current information retrieval"
8. Observability - "Know what your agents are doing"
9. Production Deployment - "Kustomize overlays for dev, prod, OpenShift"

**Vision features (Coming Soon, prominent):**
10. Sandbox Execution - "Code runs in Kubernetes, not on your laptop"
11. Agent Profiles - "Server-side SOUL.md"
12. RAG & Knowledge - "Give your agent a knowledge base"
13. Proactive Scheduling - "Agents that work while you sleep"
14. Delivery Channels - "Results where you need them"
15. Tool Registry - "Curated, security-audited tools"

### Section 5: Architecture Diagram

Clean SVG diagram showing request flow through the system:

```
Client (any OpenAI SDK)
       │
       ▼
┌──────────────────────────────────────┐
│  Antwort Gateway                     │
│  ┌─────┐ ┌──────┐ ┌───────────────┐ │
│  │Auth │ │Router│ │Rate Limiter   │ │
│  └──┬──┘ └──┬───┘ └───────────────┘ │
│     ▼       ▼                        │
│  ┌──────────────────────┐            │
│  │  Engine              │            │
│  │  ┌────────────────┐  │            │
│  │  │ Agentic Loop   │  │            │
│  │  │ ┌───┐  ┌─────┐ │  │            │
│  │  │ │LLM│◄►│Tools│ │  │            │
│  │  │ └───┘  └──┬──┘ │  │            │
│  │  └───────────┼────┘  │            │
│  └──────────────┼───────┘            │
│                 │                    │
│    ┌────────────┼────────────┐       │
│    │   Tool Executors        │       │
│    │  ┌───┐ ┌─────┐ ┌─────┐ │       │
│    │  │MCP│ │Built│ │Sand-│ │       │
│    │  │   │ │-in  │ │box  │ │       │
│    │  └───┘ └─────┘ └──┬──┘ │       │
│    └────────────────────┼────┘       │
└─────────────────────────┼────────────┘
                          │
              ┌───────────▼──────────┐
              │ Kubernetes Sandbox   │
              │ (gVisor, NetworkPol, │
              │  mTLS, audit)        │
              └──────────────────────┘
```

### Section 6: Comparison Table

Direct comparison with OpenResponses compliance as the lead differentiator. The table must be
**honest and accurate**. Key findings from research (Feb 2026):

- **LlamaStack (v0.5.1)** has shifted significantly toward OpenAI compatibility. It now implements
  the Responses API (at `/v1/responses`), Chat Completions, and deprecated its proprietary Agent
  APIs in v0.5.0. It also supports MCP. However, the Responses API implementation is still
  incomplete ("active development, unimplemented parts"). LlamaStack supports 15+ inference
  providers (not Llama-only). It has a Kubernetes operator but no built-in sandbox or multi-tenancy.
- **LangServe** is effectively deprecated (maintenance-only since Nov 2024, last release Oct 2024).
  LangChain's successor is **LangGraph Platform**, which is commercially licensed. Neither exposes
  OpenAI-compatible APIs natively. Community bridges exist but are unofficial.
- **OpenClaw** remains client-side with its own WebSocket protocol.

The table is organized with API/protocol concerns first (the main focus), then operational concerns.
Replace LangServe with LangGraph Platform since LangServe is deprecated:

```
                             Antwort        LlamaStack     OpenClaw       LangGraph Plat.  Manual K8s
──────────────────────────────────────────────────────────────────────────────────────────────────────
API & Protocol
  Responses API              ✅ Full        🔧 Partial     ❌ Custom      ❌ Own API       🔧 DIY
  Chat Completions API       ✅ Via provider✅ Yes          ❌ No          ❌ Own API       🔧 DIY
  OpenAI SDK compatible      ✅ Yes         ✅ Yes          ❌ No          ❌ No            🔧 DIY
  Conformance tested         ✅ Official    ❌ No           ❌ No          ❌ No            ❌ No
  Streaming (SSE)            ✅ Full events ✅ SSE          ✅ WebSocket   ✅ SSE           🔧 DIY
  Stateless + stateful       ✅ Per-request ❌ Stateful     ❌ Stateful    ❌ Stateful      🔧 DIY

Agentic Capabilities
  Agentic loop               ✅ Built-in    ✅ Built-in    ✅ Pi Agent    ✅ Built-in      ❌ No
  MCP tools                  ✅ Built-in    ✅ Yes          🔧 Bridge      ❌ No            🔧 DIY
  Code execution sandbox     🔜 K8s Pods    ❌ No sandbox  ❌ Off/Docker  ❌ No            🔧 DIY
  Built-in web search        ✅ SearXNG     ❌ No           ✅ Browser     ❌ No            🔧 DIY
  RAG / file_search          🔜 Coming      ✅ Built-in    ❌ No          🔧 Via LangChain 🔧 DIY
  Safety / guardrails        ❌ No          ✅ Llama Guard ❌ No          ❌ No            🔧 DIY

Security & Operations
  Sandbox by default         ✅ Always      ❌ None        ❌ Off         ❌ None          🔧 DIY
  Multi-tenant               ✅ Built-in    ❌ No          ❌ No          ❌ No            🔧 DIY
  Kubernetes native          ✅ Exclusive   ✅ Operator    ❌ Docker      ❌ No            ✅ Yes
  Observability              ✅ Prometheus  🔧 Basic       ❌ None        ✅ LangSmith     🔧 DIY
  Provider agnostic          ✅ Any LLM     ✅ 15+ providers ✅ Any LLM  ❌ LangChain     🔧 DIY
  Open source                ✅ Apache 2.0  ✅ Open source ✅ Open source ❌ Commercial*   N/A

* LangGraph library is MIT; LangGraph Platform requires commercial license for self-hosting.
```

Key messaging for the comparison (honest, not adversarial):

- **Antwort focuses on being the most complete OpenResponses implementation.** It passes the official
  conformance test suite. LlamaStack also implements the Responses API but is still catching up
  (partial implementation, no conformance testing).
- **LlamaStack is a strong project with broader scope.** It includes safety (Llama Guard), eval,
  fine-tuning, and built-in RAG that Antwort doesn't have yet. Its multi-provider support has
  grown well beyond Llama-only models.
- **Antwort's differentiators are security and Kubernetes-native operations.** Sandboxed code
  execution in gVisor-isolated Pods, multi-tenant isolation, and production-grade observability
  are capabilities neither LlamaStack nor OpenClaw provide.
- **OpenClaw solves a different problem.** It's a personal AI assistant (client-side), not a
  server-side platform. Comparing directly is like comparing a desktop app to a cloud service.
- **LangGraph Platform is capable but commercially licensed.** The library is open source, but
  self-hosting the full platform requires an enterprise license.

### Section 7: Quickstart

```
# 1. Deploy
kubectl apply -k github.com/rhuss/antwort/deploy/kustomize/base

# 2. Your first agent
curl -X POST http://localhost:8080/v1/responses \
  -H "Content-Type: application/json" \
  -d '{"model":"qwen2.5",
       "tools":[{"type":"web_search"}],
       "input":[{"role":"user",
                 "content":"What are today's top AI news?"}]}'
```

Three steps with copy buttons. Links to detailed quickstart docs.

### Section 8: Roadmap / Platform Vision

Brief section showing the phased build plan:

```
Phase 1: Sandbox Executor          ← In Development
Phase 2: Agent Profiles
Phase 3: Memory & Knowledge (RAG)
Phase 4: Proactive Scheduling
Phase 5: Delivery Channels
Phase 6: Tool Registry
```

Link to the vision document or blog post for full details.

### Section 9: Footer

```
A! antwort

The server-side agentic framework.

Docs    GitHub    Quickstarts    Blog    Apache 2.0

Built with Go. Runs on Kubernetes.
```

## Antora Documentation Structure

### Component Descriptor (antora.yml in main repo)

```yaml
name: antwort
title: Antwort
version: '0.1'
start_page: ROOT:index.adoc
nav:
  - modules/ROOT/nav.adoc
```

### Navigation (nav.adoc)

```asciidoc
* xref:index.adoc[Overview]
* Getting Started
** xref:getting-started.adoc[Quickstart]
** xref:installation.adoc[Installation]
** xref:configuration.adoc[Configuration]
* Architecture
** xref:architecture.adoc[Overview]
** xref:responses-api.adoc[Responses API]
** xref:agentic-loop.adoc[Agentic Loop]
** xref:providers.adoc[Providers]
* Features
** xref:tools.adoc[Tool Execution]
** xref:mcp.adoc[MCP Integration]
** xref:web-search.adoc[Web Search]
** xref:auth.adoc[Authentication]
** xref:storage.adoc[Storage]
** xref:observability.adoc[Observability]
* Deployment
** xref:kubernetes.adoc[Kubernetes]
** xref:openshift.adoc[OpenShift]
* Reference
** xref:api-reference.adoc[API Reference]
** xref:configuration-reference.adoc[Configuration Reference]
** xref:environment-variables.adoc[Environment Variables]
```

### Antora Playbook (in website repo)

```yaml
site:
  title: Antwort
  url: https://antwort.github.io
  start_page: antwort::index.adoc

content:
  sources:
    - url: https://github.com/rhuss/antwort.git
      branches: main
      start_path: docs

ui:
  bundle:
    url: https://gitlab.com/antora/antora-ui-default/-/jobs/artifacts/HEAD/raw/build/ui-bundle.zip?job=bundle-stable
    snapshot: true
  supplemental_files: ./supplemental-ui

output:
  dir: ./build/site
  destinations:
    - provider: fs
      path: ./docs
```

### Antora Dark Theme Override

The supplemental UI CSS overrides the default Antora theme to match the landing page color scheme. This keeps a consistent visual identity between the landing page and docs without building a custom UI bundle.

## GitHub Actions Workflow

```yaml
name: Publish Site

on:
  push:
    branches: [main]
  # Also trigger when main antwort repo docs change
  repository_dispatch:
    types: [docs-updated]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '22'

      - name: Install Antora
        run: npm i antora @antora/lunr-extension

      - name: Build Antora docs
        run: npx antora antora-playbook.yml

      - name: Copy landing page
        run: |
          cp index.html build/site/
          cp -r assets build/site/
          cp .nojekyll build/site/

      - name: Deploy to GitHub Pages
        uses: peaceiris/actions-gh-pages@v4
        with:
          github_token: ${{ secrets.GITHUB_TOKEN }}
          publish_dir: ./build/site
```

## Open Questions (Resolved)

All key questions have been resolved during brainstorming:

1. ~~Doc source location~~ → Main antwort repo
2. ~~Domain~~ → antwort.github.io
3. ~~Roadmap prominence~~ → Vision-forward, prominent
4. ~~Comparison table~~ → Yes, direct comparison
5. ~~Technology~~ → Antora + single HTML landing page
6. ~~Logo~~ → A! in circle, SVG

## Next Steps

1. Create the `antwort.github.io` repo structure
2. Generate the SVG logo (A! in circle)
3. Build the landing page (index.html + CSS)
4. Set up Antora playbook and initial doc structure in main repo
5. Configure GitHub Actions for automated deployment
6. Write initial AsciiDoc documentation pages
