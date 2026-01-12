# Lift Controls Matrix (Custom domain, controls-as-code starter)

This document is Lift’s traceability backbone: **risk / requirement → control → verifier → evidence**.

Lift is used in **security-critical production applications**, including authentication flows and deployments that operate
in cardholder-data environments. This matrix is intentionally engineering-focused: it describes verifiable controls and
how to gather evidence, but it does not itself claim compliance.

## Scope
- **System:** Lift (Go framework + libraries + examples) used to build AWS Lambda applications.
- **In-scope data classes (handled by Lift or commonly transiting Lift-based apps):**
  - Authentication context (user identity, tenant identity, authorization decisions)
  - Secrets (API keys, AWS credentials via environment / IAM, session tokens)
  - Potential **CHD** transit/handling in downstream applications (Lift must not leak CHD via logs/telemetry)
  - PII (names/emails/identifiers) in request payloads
  - Telemetry (logs/metrics/traces) that may contain sensitive identifiers
- **Environments:** dev / stage / prod.
  - “**Prod-like**” means: runs on AWS services with production IAM posture (least privilege), encryption at rest enabled
    where supported, and production-grade logging/monitoring.
- **Third parties / platforms:**
  - AWS (Lambda, API Gateway, CloudWatch Logs, X-Ray, DynamoDB, S3, Secrets Manager, SSM)
  - GitHub Actions (CI), Go module ecosystem (proxy/sumdb), npm (AWS CDK toolchain for examples)
- **Out of scope (explicit):**
  - Customer application business logic built on Lift
  - Infrastructure accounts/VPC configuration, WAF, and edge protections (owned by deployers)
  - Storage of cardholder data by downstream systems (Lift is a framework; downstream ownership)
- **Assurance target:** audit-ready, repeatable verifiers for critical quality/security properties; “fail closed” on drift.

## Threat IDs
Threats are enumerated as stable IDs (`THR-*`) in:
- `docs/planning/lift-threat-model.md`

Rule: every `THR-*` in the threat model must be referenced by at least one row in the table below. The parity check is:
- `bash scripts/hgm/threat-controls-parity.sh`

## Control status vocabulary (optional)
If you track implementation state, use evidence-driven statuses:
- **unknown**: no verifier/evidence path exists yet
- **partial**: control exists but is not enforced in CI or evidence is incomplete
- **implemented**: a deterministic verifier exists and is enforced, with reproducible evidence

## Engineering controls table

| Area | Threat IDs | Control ID | Requirement | Control (what we implement) | Verification (command/gate) | Evidence (artifact/location) |
| --- | --- | --- | --- | --- | --- | --- |
| Quality | THR-5, THR-10, THR-11 | QUA-1 | Unit tests prevent regressions | Maintain a test suite that exercises the public handler/middleware APIs and critical edge/error paths. | `./scripts/ci-check.sh` | GitHub Actions: `Test` workflow logs; local: command output |
| Quality | THR-10 | QUA-3 | Coverage threshold is enforced (no denominator games) | Maintain **core** coverage ≥ 90% while keeping measurement scope stable (no new excludes to “hit the number”). | `bash scripts/hgm/coverage-threshold-check.sh` | `coverage.out`, `coverage.core.out` (and CI artifacts once wired) |
| Consistency | THR-10 | CON-1 | Formatting is clean (no diffs) | Require gofmt-clean code (review noise reduction; avoids hidden diffs). | `bash scripts/hgm/fmt-check.sh` | Script output; PR status |
| Consistency | THR-5, THR-7, THR-10 | CON-2 | Lint/static analysis is enforced (pinned) | Enforce golangci-lint (incl. gosec/staticcheck) with a repo-owned config. | `golangci-lint run --config .golangci.yml ./...` | GitHub Actions `lint` job logs |
| Consistency | THR-1, THR-3, THR-11 | CON-3 | Public boundary contract parity (if applicable) | Ensure exported “convenience” APIs don’t silently diverge from canonical semantics (contract tests). | `bash scripts/hgm/contract-parity-check.sh` (TODO) | CI logs + contract test output |
| Completeness | THR-10, THR-12 | COM-1 | All modules compile (no hidden breakage) | Compile-check root + example modules (avoid “root builds but examples are broken”). | `bash scripts/hgm/modules-check.sh` (TODO) | Script output |
| Completeness | THR-10 | COM-2 | Toolchain pins align to repo expectations | Keep Go version + lint versions consistent across `go.mod`, workflows, and scripts. | `bash scripts/hgm/toolchain-check.sh` | Script output |
| Completeness | THR-10 | COM-3 | Lint config is schema-valid | Validate `.golangci.yml` under the pinned golangci-lint version (no silent skip). | `bash scripts/hgm/lint-config-check.sh` (TODO) | Script output |
| Completeness | THR-10 | COM-4 | Coverage threshold can’t be diluted | Coverage gate uses a hard threshold (≥ 90%) and fails if below. | `bash scripts/hgm/coverage-threshold-check.sh` | Script output |
| Completeness | THR-10 | COM-5 | Security scan config not diluted | Ensure SAST/security config doesn’t remove high-signal checks by broad exclusions. | `bash scripts/hgm/sec-config-check.sh` | Script output |
| Security | THR-2, THR-4, THR-5, THR-7 | SEC-1 | Baseline SAST stays green | Run a static security scan (at minimum gosec via golangci-lint) and fail on findings. | `bash scripts/hgm/sast-check.sh` (TODO) | CI logs or SARIF |
| Security | THR-7 | SEC-2 | Dependency vulnerability scan stays green | Run a dependency vulnerability scanner (e.g., govulncheck) with pinned versioning. | `bash scripts/hgm/vuln-scan.sh` (pinned via env) | CI logs / report output |
| Security | THR-7, THR-10 | SEC-3 | Supply-chain checks | Enforce pinned CI actions + deterministic build/release outputs (checksums, provenance later). | `bash scripts/hgm/supply-chain-check.sh` | Script output; release artifacts |
| Security | THR-2, THR-3, THR-6 | SEC-4 | Domain P0 regression tests | Add regression tests for “must never happen” failures (e.g., no raw sensitive payload logs, tenant isolation). | `bash scripts/hgm/p0-check.sh` (TODO) | Test logs |
| Docs | THR-10 | DOC-4 | Doc integrity | Ensure planning docs exist and cross-reference correctly (no broken links/claims). | `bash scripts/hgm/doc-integrity-check.sh` | Script output |
| Docs | THR-10 | DOC-5 | Threat ↔ controls parity | Fail if any `THR-*` in threat model is not mapped in this matrix (and vice versa). | `bash scripts/hgm/threat-controls-parity.sh` | Script output |

## Optional: framework mapping
Lift is currently under the **custom** domain overlay (no framework assumptions). If you later map to a framework, keep
licensed text out of the repo; store only IDs/titles and reference an external KB path.

---

### Owner notes
- This matrix is intended to stay aligned with `lift-10of10-rubric.md` and `lift-evidence-plan.md`.
- If you add a new threat ID, you must:
  1) add the threat to `lift-threat-model.md`
  2) map it here
  3) keep parity green (`scripts/hgm/threat-controls-parity.sh`)
