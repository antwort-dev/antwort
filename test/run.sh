#!/usr/bin/env bash
# API Conformance Test Pipeline
#
# Runs the full conformance pipeline:
#   1. oasdiff validation against upstream OpenResponses spec
#   2. Go integration tests with mock LLM backend
#   3. OpenResponses Zod compliance suite (if available)
#
# Usage: ./test/run.sh
#
# Exit codes:
#   0 - All tests pass
#   1 - One or more tests failed

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

PASS_COUNT=0
FAIL_COUNT=0
SKIP_COUNT=0
RESULTS=()

record_result() {
    local name="$1"
    local status="$2"
    local detail="${3:-}"

    case "$status" in
        PASS) PASS_COUNT=$((PASS_COUNT + 1)) ;;
        FAIL) FAIL_COUNT=$((FAIL_COUNT + 1)) ;;
        SKIP) SKIP_COUNT=$((SKIP_COUNT + 1)) ;;
    esac

    if [ -n "$detail" ]; then
        RESULTS+=("$status  $name: $detail")
    else
        RESULTS+=("$status  $name")
    fi
}

echo "=== Antwort API Conformance Pipeline ==="
echo ""

# --- Step 1: oasdiff validation ---
echo "--- Step 1: oasdiff Validation ---"
if command -v oasdiff &>/dev/null; then
    if "$PROJECT_DIR/api/validate-oasdiff.sh" 2>&1; then
        record_result "oasdiff" "PASS"
    else
        record_result "oasdiff" "FAIL" "breaking changes detected"
    fi
else
    record_result "oasdiff" "SKIP" "oasdiff not installed"
fi
echo ""

# --- Step 2: Go integration tests ---
echo "--- Step 2: Go Integration Tests ---"
if go test "$PROJECT_DIR/test/integration/" -timeout 120s -count=1 -v 2>&1; then
    record_result "integration-tests" "PASS"
else
    record_result "integration-tests" "FAIL" "see output above"
fi
echo ""

# --- Step 3: Zod conformance suite (optional) ---
echo "--- Step 3: Zod Compliance Suite ---"
if [ -f "$PROJECT_DIR/conformance/run.sh" ] && command -v podman &>/dev/null; then
    echo "Zod compliance suite available. Run separately with: make conformance"
    record_result "zod-suite" "SKIP" "run separately via 'make conformance'"
else
    record_result "zod-suite" "SKIP" "conformance suite or podman not available"
fi
echo ""

# --- Results ---
echo "=== Pipeline Results ==="
echo ""
for result in "${RESULTS[@]}"; do
    echo "  $result"
done
echo ""
echo "Summary: $PASS_COUNT passed, $FAIL_COUNT failed, $SKIP_COUNT skipped"
echo ""

if [ "$FAIL_COUNT" -gt 0 ]; then
    echo "PIPELINE FAILED"
    exit 1
fi

echo "PIPELINE PASSED"
exit 0
