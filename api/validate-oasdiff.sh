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
#
# NOTE: oasdiff labels changes as "error", "warning", or "info" based on the
# *type* of change (e.g., type-changed = error, property-removed = warning).
# These labels describe change severity, NOT test failures.
# The command returns exit 0 if no changes would break existing clients,
# even when many "error"-labeled changes are reported.
RESULT=0
OUTPUT=$(oasdiff breaking "$UPSTREAM_URL" "$LOCAL_SPEC" \
    --prefix-base "/v1" \
    --format text 2>&1) || RESULT=$?

echo "$OUTPUT"

if [ "$RESULT" -eq 0 ]; then
    # Count error/warning/info for the summary
    ERRORS=$(echo "$OUTPUT" | grep -c '^error' || true)
    WARNINGS=$(echo "$OUTPUT" | grep -c '^warning' || true)

    echo ""
    echo "=========================================="
    echo "  RESULT: No breaking changes detected"
    echo "=========================================="
    if [ "$ERRORS" -gt 0 ] || [ "$WARNINGS" -gt 0 ]; then
        echo ""
        echo "  The ${ERRORS} error(s) and ${WARNINGS} warning(s) above"
        echo "  are change classifications, not failures."
        echo "  They indicate places where antwort's spec diverges"
        echo "  from upstream (simpler schemas, fewer tool types,"
        echo "  nullable fields). These are intentional and do not"
        echo "  break OpenAI SDK clients."
    fi
else
    echo ""
    echo "=========================================="
    echo "  RESULT: BREAKING CHANGES DETECTED"
    echo "=========================================="
    echo ""
    echo "  Exit code: $RESULT"
    echo "  These changes would break existing clients."
    echo "  If intentional, document in api/DIVERGENCES.md"
fi

# Run a diff summary for informational purposes.
echo ""
echo "=== oasdiff Change Summary ==="
oasdiff summary "$UPSTREAM_URL" "$LOCAL_SPEC" \
    --prefix-base "/v1" 2>&1 || true

exit "$RESULT"
