# Lift: Lint Green Roadmap (Rubric v0.1.0)

Goal: get to a green `golangci-lint run --config .golangci.yml ./...` pass using the repo’s strict lint configuration,
**without** weakening thresholds or adding blanket exclusions.

This exists as a standalone roadmap because lint issues often require large, mechanical change sets that should be kept
reviewable and should not block unrelated remediation work (coverage/security/etc).

## Why this is a dedicated roadmap
- A failing linter blocks claiming CON-* and often blocks later work (tests/coverage work tends to generate lint debt).
- “Green by dilution” (disabling rules, widening excludes) is not an acceptable solution.

## Baseline (start of remediation)
Snapshot (2026-01-13):
- Primary command: `golangci-lint run --config .golangci.yml ./...`
- CI pin: `golangci/golangci-lint-action` version `v2.4.0` (from `.github/workflows/test.yml`)
- Current status: TBD (run the verifier and record issue counts)
- Top failure sources: TBD

How to capture baseline evidence:
```bash
bash ./hgm-infra/verifiers/hgm-verify-rubric.sh
# See hgm-infra/evidence/CON-2-output.log
```

## Progress snapshots
- Baseline (2026-01-13): TBD
- After M1-CON-1 (2026-01-13): CON-1 (gofmt/formatter clean) is now PASS. Applied gofmt to 57 test files. Evidence: hgm-infra/evidence/CON-1-output.log shows "gofmt clean".
- After LINT-1 (DATE): TBD
- After LINT-2 (DATE): TBD

## Guardrails (no “green by dilution”)
- Do not add blanket excludes (directory-wide or linter-wide) unless the scope is demonstrably out-of-signal.
- Prefer line-scoped suppressions with justification over disablements.
- Keep tool versions pinned (no `latest`) and keep config schema-valid (`golangci-lint config verify`).
- Keep formatter checks enabled so “fixes” don’t drift into style churn.

## Milestones (small, reviewable change sets)

### LINT-1 — Hygiene and mechanical fixes
Focus: reduce noise fast with low behavior risk.

Examples:
- Fix formatting/import ordering.
- Fix typos, dead code, unused code.
- Remove/replace stale suppressions.

Done when:
- `golangci-lint run --config .golangci.yml ./...` issue count drops meaningfully without changing linter policy.

### LINT-2 — Low-risk rule families (API-safe)
Focus: rules that are typically mechanical.

Examples:
- Unused parameter renames to `_` / `_unused`.
- Simplify repetitive patterns flagged by the linter.

Done when:
- The dominant “mechanical” linter families are cleared.

### LINT-3 — Correctness and error handling
Focus: stop ignoring errors and restore durable invariants.

Done when:
- “Ignored error” findings are eliminated or narrowly justified.

### LINT-4 — Refactors for duplication and complexity
Focus: highest behavior risk; do last.

Done when:
- Lint is green (0 issues) under the strict config.

## Helpful commands
```bash
golangci-lint --version
golangci-lint config verify --config .golangci.yml
golangci-lint run --config .golangci.yml ./...
```
