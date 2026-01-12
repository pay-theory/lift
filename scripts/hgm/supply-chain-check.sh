#!/usr/bin/env bash
set -euo pipefail

# Supply-chain check (Lift)
#
# Enforces that GitHub Actions workflow 'uses:' references are pinned to immutable SHAs.
# (This does not yet cover provenance/SLSA; those are tracked in roadmap milestones.)

wf_dir=".github/workflows"
if [[ ! -d "$wf_dir" ]]; then
  echo "[supply-chain-check] missing $wf_dir" >&2
  exit 1
fi

bad=0
while IFS= read -r line; do
  # Extract the part after 'uses:'
  ref=$(echo "$line" | sed -E 's/^.*uses:\s*//')

  # Ignore local actions (./path)
  if [[ "$ref" =~ ^\./ ]]; then
    continue
  fi

  # Require @<40-hex>
  if ! echo "$ref" | grep -Eq '@[0-9a-f]{40}'; then
    echo "[supply-chain-check] FAIL: action not pinned to commit SHA: $ref" >&2
    bad=1
  fi

done < <(grep -RhoE '^\s*-?\s*uses:\s*[^[:space:]]+' "$wf_dir"/*.yml)

if [[ "$bad" -ne 0 ]]; then
  exit 1
fi

echo "[supply-chain-check] OK"
