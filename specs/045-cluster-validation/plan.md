# Implementation Plan: Real-Cluster Validation Harness

**Branch**: `045-cluster-validation` | **Date**: 2026-03-09 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/045-cluster-validation/spec.md`

## Summary

Build a Go test suite (`//go:build cluster`) that validates Antwort against real ROSA HCP clusters with live LLM backends. The harness covers basic inference, multi-provider comparison, tool calling, and BFCL benchmarks (~180 fixed cases). A shell orchestrator handles cluster detection and report generation. Results are published as timestamped markdown files for the documentation site.

## Technical Context

**Language/Version**: Go 1.22+ (consistent with all existing specs)
**Primary Dependencies**: Go standard library + `github.com/openai/openai-go` (existing test dependency from spec 043)
**Storage**: N/A (file-based results only, no database)
**Testing**: `go test -tags cluster ./test/cluster/` (manual execution against live clusters)
**Target Platform**: Developer laptop with network access to ROSA HCP cluster
**Project Type**: Test suite + shell orchestrator + documentation
**Performance Goals**: Standard suite completes in under 15 minutes
**Constraints**: No CI integration (no GPU runners), no cluster lifecycle management
**Scale/Scope**: ~180 BFCL cases (fixed subset), expandable to ~4,700 via `--bfcl-all`

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Interface-First | N/A | Test suite, not a core package |
| II. Zero External Dependencies | PASS | Uses existing openai-go test dep only |
| III. Nil-Safe Composition | N/A | Test code |
| IV. Typed Error Domain | N/A | Test assertions, not API errors |
| V. Validate Early | PASS | TestMain validates cluster reachability before running |
| VI. Protocol-Agnostic Provider | N/A | Tests consume providers, don't define them |
| VII. Streaming First-Class | PASS | Streaming tests included as core category |
| VIII. Context Carries Cross-Cutting | N/A | Test code |
| IX. Kubernetes-Native | PASS | Tests run against K8s-deployed Antwort |
| Testing Standards | PASS | Go tests with build tag, table-driven BFCL cases |
| Documentation | PASS | Antora validation page + landing page card planned |

No violations. No complexity tracking needed.

## Project Structure

### Documentation (this feature)

```text
specs/045-cluster-validation/
├── spec.md
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
└── tasks.md             # Phase 2 output
```

### Source Code (repository root)

```text
test/cluster/
├── cluster_test.go      # TestMain, env vars, client factories, ResultCollector
├── basic_test.go         # US1: non-streaming, streaming inference
├── provider_test.go      # US2: multi-provider comparison (3 paths)
├── tools_test.go         # US3: tool calling validation
├── bfcl_test.go          # US4: BFCL benchmark runner + AST scorer
├── bfcl_loader.go        # BFCL JSONL parser, Gorilla-to-OpenAPI converter
├── bfcl_scorer.go        # AST evaluation (simple, parallel, multiple, irrelevance)
├── features_test.go      # US6: background, RAG, auth, conversations
├── results.go            # ResultCollector, JSON output, latency stats
├── run.sh                # Shell orchestrator (cluster check, model select, report gen)
├── report.sh             # Markdown report generator (reads results JSON)
├── README.md             # How to run, env vars, requirements
├── testdata/
│   └── bfcl/
│       ├── simple_python.jsonl      # First 50 cases
│       ├── multiple.jsonl           # First 50 cases
│       ├── parallel.jsonl           # First 30 cases
│       ├── parallel_multiple.jsonl  # First 20 cases
│       ├── irrelevance.jsonl        # First 30 cases
│       └── answers/                 # Ground truth for above
│           ├── simple_python.jsonl
│           ├── multiple.jsonl
│           ├── parallel.jsonl
│           ├── parallel_multiple.jsonl
│           └── irrelevance.jsonl
└── results/                         # Generated (gitignored except committed runs)
    ├── .gitkeep
    ├── latest.json                  # Machine-readable summary
    └── latest.md                    # Symlink to most recent report

docs/modules/operations/pages/
└── validation.adoc                  # Antora documentation page

.claude/
├── instills/rosa/
│   ├── antwort-minimal/
│   │   ├── INSTILL.md               # Metadata: requires model
│   │   ├── install.md               # Deploy quickstart 01-minimal
│   │   └── uninstall.md             # Remove deployment
│   ├── antwort-production/
│   │   ├── INSTILL.md               # Metadata: requires model
│   │   ├── install.md               # Deploy quickstart 02-production
│   │   └── uninstall.md
│   ├── antwort-rag/
│   │   ├── INSTILL.md               # Metadata: requires model
│   │   ├── install.md               # Deploy with file search + vector store
│   │   └── uninstall.md
│   └── antwort-background/
│       ├── INSTILL.md               # Metadata: requires model
│       ├── install.md               # Deploy gateway + worker
│       └── uninstall.md
├── rosa-recipe.yaml                 # Declarative validation stack recipe

Makefile                             # cluster-test target
```

**Structure Decision**: Test suite lives in `test/cluster/` following the existing `test/e2e/` pattern. BFCL data in `testdata/` subdirectory. Results directory is gitignored except for deliberately committed runs.

## Design Decisions

### DD1: ResultCollector in TestMain

TestMain initializes a global `ResultCollector`. Individual tests call `collector.Record(result)` after each test case. On teardown, the collector writes `results/raw/<timestamp>.json`. This avoids parsing Go test output and gives structured data directly.

### DD2: BFCL Scorer in Go

The AST evaluation logic is reimplemented in Go (not shelled out to Python). The logic is straightforward (name match, value comparison, string normalization) and keeping it in Go avoids a Python dependency for the test suite.

### DD3: Shell Orchestrator + Go Tests

`run.sh` handles the interactive parts (cluster detection, model selection prompts, report generation). Go tests handle the deterministic parts (HTTP requests, assertions, result collection). This separates concerns cleanly.

### DD4: Environment-Based Provider Path Selection

Tests don't reconfigure Antwort. Instead, `CLUSTER_ANTWORT_URL` points to the deployed Antwort instance (which uses whichever provider is configured), and `CLUSTER_VLLM_URL` points directly to vLLM for baseline comparison. To test both provider paths, redeploy Antwort with a different provider config and rerun.

### DD5: BFCL Data Committed as Subset

The fixed 180-case subset is committed to `test/cluster/testdata/bfcl/` as JSONL files (already converted from Gorilla format to OpenAPI). The full dataset is downloaded on demand by `run.sh --bfcl-download`.

### DD6: cc-rosa Instills for Antwort Deployments

Antwort deployment on ROSA HCP clusters is automated via project-level cc-rosa instills in `.claude/cc-rosa/instills/`. Each instill follows the standard format (INSTILL.md with YAML frontmatter, install.md, uninstall.md). Instills are automatically discovered by the cc-rosa plugin hook.

Instills to create:

| ID | Name | Requires | Description |
|----|------|----------|-------------|
| `antwort-minimal` | Antwort (Minimal) | model | Deploy quickstart 01-minimal against vLLM |
| `antwort-production` | Antwort (Production) | model | Deploy quickstart 02 with in-memory storage |
| `antwort-rag` | Antwort (RAG) | model | Deploy with file search and vector store |
| `antwort-background` | Antwort (Background) | model | Deploy quickstart 09 with gateway+worker |

### DD7: Declarative Recipe for Validation Stack

A single recipe file (`.claude/cc-rosa/recipe.yaml`) deploys the complete validation stack via `/rosa:setup`. The recipe specifies cluster config, GPU machinepool, and the installation chain (rhoai -> model -> antwort-minimal). Recipe parameters allow customizing the model and Antwort configuration without editing the recipe.

Recipe supports idempotent reconciliation: re-running skips already-completed steps, making it safe to use for incremental deployment or recovery after partial failures.
