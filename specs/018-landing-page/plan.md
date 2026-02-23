# Implementation Plan: Landing Page & Documentation Site

**Branch**: `018-landing-page` | **Date**: 2026-02-23 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/018-landing-page/spec.md`

## Summary

Create a GitHub Pages website at `antwort.github.io` consisting of a marketing landing page (single HTML/CSS/SVG) and an Antora-generated documentation site. The landing page communicates Antwort's value proposition as "the server-side agentic framework" with OpenResponses compliance as the lead differentiator. Documentation is authored in AsciiDoc in the main repo, aggregated by Antora in the website repo, and deployed via GitHub Actions.

## Technical Context

**Language/Version**: HTML5, CSS3, minimal vanilla JavaScript (progressive enhancement). AsciiDoc for documentation.
**Primary Dependencies**: Antora (documentation generator), @antora/lunr-extension (search), Google Fonts CDN (Inter, Inter Tight, JetBrains Mono)
**Storage**: N/A (static site, no server-side storage)
**Testing**: Lighthouse CLI for performance/accessibility audits, HTML validation, manual browser testing across viewports
**Target Platform**: GitHub Pages (static hosting), modern browsers (Chrome, Firefox, Safari, Edge)
**Project Type**: Web (static site with separate website repo + doc sources in main repo)
**Performance Goals**: Page load under 3 seconds, Lighthouse score 90+
**Constraints**: No build tools for landing page (plain HTML/CSS), Antora for docs only, no JavaScript frameworks
**Scale/Scope**: Single landing page + initial documentation scaffold (~10 AsciiDoc pages)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Applicability | Status |
|-----------|---------------|--------|
| I. Interface-First Design | N/A (website, not Go code) | Pass |
| II. Zero External Dependencies | N/A (website, not core packages) | Pass |
| III. Nil-Safe Composition | N/A | Pass |
| IV. Typed Error Domain | N/A | Pass |
| V. Validate Early, Fail Fast | N/A | Pass |
| VI. Protocol-Agnostic Provider | N/A | Pass |
| VII. Streaming as First-Class | N/A | Pass |
| VIII. Context Carries Cross-Cutting | N/A | Pass |
| IX. Kubernetes-Native Execution | N/A (website hosted on GitHub Pages, not K8s) | Pass |
| Specification-Driven Development | Applicable | Pass (this plan follows from spec) |

No constitution violations. This feature is a static website, outside the Go codebase. Constitution principles apply to the Antwort server, not to supporting assets like websites.

## Project Structure

### Documentation (this feature)

```text
specs/018-landing-page/
├── spec.md              # Feature specification
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output (content model)
├── quickstart.md        # Phase 1 output (implementation quickstart)
├── checklists/
│   └── requirements.md  # Quality checklist
└── tasks.md             # Phase 2 output (task breakdown)
```

### Source Code (two repositories)

```text
# Website repository: antwort.github.io
antwort.github.io/
├── index.html                        # Landing page
├── assets/
│   ├── css/
│   │   └── landing.css               # Landing page styles
│   ├── img/
│   │   ├── logo.svg                  # A! logo mark (circle)
│   │   ├── logo-full.svg             # A! + "antwort" wordmark
│   │   ├── architecture.svg          # Architecture diagram
│   │   ├── favicon.svg               # Favicon (SVG)
│   │   ├── favicon.ico               # Favicon (ICO fallback)
│   │   ├── apple-touch-icon.png      # iOS home screen (180x180)
│   │   └── og-image.png              # Social sharing preview (1200x630)
│   └── js/
│       └── landing.js                # Progressive enhancement (copy buttons, scroll effects)
├── supplemental-ui/
│   └── css/
│       └── custom.css                # Antora dark theme overrides
├── antora-playbook.yml               # Antora configuration
├── .github/
│   └── workflows/
│       └── publish.yml               # Build + deploy workflow
├── .nojekyll                         # Disable Jekyll
└── README.md

# Main repository: antwort (existing, new files added)
antwort/
├── antora.yml                        # Antora component descriptor
└── docs/
    └── modules/
        └── ROOT/
            ├── nav.adoc              # Navigation structure
            └── pages/
                ├── index.adoc        # Documentation home
                ├── getting-started.adoc
                ├── architecture.adoc
                ├── configuration.adoc
                ├── providers.adoc
                ├── tools.adoc
                ├── auth.adoc
                ├── storage.adoc
                ├── observability.adoc
                ├── deployment.adoc
                └── api-reference.adoc
```

**Structure Decision**: Two-repository approach. The website repo (`antwort.github.io`) contains the landing page and Antora playbook. Documentation sources live in the main `antwort` repo under `docs/`, keeping docs close to code. Antora aggregates from the main repo at build time.
