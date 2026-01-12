#!/usr/bin/env bash
set -euo pipefail

echo "[p0-check] BLOCKED: no P0 regression test suite defined yet" >&2

echo "Examples of P0 invariants for Lift:" >&2

echo "- no raw request/response payloads in logs" >&2

echo "- tenant isolation invariants hold across middleware/context" >&2

echo "- size limits and timeouts are enforced" >&2

exit 2
