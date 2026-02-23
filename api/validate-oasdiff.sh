#!/usr/bin/env bash
# Validate antwort's OpenAPI spec against the upstream OpenResponses spec
# using oasdiff to detect breaking changes.
#
# Usage: ./api/validate-oasdiff.sh
#
# Prerequisites:
#   - oasdiff installed (go install github.com/oasdiff/oasdiff@latest)
#
# Exit codes:
#   0 - No breaking changes detected
#   1 - Breaking changes detected or validation error

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
LOCAL_SPEC="$SCRIPT_DIR/openapi.yaml"

# The upstream spec uses "/responses" with server base URL "https://api.openai.com/v1".
# Our spec uses "/v1/responses" as the full path.
# We use --prefix-base to prepend /v1 to upstream paths for comparison.
UPSTREAM_URL="https://raw.githubusercontent.com/openresponses/openresponses/main/schema/openapi.json"

# Check prerequisites.
if ! command -v oasdiff &>/dev/null; then
    echo "ERROR: oasdiff not found. Install with: go install github.com/oasdiff/oasdiff@latest"
    exit 1
fi

if [ ! -f "$LOCAL_SPEC" ]; then
    echo "ERROR: Local spec not found at $LOCAL_SPEC"
    exit 1
fi

echo "=== oasdiff Breaking Change Detection ==="
echo "Base (upstream): $UPSTREAM_URL"
echo "Revision (ours): $LOCAL_SPEC"
echo ""

# Compare using remote URL (oasdiff resolves $ref references).
# --prefix-base /v1 aligns the upstream /responses path with our /v1/responses.
RESULT=0
if oasdiff breaking "$UPSTREAM_URL" "$LOCAL_SPEC" \
    --prefix-base "/v1" \
    --format text 2>&1; then
    echo "No breaking changes detected."
else
    RESULT=$?
    echo ""
    echo "Breaking changes detected (exit code: $RESULT)."
    echo ""
    echo "If these are intentional divergences, document them in api/DIVERGENCES.md"
fi

# Run a diff summary for informational purposes.
echo ""
echo "=== oasdiff Change Summary ==="
oasdiff summary "$UPSTREAM_URL" "$LOCAL_SPEC" \
    --prefix-base "/v1" 2>&1 || true

exit "$RESULT"
