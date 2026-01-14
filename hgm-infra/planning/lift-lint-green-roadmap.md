# Lift: Lint Green Roadmap (Rubric v0.8)

Goal: keep/get to a green `make lint` pass using the repo’s strict lint configuration, **without** weakening thresholds or
adding blanket exclusions.

This exists as a standalone roadmap because lint issues often require large, mechanical change sets that should be kept
reviewable and should not block unrelated remediation work (coverage/security/etc).

## Baseline (start of remediation)
Snapshot (2026-01-14):
- Primary command: `make lint`
- Current status: UNKNOWN until evidence is generated (run the verifier)
- Expected tools:
  - golangci-lint pinned in CI: `v2.4.0` (see `.github/workflows/test.yml`)
  - `.golangci.yml` config version: `"2"`

## Guardrails (no “green by dilution”)
- Do not add blanket excludes (directory-wide or linter-wide) unless the scope is demonstrably out-of-signal.
- Prefer line-scoped suppressions with justification over disablements.
- Keep tool versions pinned (no `latest`).
- Keep formatter checks enabled so “fixes” don’t drift into style churn.

## Milestones (small, reviewable change sets)

### LINT-1 — Hygiene and mechanical fixes
Done when:
- `make lint` issue count drops meaningfully without changing linter policy.

### LINT-2 — Low-risk rule families (API-safe)
Done when:
- Dominant “mechanical” linter families are cleared.

### LINT-3 — Correctness and error handling
Done when:
- Ignored-error findings are eliminated or narrowly justified.

### LINT-4 — Finish line
Done when:
- `make lint` is green (0 issues) under the strict config.

## Helpful commands
```bash
make lint
```
