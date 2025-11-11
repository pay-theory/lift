#!/usr/bin/env bash
set -euo pipefail

# GitHub runners lagging on covdata tooling make coverage unusable, so we fall
# back to a plain test sweep to keep the signal focused on correctness only.
go test ./...
