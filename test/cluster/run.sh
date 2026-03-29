#!/usr/bin/env bash
# Cluster validation orchestrator
# Usage: test/cluster/run.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Check cluster URL
if [ -z "${CLUSTER_ANTWORT_URL:-}" ]; then
    echo "ERROR: CLUSTER_ANTWORT_URL not set"
    echo "  Export it before running: export CLUSTER_ANTWORT_URL=https://antwort.apps.cluster.example.com/v1"
    exit 1
fi

# Check cluster reachability
echo "Checking cluster reachability..."
if ! curl -sf --connect-timeout 5 "${CLUSTER_ANTWORT_URL}/healthz" >/dev/null 2>&1; then
    # Try without /healthz (some deployments may not have it)
    if ! curl -sf --connect-timeout 5 "${CLUSTER_ANTWORT_URL}/" >/dev/null 2>&1; then
        echo "ERROR: Cluster not reachable at ${CLUSTER_ANTWORT_URL}"
        exit 1
    fi
fi
echo "Cluster reachable at ${CLUSTER_ANTWORT_URL}"

# Model selection
if [ -z "${CLUSTER_MODEL:-}" ]; then
    echo ""
    echo "No CLUSTER_MODEL set. Enter the model name to test:"
    read -r CLUSTER_MODEL
    export CLUSTER_MODEL
fi
echo "Model: ${CLUSTER_MODEL}"

# Version detection
if [ -z "${CLUSTER_ANTWORT_VERSION:-}" ]; then
    CLUSTER_ANTWORT_VERSION=$(cd "$REPO_ROOT" && git rev-parse --short HEAD 2>/dev/null || echo "unknown")
    export CLUSTER_ANTWORT_VERSION
fi
echo "Antwort version: ${CLUSTER_ANTWORT_VERSION}"

# Run tests
echo ""
echo "Running cluster validation tests..."
echo "=================================="

cd "$REPO_ROOT"
go test -tags cluster ./test/cluster/ -v -timeout 300s -count=1 2>&1 | tee /tmp/cluster-test-output.txt
TEST_EXIT=$?

# Generate report
echo ""
echo "=================================="
if [ $TEST_EXIT -eq 0 ]; then
    echo "All tests passed."
else
    echo "Some tests failed (exit code: $TEST_EXIT)."
fi

# Find the latest results JSON and generate report
LATEST_JSON=$(ls -t "$SCRIPT_DIR/results/raw/"*.json 2>/dev/null | head -1)
if [ -n "$LATEST_JSON" ]; then
    echo "Results JSON: $LATEST_JSON"
    bash "$SCRIPT_DIR/report.sh" "$LATEST_JSON"
else
    echo "No results JSON found (tests may have been skipped)."
fi

exit $TEST_EXIT
