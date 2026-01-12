#!/usr/bin/env bash
set -euo pipefail

# Doc integrity check (Lift)
#
# Minimal integrity checks:
# - required planning docs exist
# - core cross-references exist
# - threat/control parity passes

bash scripts/hgm/planning-docs-check.sh

controls="docs/planning/lift-controls-matrix.md"
threats="docs/planning/lift-threat-model.md"

# Cross-references (best-effort; fail if missing)
if ! grep -q "lift-threat-model.md" "$controls"; then
  echo "[doc-integrity] FAIL: controls matrix does not reference lift-threat-model.md" >&2
  exit 1
fi
if ! grep -q "lift-controls-matrix.md" "$threats"; then
  echo "[doc-integrity] FAIL: threat model does not reference lift-controls-matrix.md" >&2
  exit 1
fi

# Ensure threat model has a threats table with at least one THR-
if ! grep -Eq 'THR-[0-9]+' "$threats"; then
  echo "[doc-integrity] FAIL: threat model contains no THR-* IDs" >&2
  exit 1
fi

bash scripts/hgm/threat-controls-parity.sh

echo "[doc-integrity] OK"
