# Implementation Plan: Landing Page & Documentation Site

**Branch**: `018-landing-page` | **Date**: 2026-02-23 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/018-landing-page/spec.md`

## Summary

Create a GitHub Pages website at `antwort.github.io` using Astro with the AstroWind template for the landing page and Antora for AsciiDoc documentation. The Astro build produces the landing page at `/`, the Antora build produces docs at `/docs/`, and both are merged in CI for deployment. Documentation sources live in the main `antwort` repo under `docs/`.

## Technical Context

**Language/Version**: Astro 5.x, TypeScript/JavaScript, AsciiDoc
**Primary Dependencies**: Astro, AstroWind template, Tailwind CSS, Antora, @antora/lunr-extension
**Storage**: N/A (static site)
**Testing**: Lighthouse CLI (performance/accessibility), browser testing
**Target Platform**: GitHub Pages (static hosting), modern browsers
**Project Type**: Web (static site, two repos: website repo + doc sources in main repo)
**Performance Goals**: Page load < 3 seconds, Lighthouse 90+
**Constraints**: Astro zero-JS-by-default, dark theme, responsive 375px-1920px
**Scale/Scope**: Single landing page + ~10 AsciiDoc doc pages

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

This feature is a static website outside the Go codebase. Constitution principles (interface-first, zero external dependencies, nil-safe composition, etc.) apply to the Antwort server, not to supporting websites. The only applicable principle is Specification-Driven Development, which this plan follows.

All gates pass. No violations.

## Project Structure

### Documentation (this feature)

```text
specs/018-landing-page/
├── spec.md
├── plan.md              # This file
├── research.md
├── data-model.md
├── quickstart.md
├── checklists/
│   └── requirements.md
├── review-summary.md
└── tasks.md
```

### Source Code (two repositories)

```text
# Website repository: /Users/rhuss/Development/ai/antwort.github.io/
antwort.github.io/
├── astro.config.ts              # Astro configuration
├── tailwind.config.js           # Tailwind CSS configuration (dark theme colors)
├── package.json                 # Dependencies (astro, astrowind, tailwind)
├── tsconfig.json                # TypeScript config
├── src/
│   ├── assets/
│   │   ├── images/
│   │   │   ├── logo.svg         # A! logo mark
│   │   │   ├── logo-full.svg    # A! + "antwort" wordmark
│   │   │   └── og-image.png     # Social sharing preview
│   │   └── favicons/
│   │       ├── favicon.svg
│   │       └── favicon.ico
│   ├── components/
│   │   └── Logo.astro           # Custom logo component (overrides AstroWind default)
│   ├── pages/
│   │   └── index.astro          # Landing page (composes AstroWind widgets)
│   └── navigation.ts            # Navigation configuration
├── antora-playbook.yml          # Antora doc build config
├── supplemental-ui/
│   └── css/
│       └── custom.css           # Antora dark theme overrides
├── .github/
│   └── workflows/
│       └── publish.yml          # CI: Astro build + Antora build + deploy
├── .nojekyll
└── README.md

# Main repository: /Users/rhuss/Development/ai/antwort/ (existing, new files added)
antwort/
├── docs/
│   ├── antora.yml               # Antora component descriptor
│   └── modules/
│       └── ROOT/
│           ├── nav.adoc         # Navigation structure
│           └── pages/
│               ├── index.adoc
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
```

**Structure Decision**: Two-repository approach. The website repo contains the Astro project (landing page), Antora playbook, and CI workflow. Documentation sources live in the main repo under `docs/`. Both build independently; outputs are merged at deploy time.
