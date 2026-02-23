# Implementation Quickstart: Landing Page & Documentation Site

**Feature**: 018-landing-page
**Date**: 2026-02-23

## Prerequisites

- GitHub account with permission to create the `antwort.github.io` repository
- Node.js 18+ (for Antora build, local preview only)
- A modern browser for testing

## Step 1: Create Website Repository

```bash
mkdir -p /Users/rhuss/Development/ai/antwort.github.io
cd /Users/rhuss/Development/ai/antwort.github.io
git init
```

## Step 2: Create Landing Page

Create `index.html` with the full landing page content. Link to `assets/css/landing.css` for styles and `assets/js/landing.js` for progressive enhancement (copy buttons, scroll effects).

Key sections in order: Nav, Hero, Value Pillars, Feature Grid, Architecture, Comparison, Quickstart, Roadmap, Footer.

## Step 3: Create Logo SVG

Create `assets/img/logo.svg` with the "A!" mark inside a circle. Use SVG `<text>` and `<circle>` elements. Create `logo-full.svg` with the wordmark variant.

## Step 4: Set Up Antora in Main Repo

In the main `antwort` repo, create:

```bash
# Component descriptor at repo root
cat > antora.yml << 'EOF'
name: antwort
title: Antwort
version: '0.1'
start_page: ROOT:index.adoc
nav:
  - modules/ROOT/nav.adoc
EOF

# Documentation directory structure
mkdir -p docs/modules/ROOT/pages
```

Create initial AsciiDoc pages under `docs/modules/ROOT/pages/`.

## Step 5: Configure Antora Playbook

In the website repo, create `antora-playbook.yml` pointing to the main repo's docs.

## Step 6: Add Dark Theme Overrides

Create `supplemental-ui/css/custom.css` overriding Antora default styles with the dark color scheme.

## Step 7: Set Up GitHub Actions

Create `.github/workflows/publish.yml` that:
1. Checks out the website repo
2. Installs Antora + Lunr extension
3. Runs `npx antora antora-playbook.yml`
4. Copies landing page assets to build output
5. Deploys to GitHub Pages

## Step 8: Enable GitHub Pages

In the repository settings, configure GitHub Pages to deploy from the `gh-pages` branch.

## Step 9: Verify

- Landing page renders at `https://antwort.github.io`
- Docs render at `https://antwort.github.io/docs/`
- All navigation links work
- Mobile responsive layout verified
- Lighthouse audit scores 90+

## Key Files Checklist

```
antwort.github.io/
├── index.html                    ← Landing page
├── assets/css/landing.css        ← Styles
├── assets/js/landing.js          ← Progressive enhancement
├── assets/img/logo.svg           ← Logo mark
├── assets/img/logo-full.svg      ← Wordmark
├── assets/img/favicon.svg        ← Favicon
├── assets/img/og-image.png       ← Social preview
├── supplemental-ui/css/custom.css ← Antora dark theme
├── antora-playbook.yml           ← Doc build config
├── .github/workflows/publish.yml ← CI/CD
├── .nojekyll                     ← Disable Jekyll
└── README.md                     ← Repo documentation
```
