#!/usr/bin/env bash
set -euo pipefail

echo "[sast-check] BLOCKED: not implemented as standalone yet" >&2

echo "Minimum expectation: run gosec (directly or via golangci-lint) with pinned version." >&2

echo "Note: golangci-lint already runs gosec in CI via .golangci.yml." >&2

exit 2
