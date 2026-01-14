# Lift: 10/10 Rubric (Quality, Consistency, Completeness, Security, Compliance Readiness, Maintainability, Docs)

This rubric defines what “10/10” means and how category grades are computed. It is designed to prevent goalpost drift and
“green by dilution” by making scoring versioned, measurable, and repeatable.

## Versioning (no moving goalposts)
- **Rubric version:** `v0.4` (2026-01-14)
- **Comparability rule:** grades are comparable only within the same version.
- **Change rule:** bump the version + changelog entry for any rubric change (what changed + why).

### Changelog
- `v0.1`: Initial rubric scaffold for Lift.
- `v0.2`: Define and verify contract test surface for QUA-2 + CON-3.
- `v0.3`: Define verifiers for COM-6 (ops/logging standards) and SEC-4 (domain P0 regressions).
- `v0.4`: Define verifiers for MAI-1..MAI-3 (budgets, roadmap, duplicate code gate).

## Scoring (deterministic)
- Each category is scored 0–10.
- Point weights sum to 10 per category.
- Requirements are pass/fail (either earn full points or 0).
- A category is 10/10 only if all requirements in that category pass.

## Verification (commands + deterministic artifacts are the source of truth)
Every rubric item has exactly one verification mechanism:
- a command (`make ...`, `go test ...`, `bash ...`), or
- a deterministic artifact check (required doc exists and matches an agreed format).

Enforcement rule (anti-drift):
- If an item’s verifier is a command/script, it only counts as passing once it runs and produces evidence.
- Missing verifiers are **BLOCKED** (never treated as green).

---

## Quality (QUA) — reliable, testable, change-friendly

| ID | Points | Requirement | How to verify |
| --- | ---: | --- | --- |
| QUA-1 | 4 | Unit tests stay green | `./scripts/ci-check.sh` |
| QUA-2 | 3 | Integration or contract tests stay green | `go test -tags=contract -count=1 ./pkg/contract/...` |
| QUA-3 | 3 | Coverage ≥ 90% (no denominator games) | `./hgm-infra/verifiers/hgm-verify-rubric.sh` (QUA-3/COM-4) |

**10/10 definition:** QUA-1 through QUA-3 pass.

## Consistency (CON) — one way to do the important things

| ID | Points | Requirement | How to verify |
| --- | ---: | --- | --- |
| CON-1 | 3 | gofmt/formatter clean (no diffs) | `./hgm-infra/verifiers/hgm-verify-rubric.sh` (CON-1) |
| CON-2 | 5 | Lint/static analysis green (pinned version) | `make lint` (uses `.golangci.yml`; CI pins golangci-lint `v2.4.0`) |
| CON-3 | 2 | Public boundary contract parity (if applicable) | `go test -tags=contract -count=1 ./pkg/contract/...` |

**10/10 definition:** CON-1 through CON-3 pass.

## Completeness (COM) — verify the verifiers (anti-drift)

| ID | Points | Requirement | How to verify |
| --- | ---: | --- | --- |
| COM-1 | 2 | All modules compile (no “mystery meat”) | `go build ./...` |
| COM-2 | 2 | Toolchain pins align to repo (Go/lint/tool versions) | verifier COM-2 (checks `go 1.25` + CI uses `1.25.x` + golangci-lint pin) |
| COM-3 | 2 | Lint config schema-valid (no silent skip) | verifier COM-3 (checks `.golangci.yml` present + version + key settings) |
| COM-4 | 2 | Coverage threshold not diluted (≥ 90%) | verifier COM-4 (parses `coverage.core.out`) |
| COM-5 | 1 | Security scan config not diluted (no excluded high-signal rules) | verifier COM-5 (fails if gosec excludes high-signal IDs like G101) |
| COM-6 | 1 | Logging/operational standards enforced (if applicable) | verifier COM-6 (runs `go test -count=1 -run '^TestOps_' ./pkg/observability/zap ./pkg/middleware`) |

**10/10 definition:** COM-1 through COM-6 pass.

## Security (SEC) — abuse-resilient and reviewable

| ID | Points | Requirement | How to verify |
| --- | ---: | --- | --- |
| SEC-1 | 3 | Static security scan green (pinned version) | verifier SEC-1 (gosec via pinned golangci-lint) |
| SEC-2 | 3 | Dependency vulnerability scan green | TODO: pin and run govulncheck |
| SEC-3 | 2 | Supply-chain verification green | verifier SEC-3 (action pins, go.sum present) |
| SEC-4 | 2 | Domain-specific P0 regression tests | verifier SEC-4 (runs `go test -count=1 -run '^TestP0_' ./pkg/observability/zap ./pkg/middleware`) |

**10/10 definition:** SEC-1 through SEC-4 pass.

## Compliance Readiness (CMP) — auditability and evidence

| ID | Points | Requirement | How to verify |
| --- | ---: | --- | --- |
| CMP-1 | 4 | Controls matrix exists and is current | file check: `hgm-infra/planning/lift-controls-matrix.md` |
| CMP-2 | 3 | Evidence plan exists and is reproducible | file check: `hgm-infra/planning/lift-evidence-plan.md` |
| CMP-3 | 3 | Threat model exists and is current | file check: `hgm-infra/planning/lift-threat-model.md` |

**10/10 definition:** CMP-1 through CMP-3 pass.

## Maintainability (MAI) — convergent codebase

| ID | Points | Requirement | How to verify |
| --- | ---: | --- | --- |
| MAI-1 | 4 | File-size/complexity budgets enforced | verifier MAI-1 (file size budget + complexity budget assertions) |
| MAI-2 | 3 | Maintainability roadmap current | verifier MAI-2 (requires `hgm-infra/planning/lift-maintainability-roadmap.md`) |
| MAI-3 | 3 | Canonical implementations (no duplicate semantics) | verifier MAI-3 (runs `golangci-lint` with `dupl` only) |

**10/10 definition:** MAI-1 through MAI-3 pass.

## Docs (DOC) — integrity and parity

| ID | Points | Requirement | How to verify |
| --- | ---: | --- | --- |
| DOC-1 | 2 | Threat model present | file check: `hgm-infra/planning/lift-threat-model.md` |
| DOC-2 | 2 | Evidence plan present | file check: `hgm-infra/planning/lift-evidence-plan.md` |
| DOC-3 | 2 | Rubric + roadmap present | file check: `hgm-infra/planning/lift-10of10-rubric.md` and `.../lift-10of10-roadmap.md` |
| DOC-4 | 2 | Doc integrity (tokens, required files) | verifier DOC-4 |
| DOC-5 | 2 | Threat ↔ controls parity | verifier DOC-5 parity |

**10/10 definition:** DOC-1 through DOC-5 pass.

## Maintaining 10/10 (recommended CI surface)
Minimum command surface CI should run on protected branches (pins required; no `latest` tools):

```bash
./hgm-infra/verifiers/hgm-verify-rubric.sh
```

(That verifier internally runs the repo-specific commands declared in `hgm-infra/verifiers/hgm-verify-rubric.sh`.)
