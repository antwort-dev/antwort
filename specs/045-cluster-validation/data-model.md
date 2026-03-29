# Data Model: Real-Cluster Validation Harness

**Date**: 2026-03-09
**Feature**: 045-cluster-validation

## Entities

### BFCLCase

A single BFCL benchmark test case loaded from JSONL files.

| Field | Type | Description |
|-------|------|-------------|
| ID | string | Category-prefixed identifier (e.g., `simple_python_0`) |
| Category | string | BFCL category (simple_python, multiple, parallel, etc.) |
| Question | []Message | User messages (role + content) |
| Functions | []FunctionDef | Function definitions in OpenAPI format (converted from Gorilla) |
| GroundTruth | []FunctionCall | Expected function calls with acceptable argument values |

### FunctionCall

Expected function call from BFCL ground truth.

| Field | Type | Description |
|-------|------|-------------|
| Name | string | Function name (dots replaced with underscores) |
| Arguments | map[string][]any | Parameter name to list of acceptable values |

### TestResult

Per-test-case outcome collected during a validation run.

| Field | Type | Description |
|-------|------|-------------|
| Name | string | Go test name (e.g., `TestBasicNonStreaming`) |
| Category | string | Test category (basic, streaming, tools, bfcl, background, etc.) |
| ProviderPath | string | Which path was tested (antwort-cc, antwort-responses, vllm-direct) |
| Passed | bool | Whether the test passed |
| Duration | time.Duration | Total test execution time |
| TTFT | time.Duration | Time to first token (streaming only, zero otherwise) |
| Error | string | Error message if failed (empty if passed) |
| Details | map[string]any | Additional metadata (model output, expected output for BFCL) |

### ResultCollector

Thread-safe collector aggregating TestResults during a validation run. Writes JSON on teardown.

| Field | Type | Description |
|-------|------|-------------|
| Model | string | Model name under test |
| AntwortVersion | string | Antwort git version |
| ClusterName | string | Cluster identifier |
| StartTime | time.Time | When the run started |
| Results | []TestResult | All collected test results |

### ResultSummary

JSON output written by TestMain, consumed by run.sh for markdown generation.

| Field | Type | Description |
|-------|------|-------------|
| Model | string | Model name |
| AntwortVersion | string | Git commit or tag |
| Cluster | string | Cluster name |
| Timestamp | string | ISO 8601 timestamp |
| Categories | map[string]CategoryScore | Per-category results |
| Latency | LatencyStats | Aggregated latency percentiles |
| Failures | []FailureDetail | Details of failed tests |

### CategoryScore

| Field | Type | Description |
|-------|------|-------------|
| Passed | int | Number of passing tests |
| Total | int | Total tests in category |
| Score | float64 | Pass rate (0.0-1.0) |

### LatencyStats

| Field | Type | Description |
|-------|------|-------------|
| NonStreamingP50 | time.Duration | 50th percentile total request time |
| NonStreamingP95 | time.Duration | 95th percentile |
| NonStreamingP99 | time.Duration | 99th percentile |
| StreamingTTFTP50 | time.Duration | 50th percentile time-to-first-token |
| StreamingTTFTP95 | time.Duration | 95th percentile |
| StreamingTTFTP99 | time.Duration | 99th percentile |

### FailureDetail

| Field | Type | Description |
|-------|------|-------------|
| TestName | string | Full test name |
| Category | string | Test category |
| Error | string | Error message |
| Expected | string | Expected output (BFCL: function call) |
| Got | string | Actual output |

## State Transitions

None. All entities are ephemeral (created during test run, written to files, discarded).

## File Artifacts

| Artifact | Format | Location | Lifecycle |
|----------|--------|----------|-----------|
| BFCL test data (subset) | JSONL | `test/cluster/testdata/bfcl/` | Committed to repo |
| BFCL ground truth (subset) | JSONL | `test/cluster/testdata/bfcl/answers/` | Committed to repo |
| Result JSON | JSON | `test/cluster/results/raw/` | Generated per run |
| Result markdown | Markdown | `test/cluster/results/` | Generated per run, committable |
| Latest JSON | JSON | `test/cluster/results/latest.json` | Updated per run |
| Latest symlink | Symlink | `test/cluster/results/latest.md` | Updated per run |
