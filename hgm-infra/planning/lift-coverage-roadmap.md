# Lift: Coverage Roadmap (to 90%) (Rubric v0.1.0)

Goal: raise and maintain meaningful coverage to **≥ 90%** as measured by `make test-coverage`, without reducing the
measurement surface.

This exists as a standalone roadmap because coverage improvements are usually multi-PR efforts that need clear
intermediate milestones, guardrails, and repeatable measurement.

## Prerequisites
- Lint is green (or has a dedicated lint roadmap) so coverage work does not accumulate unreviewed lint debt.
- The coverage verifier is deterministic and uses a stable default threshold (no “lower it to pass” override).

## Current state
Snapshot (2026-01-13):
- Coverage gate: `make test-coverage`
- Evidence file: `hgm-infra/evidence/coverage.out` (copied by `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh`)
- Current result: TBD
- Measurement surface: `make test-coverage` runs tests in packages detected under `pkg/` (per Makefile) and excludes
  examples; this is acceptable as long as exclusions are stable and documented.

How to capture baseline evidence:
```bash
bash ./hgm-infra/verifiers/hgm-verify-rubric.sh
# See hgm-infra/evidence/QUA-3-output.log and hgm-infra/evidence/coverage.out
```

## Progress snapshots
- Baseline (2026-01-13): TBD
- After COV-1 (DATE): TBD
- After COV-2 (DATE): TBD

## Guardrails (no denominator games)
- Do not exclude additional production code from the coverage denominator to “hit the number”.
- Do not move logic into excluded areas (examples/tests/generated) to claim progress.
- If you need package-level floors to drive incremental work, add explicit target-based verification rather than weakening the global gate.

## How we measure
Suggested flow:
1) Generate/refresh the coverage artifact with the canonical command:
   - `make test-coverage`
2) Re-run the full quality loop as a regression gate:
   - `make test`
   - `golangci-lint run --config .golangci.yml ./...`

## Proposed milestones (incremental, reviewable)
- COV-1: remove “0% islands” (every in-scope package has at least one test)
- COV-2: broad floor (25%+ across in-scope packages)
- COV-3: meaningful safety net (50%+)
- COV-4: high confidence (70%+)
- COV-5: pre-finish (80%+)
- COV-6: finish line (≥ 90% and gate is green)

## Workstreams (target the highest-leverage paths first)
- Hotspots: TBD (identify high churn / high surface area packages from commit history)
- Common gap patterns:
  - error paths
  - boundary validation
  - retries/timeouts
  - serialization/deserialization

## Helpful commands
```bash
make test
make test-coverage
# inspect locally
go tool cover -func=coverage.out
```
