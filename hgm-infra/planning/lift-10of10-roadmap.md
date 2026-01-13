# Lift: 10/10 Roadmap (Rubric v0.1.0)

This roadmap maps milestones directly to rubric IDs with measurable acceptance criteria and verification commands.

## Current scorecard (Rubric v0.1.0)
Scoring note: a check is only treated as “passing” if it is both green **and** enforced by a trustworthy verifier
(pinned tooling, schema-valid configs, and no “green by dilution” shortcuts). Completeness failures invalidate “green by
drift”.

Status note: this is an initial scaffold. Treat category grades as **TBD** until `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh`
has been run in the target CI environment and evidence has been captured.

| Category | Grade | Blocking rubric items |
| --- | ---: | --- |
| Quality | TBD | QUA-2, QUA-3 |
| Consistency | TBD | CON-2, CON-3 |
| Completeness | TBD | COM-3, COM-4, COM-6 |
| Security | TBD | SEC-2, SEC-3, SEC-4 |
| Compliance Readiness | 10/10 (docs exist once merged) | — |
| Maintainability | TBD | MAI-2, MAI-3 |
| Docs | TBD | DOC-4, DOC-5 |

Evidence (refresh whenever behavior changes):
- `make test`
- `make test-coverage`
- `golangci-lint run --config .golangci.yml ./...`
- `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh`

## Rubric-to-milestone mapping
| Rubric ID | Status | Milestone |
| --- | --- | --- |
| QUA-1 | TBD | M2 — Enforce in CI |
| QUA-2 | BLOCKED | M3 — Add contract/integration tests |
| QUA-3 | TBD | M1.5 — Coverage/quality gates |
| CON-1 | TBD | M1 — Make core lint/build loop reproducible |
| CON-2 | TBD | M1 — Make core lint/build loop reproducible |
| CON-3 | BLOCKED | M3 — Add contract parity checks |
| COM-1 | TBD | M2 — Enforce in CI |
| COM-2 | TBD | M2 — Enforce in CI |
| COM-3 | BLOCKED (until golangci-lint binary is available to validate config) | M1 — Make core lint/build loop reproducible |
| COM-4 | TBD | M1.5 — Coverage/quality gates |
| COM-5 | TBD | M2 — Enforce in CI |
| COM-6 | BLOCKED | M3+ — Operational hardening |
| SEC-1 | TBD | M2 — Enforce in CI |
| SEC-2 | BLOCKED | M3+ — Security hardening (dependency scanning) |
| SEC-3 | BLOCKED | M3+ — Supply-chain & release hardening |
| SEC-4 | BLOCKED | M3+ — Domain P0 tests (auth/CHD env invariants) |
| CMP-1 | PASS (once merged) | M0 — Freeze rubric + planning artifacts |
| CMP-2 | PASS (once merged) | M0 — Freeze rubric + planning artifacts |
| CMP-3 | PASS (once merged) | M0 — Freeze rubric + planning artifacts |
| MAI-1 | TBD | M3+ — Maintainability budgets |
| MAI-2 | BLOCKED | M3+ — Maintainability convergence |
| MAI-3 | BLOCKED | M3+ — Canonical semantics (duplication control) |
| DOC-1 | PASS (once merged) | M0 — Freeze rubric + planning artifacts |
| DOC-2 | PASS (once merged) | M0 — Freeze rubric + planning artifacts |
| DOC-3 | PASS (once merged) | M0 — Freeze rubric + planning artifacts |
| DOC-4 | TBD | M0 — Freeze rubric + planning artifacts |
| DOC-5 | TBD | M0 — Freeze rubric + planning artifacts |

## Workstream tracking docs (when blockers require a dedicated plan)
- Lint remediation: `hgm-infra/planning/lift-lint-green-roadmap.md`
- Coverage remediation: `hgm-infra/planning/lift-coverage-roadmap.md`
- Other blocker workstreams: `hgm-infra/planning/lift-workstream-<name>-roadmap.md`

## Milestones (sequenced)
### M0 — Freeze rubric + planning artifacts
**Closes:** CMP-1, CMP-2, CMP-3, DOC-1, DOC-2, DOC-3

**Goal:** prevent goalpost drift by making the definition of “good” explicit and versioned.

**Acceptance criteria**
- Rubric exists and is versioned.
- Threat model exists and is owned.
- Evidence plan maps rubric IDs → verifiers → artifacts.

### M1 — Make core lint/build loop reproducible
**Closes:** CON-1, CON-2, COM-3

**Goal:** strict lint/format enforcement with pinned tools; no drift.

Tracking document: `hgm-infra/planning/lift-lint-green-roadmap.md`

**Acceptance criteria**
- Formatter check fails on diffs.
- Lint is green with schema-valid config (no silent skips).
- Tool versions are pinned (no `@latest`).

### M1.5 — Coverage/quality gates
**Closes:** QUA-3, COM-4

**Goal:** reach coverage floor (≥ 90%) without reducing scope.

Tracking document: `hgm-infra/planning/lift-coverage-roadmap.md`

### M2 — Enforce in CI
**Closes:** QUA-1, COM-1, COM-2, COM-5, SEC-1, DOC-4, DOC-5

**Goal:** run the rubric surface in CI with pinned tooling; upload artifacts.

### M3+ — Domain/feature hardening
Add domain-specific milestones, such as:
- Auth/CHD environment P0 tests (SEC-4)
- govulncheck in CI (SEC-2)
- Supply-chain / release verification (SEC-3)
- Contract parity tests for CLI/template contract (CON-3)
- Logging and redaction standards (COM-6)
- Maintainability convergence plan (MAI-2, MAI-3)
