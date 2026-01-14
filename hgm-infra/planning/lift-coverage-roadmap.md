# Lift: Coverage Roadmap (to 90%) (Rubric v0.4)

Goal: raise and maintain meaningful coverage to **≥ 90%** as measured by the rubric verifier coverage command, without
reducing the measurement surface.

This exists as a standalone roadmap because coverage improvements are usually multi-PR efforts that need clear
intermediate milestones, guardrails, and repeatable measurement.

## Prerequisites
- Lint stays green (or has a dedicated lint roadmap) so coverage work does not accumulate unreviewed lint debt.
- The coverage verifier is deterministic and uses a stable threshold (90%).

## Current state
Snapshot (2026-01-14):
- Coverage gate:
  - `./hgm-infra/verifiers/hgm-verify-rubric.sh` (QUA-3/COM-4)
- Current result: PASS (core_coverage=90.0%, threshold=90%; see `hgm-infra/evidence/COM-4-output.log`)
- Measurement surface:
  - Includes: `go list ./...` excluding `github.com/pay-theory/lift/examples/...`
  - Reports: “core” coverage excluding `github.com/pay-theory/lift/pkg/testing/**` (via `scripts/coverage-core-report.sh`)

## Guardrails (no denominator games)
- Do not exclude additional production code from the denominator.
- Do not move production logic into excluded areas (examples/tests/generated) to claim progress.
- If a package is legitimately out-of-signal, document the justification and make it explicit as a rubric change
  (version bump required).

## How we measure
1) Run the rubric verifier:
   - `./hgm-infra/verifiers/hgm-verify-rubric.sh`
2) Inspect artifacts:
   - `hgm-infra/evidence/coverage.out`
   - `hgm-infra/evidence/coverage.core.out`
   - `hgm-infra/evidence/QUA-3-output.log`
   - `hgm-infra/evidence/COM-4-output.log`

## Proposed milestones (incremental, reviewable)
- COV-1: eliminate “0% islands” — every in-scope package has at least one meaningful unit test.
- COV-2: reach 50% core coverage.
- COV-3: reach 70% core coverage.
- COV-4: reach 80% core coverage.
- COV-5: reach 90% core coverage and keep the gate green.

## High-leverage workstreams (Lift-specific)
Prioritize tests for:
- Request parsing + validation boundaries.
- Middleware ordering and short-circuit behavior.
- JWT auth behavior (HS/RS, algorithm restrictions, token extraction).
- Tenant isolation helpers and error shaping.
- WebSocket routing/handler selection.
- Guardrails: request size, timeout, response size.

## Helpful commands
```bash
./hgm-infra/verifiers/hgm-verify-rubric.sh
```
