# Brainstorm: Documentation Site

**Date:** 2026-02-28
**Status:** spec-created
**Spec:** specs/032-documentation-site/

## Problem Framing

The antwort project has 30+ specs, 6 quickstarts, comprehensive Go interfaces, and a detailed config system, but almost no user-facing documentation. The existing Antora setup has 12 stub pages, most marked "under construction". Developers and operators have no reference material outside reading source code.

## Approaches Considered

### A: Single Antora Module (Flat)
- Pros: Simple structure, minimal overhead
- Cons: Poor navigation at scale, no logical grouping

### B: Multiple Antora Modules (Chosen)
- Pros: Clear separation (tutorial, reference, extensions, quickstarts, operations), independent navigation per module, matches how different audiences browse
- Cons: More structure overhead, cross-module xrefs needed

### C: Multiple Antora Components
- Pros: Maximum independence, separate versioning
- Cons: Complex playbook, overkill for this project size

## Decision

Approach B (multiple modules). Six modules: ROOT, tutorial, reference, extensions, quickstarts, operations. The structure maps to distinct audiences (newcomers, operators, contributors) and can grow independently.

### Key Design Decisions

- **Voice**: kubernetes-patterns profile (existing at `~/.claude/style/voices/kubernetes-patterns.yaml`), authoritative O'Reilly-style with OOP analogies and no humor
- **Quickstarts**: Convert from Markdown to AsciiDoc (AsciiDoc becomes source of truth)
- **Config reference**: Both annotated examples (for learning) and reference tables (for lookup)
- **Operations**: Dedicated module covering monitoring, deployment, troubleshooting, security
- **Build**: `make docs` target using npx antora with lunr search
- **Scope**: Full content for all modules (not just skeletons)

## Open Threads

- How to handle the landing page (Spec 018) integration with the Antora docs
- Whether to generate Markdown READMEs from AsciiDoc source (build step) or maintain both
