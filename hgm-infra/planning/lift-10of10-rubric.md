# Lift: 10/10 Rubric (Quality, Consistency, Completeness, Security, Compliance Readiness, Maintainability, Docs)

This rubric defines what “10/10” means and how category grades are computed. It is designed to prevent goalpost drift and
“green by dilution” by making scoring **versioned, measurable, and repeatable**.

## Versioning (no moving goalposts)
- **Rubric version:** `v0.1.0` (2026-01-13)
- **Comparability rule:** grades are comparable only within the same version.
- **Change rule:** bump the version + changelog entry for any rubric change (what changed + why).

### Changelog
- `v0.1.0`: Initial rubric scaffold for Lift (custom domain) with strict anti-drift defaults.

## Scoring (deterministic)
- Each category is scored **0–10**.
- Point weights sum to **10** per category.
- Requirements are **pass/fail** (either earn full points or 0).
- A category is **10/10 only if all requirements in that category pass**.

## Verification (commands + deterministic artifacts are the source of truth)
Every rubric item has exactly one verification mechanism:
- a command (`make ...`, `go test ...`, `bash ...`), or
- a deterministic artifact check (required doc exists and matches an agreed format).

Enforcement rule (anti-drift):
- If an item’s verifier is a command/script, it only counts as passing once it runs in CI and produces evidence.

---

## Quality (QUA) — reliable, testable, change-friendly
| ID | Points | Requirement | How to verify |
| --- | ---: | --- | --- |
| QUA-1 | 4 | Unit tests stay green | `make test` |
| QUA-2 | 3 | Integration or contract tests stay green | `go test -tags=contract ./internal/contracts -count=1` |
| QUA-3 | 3 | Coverage ≥ 90% (no denominator games) | `make test-coverage` (evidence copied to `hgm-infra/evidence/coverage.out`) |

**10/10 definition:** QUA-1 through QUA-3 pass.

## Consistency (CON) — one way to do the important things
| ID | Points | Requirement | How to verify |
| --- | ---: | --- | --- |
| CON-1 | 3 | gofmt/formatter clean (no diffs) | `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh` (CON-1) |
| CON-2 | 5 | Lint/static analysis green (pinned version) | `golangci-lint run --config .golangci.yml ./...` |
| CON-3 | 2 | Public boundary contract parity (if applicable) | `go test -tags=contract ./internal/contracts -count=1` |

**10/10 definition:** CON-1 through CON-3 pass (or document why CON-3 is N/A and remove it with a version bump).

## Completeness (COM) — verify the verifiers (anti-drift)
| ID | Points | Requirement | How to verify |
| --- | ---: | --- | --- |
| COM-1 | 2 | All modules compile (no “mystery meat”) | `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh` (COM-1) |
| COM-2 | 2 | Toolchain pins align to repo (Go/lint/tool versions) | `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh` (COM-2) |
| COM-3 | 2 | Lint config schema-valid (no silent skip) | `golangci-lint config verify --config .golangci.yml` |
| COM-4 | 2 | Coverage threshold not diluted (≥ 90%) | `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh` (COM-4) |
| COM-5 | 1 | Security scan config not diluted (no excluded high-signal rules) | `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh` (COM-5) |
| COM-6 | 1 | Logging/operational standards enforced (if applicable) | `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh` (COM-6) |

**10/10 definition:** COM-1 through COM-6 pass.

## Security (SEC) — abuse-resilient and reviewable
| ID | Points | Requirement | How to verify |
| --- | ---: | --- | --- |
| SEC-1 | 3 | Static security scan green (pinned version) | `golangci-lint run --enable-only=gosec --config .golangci.yml ./...` |
| SEC-2 | 3 | Dependency vulnerability scan green | `TODO: add govulncheck in CI (currently BLOCKED)` |
| SEC-3 | 2 | Supply-chain verification green | `TODO: add supply-chain verification (currently BLOCKED)` |
| SEC-4 | 2 | Domain-specific P0 regression tests (auth/CHD env invariants) | `TODO: add P0 regression test suite (currently BLOCKED)` |

**10/10 definition:** SEC-1 through SEC-4 pass.

## Compliance Readiness (CMP) — auditability and evidence
| ID | Points | Requirement | How to verify |
| --- | ---: | --- | --- |
| CMP-1 | 4 | Controls matrix exists and is current | File exists: `hgm-infra/planning/lift-controls-matrix.md` |
| CMP-2 | 3 | Evidence plan exists and is reproducible | File exists: `hgm-infra/planning/lift-evidence-plan.md` |
| CMP-3 | 3 | Threat model exists and is current | File exists: `hgm-infra/planning/lift-threat-model.md` |

**10/10 definition:** CMP-1 through CMP-3 pass.

## Maintainability (MAI) — convergent codebase (recommended for AI-heavy repos)
| ID | Points | Requirement | How to verify |
| --- | ---: | --- | --- |
| MAI-1 | 4 | File-size/complexity budgets enforced | `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh` (MAI-1) |
| MAI-2 | 3 | Maintainability roadmap current | `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh` (MAI-2) |
| MAI-3 | 3 | Canonical implementations (no duplicate semantics) | `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh` (MAI-3) |

**10/10 definition:** MAI-1 through MAI-3 pass.

## Docs (DOC) — integrity and parity
| ID | Points | Requirement | How to verify |
| --- | ---: | --- | --- |
| DOC-1 | 2 | Threat model present | File exists: `hgm-infra/planning/lift-threat-model.md` |
| DOC-2 | 2 | Evidence plan present | File exists: `hgm-infra/planning/lift-evidence-plan.md` |
| DOC-3 | 2 | Rubric + roadmap present | Files exist under `hgm-infra/planning/` |
| DOC-4 | 2 | Doc integrity (links, version claims) | `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh` (DOC-4) |
| DOC-5 | 2 | Threat ↔ controls parity | `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh` (DOC-5) |

**10/10 definition:** DOC-1 through DOC-5 pass.

## Maintaining 10/10 (recommended CI surface)
The minimal command set protected branches should run (pinned tools only):

```bash
bash ./hgm-infra/verifiers/hgm-verify-rubric.sh
```

Notes:
- The verifier fails closed: missing tools/commands are **BLOCKED** (not green).
- When you change what “good” means, bump the rubric version and update `lift-10of10-roadmap.md`.
