# Lift: Maintainability Roadmap (Rubric v0.5)

Goal: prevent AI/agent-driven drift that makes security fixes risky by enforcing lightweight budgets and keeping a clear,
reviewable plan for paydown.

## How we measure
- `./hgm-infra/verifiers/hgm-verify-rubric.sh`
- Evidence:
  - `hgm-infra/evidence/MAI-1-output.log`
  - `hgm-infra/evidence/MAI-2-output.log`
  - `hgm-infra/evidence/MAI-3-output.log`

## Budgets
Hard gates (must stay green):
- File size budget (MAI-1): no single Go file exceeds `2500` lines (excludes `examples/` and `hgm-infra/`).
- Complexity budget (MAI-1): cyclomatic/cognitive budgets remain enabled in `.golangci.yml` (e.g., `gocyclo` with
  `min-complexity: 15`).

## Duplicate Semantics
Hard gate (must stay green):
- Duplicate code detector (MAI-3): `dupl` remains enabled and green via the rubric verifier.

## Current debt (tracked for paydown)
Large files are accepted temporarily but must not grow without an explicit refactor plan:
- `pkg/cli/dynamorm_commands.go` (split command groups and helpers)
- `pkg/lift/app.go` (split app wiring vs. runtime behavior)
