#!/usr/bin/env bash
set -euo pipefail

echo "[vuln-scan] BLOCKED: no pinned vulnerability scanner configured yet" >&2

echo "Recommended: pin govulncheck version and run it against ./..." >&2

echo "Example (pin required): go run golang.org/x/vuln/cmd/govulncheck@<version> ./..." >&2

exit 2
