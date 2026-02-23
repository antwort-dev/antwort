# Feature Specification: Landing Page & Documentation Site

**Feature Branch**: `018-landing-page`
**Created**: 2026-02-23
**Status**: Draft
**Input**: User description: "Landing page and documentation site for Antwort, based on brainstorm/21-landing-page.md"

## Overview

Antwort needs a public-facing website that serves two purposes: a marketing landing page that communicates the project's value proposition to platform engineers and AI developers, and a reference documentation site for users who adopt the project. The website is hosted on GitHub Pages at `antwort.github.io`.

The landing page positions Antwort as "the server-side agentic framework" with three core differentiators: OpenResponses API compliance, Kubernetes-native security, and production-grade operations. It includes an honest feature comparison with LlamaStack, OpenClaw, and LangGraph Platform.

The documentation is authored in AsciiDoc and managed by Antora, with sources living in the main `antwort` repository alongside the code. The Antora playbook in the website repository aggregates docs from the main repo and builds the documentation site.

## Assumptions

- The website repository is `antwort.github.io` under the same GitHub account as the main `antwort` repo
- No custom domain is needed at this time (GitHub Pages default URL is sufficient)
- The landing page is a single self-contained HTML page with linked CSS and SVG assets
- Documentation sources live in the main `antwort` repo under a `docs/` directory
- The Antora build runs via GitHub Actions and deploys to GitHub Pages
- Fonts (Inter, Inter Tight, JetBrains Mono) are loaded from a CDN (Google Fonts or similar), not self-hosted
- The logo is an SVG that can be generated programmatically (no external design tool required)
- The comparison table reflects the state of competing projects as of February 2026 and will need periodic updates

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Evaluator Discovers Antwort (Priority: P1)

A platform engineer searching for an agentic AI framework finds the Antwort landing page. Within 60 seconds of landing, they understand what Antwort does, how it differs from alternatives, and whether it's worth evaluating further. They can navigate to the quickstart documentation or the GitHub repository from the landing page.

**Why this priority**: First impressions determine whether a potential user invests time in evaluating the project. Without a clear landing page, users bounce to competitors.

**Independent Test**: Can be tested by opening `antwort.github.io` in a browser and verifying the hero section, value pillars, feature grid, and navigation links render correctly and communicate the value proposition.

**Acceptance Scenarios**:

1. **Given** a visitor loads the landing page, **When** the page finishes rendering, **Then** the hero section displays the tagline "The server-side agentic framework", a brief description, and two call-to-action buttons ("Get Started" and "View on GitHub")
2. **Given** a visitor scrolls past the hero, **When** they reach the value pillars section, **Then** three cards are visible: "OpenResponses API", "Secure by Default", and "Kubernetes Native", each with a concise description
3. **Given** a visitor clicks "Get Started", **When** the link activates, **Then** they are navigated to the quickstart documentation page
4. **Given** a visitor clicks "View on GitHub", **When** the link activates, **Then** they are navigated to the Antwort GitHub repository in a new tab
5. **Given** a visitor views the page on a mobile device (viewport width < 768px), **When** the page renders, **Then** all sections stack vertically and remain readable without horizontal scrolling

---

### User Story 2 - Evaluator Compares Alternatives (Priority: P1)

A platform engineer evaluating multiple agentic frameworks scrolls to the comparison table on the landing page. The table provides an honest, accurate comparison of Antwort against LlamaStack, OpenClaw, LangGraph Platform, and manual Kubernetes setups. The comparison helps the evaluator understand where Antwort leads and where competitors have strengths.

**Why this priority**: Decision-makers need honest comparisons to justify technology choices. An inaccurate or unfair table damages credibility.

**Independent Test**: Can be tested by verifying the comparison table renders correctly, contains accurate information for each competitor, and uses clear visual indicators (checkmarks, partial marks, "coming soon" badges) for each capability.

**Acceptance Scenarios**:

1. **Given** a visitor scrolls to the comparison section, **When** the table renders, **Then** it shows five columns (Antwort, LlamaStack, OpenClaw, LangGraph Platform, Manual K8s) and is organized into three groups: "API & Protocol", "Agentic Capabilities", and "Security & Operations"
2. **Given** the comparison table is visible, **When** a visitor reads the LlamaStack column, **Then** it accurately reflects that LlamaStack supports the Responses API (partially), Chat Completions, MCP, 15+ providers, and has a Kubernetes operator, while lacking multi-tenancy and code execution sandboxing
3. **Given** the comparison table is visible, **When** a visitor reads the footnote about LangGraph Platform, **Then** it explains that the library is open source (MIT) but self-hosting the platform requires a commercial license

---

### User Story 3 - Evaluator Explores Features (Priority: P1)

A visitor scrolls through the feature grid to understand the full scope of Antwort's capabilities. Each feature card has a title, brief description, and visual indicator showing whether the feature is implemented or coming soon. The visitor can distinguish between features available today and features on the roadmap.

**Why this priority**: The feature grid is the primary mechanism for communicating what Antwort does. Without it, visitors must read documentation to understand capabilities.

**Independent Test**: Can be tested by verifying the feature grid renders correctly with all cards, each card has a title and description, and "Coming Soon" badges appear on unreleased features.

**Acceptance Scenarios**:

1. **Given** a visitor scrolls to the feature grid, **When** the section renders, **Then** at least nine feature cards for implemented features are visible (Agentic Loop, Multi-Provider, MCP Tools, Multi-Tenant Auth, Conversation Memory, SSE Streaming, Web Search, Observability, Production Deployment)
2. **Given** a visitor scrolls to the feature grid, **When** they view a "Coming Soon" feature card (Sandbox Execution, Agent Profiles, RAG, Scheduling, Delivery, Tool Registry), **Then** the card has a visually distinct "Coming Soon" badge
3. **Given** a visitor views the feature grid on a desktop viewport, **When** the section renders, **Then** cards are arranged in a responsive grid (2-3 columns)

---

### User Story 4 - User Reads Documentation (Priority: P2)

A developer who has decided to try Antwort navigates from the landing page to the documentation site. The documentation is organized into logical sections (Getting Started, Architecture, Features, Deployment, Reference) and is searchable. The documentation has a consistent dark theme that visually connects it to the landing page.

**Why this priority**: Documentation is essential for adoption, but the landing page must exist first to drive traffic. Documentation can start minimal and grow incrementally.

**Independent Test**: Can be tested by navigating to the `/docs/` path, verifying the Antora-generated site renders with navigation, content pages, and search functionality.

**Acceptance Scenarios**:

1. **Given** a visitor clicks "Docs" in the landing page navigation, **When** the link activates, **Then** they are navigated to the Antora documentation site at `/docs/`
2. **Given** a visitor is on the documentation site, **When** they view the sidebar navigation, **Then** sections are visible for Getting Started, Architecture, Features, Deployment, and Reference
3. **Given** a visitor is on the documentation site, **When** they use the search function, **Then** results are returned for relevant queries
4. **Given** a visitor is on the documentation site, **When** they view the page, **Then** the visual theme (dark background, accent colors) is consistent with the landing page

---

### User Story 5 - Quickstart Experience (Priority: P2)

A developer reads the quickstart section on the landing page, copies the deployment command, and has Antwort running on their Kubernetes cluster. The quickstart shows concrete, copy-pasteable commands.

**Why this priority**: Reducing time-to-first-experience is critical for open source adoption. The quickstart must be visible on the landing page, not buried in docs.

**Independent Test**: Can be tested by verifying the quickstart section on the landing page displays at least two concrete commands (deploy and first request) with copy buttons.

**Acceptance Scenarios**:

1. **Given** a visitor scrolls to the quickstart section, **When** the section renders, **Then** at least two code blocks are visible: one for deployment and one for sending a first request
2. **Given** a visitor clicks the copy button on a code block, **When** the click is processed, **Then** the code is copied to the clipboard and the button provides visual feedback (e.g., changes to "Copied")

---

### User Story 6 - Site Deploys Automatically (Priority: P2)

When a developer pushes changes to the website repository (landing page updates) or to the documentation in the main repository (AsciiDoc content), the site rebuilds and deploys automatically via GitHub Actions.

**Why this priority**: Manual deployment creates friction and delays. Automated deployment ensures the site is always current.

**Independent Test**: Can be tested by pushing a change to the website repo and verifying the GitHub Actions workflow completes and the change appears on the live site.

**Acceptance Scenarios**:

1. **Given** a commit is pushed to the `main` branch of the website repository, **When** GitHub Actions detects the push, **Then** the workflow builds the Antora docs, copies the landing page assets, and deploys to GitHub Pages
2. **Given** the Antora build runs, **When** it fetches documentation sources from the main `antwort` repo, **Then** it successfully clones the repo and builds the docs from the `docs/` directory

---

### User Story 7 - Social Sharing (Priority: P3)

When someone shares the Antwort landing page URL on social media, Slack, or other platforms, the link preview shows the project name, tagline, and a branded preview image.

**Why this priority**: Social sharing previews drive click-through from shared links. Low effort, high impact for organic discovery.

**Independent Test**: Can be tested by pasting the URL into a social media platform preview tool and verifying the Open Graph metadata renders correctly.

**Acceptance Scenarios**:

1. **Given** the landing page HTML, **When** a social platform reads the Open Graph meta tags, **Then** it displays the title "Antwort", the description "The server-side agentic framework", and the branded OG image

---

### Edge Cases

- What happens when the Antora build fails (e.g., main repo docs have AsciiDoc syntax errors)? The GitHub Actions workflow should fail visibly and not deploy a broken site.
- What happens when a visitor has JavaScript disabled? The landing page should render all content (features, comparison, quickstart) without JavaScript. Animations and interactive elements (copy buttons) degrade gracefully.
- What happens on very wide viewports (>1920px)? Content should be centered with a maximum width, not stretch to fill the entire screen.
- What happens when the main `antwort` repo is private or inaccessible? The Antora build should fail with a clear error, not deploy stale docs.

## Requirements *(mandatory)*

### Functional Requirements

**Landing Page:**

- **FR-001**: The landing page MUST be a single HTML file with linked CSS and SVG assets, deployable as a static site on GitHub Pages
- **FR-002**: The landing page MUST include a hero section with the tagline, a description, two CTAs ("Get Started" linking to docs, "View on GitHub" linking to the repository), and a code snippet
- **FR-003**: The landing page MUST include three value pillar cards: "OpenResponses API", "Secure by Default", and "Kubernetes Native"
- **FR-004**: The landing page MUST include a feature grid with cards for all implemented features and "Coming Soon" badges for roadmap features
- **FR-005**: The landing page MUST include an architecture diagram showing the request flow through Antwort
- **FR-006**: The landing page MUST include an honest comparison table with Antwort, LlamaStack, OpenClaw, LangGraph Platform, and Manual K8s, organized by API/Protocol, Agentic Capabilities, and Security/Operations
- **FR-007**: The landing page MUST include a quickstart section with copy-pasteable commands for deployment and first request
- **FR-008**: The landing page MUST include a roadmap section showing the six platform phases
- **FR-009**: The landing page MUST include a navigation bar with links to Features (anchor), Docs, GitHub, and a "Get Started" CTA
- **FR-010**: The landing page MUST include Open Graph meta tags for social sharing (title, description, image)
- **FR-011**: The landing page MUST be fully responsive, rendering correctly on viewports from 375px (mobile) to 1920px (desktop)
- **FR-012**: The landing page MUST render all content without JavaScript (progressive enhancement: JS enhances but is not required for content)

**Visual Design:**

- **FR-013**: The landing page MUST use a dark background color scheme with cyan-teal accent colors
- **FR-014**: The landing page MUST use sans-serif fonts for headlines and body text, and a monospace font for code blocks
- **FR-015**: The landing page MUST include the "A!" logo mark as an SVG, used in the navigation bar and as a favicon

**Documentation Site:**

- **FR-016**: The documentation site MUST be generated from AsciiDoc sources in the main `antwort` repository, using Antora as the documentation toolchain (explicit design decision from brainstorming, aligned with project-wide AsciiDoc standard)
- **FR-017**: The documentation site MUST be accessible at the `/docs/` path relative to the landing page
- **FR-018**: The documentation site MUST include a navigation structure with sections for Getting Started, Architecture, Features, Deployment, and Reference
- **FR-019**: The documentation site MUST include search functionality
- **FR-020**: The documentation site MUST use a dark theme that is visually consistent with the landing page

**Infrastructure:**

- **FR-021**: The website MUST be hosted on GitHub Pages at `antwort.github.io`
- **FR-022**: The website MUST deploy automatically via GitHub Actions when changes are pushed to the website repository
- **FR-023**: The Antora build MUST fetch documentation sources from the main `antwort` repository's `main` branch
- **FR-024**: The main `antwort` repository MUST include an Antora component descriptor (`antora.yml`) and AsciiDoc sources under a `docs/` directory

**Logo:**

- **FR-025**: The logo MUST be an SVG containing the text "A!" inside a circle border
- **FR-026**: The logo MUST be readable at sizes from 16x16 (favicon) to 128x128 (marketing)
- **FR-027**: A wordmark variant MUST exist combining the circle-A! mark with the text "antwort"

### Key Entities

- **Landing Page**: A single-page HTML document with CSS and SVG assets that communicates the Antwort value proposition, features, comparison, quickstart, and roadmap
- **Documentation Site**: An Antora-generated multi-page documentation site built from AsciiDoc sources, providing reference documentation for Antwort users
- **Logo Mark**: An SVG graphic ("A!" in a circle) used as the project identity across the landing page, documentation, favicon, and social sharing
- **Feature Card**: A reusable content unit on the landing page consisting of a title, description, and optional "Coming Soon" badge
- **Comparison Entry**: A row in the comparison table with capability name and status indicators for each competing project

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A first-time visitor can identify what Antwort does and how it differs from alternatives within 60 seconds of landing on the page
- **SC-002**: The landing page loads and renders completely in under 3 seconds on a standard broadband connection
- **SC-003**: The landing page scores 90+ on Lighthouse accessibility and performance audits
- **SC-004**: All landing page content is readable and navigable on mobile viewports (375px width) without horizontal scrolling
- **SC-005**: The "Get Started" CTA successfully navigates to the quickstart documentation
- **SC-006**: The GitHub Actions workflow builds and deploys the complete site (landing page + Antora docs) without manual intervention
- **SC-007**: Documentation search returns relevant results for queries matching page titles and content headings
- **SC-008**: Social sharing previews (Open Graph) display the project name, tagline, and branded image on major platforms
