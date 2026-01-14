# Lift Controls Matrix (custom — v0.1)

This matrix is the “requirements → controls → verifiers → evidence” backbone for Lift. It is intentionally
engineering-focused: it does not claim compliance, but it makes security/quality assertions traceable and repeatable.

## Scope
- **System:** Lift (github.com/pay-theory/lift) — a Go framework and runtime for building AWS Lambda functions with
  standardized request parsing/validation, middleware, logging/observability, multi-tenant helpers, and guardrails.
- **In-scope data:** authentication tokens (JWTs), tenant/user identifiers, secrets (configuration, signing keys), PII
  (customer/user records), and **potentially cardholder-data-adjacent payloads** because Lift is used in cardholder data
  environments (Lift should not encourage storing CHD, but it may process CHD transiently in handlers).
- **Environments:** dev, stage, prod; “prod-like” means same guardrails/middleware chain, same request limits/timeouts,
  and same logging/redaction policy.
- **Third parties:** AWS (Lambda, API Gateway, CloudWatch Logs, X-Ray, DynamoDB, Secrets Manager, SNS/SQS/S3 as used by
  applications), GitHub Actions, npm registry (for CDK), Go module ecosystem.
- **Out of scope:** customer application business logic, customer AWS account configuration (IAM, VPC, WAF), AWS service
  correctness/availability, and any infrastructure deployed outside this repo.
- **Assurance target:** Audit-ready engineering controls (repeatable verification + evidence paths; fail closed when
  verifiers are missing).

## Threats (reference IDs)
- Threats are enumerated as stable IDs (`THR-*`) in `hgm-infra/planning/lift-threat-model.md`.
- Each `THR-*` must map to ≥1 row in the controls table below (validated by a deterministic parity check in the
  Hypergenium verifier).

## Status (evidence-driven)
If you track implementation status, treat it as evidence-driven:
- `unknown`: no verifier/evidence yet
- `partial`: some controls exist but coverage/evidence is incomplete
- `implemented`: verifier exists and evidence path is repeatable

## Engineering Controls (Threat → Control → Verifier → Evidence)

| Area | Threat IDs | Control ID | Requirement | Control (what we implement) | Verification (command/gate) | Evidence (artifact/location) |
| --- | --- | --- | --- | --- | --- | --- |
| Quality | THR-7 | QUA-1 | Unit tests prevent regressions | Unit tests for core packages and security guardrails. | `./scripts/ci-check.sh` | `hgm-infra/evidence/QUA-1-output.log` |
| Quality | THR-7 | QUA-2 | Integration or contract tests stay green | Add integration/contract tests for public API behaviors (auth, routing, middleware ordering, tenant isolation semantics). | TODO (see rubric) | `hgm-infra/evidence/QUA-2-output.log` |
| Quality | THR-7 | QUA-3 | Coverage threshold is enforced (no denominator games) | Coverage is measured on in-scope packages; threshold is a hard floor (90% core coverage). | `hgm-infra/verifiers/hgm-verify-rubric.sh` (QUA-3/COM-4) | `hgm-infra/evidence/coverage.core.out`, `hgm-infra/evidence/QUA-3-output.log` |
| Consistency | — | CON-1 | Formatting is clean (no diffs) | gofmt is applied consistently across in-scope Go code. | `gofmt -l` (see verifier) | `hgm-infra/evidence/CON-1-output.log` |
| Consistency | THR-7 | CON-2 | Lint/static analysis is enforced (pinned toolchain) | golangci-lint runs with a schema-valid config; no blanket excludes for production code. | `make lint` | `hgm-infra/evidence/CON-2-output.log` |
| Consistency | THR-1, THR-4 | CON-3 | Public boundary contract parity (if applicable) | Exported/public API behaviors are backed by deterministic contract tests to prevent accidental semantic drift. | TODO (see rubric) | `hgm-infra/evidence/CON-3-output.log` |
| Completeness | THR-7 | COM-1 | All modules compile (no “mystery meat”) | All packages compile; build failures in “secondary” packages are not ignored. | `go build ./...` | `hgm-infra/evidence/COM-1-output.log` |
| Completeness | THR-7 | COM-2 | CI/toolchain pins align to repo expectations | go.mod `go` version aligns to CI; lint tool versions are pinned (no `latest`). | verifier COM-2 | `hgm-infra/evidence/COM-2-output.log` |
| Completeness | THR-7 | COM-3 | Lint config schema-valid (no silent skip) | `.golangci.yml` declares schema/version and required settings (download mode, enabled linters, security linter enabled). | verifier COM-3 | `hgm-infra/evidence/COM-3-output.log` |
| Completeness | THR-7 | COM-4 | Coverage threshold not diluted (≥ 90%) | Threshold is checked from a deterministic coverage artifact; no lowering for convenience. | verifier COM-4 | `hgm-infra/evidence/COM-4-output.log` |
| Completeness | THR-2 | COM-6 | Logging/operational standards enforced | Prevent logging of secrets/CHD-adjacent payloads; require structured logs; enforce correlation IDs. | TODO (see rubric) | `hgm-infra/evidence/COM-6-output.log` |
| Security | THR-1, THR-4 | SEC-1 | Baseline SAST stays green | Security static analysis runs (gosec via pinned golangci-lint) and stays green. | verifier SEC-1 | `hgm-infra/evidence/SEC-1-output.log` |
| Security | THR-5 | SEC-2 | Dependency vulnerability scan stays green | govulncheck (pinned) is run and results are reviewed; CI blocks on findings. | TODO (see rubric) | `hgm-infra/evidence/SEC-2-output.log` |
| Security | THR-5 | SEC-3 | Supply-chain verification green | Actions are SHA-pinned; go.sum present; release artifacts include checksums. | verifier SEC-3 | `hgm-infra/evidence/SEC-3-output.log` |
| Security | THR-1, THR-2, THR-3, THR-4, THR-6, THR-8, THR-9 | SEC-4 | Domain-specific P0 regression tests | Add P0 tests asserting critical security invariants (no auth bypass, safe JWT handling, request size guardrails, safe logging defaults, secrets handling). | TODO (see rubric) | `hgm-infra/evidence/SEC-4-output.log` |
| Docs | THR-7 | DOC-4 | Doc integrity (links, version claims) | Planning docs contain no unrendered tokens; required docs exist. | verifier DOC-4 | `hgm-infra/evidence/DOC-4-output.log` |
| Docs | THR-7 | DOC-5 | Threat model ↔ controls parity (no unmapped threats) | Every `THR-*` in the threat model appears in this controls matrix. | verifier DOC-5 parity | `hgm-infra/evidence/DOC-5-parity.log` |

### Notes / future control candidates (Lift-specific)
- **JWT hardening:** verify algorithm restrictions, key types, and default token extraction behaviors.
- **Tenant isolation:** ensure helpers never “default” to an empty tenant/user in a way that could bypass isolation.
- **Logging redaction:** enforce that known sensitive keys are redacted (or that raw payload logging is prohibited).
- **Request guardrails:** enforce max request size / timeouts / response size caps.

## Framework Mapping (Optional)
No external compliance framework text is embedded in this repo (domain is `custom`). If a framework mapping is required
later, store only IDs + short titles and reference the standards content out-of-repo.
