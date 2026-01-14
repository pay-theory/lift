# AI/Agent Drift: Failure Modes + Recovery (Lift)

Document common failure modes when using LLMs/agents and the guardrails to prevent “green by dilution.”

## Why this matters for Lift
Lift is used in security-critical production applications including authentication and cardholder data environments.
That makes "small" drift (like a loosened linter exclusion or a lowered coverage gate) disproportionately risky.

## Common failure modes
- **Green by exclusion:** making checks pass by shrinking scope (skipping packages, excluding directories, lowering
  coverage thresholds, or ignoring “hard” packages).
- **Toolchain drift:** CI runs different versions than local and/or the repo expectations (especially `golangci-lint`,
  Go version, and CDK/node tooling).
- **Scope dodging:** only validating “core” code while leaving secondary packages/examples in a broken state.
- **Evidence drift:** docs claim controls exist but no verifier/evidence path backs them.
- **Metadata-only tags/flags:** security affordances (e.g., “redact this field”) without enforced semantics.
- **Public boundary drift:** exported helpers and middleware behavior changes without contract tests.
- **Maintainability erosion:** large files and duplicated implementations that make security fixes risky.

## Guardrails (what to enforce)
- Use the versioned rubric (`hgm-infra/planning/lift-10of10-rubric.md`) as the source of truth; bump version on any
  rubric change.
- Keep Completeness/anti-drift checks in CI:
  - toolchain pin checks (Go version and pinned lint versions),
  - lint config schema validation and “no high-signal excludes”,
  - coverage threshold floors (90% for Lift),
  - parity checks (threat model ↔ controls matrix).
- Prefer narrow suppressions (`//nolint:<rule> // justification`, `#nosec`) with justification; avoid blanket excludes.
- Do not treat TODO verifiers as passing: missing verifiers are **BLOCKED**.

## Recovery playbook
1. Run the full rubric surface:
   - `./hgm-infra/verifiers/hgm-verify-rubric.sh`
2. Review the report:
   - `hgm-infra/evidence/hgm-rubric-report.json`
3. Fix failing gates before reducing scope.
   - If scope must shrink (rare), time-box it and document it explicitly in the roadmap with a rollback plan.
4. If a verifier is wrong/flaky, fix the verifier (not the rubric requirement), and bump rubric version if the
   requirement changes.
5. Refresh planning docs when controls move:
   - controls matrix, threat model, roadmap, and evidence plan must stay consistent.

## Turning discoveries into durable gates
When a new class of failure is discovered (security, quality, compliance readiness, or AI drift), treat it as
candidate rubric surface, not a one-off note:
1) Propose a new verifier (what it checks, why it matters, how it avoids false-green).
2) Implement the verifier as a command.
3) Adopt it by bumping rubric version (if needed), wiring into CI, and mapping to evidence.
