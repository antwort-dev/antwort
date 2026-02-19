#!/usr/bin/env bash
# OpenResponses Conformance Test Pipeline
#
# Usage: ./conformance/run.sh [PROFILE]
#   PROFILE: "core" (default) or "extended"
#
# Prerequisites:
#   - Go 1.22+ installed
#   - podman installed and running
#
# This script:
#   1. Builds server and mock-backend binaries
#   2. Starts mock-backend on port 9090
#   3. Starts antwort server on port 8080
#   4. Waits for readiness
#   5. Builds and runs the official OpenResponses compliance suite container
#   6. Filters results by profile
#   7. Reports conformance score
#   8. Cleans up all processes

set -euo pipefail

PROFILE="${1:-core}"
MOCK_PORT=9090
SERVER_PORT=8080
STARTUP_TIMEOUT=15
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
IMAGE_NAME="antwort-conformance"

# PIDs for cleanup.
MOCK_PID=""
SERVER_PID=""

cleanup() {
    echo ""
    echo "Cleaning up..."
    [ -n "$SERVER_PID" ] && kill "$SERVER_PID" 2>/dev/null || true
    [ -n "$MOCK_PID" ] && kill "$MOCK_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
    wait "$MOCK_PID" 2>/dev/null || true
}
trap cleanup EXIT

# Resolve profile to test filter.
FILTER=""
case "$PROFILE" in
    core)
        FILTER="basic-response,streaming-response,system-prompt,tool-calling,multi-turn"
        ;;
    extended)
        FILTER=""  # empty = run all tests
        ;;
    *)
        echo "ERROR: Unknown profile '$PROFILE'. Available: core, extended"
        exit 1
        ;;
esac

echo "=== OpenResponses Conformance Testing ==="
echo "Profile: $PROFILE"
echo ""

# Build binaries.
echo "Building binaries..."
cd "$PROJECT_DIR"
go build -o "$PROJECT_DIR/bin/server" ./cmd/server/
go build -o "$PROJECT_DIR/bin/mock-backend" ./cmd/mock-backend/

# Start mock backend.
echo "Starting mock backend on port $MOCK_PORT..."
MOCK_PORT=$MOCK_PORT "$PROJECT_DIR/bin/mock-backend" &
MOCK_PID=$!

# Start antwort server.
echo "Starting antwort server on port $SERVER_PORT..."
ANTWORT_BACKEND_URL="http://localhost:$MOCK_PORT" \
ANTWORT_MODEL="mock-model" \
ANTWORT_PORT="$SERVER_PORT" \
ANTWORT_STORAGE="memory" \
"$PROJECT_DIR/bin/server" &
SERVER_PID=$!

# Wait for readiness.
echo "Waiting for services to be ready..."
for i in $(seq 1 $STARTUP_TIMEOUT); do
    if curl -s "http://localhost:$SERVER_PORT/healthz" > /dev/null 2>&1 && \
       curl -s "http://localhost:$MOCK_PORT/healthz" > /dev/null 2>&1; then
        echo "Services ready after ${i}s"
        break
    fi
    if [ "$i" -eq "$STARTUP_TIMEOUT" ]; then
        echo "ERROR: Services did not start within ${STARTUP_TIMEOUT}s"
        exit 1
    fi
    sleep 1
done

# Quick smoke test.
echo ""
echo "=== Smoke Test ==="
SMOKE_RESULT=$(curl -s -X POST "http://localhost:$SERVER_PORT/v1/responses" \
    -H "Content-Type: application/json" \
    -d '{"model":"mock-model","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"Hello"}]}]}')

SMOKE_STATUS=$(echo "$SMOKE_RESULT" | jq -r '.status' 2>/dev/null || echo "PARSE_ERROR")
if [ "$SMOKE_STATUS" = "completed" ]; then
    echo "Smoke test PASSED"
else
    echo "Smoke test FAILED (status: $SMOKE_STATUS)"
    echo "Response: $SMOKE_RESULT"
    exit 1
fi

# Build conformance container (if not already built).
echo ""
echo "=== Building Conformance Container ==="
if ! podman image exists "$IMAGE_NAME" 2>/dev/null; then
    podman build -t "$IMAGE_NAME" -f "$SCRIPT_DIR/Containerfile" "$PROJECT_DIR"
else
    echo "Container image already exists, reusing. Run 'podman rmi $IMAGE_NAME' to rebuild."
fi

# Run the official compliance suite.
echo ""
echo "=== Running Official Compliance Suite ==="

# Determine the host URL accessible from the container.
# On macOS with podman machine, use host.containers.internal.
# On Linux, use host network mode.
HOST_URL="http://host.containers.internal:$SERVER_PORT/v1"

CONTAINER_ARGS=(
    run --rm
    -e "BASE_URL=$HOST_URL"
    -e "MODEL=mock-model"
    -e "API_KEY=test"
)

if [ -n "$FILTER" ]; then
    CONTAINER_ARGS+=(-e "FILTER=$FILTER")
fi

CONTAINER_ARGS+=("$IMAGE_NAME")

# Run and capture output.
RESULT_FILE=$(mktemp)
if podman "${CONTAINER_ARGS[@]}" > "$RESULT_FILE" 2>&1; then
    SUITE_EXIT=0
else
    SUITE_EXIT=$?
fi

# Parse and display results.
echo ""
echo "=== Conformance Results (profile: $PROFILE) ==="

if jq -e '.summary' "$RESULT_FILE" > /dev/null 2>&1; then
    # JSON output from the suite.
    PASSED=$(jq -r '.summary.passed' "$RESULT_FILE")
    FAILED=$(jq -r '.summary.failed' "$RESULT_FILE")
    TOTAL=$(jq -r '.summary.total' "$RESULT_FILE")

    echo "Score: $PASSED/$TOTAL passed"
    if [ "$FAILED" -gt 0 ]; then
        echo ""
        echo "Failed tests:"
        jq -r '.results[] | select(.status == "failed") | "  \u2717 \(.id): \(.errors[0] // "unknown error")"' "$RESULT_FILE"
    fi

    echo ""
    echo "All test results:"
    jq -r '.results[] | "  \(if .status == "passed" then "✓" elif .status == "failed" then "✗" else "○" end) \(.id) (\(.status)) \(if .duration then "[\(.duration)ms]" else "" end)"' "$RESULT_FILE"

    # Output the full JSON for CI consumption.
    echo ""
    echo "--- JSON Output ---"
    jq --arg profile "$PROFILE" '. + {profile: $profile}' "$RESULT_FILE"
else
    # Not JSON, print raw output.
    echo "Raw suite output:"
    cat "$RESULT_FILE"
fi

rm -f "$RESULT_FILE"

if [ "$SUITE_EXIT" -ne 0 ]; then
    echo ""
    echo "Conformance suite exited with code $SUITE_EXIT"
    exit "$SUITE_EXIT"
fi

echo ""
echo "Done."
