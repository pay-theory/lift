# Lift Maintainability Roadmap (Rubric v0.1.0)

Purpose
Maintain a convergent codebase by preventing AI-driven duplication/entropy and keeping core packages reviewable and predictable.

## Scope
- In scope: `pkg/**`, `cmd/**`
- Out of scope for now: `examples/**`, `docs/**`, `hgm-infra/**`

## Current maintainability controls
- MAI-1: File-size/complexity budget enforced by the verifier (max 1500 lines per non-test Go file).
- MAI-3: Canonical semantics / duplication checks enforced by the verifier.

## Known hotspots / debt list
- MAI-1 refactor: split oversized files in `pkg/cli`, `pkg/lift`, and `pkg/testing` to meet the 1500-line budget.

## MAI-3 duplication policy (v0.1.0 implementation)
- Scope: Go files in `pkg/**` and `cmd/**`
- Ignore: `*_test.go`
- Detection: exact duplicate file bodies after stripping comments and whitespace via token scanning
- Output: stable sha256 fingerprint per duplicate group with sorted file paths
- Exit codes: 0 when no duplicates, 1 when duplicates are found

## Next work items
- MAI-3-REM-1: If duplicates are detected, plan a targeted dedupe pass by package boundary.

## How to refresh/measure
- Run: `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh`
- Evidence:
  - `hgm-infra/evidence/MAI-1-output.log`
  - `hgm-infra/evidence/MAI-2-output.log`
  - `hgm-infra/evidence/MAI-3-output.log`
