#!/usr/bin/env bash
set -euo pipefail

# Run tests with package coverage limited to the production pkg tree
TMP_PROFILE=$(mktemp)
cleanup() {
  rm -f "$TMP_PROFILE"
}
trap cleanup EXIT

# Always produce coverage artefact for later inspection
mkdir -p coverage
PROFILE="coverage/coverage.out"

# Some GitHub runners (Go 1.23.x matrix) lack the covdata tool when the auto
# toolchain shim downloads Go 1.25, so fall back to vanilla coverage in that case.
COVER_ARGS=("-coverprofile=$TMP_PROFILE")
if go tool -n covdata >/dev/null 2>&1; then
  COVER_ARGS+=("-coverpkg=./pkg/...")
else
  echo "covdata tool missing; skipping -coverpkg for compatibility with $(go env GOVERSION)" >&2
fi

if ! go test "${COVER_ARGS[@]}" ./...; then
  echo "go test failed" >&2
  exit 1
fi

mv "$TMP_PROFILE" "$PROFILE"

THRESHOLD=35
TOTAL=$(go tool cover -func="$PROFILE" | awk 'END { sub("%", "", $3); print $3 }')
TOTAL_INT=${TOTAL%.*}

if (( TOTAL_INT < THRESHOLD )); then
  echo "Coverage check failed: ${TOTAL}% < ${THRESHOLD}%" >&2
  exit 1
fi

echo "Coverage check passed: ${TOTAL}% >= ${THRESHOLD}%"
