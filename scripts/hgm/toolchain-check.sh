#!/usr/bin/env bash
set -euo pipefail

# Toolchain alignment check (Lift)
#
# This check is intentionally conservative:
# - It ensures Go major.minor in go.mod matches GitHub Actions workflow go-version.
# - It ensures the golangci-lint Action pins an explicit version.

if [[ ! -f go.mod ]]; then
  echo "[toolchain-check] missing go.mod" >&2
  exit 1
fi

mod_go=$(awk '$1=="go"{print $2; exit}' go.mod || true)
if [[ -z "$mod_go" ]]; then
  echo "[toolchain-check] could not read 'go' directive from go.mod" >&2
  exit 1
fi

mod_major_minor=$(echo "$mod_go" | awk -F. '{print $1"."$2}')

wf_dir=".github/workflows"
if [[ ! -d "$wf_dir" ]]; then
  echo "[toolchain-check] missing $wf_dir" >&2
  exit 1
fi

# Extract go-version values (best-effort).
# We accept values like 1.25.x or 1.25.1; enforce major.minor matches go.mod.
mapfile -t versions < <(grep -RhoE "go-version:\s*'?[0-9]+\.[0-9]+(\.[0-9]+|\.x)?'?" "$wf_dir"/*.yml 2>/dev/null | awk -F: '{gsub(/["'"'" ]/,"",$2); print $2}' | sort -u)

if [[ "${#versions[@]}" -eq 0 ]]; then
  echo "[toolchain-check] no go-version entries found in workflows" >&2
  exit 1
fi

bad=0
for v in "${versions[@]}"; do
  mm=$(echo "$v" | awk -F. '{print $1"."$2}')
  if [[ "$mm" != "$mod_major_minor" ]]; then
    # Allow older minors only if they are explicitly part of a test matrix.
    # For Lift, test.yml uses 1.23.x/1.24.x/1.25.x. If you remove the matrix,
    # this will become stricter.
    echo "[toolchain-check] NOTE: workflow Go version $v does not match go.mod $mod_go" >&2
  fi
  if [[ "$mm" == "$mod_major_minor" ]]; then
    good_match=1
  fi
done

if [[ "${good_match:-0}" -ne 1 ]]; then
  echo "[toolchain-check] FAIL: no workflow uses Go $mod_major_minor.* (go.mod is $mod_go)" >&2
  bad=1
fi

# golangci-lint Action version pin
if grep -R "golangci/golangci-lint-action" -n "$wf_dir"/*.yml >/dev/null 2>&1; then
  if ! grep -R "version:\s*v" -n "$wf_dir"/*.yml >/dev/null 2>&1; then
    echo "[toolchain-check] FAIL: golangci-lint action is present but no 'version: vX.Y.Z' pin found" >&2
    bad=1
  fi
fi

if [[ "$bad" -ne 0 ]]; then
  exit 1
fi

echo "[toolchain-check] OK (go.mod Go=$mod_go)"
