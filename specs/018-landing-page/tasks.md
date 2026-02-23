# Tasks: Landing Page & Documentation Site

**Input**: Design documents from `/specs/018-landing-page/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, quickstart.md

**Tests**: No test tasks generated. Lighthouse audit in polish phase.

**Organization**: Tasks grouped by user story. Astro + AstroWind replaces hand-crafted HTML/CSS.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to

## Phase 1: Setup (AstroWind Project)

**Purpose**: Scaffold the AstroWind project in the website repo, configure theme and branding

- [ ] T001 Scaffold AstroWind template into /Users/rhuss/Development/ai/antwort.github.io/ using `npx degit onwidget/astrowind`. Preserve existing .git/, .github/, antora-playbook.yml, supplemental-ui/, README.md, .nojekyll
- [ ] T002 Run `npm install` to install Astro and AstroWind dependencies
- [ ] T003 Customize tailwind.config.js: set primary accent to #00e5c0 (cyan-teal), secondary to #22d3a0 (green-teal), configure dark mode as default color scheme
- [ ] T004 [P] Copy existing logo SVGs (logo.svg, logo-full.svg, favicon.svg) from previous assets/img/ into src/assets/images/. If previous files were removed, recreate the A! circle logo SVG and wordmark SVG.
- [ ] T005 [P] Override src/components/Logo.astro to render the A! wordmark SVG inline instead of AstroWind's default logo
- [ ] T006 Update src/config.yaml (or equivalent AstroWind site config) with: site name "Antwort", tagline "The server-side agentic framework", default dark mode, metadata, and social links (GitHub repo URL)
- [ ] T007 Remove AstroWind demo pages not needed: src/pages/homes/, src/pages/landing/, src/pages/about.astro, src/pages/contact.astro, src/pages/pricing.astro, src/pages/services.astro, src/pages/[...blog]/. Keep index.astro and 404.astro.
- [ ] T008 Verify local dev server works: `npm run dev` serves the landing page at localhost:4321 with dark theme and custom colors

**Checkpoint**: AstroWind project scaffolded, dark theme with cyan accents, custom logo, dev server running.

---

## Phase 2: User Story 1 - Evaluator Discovers Antwort (Priority: P1) MVP

**Goal**: Hero section, value pillars, and CTAs render on the landing page.

**Independent Test**: Open localhost:4321 in browser. Hero displays tagline, description, two CTA buttons. Three value pillar cards are visible below.

### Implementation for User Story 1

- [ ] T009 [US1] Replace content of src/pages/index.astro with Hero widget: tagline "OpenResponses-compliant. Kubernetes-native.", title "The server-side agentic framework.", subtitle describing the value proposition, two action buttons ("Get Started" linking to /docs/antwort/0.1/getting-started.html, "View on GitHub" linking to https://github.com/rhuss/antwort target _blank)
- [ ] T010 [US1] Add code snippet to Hero widget content slot in src/pages/index.astro: curl example showing POST /v1/responses with model, tools (code_interpreter), and input. Use AstroWind's code block styling or a `<pre><code>` block.
- [ ] T011 [US1] Add Brands widget to src/pages/index.astro below hero: provider logos for vLLM, LiteLLM, OpenAI, Anthropic. Use text placeholders if SVG logos are unavailable.
- [ ] T012 [US1] Add Features widget (3 items) to src/pages/index.astro for value pillars: "OpenResponses API" (icon: tabler:api), "Secure by Default" (icon: tabler:shield-lock), "Kubernetes Native" (icon: tabler:brand-kubernetes). Each with 2-3 sentence description from brainstorm/21-landing-page.md.

**Checkpoint**: Landing page communicates what Antwort is. Visitor can navigate to docs or GitHub.

---

## Phase 3: User Story 3 - Evaluator Explores Features (Priority: P1)

**Goal**: Feature grid with 15 cards, distinguishing implemented from coming-soon.

**Independent Test**: Scroll to features section. 9 implemented cards and 6 coming-soon cards render correctly.

### Implementation for User Story 3

- [ ] T013 [US3] Add Features widget (9 implemented features) to src/pages/index.astro with id="features": Agentic Loop, Multi-Provider, MCP Tools, Multi-Tenant Auth, Conversation Memory, SSE Streaming, Web Search, Observability, Production Deployment. Each with icon, title, and description.
- [ ] T014 [US3] Add Features widget (6 coming-soon features) to src/pages/index.astro below implemented features: Sandbox Execution, Agent Profiles, RAG & Knowledge, Proactive Scheduling, Delivery Channels, Tool Registry. Each with icon, title, description, and a "Coming Soon" tag/callToAction.

**Checkpoint**: All 15 features visible with clear implemented vs. coming-soon distinction.

---

## Phase 4: User Story 2 - Evaluator Compares Alternatives (Priority: P1)

**Goal**: Honest comparison table with 5 competitors across 3 categories.

**Independent Test**: Scroll to comparison section. Table renders with correct data for all competitors.

### Implementation for User Story 2

- [ ] T015 [US2] Add comparison table section to src/pages/index.astro using a WidgetWrapper with custom HTML table inside. Five columns (Antwort, LlamaStack, OpenClaw, LangGraph Platform, Manual K8s), three row groups (API & Protocol, Agentic Capabilities, Security & Operations). Use Tailwind classes for styling: dark background, responsive horizontal scroll, colored indicators (green checkmark, amber partial, red cross). Include footnote about LangGraph licensing and "Last updated: February 2026" date. Source data from brainstorm/21-landing-page.md.

**Checkpoint**: Comparison table renders honestly and accurately.

---

## Phase 5: User Story 5 - Quickstart Experience (Priority: P2)

**Goal**: Quickstart section with copy-pasteable commands.

**Independent Test**: Scroll to quickstart. Two code blocks visible. Copy functionality works.

### Implementation for User Story 5

- [ ] T016 [US5] Add Steps widget to src/pages/index.astro for quickstart: Step 1 "Deploy" (kubectl apply command), Step 2 "Send a request" (curl POST /v1/responses command), Step 3 "Go agentic" (curl with tools array). Use AstroWind's Steps component with icons (tabler:rocket, tabler:send, tabler:robot).

**Checkpoint**: Quickstart commands are visible and copy-pasteable.

---

## Phase 6: User Story 4 - User Reads Documentation (Priority: P2)

**Goal**: Antora docs at /docs/ with dark theme, navigation, and search.

**Independent Test**: Navigate to /docs/. Antora site renders with sidebar, content, search, dark theme.

### Implementation for User Story 4

- [ ] T017 [US4] Verify Antora component descriptor exists at /Users/rhuss/Development/ai/antwort/docs/antora.yml (already created)
- [ ] T018 [US4] Verify AsciiDoc pages exist under /Users/rhuss/Development/ai/antwort/docs/modules/ROOT/pages/ (already created: 11 pages)
- [ ] T019 [US4] Update antora-playbook.yml output dir to `./dist/docs` so Antora writes directly into Astro's build output
- [ ] T020 [US4] Verify supplemental-ui/css/custom.css dark theme overrides are present and comprehensive
- [ ] T021 [US4] Test local Antora build: `npx antora antora-playbook.yml` produces docs at dist/docs/ with dark theme and search

**Checkpoint**: Antora builds locally and generates dark-themed docs with navigation and search.

---

## Phase 7: User Story 6 - Site Deploys Automatically (Priority: P2)

**Goal**: GitHub Actions workflow builds Astro + Antora and deploys to GitHub Pages.

**Independent Test**: Push to main. Workflow completes. Site is live.

### Implementation for User Story 6

- [ ] T022 [US6] Update .github/workflows/publish.yml: Step 1 checkout, Step 2 setup Node.js 22, Step 3 npm install, Step 4 npm run build (Astro), Step 5 install and run Antora (output to dist/docs/), Step 6 deploy dist/ to GitHub Pages
- [ ] T023 [US6] Add .nojekyll to Astro's public/ directory so it's included in the build output
- [ ] T024 [US6] Configure GitHub Pages in repository settings (manual step): enable Pages, source gh-pages branch
- [ ] T025 [US6] Push to main and verify end-to-end deployment

**Checkpoint**: Automated deployment works.

---

## Phase 8: User Story 7 - Social Sharing (Priority: P3)

**Goal**: Social sharing previews show branded content.

**Independent Test**: OG preview tool shows title, description, and image.

### Implementation for User Story 7

- [ ] T026 [US7] Configure Open Graph metadata in AstroWind's site config or src/pages/index.astro metadata: og:title "Antwort", og:description "The server-side agentic framework", og:image
- [ ] T027 [US7] Create OG image (1200x630 PNG): dark background with A! logo and tagline. Place in src/assets/images/og-image.png

**Checkpoint**: Social sharing previews render correctly.

---

## Phase 9: Polish & Cross-Cutting

**Purpose**: Architecture diagram, roadmap, final quality

- [ ] T028 Add architecture diagram to src/pages/index.astro using Content widget: SVG diagram showing Client -> Gateway -> Engine -> Agentic Loop -> Tool Executors -> Sandbox. Create as inline SVG or src/assets/images/architecture.svg
- [ ] T029 Add roadmap section to src/pages/index.astro using Steps or Timeline widget: 6 phases (Sandbox Executor "In Development", Agent Profiles, Memory & Knowledge, Proactive Scheduling, Delivery Channels, Tool Registry)
- [ ] T030 Add bottom CallToAction widget to src/pages/index.astro: "Ready to deploy agentic AI?" with Get Started and GitHub buttons
- [ ] T031 Update navigation in AstroWind config to include: Features (anchor #features), Docs (/docs/), GitHub (external link), Get Started CTA
- [ ] T032 Run Lighthouse audit, fix issues until scores are 90+
- [ ] T033 Create/update README.md for the website repo

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies. Start immediately.
- **Phases 2-5 (US1, US3, US2, US5)**: Depend on Phase 1. All modify src/pages/index.astro. Execute sequentially.
- **Phase 6 (US4 Docs)**: Depends on Phase 1. Independent of Phases 2-5 (different files/repo). Can run in parallel.
- **Phase 7 (US6 Deploy)**: Depends on Phases 2-6 (needs content to deploy).
- **Phase 8 (US7 Social)**: Depends on Phase 1. Can run in parallel with others.
- **Phase 9 (Polish)**: Depends on all content phases being complete.

### Recommended Execution Order

1. Phase 1 (scaffold AstroWind, configure theme)
2. Phase 2 (US1: hero, pillars) as MVP checkpoint
3. Phase 3 (US3: feature grid)
4. Phase 4 (US2: comparison table)
5. Phase 5 (US5: quickstart)
6. Phase 6 (US4: verify Antora docs, update playbook)
7. Phase 7 (US6: CI/CD workflow)
8. Phase 8 (US7: social sharing)
9. Phase 9 (polish: architecture, roadmap, CTA, Lighthouse)

---

## Notes

- AstroWind provides all layout, spacing, typography, and responsive behavior. No custom CSS needed for standard sections.
- The comparison table is the one section that needs custom HTML (AstroWind has no table widget). Use Tailwind classes within a WidgetWrapper.
- Demo pages (homes/, landing/, blog/, about, contact, pricing, services) should be removed to keep the project clean.
- The Antora docs and AsciiDoc pages in the main repo are already created and verified from the previous implementation attempt.
