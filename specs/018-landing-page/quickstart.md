# Implementation Quickstart: Landing Page & Documentation Site

**Feature**: 018-landing-page
**Date**: 2026-02-23

## Prerequisites

- Node.js 22+ (for Astro and Antora builds)
- npm or pnpm

## Step 1: Scaffold AstroWind Project

```bash
cd /Users/rhuss/Development/ai/antwort.github.io
# Remove previous hand-crafted files
rm -rf assets index.html supplemental-ui package.json package-lock.json node_modules build .nojekyll

# Scaffold AstroWind (keep .git, .github, README, antora-playbook.yml)
npx degit onwidget/astrowind --force .
```

## Step 2: Install Dependencies

```bash
npm install
```

## Step 3: Customize Content

Edit `src/pages/index.astro` to compose AstroWind widgets with Antwort content (hero, features, comparison, quickstart, roadmap).

## Step 4: Customize Theme

Edit `tailwind.config.js` to set the cyan-teal accent palette. Configure dark mode as default in AstroWind's site config.

## Step 5: Add Logo

Replace `src/components/Logo.astro` with the A! wordmark SVG. Copy `logo.svg` and `favicon.svg` to `src/assets/`.

## Step 6: Add Antora Integration

Keep `antora-playbook.yml` and `supplemental-ui/` from previous implementation. Update the GitHub Actions workflow to run both Astro build and Antora build.

## Step 7: Preview Locally

```bash
npm run dev     # Astro dev server at http://localhost:4321
```

## Step 8: Build and Verify

```bash
npm run build   # Produces dist/
npx antora antora-playbook.yml  # Produces dist/docs/ (configure output.dir)
```

## Step 9: Deploy

Push to GitHub. The GitHub Actions workflow handles build + deploy.
