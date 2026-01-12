#!/usr/bin/env bash
set -euo pipefail

# hgm planning docs presence check for Lift
# Fail closed: missing docs => non-zero.

required=(
  "docs/planning/lift-controls-matrix.md"
  "docs/planning/lift-threat-model.md"
  "docs/planning/lift-10of10-rubric.md"
  "docs/planning/lift-10of10-roadmap.md"
  "docs/planning/lift-evidence-plan.md"
  "docs/planning/lift-ai-drift-recovery.md"
  "docs/planning/lift-hgm-pack.json"
)

missing=0
for f in "${required[@]}"; do
  if [[ ! -f "$f" ]]; then
    echo "[planning-docs-check] missing: $f" >&2
    missing=1
    continue
  fi
  if [[ ! -s "$f" ]]; then
    echo "[planning-docs-check] empty: $f" >&2
    missing=1
  fi
done

if [[ "$missing" -ne 0 ]]; then
  echo "[planning-docs-check] FAIL" >&2
  exit 1
fi

echo "[planning-docs-check] OK"
