# Feature Specification: Documentation Site

**Feature Branch**: `032-documentation-site`
**Created**: 2026-02-28
**Status**: Draft
**Input**: Comprehensive AsciiDoc documentation site using Antora with seven modules, organized by audience (platform engineers, application developers, extension developers). Uses the countableSet/antora-ui bundle with dark mode toggle and Inter font.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Tutorial: First Deployment to First Response (Priority: P1)

A developer new to antwort wants to go from zero to a working deployment. They follow the tutorial module, which walks them through deploying antwort on Kubernetes, sending their first request, adding tools, and progressing to a production setup. Each tutorial page is self-contained and builds on the previous one.

**Why this priority**: The tutorial is the front door. Without a clear getting-started experience, developers cannot evaluate the project.

**Independent Test**: A developer with Kubernetes access can follow the tutorial from start to finish, completing each page's exercises, and have a working antwort deployment with tools and monitoring at the end.

**Acceptance Scenarios**:

1. **Given** a developer reads the tutorial, **When** they follow the getting-started page, **Then** they have a running antwort instance and can send a text completion request within 15 minutes.
2. **Given** a developer has completed getting-started, **When** they follow the tools page, **Then** they have MCP tools or code interpreter configured and working.
3. **Given** a developer has completed the tutorial, **When** they follow the production page, **Then** they have PostgreSQL persistence, authentication, and monitoring running.

---

### User Story 2 - Reference Manual: Configuration Lookup (Priority: P1)

A developer or operator needs to look up a specific configuration option to understand its type, default value, environment variable mapping, and effect. The reference module provides both annotated YAML examples (for learning) and a complete reference table (for lookup).

**Why this priority**: Configuration is the most frequently consulted documentation. Operators need precise reference when deploying and tuning.

**Independent Test**: For any configuration key in `config.example.yaml`, the developer can find its documentation in the reference module with type, default, env var, and description.

**Acceptance Scenarios**:

1. **Given** a developer looks up `engine.provider`, **When** they check the configuration page, **Then** they find an annotated example showing valid values (vllm, litellm, vllm-responses) with callout explanations.
2. **Given** an operator needs to check all environment variables, **When** they open the env var reference, **Then** they find a complete table mapping every config key to its `ANTWORT_*` env var.
3. **Given** a developer needs the default for `storage.max_size`, **When** they check the config reference table, **Then** they find the key, type (integer), default (10000), and description.

---

### User Story 3 - Developer Guide: Architecture and Extensions (Priority: P2)

A developer wants to understand antwort's internal architecture or add support for a new LLM backend. The developer module consolidates architecture internals and extension interfaces in a single section. It explains the layered request flow, the Provider interface, required methods (Complete, Stream, ListModels, Capabilities), and how to register custom implementations.

**Why this priority**: Extension points are what make antwort a platform rather than a product. Grouping architecture and extension docs together gives contributors the full picture.

**Independent Test**: A developer can read the developer guide and understand both the internal architecture and the interface contracts well enough to sketch a new provider implementation without reading the source code.

**Acceptance Scenarios**:

1. **Given** a developer reads the architecture page in the developer module, **When** they look for the request flow, **Then** they find a visual diagram showing transport, engine, and provider layers.
2. **Given** a developer reads the provider extension guide, **When** they look for the interface definition, **Then** they find the complete method signatures with parameter and return types explained in prose.
3. **Given** a developer reads the storage extension guide, **When** they look for the ResponseStore interface, **Then** they find the contract, tenant context propagation pattern, and a reference to the PostgreSQL adapter.
4. **Given** a developer reads the auth extension guide, **When** they look for the Authenticator interface, **Then** they find the three-outcome voting model (Yes/No/Abstain) explained with the chain composition pattern.
5. **Given** a developer reads the tools extension guide, **When** they look for the FunctionProvider interface, **Then** they find the registration pattern, route mounting, and Prometheus collector integration.

---

### User Story 4 - Quickstarts in Documentation (Priority: P2)

A developer browsing the documentation site wants to access the quickstart guides without leaving the site. The quickstarts module presents all six quickstarts as AsciiDoc pages, converted from the existing Markdown READMEs, with proper cross-references and Antora navigation.

**Why this priority**: Quickstarts are the most practical documentation. Having them integrated into the doc site improves discoverability and enables cross-referencing.

**Independent Test**: All six quickstarts render correctly in the Antora site with working code blocks, proper formatting, and navigation links between them.

**Acceptance Scenarios**:

1. **Given** a developer visits the quickstarts module, **When** they navigate to any quickstart, **Then** the content matches the original README with proper AsciiDoc formatting (admonitions, callouts, tables).
2. **Given** a developer follows the quickstart progression, **When** they finish one quickstart, **Then** they find a link to the next quickstart in the series.
3. **Given** a developer reads a quickstart, **When** they encounter a configuration concept, **Then** they find an xref link to the relevant reference page.

---

### User Story 5 - Operations Guide: Production Monitoring (Priority: P2)

An operator deploying antwort in production needs guidance on monitoring, troubleshooting, security hardening, and deployment patterns. The operations module covers Prometheus metrics, Grafana dashboards, debug logging categories, health check interpretation, and OpenShift-specific deployment considerations.

**Why this priority**: Operators are a key audience. Production documentation reduces support burden and builds confidence in the project.

**Independent Test**: An operator can follow the monitoring guide to set up Prometheus scraping and Grafana dashboards for an antwort deployment.

**Acceptance Scenarios**:

1. **Given** an operator reads the monitoring page, **When** they look for available metrics, **Then** they find a table of all Prometheus metrics with descriptions and labels.
2. **Given** an operator encounters an issue, **When** they check the troubleshooting page, **Then** they find debug logging categories and instructions to enable verbose output.
3. **Given** an operator reads the deployment page, **When** they look for OpenShift specifics, **Then** they find SCC requirements, Route configuration, and image registry considerations.

---

### User Story 6 - Client SDKs: Using Antwort from Application Code (Priority: P2)

An application developer wants to use antwort from their code using the OpenAI SDK. The client module shows how to configure the SDK to point at an antwort instance, with examples for Python and TypeScript covering text responses, streaming, tool calling, conversation chaining, structured output, and reasoning.

**Why this priority**: Antwort's value proposition is OpenAI SDK compatibility. Without client-side documentation, application developers cannot discover or use this compatibility.

**Independent Test**: A developer with a running antwort instance can follow the Python or TypeScript SDK page and successfully send requests, stream responses, and use tool calling.

**Acceptance Scenarios**:

1. **Given** a developer reads the client overview, **When** they look for supported SDKs, **Then** they find a table listing Python, TypeScript, Go, Java, and .NET with install commands.
2. **Given** a developer reads the Python SDK page, **When** they follow the examples, **Then** they can send a text request, stream a response, and chain conversations using `previous_response_id`.
3. **Given** a developer reads the patterns page, **When** they look for server-side vs client-side tools, **Then** they find clear explanations of both execution models with code examples.
4. **Given** a developer encounters an error, **When** they check the client troubleshooting page, **Then** they find guidance for common issues (connection errors, 401, model not found).

---

### User Story 7 - Documentation Build and Preview (Priority: P1)

A documentation contributor wants to build and preview the docs locally. They run a single command (`make docs`) and get a local preview with search functionality.

**Why this priority**: Without a build system, no documentation can be created or reviewed.

**Independent Test**: Running `make docs` produces a browsable HTML site in the build output directory.

**Acceptance Scenarios**:

1. **Given** a contributor has cloned the repo, **When** they run `make docs`, **Then** the Antora build completes and produces an HTML site.
2. **Given** the docs are built, **When** the contributor opens the output in a browser, **Then** all modules are navigable and search works.
3. **Given** a contributor modifies an AsciiDoc page, **When** they rebuild, **Then** the change is reflected in the output.

---

### Edge Cases

- What happens when the contributor does not have Node.js installed? The Makefile checks for `npx` and prints a clear error message with installation instructions.
- What happens when a quickstart references a config option? Cross-references (xrefs) link to the reference module page.
- What happens when the kubernetes-patterns voice profile is not available? The prose plugin falls back to default style. A note in CONTRIBUTING.adoc explains the voice setup.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The documentation site MUST use Antora as the static site generator with an antora-playbook.yml.
- **FR-002**: The documentation MUST be organized into seven Antora modules: ROOT, tutorial, client, quickstarts, operations, reference, and developer. The modules are ordered by audience: platform engineers (tutorial, quickstarts, operations), application developers (client), then extension developers (developer).
- **FR-003**: Each module MUST have its own nav.adoc defining the navigation structure for that module.
- **FR-004**: The tutorial module MUST include pages for getting-started, first-tools, code-execution, and going-production.
- **FR-005**: The reference module MUST include an annotated configuration page with YAML examples and callouts for every configuration section.
- **FR-006**: The reference module MUST include a configuration reference table listing every config key with its type, default value, environment variable, and description.
- **FR-007**: The developer module (formerly extensions) MUST document the Provider, ResponseStore, Authenticator, and FunctionProvider interfaces with prose explanations of each method.
- **FR-008**: The quickstarts module MUST contain all six quickstarts (01-06) converted from Markdown to AsciiDoc format.
- **FR-009**: The operations module MUST include pages for monitoring, deployment, troubleshooting, and security.
- **FR-010**: All documentation content MUST be written using the `kubernetes-patterns` voice profile via the prose plugin.
- **FR-011**: All AsciiDoc files MUST follow semantic line breaks (one sentence per line).
- **FR-012**: The project MUST include a `make docs` target that builds the Antora site locally.
- **FR-013**: The Antora site MUST include search functionality via the @antora/lunr-extension.
- **FR-014**: The developer module MUST include an architecture page with a visual request flow diagram showing the transport, engine, and provider layers.
- **FR-015**: The reference module MUST include an API reference page documenting the Responses API wire format, SSE event types, and error codes.
- **FR-016**: Cross-references (xrefs) MUST link between modules where concepts overlap (e.g., quickstart config sections linking to reference pages).
- **FR-017**: The reference module MUST include an environment variables page mapping all config keys to their `ANTWORT_*` environment variable equivalents.
- **FR-018**: The operations module MUST list all available Prometheus metrics with their descriptions and label dimensions.
- **FR-019**: The client module MUST include an overview page listing supported OpenAI SDKs, a Python SDK page, a TypeScript SDK page, a common patterns page (streaming, tool calling, conversation chaining, structured output), and a troubleshooting page.
- **FR-020**: The ROOT module MUST contain only the project overview page. Architecture and API reference pages MUST reside in the developer and reference modules respectively.
- **FR-021**: The published documentation site MUST support light and dark mode via the countableSet/antora-ui bundle with a toggle button in the header. Theme preference MUST be stored in localStorage and shared with the main landing page.
- **FR-022**: The documentation site MUST use the Inter font for body text, with a 15% larger font size for the main doc content area relative to the navigation sidebar.

### Key Entities

- **Module**: An Antora documentation module containing related pages, partials, and navigation. Each module maps to a URL path segment in the built site.
- **Page**: A single AsciiDoc file within a module that renders as one documentation page.
- **Voice Profile**: A YAML configuration file defining the writing style, tone, vocabulary, and sentence patterns for consistent documentation.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A developer new to the project can follow the tutorial and have a working antwort deployment within 30 minutes.
- **SC-002**: 100% of configuration keys from config.example.yaml are documented in the reference table.
- **SC-003**: All six quickstarts render in the Antora site with identical content to the original READMEs (verified by visual comparison).
- **SC-004**: The `make docs` command completes in under 60 seconds on a standard development machine.
- **SC-005**: Every extension interface (Provider, ResponseStore, Authenticator, FunctionProvider) has a dedicated documentation page explaining all methods.
- **SC-006**: The documentation site search returns relevant results for common queries like "configuration", "streaming", "auth", "tools", and "deploy".
- **SC-007**: All documentation pages pass a prose plugin voice check against the kubernetes-patterns profile without warnings.

## Assumptions

- Node.js (v18+) and npx are available for the Antora build. This is a documentation build dependency, not a runtime dependency.
- The kubernetes-patterns voice profile exists at `~/.claude/style/voices/kubernetes-patterns.yaml` and is compatible with the prose plugin.
- Quickstart conversion from Markdown to AsciiDoc is a manual process (not auto-converted). The AsciiDoc versions become the source of truth.
- The existing `docs/` directory structure will be restructured to match the new multi-module layout. Existing stub pages will be replaced.

## Dependencies

- Spec 015 (Quickstarts): provides the six quickstart guides to convert
- Spec 012 (Configuration): provides the config system to document
- Spec 031 (Quickstart Updates): provides the 05 and 06 quickstarts
- Prose plugin with kubernetes-patterns voice profile
- Antora and @antora/lunr-extension (installed via npx, no global install required)

## Out of Scope

- Landing page (Spec 018, separate Astro-based site)
- API documentation auto-generation from Go source code
- Hosting setup (GitHub Pages, Netlify, etc.)
- Internationalization or translations
- PDF export of documentation
