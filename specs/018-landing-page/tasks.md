# Tasks: Landing Page & Documentation Site

**Input**: Design documents from `/specs/018-landing-page/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, quickstart.md

**Tests**: No test tasks generated (not requested in spec). Lighthouse audit included in polish phase.

**Organization**: Tasks grouped by user story for independent implementation.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create the website repository structure and shared assets

- [ ] T001 Create website repository at /Users/rhuss/Development/ai/antwort.github.io with git init and initial README.md
- [ ] T002 Create directory structure: assets/css/, assets/img/, assets/js/, supplemental-ui/css/, .github/workflows/
- [ ] T003 [P] Create .nojekyll file at repository root
- [ ] T004 [P] Create SVG logo mark (A! in circle) at assets/img/logo.svg with cyan stroke (#00e5c0) and warm white text (#e0e6ed)
- [ ] T005 [P] Create SVG wordmark (A! mark + "antwort" text) at assets/img/logo-full.svg
- [ ] T006 [P] Create favicon.svg at assets/img/favicon.svg (simplified A! mark for small sizes)
- [ ] T007 [P] Create CSS file at assets/css/landing.css with dark color scheme variables, typography (Inter, Inter Tight, JetBrains Mono via Google Fonts), responsive breakpoints (375px, 768px, 1024px, 1920px), and base element styles

---

## Phase 2: Foundational (Landing Page Shell)

**Purpose**: Create the HTML skeleton with navigation and footer that all content sections depend on

**CRITICAL**: This establishes the page structure that all user story phases populate

- [ ] T008 Create index.html at repository root with HTML5 doctype, meta charset, viewport meta, Google Fonts preconnect and stylesheet links, landing.css link, and empty body structure with section placeholders
- [ ] T009 [P] Add navigation bar to index.html: sticky header with logo-full.svg, nav links (Features anchor, Docs, GitHub), and "Get Started" CTA button
- [ ] T010 [P] Add footer section to index.html: logo mark, tagline "The server-side agentic framework", links (Docs, GitHub, Quickstarts, Blog, Apache 2.0), byline "Built with Go. Runs on Kubernetes."
- [ ] T011 Add responsive navigation styles to assets/css/landing.css: mobile hamburger menu, sticky header with backdrop-blur on scroll, max-width container (1200px centered)

**Checkpoint**: Page shell loads with working navigation and footer. All section placeholders are empty.

---

## Phase 3: User Story 1 - Evaluator Discovers Antwort (Priority: P1) MVP

**Goal**: Visitor understands what Antwort is within 60 seconds of landing. Hero, value pillars, and CTAs are functional.

**Independent Test**: Open index.html in browser, verify hero section renders with tagline, description, CTAs, and code snippet. Value pillars visible below hero. Navigation links work.

### Implementation for User Story 1

- [ ] T012 [US1] Add hero section to index.html: small label ("OpenResponses-compliant. Kubernetes-native."), headline ("The server-side agentic framework."), description paragraph, two CTA buttons ("Get Started" linking to docs/antwort/getting-started.html, "View on GitHub" linking to github.com/rhuss/antwort opening in new tab)
- [ ] T013 [US1] Add code snippet to hero section in index.html: curl example showing POST /v1/responses with model, tools (code_interpreter), and input, styled as a dark code block with syntax highlighting via CSS classes
- [ ] T014 [US1] Add provider logo bar below hero code snippet in index.html: grayscale logos for vLLM, LiteLLM, OpenAI, Anthropic (placeholder SVG icons or text if logos unavailable)
- [ ] T015 [US1] Add three value pillar cards section to index.html: "OpenResponses API" (only open-source implementation, any OpenAI SDK works), "Secure by Default" (gVisor, NetworkPolicy, mTLS, audit logging), "Kubernetes Native" (kubectl apply, HPA, Prometheus, multi-tenant)
- [ ] T016 [US1] Add hero and value pillar styles to assets/css/landing.css: hero background with subtle gradient, headline typography (Inter Tight Bold, large size), CTA button styles (primary cyan, secondary ghost), code block dark background with monospace font, pillar card grid (3-column desktop, stacked mobile), responsive behavior at 768px breakpoint

**Checkpoint**: Landing page has a compelling hero section and three value pillars. A visitor can understand the product and navigate to docs or GitHub.

---

## Phase 4: User Story 2 - Evaluator Compares Alternatives (Priority: P1)

**Goal**: Visitor sees an honest, accurate comparison table and understands where Antwort leads and where competitors have strengths.

**Independent Test**: Scroll to comparison section, verify table renders with 5 columns, 3 row groups, accurate data for each competitor, and footnote about LangGraph licensing.

### Implementation for User Story 2

- [ ] T017 [US2] Add comparison table section to index.html with section heading "How Antwort Compares", five columns (Antwort, LlamaStack, OpenClaw, LangGraph Platform, Manual K8s), three row groups (API & Protocol, Agentic Capabilities, Security & Operations), and cell values using visual indicators (checkmark, partial, cross, coming-soon, DIY)
- [ ] T018 [US2] Add comparison table footnote to index.html: LangGraph Platform licensing note ("Library is MIT; platform requires commercial license for self-hosting"), and "Last updated: February 2026" date
- [ ] T019 [US2] Add comparison table styles to assets/css/landing.css: responsive table layout (horizontal scroll on mobile), row group headers, cell indicator colors (green #22d3a0 for full support, amber #f0c040 for partial/coming-soon, red #e05050 for no support), sticky first column on mobile, alternating row backgrounds

**Checkpoint**: Comparison table renders correctly and provides honest, accurate information.

---

## Phase 5: User Story 3 - Evaluator Explores Features (Priority: P1)

**Goal**: Visitor sees a grid of all features with clear distinction between implemented and coming-soon.

**Independent Test**: Scroll to feature grid, verify 9 implemented feature cards and 6 coming-soon cards render with titles, descriptions, and badges.

### Implementation for User Story 3

- [ ] T020 [US3] Add feature grid section to index.html with section heading "Features", responsive grid of feature cards, each card containing: title (cyan accent), tagline, 2-3 sentence description. Include 9 implemented features: Agentic Loop, Multi-Provider, MCP Tools, Multi-Tenant Auth, Conversation Memory, SSE Streaming, Web Search, Observability, Production Deployment
- [ ] T021 [US3] Add 6 coming-soon feature cards to the feature grid in index.html: Sandbox Execution, Agent Profiles, RAG & Knowledge, Proactive Scheduling, Delivery Channels, Tool Registry. Each with amber "Coming Soon" badge
- [ ] T022 [US3] Add feature grid styles to assets/css/landing.css: card grid layout (3 columns desktop, 2 columns tablet, 1 column mobile), card styling (dark surface background, subtle border, hover effect), "Coming Soon" badge styling (amber background, small text), title accent color (cyan)

**Checkpoint**: Feature grid shows all 15 features with clear visual distinction between implemented and upcoming.

---

## Phase 6: User Story 4 - User Reads Documentation (Priority: P2)

**Goal**: Documentation site is accessible at /docs/ with Antora, dark theme, navigation, and search.

**Independent Test**: Navigate to /docs/ path, verify Antora site renders with sidebar navigation, content pages, search, and dark theme consistent with landing page.

### Implementation for User Story 4

- [ ] T023 [US4] Create Antora component descriptor at /Users/rhuss/Development/ai/antwort/antora.yml with name "antwort", title "Antwort", version "0.1", and nav reference
- [ ] T024 [US4] Create documentation directory structure in main repo: /Users/rhuss/Development/ai/antwort/docs/modules/ROOT/pages/ and /Users/rhuss/Development/ai/antwort/docs/modules/ROOT/nav.adoc with navigation sections (Getting Started, Architecture, Features, Deployment, Reference)
- [ ] T025 [P] [US4] Create initial index.adoc at /Users/rhuss/Development/ai/antwort/docs/modules/ROOT/pages/index.adoc with documentation overview and links to sections
- [ ] T026 [P] [US4] Create getting-started.adoc at /Users/rhuss/Development/ai/antwort/docs/modules/ROOT/pages/getting-started.adoc with quickstart content (deploy command, first request, next steps)
- [ ] T027 [P] [US4] Create placeholder pages for remaining documentation sections: architecture.adoc, configuration.adoc, providers.adoc, tools.adoc, auth.adoc, storage.adoc, observability.adoc, deployment.adoc, api-reference.adoc (each with title and TODO content)
- [ ] T028 [US4] Create Antora playbook at /Users/rhuss/Development/ai/antwort.github.io/antora-playbook.yml referencing main repo, with Lunr search extension
- [ ] T029 [US4] Create Antora dark theme override CSS at /Users/rhuss/Development/ai/antwort.github.io/supplemental-ui/css/custom.css overriding default UI background (#0a0e14), text (#e0e6ed), sidebar, code blocks, search overlay, and link colors to match landing page

**Checkpoint**: Antora builds and generates a dark-themed documentation site with navigation and search.

---

## Phase 7: User Story 5 - Quickstart Experience (Priority: P2)

**Goal**: Landing page has a copy-pasteable quickstart section with concrete commands.

**Independent Test**: Scroll to quickstart section, verify code blocks render with copy buttons, and clicking copy puts commands on clipboard.

### Implementation for User Story 5

- [ ] T030 [US5] Add quickstart section to index.html with section heading "Get Started in Minutes", two code blocks (kubectl deploy command and curl first-request command), and link to full quickstart documentation
- [ ] T031 [US5] Add quickstart styles to assets/css/landing.css: code block dark background, copy button positioning (top-right corner), numbered step labels
- [ ] T032 [US5] Create assets/js/landing.js with copy-to-clipboard functionality for code blocks (progressive enhancement: buttons appear only when JS is available), "Copied!" feedback on click

**Checkpoint**: Quickstart section shows concrete commands with working copy buttons.

---

## Phase 8: User Story 6 - Site Deploys Automatically (Priority: P2)

**Goal**: GitHub Actions workflow builds Antora docs and deploys the combined site on push.

**Independent Test**: Push a change to the website repo, verify the GitHub Actions workflow completes and the site is deployed.

### Implementation for User Story 6

- [ ] T033 [US6] Create GitHub Actions workflow at /Users/rhuss/Development/ai/antwort.github.io/.github/workflows/publish.yml: trigger on push to main, setup Node.js 22, install Antora and Lunr extension, build docs, copy landing page assets to build output, deploy to GitHub Pages via peaceiris/actions-gh-pages
- [ ] T034 [US6] Configure GitHub Pages in repository settings: enable GitHub Pages, set source to gh-pages branch
- [ ] T035 [US6] Verify end-to-end: push to main, confirm workflow runs, verify site is live at antwort.github.io with both landing page and docs

**Checkpoint**: Automated deployment works. Pushing to the website repo rebuilds and deploys the site.

---

## Phase 9: User Story 7 - Social Sharing (Priority: P3)

**Goal**: Sharing the URL on social platforms shows branded preview with title, description, and image.

**Independent Test**: Paste URL in a social media preview tool, verify OG metadata renders correctly.

### Implementation for User Story 7

- [ ] T036 [P] [US7] Create OG image at assets/img/og-image.png (1200x630): dark background with A! logo, "Antwort" text, and tagline "The server-side agentic framework" (can be generated from SVG or created manually)
- [ ] T037 [US7] Add Open Graph and Twitter Card meta tags to index.html head: og:title ("Antwort"), og:description ("The server-side agentic framework"), og:image (path to og-image.png), og:url, og:type ("website"), twitter:card ("summary_large_image")

**Checkpoint**: Social sharing previews display correctly on major platforms.

---

## Phase 10: Polish & Cross-Cutting Concerns

**Purpose**: Final quality, accessibility, and performance improvements

- [ ] T038 Add architecture diagram section to index.html: SVG diagram showing request flow (Client, Gateway, Engine, Agentic Loop, Tool Executors, Sandbox). Create diagram as inline SVG or separate assets/img/architecture.svg
- [ ] T039 Add roadmap section to index.html: six phases (Sandbox Executor "In Development", Agent Profiles, Memory & Knowledge, Proactive Scheduling, Delivery Channels, Tool Registry) with visual timeline or list
- [ ] T040 [P] Add favicon.ico (16x16, 32x32 multi-size ICO) and apple-touch-icon.png (180x180) to assets/img/, add link tags to index.html head
- [ ] T041 [P] Add prefers-reduced-motion media query to assets/css/landing.css disabling all animations and transitions
- [ ] T042 [P] Add scroll-based navigation style change to assets/js/landing.js: transparent nav on top, solid background with backdrop-blur after scrolling past hero
- [ ] T043 Validate HTML with W3C validator, fix any errors
- [ ] T044 Run Lighthouse audit (performance, accessibility, best practices, SEO), fix issues until all scores are 90+
- [ ] T045 Create README.md for the website repository explaining: project purpose, local development (how to preview), how to update landing page content, how to add documentation pages, how the GitHub Actions workflow works

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (T001-T007)
- **US1-US3 (Phases 3-5)**: Depend on Phase 2 (T008-T011). Can run in parallel since they affect different sections of the same HTML file (but sequential is safer for a single file)
- **US4 (Phase 6)**: Depends on Phase 2. Independent of US1-US3 (different repository/files)
- **US5 (Phase 7)**: Depends on Phase 2. Can run in parallel with other stories
- **US6 (Phase 8)**: Depends on US4 (needs Antora playbook) and US1 (needs landing page content)
- **US7 (Phase 9)**: Depends on Phase 2 (needs index.html to exist)
- **Polish (Phase 10)**: Depends on all user stories being complete

### User Story Dependencies

- **US1 (P1)**: Depends on Phase 2. No dependencies on other stories.
- **US2 (P1)**: Depends on Phase 2. No dependencies on other stories.
- **US3 (P1)**: Depends on Phase 2. No dependencies on other stories.
- **US4 (P2)**: Depends on Phase 2. Independent (works on different repo/files).
- **US5 (P2)**: Depends on Phase 2. No dependencies on other stories.
- **US6 (P2)**: Depends on US1 (landing page) + US4 (Antora playbook).
- **US7 (P3)**: Depends on Phase 2. No dependencies on other stories.

### Parallel Opportunities

- T003-T007 (Phase 1 assets) can all run in parallel
- T009-T010 (nav and footer) can run in parallel
- US4 (documentation in main repo) can run in parallel with US1-US3 (landing page content)
- T025-T027 (AsciiDoc pages) can all run in parallel
- T036-T037 (OG assets) can run in parallel with other polish tasks

---

## Parallel Example: Phase 1 Setup

```bash
# All asset creation tasks can run simultaneously:
Task T003: "Create .nojekyll file"
Task T004: "Create SVG logo mark at assets/img/logo.svg"
Task T005: "Create SVG wordmark at assets/img/logo-full.svg"
Task T006: "Create favicon.svg at assets/img/favicon.svg"
Task T007: "Create CSS file at assets/css/landing.css"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (repo, assets, CSS)
2. Complete Phase 2: Foundational (HTML shell, nav, footer)
3. Complete Phase 3: User Story 1 (hero, value pillars, CTAs)
4. **STOP and VALIDATE**: Open in browser, verify first impression
5. Can deploy immediately as a minimal landing page

### Incremental Delivery

1. Setup + Foundational, then US1 (hero) as MVP
2. Add US2 (comparison table) + US3 (feature grid) for complete landing page content
3. Add US4 (docs) + US5 (quickstart) for developer onboarding
4. Add US6 (CI/CD) for automated deployment
5. Add US7 (social sharing) + Polish for final quality

### Recommended Execution Order

Since this is a single-developer project and most tasks modify the same index.html:

1. Phase 1 (setup, all parallel)
2. Phase 2 (HTML shell)
3. Phase 3 (US1: hero) as MVP checkpoint
4. Phase 5 (US3: feature grid, natural next section)
5. Phase 4 (US2: comparison table)
6. Phase 7 (US5: quickstart section)
7. Phase 6 (US4: docs in main repo, then Antora setup)
8. Phase 8 (US6: CI/CD deployment)
9. Phase 9 (US7: social sharing)
10. Phase 10 (polish, architecture diagram, roadmap, Lighthouse)

---

## Notes

- All landing page content goes in a single index.html. Tasks are organized by section/story but modify the same file.
- The documentation tasks (US4) modify the main `antwort` repo, not the website repo. These can run in parallel with landing page work.
- Logo SVGs should be hand-crafted (simple enough for direct SVG markup). No external design tool needed.
- The OG image (T036) is the only raster asset. It can be a PNG generated from the SVG logo or created with a simple image editor.
- The comparison table data is sourced from brainstorm/21-landing-page.md, which contains researched, accurate competitor information as of February 2026.


<!-- SDD-TRAIT:beads -->
## Beads Task Management

This project uses beads (`bd`) for persistent task tracking across sessions:
- Run `/sdd:beads-task-sync` to create bd issues from this file
- `bd ready --json` returns unblocked tasks (dependencies resolved)
- `bd close <id>` marks a task complete (use `-r "reason"` for close reason, NOT `--comment`)
- `bd comments add <id> "text"` adds a detailed comment to an issue
- `bd sync` persists state to git
- `bd create "DISCOVERED: [short title]" --labels discovered` tracks new work
  - Keep titles crisp (under 80 chars); add details via `bd comments add <id> "details"`
- Run `/sdd:beads-task-sync --reverse` to update checkboxes from bd state
- **Always use `jq` to parse bd JSON output, NEVER inline Python one-liners**
