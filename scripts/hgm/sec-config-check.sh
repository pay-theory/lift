#!/usr/bin/env bash
set -euo pipefail

# Security config anti-drift check (Lift)
#
# Purpose:
# - Ensure security-relevant linters remain enabled.
# - Prevent silent dilution via broad gosec excludes.

cfg=".golangci.yml"
if [[ ! -f "$cfg" ]]; then
  echo "[sec-config-check] missing $cfg" >&2
  exit 1
fi

# Ensure gosec is enabled
if ! grep -Eq "^\s*-\s*gosec\b" "$cfg"; then
  echo "[sec-config-check] FAIL: gosec is not enabled in $cfg" >&2
  exit 1
fi

# Ensure staticcheck is enabled
if ! grep -Eq "^\s*-\s*staticcheck\b" "$cfg"; then
  echo "[sec-config-check] FAIL: staticcheck is not enabled in $cfg" >&2
  exit 1
fi

# Check gosec excludes list: allow only G104 (repository currently documents this).
# Extract lines that look like "- G###" anywhere under the gosec section.
# This is not a full YAML parser, but it is good enough to detect broad dilution.
allowed=("G104")

# Collect all unique gosec exclude codes.
mapfile -t excludes < <(
  awk '
    $1=="gosec:" {in=1}
    in && /^[^[:space:]]/ && $1!="gosec:" {in=0}
    in && /- G[0-9]+/ {for(i=1;i<=NF;i++){if($i ~ /^G[0-9]+$/) print $i}}
  ' "$cfg" | sort -u
)

for code in "${excludes[@]:-}"; do
  ok=0
  for a in "${allowed[@]}"; do
    if [[ "$code" == "$a" ]]; then ok=1; fi
  done
  if [[ "$ok" -ne 1 ]]; then
    echo "[sec-config-check] FAIL: unexpected gosec exclude present: $code" >&2
    echo "[sec-config-check] Allowed excludes: ${allowed[*]}" >&2
    exit 1
  fi
done

echo "[sec-config-check] OK"
