#!/usr/bin/env bash
set -euo pipefail

# Lint config validity check.
#
# Goal: fail if the config is invalid for the installed golangci-lint.
# This is best-effort because config schema validation is tool-version-specific.

cfg=".golangci.yml"
if [[ ! -f "$cfg" ]]; then
  echo "[lint-config-check] missing $cfg" >&2
  exit 1
fi

if ! command -v golangci-lint >/dev/null 2>&1; then
  echo "[lint-config-check] BLOCKED: golangci-lint not installed" >&2
  echo "[lint-config-check] Install the pinned version used in CI (see .github/workflows/test.yml)." >&2
  exit 2
fi

# Try dedicated config verify subcommand (not guaranteed to exist across versions).
if golangci-lint help config >/dev/null 2>&1; then
  if golangci-lint config verify --config "$cfg" >/dev/null 2>&1; then
    echo "[lint-config-check] OK"
    exit 0
  fi
fi

# Fallback: attempt to parse config by running a no-op lint on an empty package set.
# If the version cannot do that, fail closed as BLOCKED (exit 2) rather than claiming green.
if golangci-lint run --config "$cfg" --help >/dev/null 2>&1; then
  echo "[lint-config-check] BLOCKED: no supported standalone config verification in this golangci-lint version" >&2
  exit 2
fi

echo "[lint-config-check] BLOCKED: unable to validate config with installed golangci-lint" >&2
exit 2
