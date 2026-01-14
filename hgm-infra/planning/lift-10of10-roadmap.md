# Lift: 10/10 Roadmap (Rubric v0.3)

This roadmap maps milestones directly to rubric IDs with measurable acceptance criteria and verification commands.

## Current scorecard (Rubric v0.3)

Scoring note: a check is only treated as “passing” if it is both green and enforced by a trustworthy verifier
(pinned tooling, schema-valid configs, and no “green by dilution” shortcuts). Completeness failures invalidate “green by
drift”.

Because `hgm.init` does not execute verifiers, grades below are *expected to be provisional* until
`./hgm-infra/verifiers/hgm-verify-rubric.sh` is run and evidence is captured.

Latest evidence: `hgm-infra/evidence/hgm-rubric-report.json`

| Category | Grade | Blocking rubric items |
| --- | ---: | --- |
| Quality | 10/10 | — |
| Consistency | 10/10 | — |
| Completeness | 10/10 | — |
| Security | BLOCKED | SEC-2 |
| Compliance Readiness | 10/10 | — |
| Maintainability | BLOCKED | MAI-1, MAI-2, MAI-3 |
| Docs | 10/10 | — |

Evidence commands (refresh whenever behavior changes):
- `./scripts/ci-check.sh`
- `go test -tags=contract -count=1 ./pkg/contract/...`
- `./hgm-infra/verifiers/hgm-verify-rubric.sh` (coverage, fmt, pins, parity)
- `make lint`
- `go build ./...`
- TODO: govulncheck

## Rubric-to-milestone mapping

| Rubric ID | Status | Milestone |
| --- | --- | --- |
| QUA-1 | DONE | M1 (core loop) |
| QUA-2 | DONE | M2 (contract/integration tests) |
| QUA-3 | DONE | M3 (coverage to 90%) |
| CON-1 | DONE | M1 (core loop) |
| CON-2 | DONE | M1 (core loop) |
| CON-3 | DONE | M2 (contract tests) |
| COM-1 | DONE | M1 (core loop) |
| COM-2 | DONE | M1 (core loop) |
| COM-3 | DONE | M1 (core loop) |
| COM-4 | DONE | M3 (coverage to 90%) |
| COM-5 | DONE | M1 (core loop) |
| COM-6 | DONE | M4 (operational/logging standards) |
| SEC-1 | DONE | M1.5 (security gates) |
| SEC-2 | BLOCKED | M1.5 (security gates) |
| SEC-3 | DONE | M1.5 (security gates) |
| SEC-4 | DONE | M4 (P0 regressions) |
| CMP-1 | DONE | M0 (planning scaffold) |
| CMP-2 | DONE | M0 (planning scaffold) |
| CMP-3 | DONE | M0 (planning scaffold) |
| MAI-1 | BLOCKED | M5 (maintainability budgets) |
| MAI-2 | BLOCKED | M5 (maintainability budgets) |
| MAI-3 | BLOCKED | M5 (maintainability budgets) |
| DOC-1 | DONE | M0 (planning scaffold) |
| DOC-2 | DONE | M0 (planning scaffold) |
| DOC-3 | DONE | M0 (planning scaffold) |
| DOC-4 | DONE | M0 (planning scaffold) |
| DOC-5 | DONE | M0 (planning scaffold) |

## Workstream tracking docs (recommended)
- Lint remediation (if needed): `hgm-infra/planning/lift-lint-green-roadmap.md`
- Coverage remediation: `hgm-infra/planning/lift-coverage-roadmap.md`
- Other workstreams: `hgm-infra/planning/lift-workstream-<name>-roadmap.md`

## Milestones (sequenced)

### M0 — Freeze rubric + planning artifacts
**Closes:** CMP-1, CMP-2, CMP-3, DOC-1..DOC-5
**Status:** DONE

**Acceptance criteria**
- Rubric exists and is versioned.
- Threat model exists and has stable `THR-*` IDs.
- Controls matrix maps `THR-*` IDs to control IDs and verifiers.
- Evidence plan maps rubric IDs → verifiers → evidence paths under `hgm-infra/`.

### M1 — Make core lint/build loop reproducible
**Closes:** QUA-1, CON-1, CON-2, COM-1, COM-2, COM-3, COM-5, SEC-1, SEC-3
**Status:** DONE

**Acceptance criteria**
- `make lint` passes with pinned golangci-lint (CI uses `v2.4.0`).
- Formatter check is deterministic.
- Toolchain pin checks are deterministic.
- SAST (gosec) runs with pinned tooling.

### M2 — Public API contract parity + integration tests
**Closes:** QUA-2, CON-3
**Status:** DONE

**Acceptance criteria**
- Add deterministic tests that lock down public behavior and prevent silent semantic drift.
- Tests cover: JWT middleware behavior, error/response shaping, middleware ordering, tenant isolation invariants, and
  WebSocket routing.

### M3 — Coverage to 90% (no denominator games)
**Closes:** QUA-3, COM-4
**Status:** DONE

Tracking document: `hgm-infra/planning/lift-coverage-roadmap.md`

**Acceptance criteria**
- Coverage measured by the rubric verifier is ≥ 90% (core coverage).
- Coverage command writes artifacts under `hgm-infra/evidence/`.

### M4 — Operational/logging standards + domain P0 regressions
**Closes:** COM-6, SEC-4
**Status:** DONE

**Acceptance criteria**
- Add verifiers/tests asserting that sensitive inputs are not logged, secrets are handled safely, and defaults are
  secure.

### M5 — Maintainability budgets (AI drift resilience)
**Closes:** MAI-1, MAI-2, MAI-3

**Acceptance criteria**
- Add file-size/complexity budgets.
- Add duplicate-semantics checks for key public helpers.
- Keep a maintainability roadmap updated as code evolves.
