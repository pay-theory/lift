# Lift Controls Matrix (custom — v0.1.0)

This matrix is the “requirements → controls → verifiers → evidence” backbone for Lift.
It is intentionally engineering-focused: it does not claim compliance, but it makes security/quality assertions traceable
and repeatable.

## Scope
- **System:** Lift (Go library + CLI) used to build and deploy AWS Lambda/CDK applications; includes CI/release automation.
- **In-scope data:** authentication material (session tokens, JWTs), secrets, PII, operational telemetry/logs. Lift may be used
  in cardholder data environments (CHD); storage/handling of sensitive authentication data (SAD) must be prohibited/guarded.
- **Environments:** local dev, CI, and deployed stages (dev/staging/live). “prod-like” means: same Go toolchain major/minor
  as `go.mod` (Go 1.25), same lint config, and the same build flags used for releases.
- **Third parties:** GitHub Actions, AWS (CDK/CloudFormation/Lambda), npm registry (AWS CDK), Go module ecosystem.
- **Out of scope:** customer application business logic built with Lift; customer AWS account controls (IAM boundaries,
  key management, runtime monitoring) except where Lift can enforce or validate.
- **Assurance target:** audit-ready engineering controls (repeatable gates + evidence) for a security-critical codebase.

## Threats (reference IDs)
- Threats are stable IDs (`THR-*`) in `hgm-infra/planning/lift-threat-model.md`.
- Each `THR-*` must map to ≥1 row in the controls table below.
- Parity is enforced by `hgm-infra/verifiers/hgm-verify-rubric.sh` (DOC-5).

## Status (evidence-driven)
If you track implementation status, treat it as evidence-driven:
- `unknown`: no verifier/evidence yet
- `partial`: some controls exist but coverage/evidence is incomplete
- `implemented`: verifier exists and evidence path is repeatable

## Engineering Controls (Threat → Control → Verifier → Evidence)

| Area | Threat IDs | Control ID | Requirement | Control (what we implement) | Verification (command/gate) | Evidence (artifact/location) |
| --- | --- | --- | --- | --- | --- | --- |
| Quality | THR-3, THR-6 | QUA-1 | Unit tests prevent regressions | Unit tests executed in CI for core packages. | `make test` | `hgm-infra/evidence/QUA-1-output.log` |
| Quality | THR-6 | QUA-3 | Coverage threshold is enforced (no dilution) | Coverage is measured consistently and compared against a fixed floor (90%). | `make test-coverage` + threshold check | `hgm-infra/evidence/coverage.out`, `hgm-infra/evidence/QUA-3-output.log` |
| Consistency | THR-6 | CON-1 | Formatting is clean (no diffs) | All tracked Go files are gofmt-clean. | `git ls-files '*.go' | xargs gofmt -l` (fails if output) | `hgm-infra/evidence/CON-1-output.log` |
| Consistency | THR-6 | CON-2 | Lint/static analysis is enforced (pinned toolchain) | `golangci-lint` is required to pass under repo config; CI pins the version. | `golangci-lint run --config .golangci.yml ./...` | `hgm-infra/evidence/CON-2-output.log` |
| Completeness | THR-6 | COM-1 | All modules compile | Root module and nested example modules compile (no “broken examples”). | `go test -run TestNonexistent -count=0 ./...` (plus example modules) | `hgm-infra/evidence/COM-1-output.log` |
| Completeness | THR-6, THR-8 | COM-2 | Toolchain pins align to repo expectations | Go major/minor matches `go.mod`; CI pins golangci-lint version; release uses pinned action SHAs. | Toolchain pin check (see rubric) | `hgm-infra/evidence/COM-2-output.log` |
| Completeness | THR-6 | COM-3 | Lint config schema-valid (no silent skip) | `golangci-lint config verify` passes; prevents typos silently disabling checks. | `golangci-lint config verify --config .golangci.yml` | `hgm-infra/evidence/COM-3-output.log` |
| Completeness | THR-6 | COM-4 | Coverage floor is not diluted | Coverage must remain ≥ 90% (no lowering). | Coverage threshold check (parses `hgm-infra/evidence/coverage.out`) | `hgm-infra/evidence/COM-4-output.log` |
| Completeness | THR-9 | COM-5 | Security scan config not diluted | `gosec` remains enabled; excludes are narrowly scoped and justified. | `.golangci.yml` policy check | `hgm-infra/evidence/COM-5-output.log` |
| Security | THR-1, THR-2, THR-9 | SEC-1 | Baseline SAST stays green | Run `gosec` via golangci-lint under pinned config. | `golangci-lint run --disable-all --enable=gosec --config .golangci.yml ./...` | `hgm-infra/evidence/SEC-1-output.log` |
| Security | THR-2 | SEC-2 | Dependency vulnerability scan stays green | Add govulncheck to CI and keep it green. | TODO (BLOCKED until added) | `hgm-infra/evidence/SEC-2-output.log` |
| Supply chain | THR-8 | SEC-3 | Supply-chain verification stays green | Verify action SHA pins, version pins, and release artifact checksums. | TODO (BLOCKED until scripted) | `hgm-infra/evidence/SEC-3-output.log` |
| Security (P0) | THR-4, THR-5 | SEC-4 | Domain-specific P0 regression tests | Prohibit obvious unsafe behavior in CHD/auth environments (e.g., logging secrets/tokens; persisting SAD). | `go test -tags=security_p0 ./internal/securityp0 -count=1` | `hgm-infra/evidence/SEC-4-output.log` |
| Docs | THR-6, THR-7 | DOC-5 | Threat model ↔ controls parity | Every threat ID in threat model appears in this matrix. | Built into `hgm-verify-rubric.sh` parity check | `hgm-infra/evidence/DOC-5-parity.log` |

> Add rows as needed for additional anti-drift (release policy, multi-module behavior, contract parity, encryption/tag semantics).

## Framework Mapping (Optional)
No framework assumptions are embedded for `custom` domain. If a framework later applies, store only IDs/titles (no licensed
standards text) and reference an external KB path/env var.

## Notes
- Keep any licensed standards text out of-repo.
- Prefer deterministic verifiers (tests, static analysis, IaC assertions) over manual checklists.
- Treat this matrix as “source material”: the rubric/roadmap/evidence plan must stay consistent with the Control IDs here.
