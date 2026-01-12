#!/usr/bin/env bash
set -euo pipefail

# Coverage threshold gate (Lift)
#
# Measures "core coverage" (excluding pkg/testing/**) and enforces a minimum threshold.
#
# Default threshold is 90% (overridable via COV_THRESHOLD).

threshold="${COV_THRESHOLD:-90}"

if [[ ! -f Makefile ]]; then
  echo "[coverage] missing Makefile (expected to run 'make test-coverage-core')" >&2
  exit 1
fi

# Run canonical coverage target.
# This will generate coverage.out and coverage.core.out.
make test-coverage-core

if [[ ! -f coverage.core.out ]]; then
  echo "[coverage] missing coverage.core.out after running make test-coverage-core" >&2
  exit 1
fi

# Extract "total" coverage percent.
core_cov=$(go tool cover -func=coverage.core.out | awk '/^total:/{gsub(/%/,"",$3); print $3; exit}')
if [[ -z "$core_cov" ]]; then
  echo "[coverage] could not parse core coverage percent" >&2
  exit 1
fi

pass=$(awk -v c="$core_cov" -v t="$threshold" 'BEGIN{if (c+0 >= t+0) print 1; else print 0}')

echo "[coverage] core_coverage=${core_cov}% threshold=${threshold}%"

if [[ "$pass" -ne 1 ]]; then
  echo "[coverage] FAIL: core coverage below threshold" >&2
  exit 1
fi

echo "[coverage] OK"
