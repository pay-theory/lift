# Lift: Lint Green Roadmap (Rubric v0.1)

Goal: keep Lift’s lint surface green under the existing strict config **without** weakening the rules or widening
exclusions.

Primary command (as run in CI):
- `golangci-lint run --timeout=10m --config .golangci.yml ./...`

## Baseline (start of remediation)
Snapshot (2026-01-12):
- Current status: **unknown** (run the command locally or via CI logs; hgm.init does not execute verifiers)
- Tool pin in CI: golangci-lint `v2.4.0` (via `golangci/golangci-lint-action`)
- Config file: `.golangci.yml`

## Guardrails (no “green by dilution”)
- Do not add blanket excludes (directory-wide or repo-wide) to silence findings.
- Prefer narrow suppressions (`//nolint:<linter>`) with justification.
- Keep security-relevant linters enabled (at minimum: `gosec`, `staticcheck`, `errcheck`).
- If a linter is too noisy, propose a targeted rule adjustment + bump rubric version with rationale.

## Milestones
### LINT-1 — Mechanical / low-risk cleanup
Done when:
- Most issues are resolved via formatting/import cleanup and small mechanical edits.
- The change set does not modify exported API behavior.

### LINT-2 — Correctness and error handling
Focus:
- Address `errcheck` and correctness findings that could surface as runtime failures.

Done when:
- `errcheck` issues are eliminated or narrowly justified.

### LINT-3 — Security findings remediation
Focus:
- `gosec` findings: fix code or justify narrowly with `#nosec` + explanation.

Done when:
- No high-signal security findings remain.

### LINT-4 — Keep it durable
Done when:
- Lint is green in protected-branch CI on every PR.
- `.golangci.yml` changes are reviewed as control changes (anti-drift mindset).

## Helpful commands
```bash
golangci-lint run --timeout=10m --config .golangci.yml ./...
```
