# Lift: 10/10 Rubric

This rubric defines what “10/10” means for Lift in a way that is **versioned and measurable**, so improvements can be
tracked without moving the goalposts.

It is designed to prevent “green by dilution” (passing checks by excluding code, lowering thresholds, or disabling
high-signal rules).

## Versioning
- **Rubric version:** `v0.1` (2026-01-12)
- **Change rule:** if any rubric requirement, threshold, or verifier changes, bump the version and add a changelog entry.

### Changelog
- `v0.1`: initial rubric scaffold for Lift (custom domain overlay).

## Scoring model
- Each category is 0–10.
- Each requirement is pass/fail: either earn the full points listed or earn 0.
- A category is 10/10 only when **every** requirement in that category passes.

## Canonical verifier commands (pinned/wired to this repo)
Where possible, verifiers are commands that can be run from a clean checkout.

> Important: a verifier only counts as “passing” once it is enforced in protected-branch CI and has a reproducible
> evidence path.

### Commands
- Unit tests: `./scripts/ci-check.sh`
- Integration/contract tests (current CI surface):
  - `go test -v ./pkg/cdk/...`
  - `cd pkg/cdk/examples/basic-api && cdk synth --quiet` (requires Node 20 + aws-cdk@2)
- Coverage gate (core coverage ≥ 90%): `bash scripts/hgm/coverage-threshold-check.sh`
- Format check: `bash scripts/hgm/fmt-check.sh`
- Lint: `golangci-lint run --timeout=10m --config .golangci.yml ./...` (CI pins golangci-lint `v2.4.0`)
- Compile/module sweep (root + examples): `bash scripts/hgm/modules-check.sh` (TODO)
- Toolchain pin check: `bash scripts/hgm/toolchain-check.sh`
- Lint config validity: `bash scripts/hgm/lint-config-check.sh` (TODO)
- Coverage threshold anti-drift: `bash scripts/hgm/coverage-threshold-check.sh`
- Security config anti-drift: `bash scripts/hgm/sec-config-check.sh`
- SAST gate (dedicated): `bash scripts/hgm/sast-check.sh` (TODO; minimum is gosec)
- Dependency vuln scan: `bash scripts/hgm/vuln-scan.sh` (pinned via env var)
- Supply-chain checks: `bash scripts/hgm/supply-chain-check.sh`
- Planning docs present: `bash scripts/hgm/planning-docs-check.sh`
- Doc integrity: `bash scripts/hgm/doc-integrity-check.sh`
- Threat ↔ controls parity: `bash scripts/hgm/threat-controls-parity.sh`

---

## Quality (QUA)
| ID | Pts | Requirement | Verifier |
| --- | ---: | --- | --- |
| QUA-1 | 4 | Unit tests stay green | `./scripts/ci-check.sh` |
| QUA-2 | 3 | Integration or contract tests stay green | `go test -v ./pkg/cdk/...` (and CDK synth in CI) |
| QUA-3 | 3 | Coverage ≥ 90% (no denominator games) | `bash scripts/hgm/coverage-threshold-check.sh` |

## Consistency (CON)
| ID | Pts | Requirement | Verifier |
| --- | ---: | --- | --- |
| CON-1 | 3 | Formatter clean (no diffs) | `bash scripts/hgm/fmt-check.sh` |
| CON-2 | 5 | Lint/static analysis green (pinned) | `golangci-lint run --timeout=10m --config .golangci.yml ./...` |
| CON-3 | 2 | Public boundary contract parity (if applicable) | `bash scripts/hgm/contract-parity-check.sh` (TODO) |

## Completeness (COM) — anti-drift (“verify the verifiers”)
| ID | Pts | Requirement | Verifier |
| --- | ---: | --- | --- |
| COM-1 | 2 | All modules compile (no “mystery meat”) | `bash scripts/hgm/modules-check.sh` (TODO) |
| COM-2 | 2 | Toolchain pins align to repo expectations | `bash scripts/hgm/toolchain-check.sh` |
| COM-3 | 2 | Lint config schema-valid | `bash scripts/hgm/lint-config-check.sh` (TODO) |
| COM-4 | 2 | Coverage threshold not diluted (≥ 90%) | `bash scripts/hgm/coverage-threshold-check.sh` |
| COM-5 | 1 | Security scan config not diluted | `bash scripts/hgm/sec-config-check.sh` |
| COM-6 | 1 | Logging/operational standards enforced (if applicable) | `bash scripts/hgm/logging-check.sh` (TODO) |

## Security (SEC)
| ID | Pts | Requirement | Verifier |
| --- | ---: | --- | --- |
| SEC-1 | 3 | Static security scan green (pinned) | `bash scripts/hgm/sast-check.sh` (TODO; minimum gosec) |
| SEC-2 | 3 | Dependency vulnerability scan green | `bash scripts/hgm/vuln-scan.sh` (pinned via env) |
| SEC-3 | 2 | Supply-chain verification green | `bash scripts/hgm/supply-chain-check.sh` |
| SEC-4 | 2 | Domain-specific P0 regression tests | `bash scripts/hgm/p0-check.sh` (TODO) |

## Compliance readiness (CMP)
(Custom domain overlay: no framework assumptions; still requires auditability artifacts.)

| ID | Pts | Requirement | Verifier |
| --- | ---: | --- | --- |
| CMP-1 | 4 | Controls matrix exists and is current | `bash scripts/hgm/planning-docs-check.sh` |
| CMP-2 | 3 | Evidence plan exists and is reproducible | `bash scripts/hgm/planning-docs-check.sh` |
| CMP-3 | 3 | Threat model exists and is current | `bash scripts/hgm/planning-docs-check.sh` |

## Maintainability (MAI)
| ID | Pts | Requirement | Verifier |
| --- | ---: | --- | --- |
| MAI-1 | 4 | Complexity budgets enforced | `golangci-lint run --config .golangci.yml -E gocyclo -E gocognit ./...` |
| MAI-2 | 3 | Maintainability roadmap current | `test -f docs/planning/lift-10of10-roadmap.md` |
| MAI-3 | 3 | Canonical implementations (no duplicate semantics) | `bash scripts/hgm/singleton-check.sh` (TODO) |

## Docs (DOC)
| ID | Pts | Requirement | Verifier |
| --- | ---: | --- | --- |
| DOC-1 | 2 | Threat model present | `bash scripts/hgm/planning-docs-check.sh` |
| DOC-2 | 2 | Evidence plan present | `bash scripts/hgm/planning-docs-check.sh` |
| DOC-3 | 2 | Rubric + roadmap present | `bash scripts/hgm/planning-docs-check.sh` |
| DOC-4 | 2 | Doc integrity (links/version claims) | `bash scripts/hgm/doc-integrity-check.sh` |
| DOC-5 | 2 | Threat ↔ controls parity | `bash scripts/hgm/threat-controls-parity.sh` |

---

## Recommended CI surface (minimum set for protected branches)

These are the minimum commands Lift should run in protected branches to sustain 10/10 **without** relying on “green by
configuration.”

```bash
bash scripts/hgm/planning-docs-check.sh
bash scripts/hgm/doc-integrity-check.sh
bash scripts/hgm/threat-controls-parity.sh

./scripts/ci-check.sh
bash scripts/hgm/fmt-check.sh

# Lint (CI pins golangci-lint v2.4.0)
golangci-lint run --timeout=10m --config .golangci.yml ./...

bash scripts/hgm/toolchain-check.sh
bash scripts/hgm/sec-config-check.sh
bash scripts/hgm/supply-chain-check.sh

# Quality gate (wire into CI once stable)
bash scripts/hgm/coverage-threshold-check.sh

# Security gates (wire into CI once pinned)
bash scripts/hgm/vuln-scan.sh
bash scripts/hgm/sast-check.sh
```
