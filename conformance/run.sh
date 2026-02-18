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
#   5. Runs the official OpenResponses compliance suite
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

# PIDs for cleanup.
MOCK_PID=""
SERVER_PID=""

cleanup() {
    echo "Cleaning up..."
    [ -n "$SERVER_PID" ] && kill "$SERVER_PID" 2>/dev/null || true
    [ -n "$MOCK_PID" ] && kill "$MOCK_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
    wait "$MOCK_PID" 2>/dev/null || true
}
trap cleanup EXIT

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

# Run a quick manual smoke test to verify basic functionality.
echo ""
echo "=== Quick Smoke Test ==="
SMOKE_RESULT=$(curl -s -X POST "http://localhost:$SERVER_PORT/v1/responses" \
    -H "Content-Type: application/json" \
    -d '{"model":"mock-model","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"Hello"}]}]}')

SMOKE_STATUS=$(echo "$SMOKE_RESULT" | jq -r '.status' 2>/dev/null || echo "PARSE_ERROR")
if [ "$SMOKE_STATUS" = "completed" ]; then
    echo "Smoke test PASSED (status: completed)"
else
    echo "Smoke test FAILED (status: $SMOKE_STATUS)"
    echo "Response: $SMOKE_RESULT"
    exit 1
fi

# Load profile.
echo ""
echo "=== Running Conformance Tests (profile: $PROFILE) ==="
PROFILE_TESTS=$(jq -r ".[\"$PROFILE\"].tests[]" "$SCRIPT_DIR/profiles.json" 2>/dev/null)
if [ -z "$PROFILE_TESTS" ]; then
    echo "ERROR: Unknown profile '$PROFILE'. Available: core, extended"
    exit 1
fi

PROFILE_COUNT=$(echo "$PROFILE_TESTS" | wc -l | tr -d ' ')
echo "Profile '$PROFILE' includes $PROFILE_COUNT tests"

# TODO: Run the official compliance suite via podman container.
# For now, report the smoke test result and the profile configuration.
# The full containerized suite integration requires the Containerfile to be built.
echo ""
echo "=== Conformance Results ==="
echo "{"
echo "  \"profile\": \"$PROFILE\","
echo "  \"smoke_test\": \"passed\","
echo "  \"note\": \"Full compliance suite requires: podman build -t antwort-conformance -f conformance/Containerfile .\","
echo "  \"profile_tests\": $(jq -c ".[\"$PROFILE\"].tests" "$SCRIPT_DIR/profiles.json")"
echo "}"

echo ""
echo "To run the full official compliance suite:"
echo "  1. podman build -t antwort-conformance -f conformance/Containerfile ."
echo "  2. podman run --rm --network=host -e BASE_URL=http://localhost:$SERVER_PORT/v1 -e MODEL=mock-model antwort-conformance"
echo ""
echo "Done."
