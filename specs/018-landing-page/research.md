# Research: Landing Page & Documentation Site

**Feature**: 018-landing-page
**Date**: 2026-02-23

## R1: Astro + AstroWind as Landing Page Framework

**Decision**: Use Astro 5.x with the AstroWind template (onwidget/astrowind) for the landing page.

**Rationale**: AstroWind is the most-starred Astro template (5.1K stars, MIT licensed) with production-ready components: Hero, Features, Steps, Content, FAQs, Stats, CallToAction, Brands, Comparison. It ships with Tailwind CSS, dark mode, responsive design, and zero JavaScript by default. Content is authored as Astro component props, not raw HTML, which means polished design without custom CSS work.

**Alternatives considered**:
- **Hand-crafted HTML/CSS**: Tried first, produced unsatisfactory results. No design system means inconsistent spacing, typography, and layout.
- **Hugo**: Go-based SSG. Weaker landing page template ecosystem. Most themes look dated compared to Astro templates.
- **Pico CSS**: Zero build step, clean defaults. But limited to basic typography and forms. No pre-built hero, feature grid, or comparison components.
- **Docusaurus**: Cannot do AsciiDoc. Not an option.

**Key findings**:
- AstroWind uses `degit` for scaffolding: `npx degit onwidget/astrowind`
- The template includes all needed widget types for the Antwort landing page
- Dark mode is built in, controlled via `data-theme` attribute and `prefers-color-scheme`
- Tailwind config can be customized for the cyan-teal accent palette
- Astro outputs zero JS by default (static HTML), meeting performance goals
- Node.js is already required for Antora, so no new build dependency

## R2: AstroWind Widget Mapping

**Decision**: Map each landing page section to an AstroWind widget component.

| Landing Page Section | AstroWind Widget | Notes |
|---------------------|-----------------|-------|
| Hero | `Hero.astro` | Title, subtitle, CTA buttons, image/code area |
| Value Pillars | `Features.astro` | 3-item grid with icons and descriptions |
| Feature Grid | `Features2.astro` or `Features.astro` | 15 cards, supports badge/tag |
| Comparison Table | Custom or `Content.astro` | May need custom component for table |
| Quickstart | `Steps.astro` | Numbered steps with code blocks |
| Architecture | `Content.astro` | Image/diagram with description |
| Roadmap | `Timeline.astro` or `Steps.astro` | Phased timeline |
| Provider Logos | `Brands.astro` | Logo bar with grayscale images |
| CTA | `CallToAction.astro` | Bottom CTA section |

**Key finding**: AstroWind does not have a built-in comparison table widget. Options:
1. Use the `Content.astro` widget with a custom HTML table inside
2. Create a minimal custom Astro component for the comparison table
3. Use Tailwind's table utilities directly in `index.astro`

Option 3 is simplest. The comparison table HTML with Tailwind classes can be inlined directly in the page.

## R3: Antora Coexistence with Astro

**Decision**: Build Astro and Antora independently in the same CI workflow. Astro output goes to `dist/`, Antora output goes to `dist/docs/`.

**Rationale**: Astro and Antora are completely independent build systems. They don't need to know about each other. The CI workflow runs both sequentially and merges the output.

**Build flow**:
```
1. npm run build          # Astro -> dist/
2. npx antora playbook    # Antora -> dist/docs/
3. Deploy dist/           # GitHub Pages
```

**Key findings**:
- Antora's `output.dir` in the playbook can be set to `./dist/docs` to write directly into Astro's output directory
- The `.nojekyll` file must be in the Astro `public/` directory (copied to `dist/` at build time)
- Astro's `public/` directory is for static files that bypass processing. Antora output should NOT go in `public/` because it's generated at build time.

## R4: Dark Theme Customization

**Decision**: Customize AstroWind's Tailwind config to use the Antwort color palette.

**Approach**: Override the `tailwind.config.js` with custom colors:
- Primary accent: `#00e5c0` (cyan-teal)
- Secondary accent: `#22d3a0` (green-teal)
- Dark background: via AstroWind's built-in dark mode classes

AstroWind already uses Tailwind's `dark:` variant classes throughout all components. Setting the default theme to dark via the site config is sufficient.

## R5: Logo Integration

**Decision**: Use the existing A! SVG logos, integrated as an Astro component.

**Approach**: Override AstroWind's `Logo.astro` component with a custom version that renders the A! wordmark SVG. The SVG source is inlined in the component for optimal loading (no external request).

## R6: Antora Dark Theme

**Decision**: Use supplemental CSS overrides to apply the dark theme to the default Antora UI bundle. Same approach as the previous implementation.

**Rationale**: Building a custom Antora UI bundle is high maintenance. Supplemental CSS overrides are sufficient for color changes and are forward-compatible with Antora UI updates.
