# Research: Landing Page & Documentation Site

**Feature**: 018-landing-page
**Date**: 2026-02-23

## R1: Antora Setup for Multi-Repo Documentation

**Decision**: Use Antora with a playbook in the website repo that pulls AsciiDoc sources from the main `antwort` repo.

**Rationale**: Antora is the standard documentation toolchain for AsciiDoc projects. It supports multi-repository aggregation, versioning, and search (via Lunr extension). The playbook-based approach keeps doc sources close to code while building the final site in a separate repo.

**Alternatives considered**:
- **Hugo with AsciiDoc**: Hugo supports AsciiDoc via asciidoctor, but AsciiDoc support is a secondary concern for Hugo. Cross-referencing, includes, and multi-module navigation are weaker than Antora.
- **MkDocs**: Markdown-only. Not compatible with AsciiDoc project standard.
- **Docusaurus**: React-based, Markdown-focused. Overkill for a documentation site and incompatible with AsciiDoc.

**Key findings**:
- Antora requires Node.js 18+ for the build. GitHub Actions provides this.
- The `@antora/lunr-extension` provides client-side search without a backend.
- Antora's default UI is light-themed. Dark theme requires supplemental CSS overrides (not a full custom UI bundle).
- The playbook can reference the main repo by HTTPS URL. No SSH keys needed if the repo is public.
- Antora component descriptor (`antora.yml`) goes in the root of the content source path.

## R2: GitHub Pages Deployment with Antora

**Decision**: Use `peaceiris/actions-gh-pages` GitHub Action to deploy the combined site (landing page + Antora output).

**Rationale**: This action handles the `gh-pages` branch deployment, CNAME preservation, and `.nojekyll` injection. It's the most widely used GitHub Pages deployment action.

**Alternatives considered**:
- **Official `actions/deploy-pages`**: Newer, uses GitHub Pages artifacts. More complex setup but officially supported. Could switch later.
- **Manual `gh-pages` branch push**: Works but requires manual Git operations in the workflow.

**Key findings**:
- GitHub Pages serves from the `gh-pages` branch or the `/docs` folder on `main`. The `gh-pages` branch approach is cleaner for generated sites.
- The workflow builds Antora output to `build/site/docs/`, copies landing page to `build/site/`, then deploys `build/site/` as the root.
- Cross-repo triggering (rebuilding when main repo docs change) can use `repository_dispatch` events or a scheduled rebuild.

## R3: Dark Theme for Antora

**Decision**: Override the default Antora UI with supplemental CSS that applies the landing page's dark color scheme.

**Rationale**: Building a full custom Antora UI bundle is significant work (clone the UI repo, modify Handlebars templates, rebuild). Supplemental CSS overrides achieve the dark theme with minimal effort while preserving the default UI's structure and functionality.

**Alternatives considered**:
- **Custom UI bundle**: Full control but high maintenance cost. Every Antora UI update requires manual merging.
- **Third-party dark UI**: Some community forks exist but none are actively maintained or well-tested.
- **Accept default light theme**: Would create a jarring visual disconnect between landing page and docs.

**Key findings**:
- Antora's supplemental UI mechanism allows adding or overriding CSS, JavaScript, and partials without forking the UI bundle.
- The supplemental CSS needs to override background colors, text colors, sidebar styles, code block styles, and link colors.
- The Antora UI uses CSS custom properties in some areas, making overrides easier.
- The search results overlay and navigation sidebar need special attention for dark theme readability.

## R4: SVG Logo Generation

**Decision**: Create the "A!" logo as hand-crafted SVG markup. No external design tool required.

**Rationale**: The logo is simple enough (text + circle) to define directly in SVG. This keeps the asset version-controlled, editable by developers, and resolution-independent.

**Alternatives considered**:
- **Figma/Sketch export**: Requires a design tool license and creates a dependency on the designer.
- **Icon font**: Unnecessarily complex for a single logo mark.

**Key findings**:
- SVG `<text>` elements need a `font-family` attribute. Since the SVG may be rendered without the web font loaded, use a fallback stack: `"Inter Tight", "Arial", sans-serif`.
- For the favicon, SVG favicons are supported in all modern browsers. An ICO fallback is needed for older browsers.
- The `apple-touch-icon` (180x180 PNG) can be generated from the SVG using a build step or created manually.
- The OG image (1200x630 PNG) should include the logo, tagline, and dark background. This can be a manually created PNG or generated from the SVG.

## R5: Landing Page Performance

**Decision**: Keep the landing page dependency-free. No JavaScript frameworks, no CSS frameworks, no bundlers. Plain HTML + CSS + minimal vanilla JS.

**Rationale**: A static landing page with no framework dependencies loads faster, has fewer failure modes, and is easier to maintain. The Lighthouse 90+ target is achievable with hand-written HTML/CSS and proper asset optimization.

**Alternatives considered**:
- **Tailwind CSS**: Popular but adds a build step and increases CSS size unless purged.
- **Bootstrap**: Adds unnecessary weight for a single-page design.
- **Astro/11ty**: Static site generators add build complexity without proportional benefit for a single page.

**Key findings**:
- Google Fonts can be loaded asynchronously with `font-display: swap` to avoid blocking render.
- CSS custom properties enable theming without a preprocessor.
- The `prefers-reduced-motion` media query should disable animations for accessibility.
- Inline critical CSS in the `<head>` for above-the-fold content, defer the rest.
- Images (SVGs) are lightweight. The only potentially heavy asset is the OG image (PNG), which is not loaded by the page itself.

## R6: Comparison Table Accuracy

**Decision**: The comparison table reflects the state of projects as of February 2026 and includes a "last updated" date. The brainstorm document (21-landing-page.md) contains the researched data.

**Rationale**: Competitor capabilities change. An outdated comparison damages credibility more than no comparison at all. Dating the table sets expectations.

**Key findings (February 2026)**:
- **LlamaStack v0.5.1**: Implements Responses API (partial, active development), Chat Completions, MCP support, 15+ providers, Kubernetes operator. No multi-tenancy, no code sandbox. Deprecated proprietary Agent APIs in v0.5.0.
- **OpenClaw**: Client-side (WebSocket), Pi Agent framework, Docker sandbox (off by default), 200K GitHub stars. CVE-2026-25253 (CVSS 8.8). Creator joined OpenAI.
- **LangGraph Platform**: Successor to LangServe (deprecated Nov 2024). Own API (not OpenAI-compatible). LangGraph library is MIT; platform requires commercial license for self-hosting.
