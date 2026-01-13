# Lift Threat Model (custom — v0.1.0)

This document enumerates the highest-risk threats for the in-scope system and assigns stable IDs (`THR-*`) that must map
to controls in `hgm-infra/planning/lift-controls-matrix.md`.

## Scope (must be explicit)
- **System:** Lift (Go CLI + library) used to create/deploy AWS Lambda/CDK applications and associated CI/release automation.
- **In-scope data:** auth material (JWTs/session identifiers), secrets, PII, telemetry/log data; may be used in CHD
  environments.
- **Environments:** local dev, CI, and deployed stages (dev/staging/live). “prod-like” means: Go 1.25 toolchain and the same
  lint/test/build configuration as CI/release.
- **Third parties:** GitHub Actions; AWS (Lambda, CloudFormation, API Gateway, DynamoDB); npm (AWS CDK distribution);
  Go module ecosystem.
- **Out of scope:** customer-specific AWS account controls and business logic, except where Lift can validate or enforce invariants.
- **Assurance target:** audit-ready engineering controls with repeatable evidence.

## Assets and Trust Boundaries (high level)
- **Primary assets:**
  - Release artifacts (binaries, checksums) and tags
  - CI definitions (GitHub Actions workflows)
  - Infrastructure definitions and deployment behavior (CDK synthesis/deploy)
  - Any secrets/credentials used during CI/deploy (OIDC, AWS credentials)
  - Customer configuration files (`lift.yaml`) and generated project layouts
  - Logs/telemetry emitted by applications and Lift itself
- **Trust boundaries:**
  - Developer workstation ↔ GitHub repository
  - GitHub Actions runner ↔ AWS account (OIDC / API calls)
  - Lift-generated code ↔ customer application code
  - npm/go module registries ↔ dependency resolution
- **Entry points:**
  - CLI inputs/flags and `lift.yaml`
  - Template rendering / code generation
  - GitHub Actions workflows and reusable actions
  - CDK synthesis/deploy commands

## Top Threats (stable IDs)

| Threat ID | Title | What can go wrong | Primary controls (Control IDs) | Verification (gate) |
| --- | --- | --- | --- | --- |
| THR-1 | Secret leakage via logs | Tokens, keys, or CHD-derived data ends up in logs or CI output. | SEC-4, COM-6 | P0 test suite (TODO) + logging standards (TODO) |
| THR-2 | Vulnerable dependencies | A known vulnerable Go/npm dependency is introduced and shipped. | SEC-2, SEC-3 | govulncheck (TODO), supply-chain checks (TODO) |
| THR-3 | Regression in auth-critical behavior | Changes break authentication flows or critical invariants. | QUA-1, QUA-3 | `make test`, coverage gate |
| THR-4 | Unsafe use in CHD environments | Lift encourages patterns that could persist SAD or mishandle CHD-related data. | SEC-4 | P0 tests (TODO) |
| THR-5 | Multi-tenant isolation failure | Tenant boundary assumptions are broken (e.g., shared keys, incorrect partitioning). | SEC-4, QUA-1 | Targeted tests (TODO) |
| THR-6 | “Green by dilution” drift | Checks pass by reducing scope (excluding dirs, lowering thresholds) rather than fixing issues. | COM-3, COM-4, COM-5, DOC-5 | Verifier anti-drift gates |
| THR-7 | Documentation/evidence drift | Docs claim controls exist but there is no verifier or evidence path. | CMP-1..3, DOC-4 | Doc integrity checks |
| THR-8 | Supply-chain / release tampering | CI or release pipeline is modified to publish unverified artifacts or pull mutable versions. | SEC-3, COM-2 | Supply-chain checks (TODO) |
| THR-9 | Static security signal suppressed | High-signal SAST findings are silenced via blanket excludes or config drift. | COM-5, SEC-1 | Config checks + gosec gate |

## Parity Rule (no “named threat without control”)
- Every `THR-*` listed above must appear at least once in the controls matrix “Threat IDs” column.
- The repo must have a deterministic parity check that fails if any threat is unmapped.

## Notes
- Keep licensed standards text out of the repo; reference externally by ID/path if a framework applies later.
- Prefer threats phrased as failure modes the repo can prevent or detect.
