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

if ! go test ./... -coverpkg=./pkg/... -coverprofile="$TMP_PROFILE"; then
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
