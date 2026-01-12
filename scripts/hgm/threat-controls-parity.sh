#!/usr/bin/env bash
set -euo pipefail

# Threat ↔ Controls parity check (Lift)
#
# Rule:
# - Every THR-* in docs/planning/lift-threat-model.md must appear in docs/planning/lift-controls-matrix.md
# - Every THR-* referenced in docs/planning/lift-controls-matrix.md must exist in the threat model

threat_model="docs/planning/lift-threat-model.md"
controls_matrix="docs/planning/lift-controls-matrix.md"

if [[ ! -f "$threat_model" ]]; then
  echo "[parity] missing threat model: $threat_model" >&2
  exit 1
fi
if [[ ! -f "$controls_matrix" ]]; then
  echo "[parity] missing controls matrix: $controls_matrix" >&2
  exit 1
fi

# Extract stable IDs (THR-<number>)
mapfile -t thr_ids < <(grep -oE 'THR-[0-9]+' "$threat_model" | sort -u)
mapfile -t cm_ids < <(grep -oE 'THR-[0-9]+' "$controls_matrix" | sort -u)

if [[ "${#thr_ids[@]}" -eq 0 ]]; then
  echo "[parity] no THR-* IDs found in threat model" >&2
  exit 1
fi

thr_tmp="$(mktemp)"
cm_tmp="$(mktemp)"
trap 'rm -f "$thr_tmp" "$cm_tmp"' EXIT

printf "%s\n" "${thr_ids[@]}" > "$thr_tmp"
printf "%s\n" "${cm_ids[@]}" > "$cm_tmp"

missing_in_matrix=$(comm -23 "$thr_tmp" "$cm_tmp" || true)
missing_in_model=$(comm -13 "$thr_tmp" "$cm_tmp" || true)

status=0
if [[ -n "$missing_in_matrix" ]]; then
  echo "[parity] FAIL: threat IDs missing in controls matrix:" >&2
  echo "$missing_in_matrix" >&2
  status=1
fi

if [[ -n "$missing_in_model" ]]; then
  echo "[parity] FAIL: threat IDs referenced in controls matrix but missing in threat model:" >&2
  echo "$missing_in_model" >&2
  status=1
fi

if [[ "$status" -ne 0 ]]; then
  exit 1
fi

echo "[parity] OK ($(wc -l < "$thr_tmp" | tr -d ' ') threat IDs)"
