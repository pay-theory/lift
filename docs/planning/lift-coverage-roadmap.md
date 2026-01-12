# Lift: Coverage Roadmap (to 90%) (Rubric v0.1)

Goal: raise and maintain meaningful **core coverage ≥ 90%** as measured by:
- `bash scripts/hgm/coverage-threshold-check.sh`

“Core coverage” here is the repository’s own definition (currently: excludes `pkg/testing/**` via
`bash scripts/coverage-core-report.sh`).

## Prerequisites
- Lint remains green or is actively tracked (see `docs/planning/lift-lint-green-roadmap.md`).
- The coverage verifier is deterministic (no flakiness, stable package selection, stable excludes).

## Current state
Snapshot (2026-01-12):
- Gate command: `bash scripts/hgm/coverage-threshold-check.sh`
- Current result: **unknown** (run the command; hgm.init does not execute verifiers)
- Measurement surface: defined by the Makefile target `make test-coverage-core`.

## Guardrails (no denominator games)
- Do not exclude additional production packages/directories to “hit 90%”.
- Do not move logic into excluded directories to improve the numerator.
- If excluding code is required (e.g., generated code), document it explicitly and treat it as a rubric change.

## How to measure (canonical)
```bash
# Generates coverage.out and coverage.core.out, prints core totals
bash scripts/hgm/coverage-threshold-check.sh

# If debugging coverage composition:
make test-coverage-core
bash scripts/coverage-core-report.sh coverage.out coverage.core.out
```

## Milestones (incremental)
- **COV-1:** remove “0% islands” (every in-scope package has at least one test)
- **COV-2:** broad minimum floor (25%+ across in-scope packages)
- **COV-3:** meaningful safety net (50%+)
- **COV-4:** high confidence (70%+)
- **COV-5:** pre-finish (80%+)
- **COV-6:** finish line (≥ 90% and the gate is enforced in protected-branch CI)

## High-leverage targets (suggested)
Prioritize packages that are:
- on the hot path for request parsing/validation
- responsible for context propagation (tenant/user IDs)
- responsible for logging/observability
- adapters (event source normalization)

Coverage should emphasize:
- error paths (parsing failures, invalid inputs, timeouts)
- boundary conditions (empty values, max sizes, unexpected event shapes)
- security invariants (no raw payload logging; tenant separation)
