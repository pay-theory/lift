#!/usr/bin/env bash
set -euo pipefail

# gofmt cleanliness check

# Exclude local caches and module caches from formatting checks.
# NOTE: This is a best-effort filter; if you add other generated directories,
# add them here explicitly rather than weakening the check.

mapfile -t files < <(
  find . -type f -name '*.go' \
    -not -path './.git/*' \
    -not -path './.gocache/*' \
    -not -path './.gomodcache/*' \
    -not -path './.golangci-lint-cache/*' \
    -not -path './bin/*' \
    -print
)

if [[ "${#files[@]}" -eq 0 ]]; then
  echo "[fmt-check] no go files found" >&2
  exit 1
fi

out=$(gofmt -l "${files[@]}" || true)
if [[ -n "$out" ]]; then
  echo "[fmt-check] FAIL: gofmt would modify:" >&2
  echo "$out" >&2
  echo >&2
  echo "Fix with: gofmt -w <files>" >&2
  exit 1
fi

echo "[fmt-check] OK"
