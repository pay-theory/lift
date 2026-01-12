# Lift: 10/10 Roadmap (Rubric v0.1)

This roadmap ties execution milestones directly to rubric IDs. It is intentionally mechanical: a milestone is “done”
only when the corresponding verifier is deterministic and enforced.

## Baseline scorecard
This repository has CI, lint, and test workflows, but **hgm.init does not execute verifiers**. Treat this scorecard as a
planning baseline; use `hgm validate` to compute PASS/FAIL/BLOCKED with evidence.

| Category | Grade | Blocking rubric items (initial expectation) |
| --- | ---: | --- |
| Quality | TBD | QUA-3 (coverage gate not enforced) |
| Consistency | TBD | CON-1 (explicit fmt check), CON-3 (contract parity) |
| Completeness | TBD | COM-1, COM-3, COM-4, COM-6 |
| Security | TBD | SEC-2, SEC-4 (and dedicated SEC-1 verifier) |
| Compliance Readiness | TBD | CI wiring for doc checks (CMP-* as enforcement, not existence) |
| Maintainability | TBD | MAI-3 (canonical semantics), plus budgets enforcement clarity |
| Docs | TBD | DOC-4 (integrity gate enforcement), DOC-5 (parity gate enforcement) |

## Evidence refresh commands (canonical)
- `./scripts/ci-check.sh`
- `go test -v ./pkg/cdk/...`
- `bash scripts/hgm/fmt-check.sh`
- `golangci-lint run --timeout=10m --config .golangci.yml ./...`
- `bash scripts/hgm/toolchain-check.sh`
- `bash scripts/hgm/sec-config-check.sh`
- `bash scripts/hgm/supply-chain-check.sh`
- `bash scripts/hgm/coverage-threshold-check.sh`
- `bash scripts/hgm/planning-docs-check.sh`
- `bash scripts/hgm/doc-integrity-check.sh`
- `bash scripts/hgm/threat-controls-parity.sh`

## Rubric → milestone mapping

| Rubric ID | Status | Milestone |
| --- | --- | --- |
| DOC-1 | Implemented (doc exists) | M0 |
| DOC-2 | Implemented (doc exists) | M0 |
| DOC-3 | Implemented (doc exists) | M0 |
| COM-3 | Blocked (needs verifier) | M0 |
| CMP-1 | Implemented (doc exists) | M0 |
| CMP-2 | Implemented (doc exists) | M0 |
| CMP-3 | Implemented (doc exists) | M0 |
| DOC-4 | Partial (verifier exists; wire into CI) | M0 → M2 |
| DOC-5 | Partial (verifier exists; wire into CI) | M0 → M2 |
| QUA-1 | Enforced in CI | M1 |
| QUA-2 | Enforced in CI (cdk-test) | M1 |
| CON-2 | Enforced in CI (lint job; golangci-lint v2.4.0) | M1 |
| CON-1 | Partial (verifier exists; wire into CI) | M1 |
| COM-2 | Partial (toolchain checks exist; tighten pins) | M2 |
| COM-1 | Blocked (needs module sweep verifier) | M2 |
| COM-4 | Blocked (coverage gate not enforced) | M1.5 |
| QUA-3 | Blocked (coverage gate not enforced) | M1.5 |
| COM-5 | Partial (config check exists; needs CI enforcement) | M2 |
| SEC-1 | Partial (gosec via lint; dedicated verifier TODO) | M2 |
| SEC-2 | Blocked (no vuln scan gate yet) | M2 |
| SEC-3 | Partial (actions pinned; strengthen provenance/signing later) | M2 → M3 |
| SEC-4 | Blocked (P0 regression tests missing) | M3 |
| MAI-1 | Partial (lint enforces complexity; clarify budget command) | M3 |
| MAI-2 | Implemented (this roadmap exists) | M0 |
| MAI-3 | Blocked (canonical semantics verifier missing) | M3 |
| CON-3 | Blocked (contract parity tests missing) | M3 |
| COM-6 | Blocked (logging/ops standards verifier missing) | M3 |

## Workstream tracking docs
- Lint remediation: `docs/planning/lift-lint-green-roadmap.md`
- Coverage: `docs/planning/lift-coverage-roadmap.md`
- Dependency vulnerability scanning: `docs/planning/lift-workstream-dependency-vuln-scan-roadmap.md`
- Supply-chain hardening: `docs/planning/lift-workstream-supply-chain-roadmap.md`

## Milestones (sequenced)

### M0 — Freeze rubric + planning artifacts
**Goal:** make definitions explicit and versioned; prevent drift.

**Acceptance criteria**
- Required planning docs exist under `docs/planning/`.
- Threat IDs are stable and parity tooling exists.
- Evidence plan maps rubric IDs to commands/artifacts.

### M1 — Make the core correctness loop strict
**Closes (target):** QUA-1, QUA-2, CON-2.

**Acceptance criteria**
- Tests are green via `./scripts/ci-check.sh`.
- Lint is green via CI (`golangci-lint` pinned).

### M1.5 — Coverage gate to 90%
**Closes (target):** QUA-3, COM-4.

Tracking doc: `docs/planning/lift-coverage-roadmap.md`

**Acceptance criteria**
- `bash scripts/hgm/coverage-threshold-check.sh` is deterministic.
- Protected-branch CI runs the coverage gate and fails if below 90%.

### M2 — Anti-drift + security baseline in CI
**Closes (target):** COM-1..5, SEC-1..3, DOC-4..5.

**Acceptance criteria**
- Toolchain drift is detected (`toolchain-check.sh`).
- Security config drift is detected (`sec-config-check.sh`).
- Supply-chain checks are enforced (`supply-chain-check.sh`).
- Dependency vulnerability scanning is pinned and enforced.

### M3 — Security-critical hardening (P0s + contracts)
**Closes (target):** SEC-4, CON-3, COM-6, MAI-3.

**Acceptance criteria**
- “Must never happen” regressions are tested (sensitive log leakage, tenant isolation invariants).
- Contract parity exists for exported public APIs.
- Logging/operational invariants are enforced.
