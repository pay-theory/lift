# Lift Threat Model (Custom domain)

This threat model enumerates the highest-impact failure modes relevant to Lift as a framework used in **security
critical production applications** (including authentication systems and deployments operating in cardholder-data
environments).

Threat IDs are stable (`THR-*`). Every `THR-*` listed here must be mapped in:
- `docs/planning/lift-controls-matrix.md`

Parity is verified by:
- `bash scripts/hgm/threat-controls-parity.sh`

## Scope
- **System:** Lift (Go framework + middleware + adapters + CDK examples) used to build AWS Lambda applications.
- **In-scope data classes:** auth context, secrets, PII; potential CHD transit (Lift must not leak it through logs/telemetry).
- **Environments:** dev / stage / prod (prod-like = least-privilege IAM, encryption at rest where supported, production logging).
- **Third parties:** AWS platform services; GitHub Actions; Go modules; npm/CDK toolchain for examples.
- **Out of scope:** downstream application business logic, account/VPC posture, WAF/edge; persistence of CHD/SAD (downstream).
- **Assurance target:** repeatable, CI-enforced verifiers for core quality + security properties; detect drift early.

## Assets and trust boundaries (high level)
### Primary assets
- Correctness of request parsing/validation and handler behavior
- Tenant/user identity context propagation (multi-tenant isolation)
- Confidentiality of secrets and sensitive payloads (including CHD/PII) within logs/telemetry
- Availability and predictable failure behavior under load (timeouts, retries, size limits)
- Build/release integrity (dependency and CI supply chain)

### Trust boundaries
- External event sources (API Gateway, ALB, SQS, SNS, etc.) → Lift adapters/context
- Middleware chain → handler logic (authN/authZ decisions, request validation)
- Framework → AWS SDK clients (IAM credentials, network calls)
- Local dev/CI toolchain → released artifacts

### Entry points
- Framework handler entry (`app.HandleRequest`) and adapters under `pkg/lift/adapters/...`
- Middleware (auth, logging, observability, rate limiting, load shedding)
- Build and release workflows (`.github/workflows/*`)
- Example modules under `examples/*`

## Top threats (stable IDs)

| Threat ID | Title | What can go wrong | Primary controls (Control IDs) | Verification (gate) |
| --- | --- | --- | --- | --- |
| THR-1 | Authentication/authorization bypass due to unsafe defaults | Applications accidentally ship without auth middleware or with permissive defaults. | CON-3, SEC-4, DOC-4 | `bash scripts/hgm/p0-check.sh` (TODO) |
| THR-2 | Sensitive data leakage via logs/metrics/traces | Raw request/response payloads (PII/CHD) appear in CloudWatch logs or tracing metadata. | SEC-4, COM-6 | `bash scripts/hgm/p0-check.sh` (TODO) |
| THR-3 | Cross-tenant data leakage | Tenant scoping fails (context mixups, missing tenant extraction, shared caches). | CON-3, SEC-4 | `bash scripts/hgm/contract-parity-check.sh` (TODO) |
| THR-4 | Secret exposure or insecure secret handling | Secrets embedded in code/logs or passed through unsafe interfaces. | SEC-1, SEC-4 | `bash scripts/hgm/sast-check.sh` (TODO) |
| THR-5 | Input parsing/validation bugs lead to crashes or injection | Unvalidated inputs cause panics, resource exhaustion, or security bugs. | QUA-1, CON-2, SEC-1 | `./scripts/ci-check.sh` + `golangci-lint ...` |
| THR-6 | Denial of service (DoS) via request size / slow execution | Oversized payloads, unbounded retries, or costly handlers lead to timeouts and cost spikes. | SEC-4, COM-6 | `bash scripts/hgm/p0-check.sh` (TODO) |
| THR-7 | Supply chain compromise (deps, CI actions, build tooling) | Compromised dependencies or CI actions produce malicious artifacts. | SEC-2, SEC-3, COM-2 | `bash scripts/hgm/supply-chain-check.sh` |
| THR-8 | Misconfiguration of security middleware in examples | Examples are copied into production with insecure settings (CORS, auth, rate limiting). | COM-1, DOC-4 | `bash scripts/hgm/modules-check.sh` (TODO) |
| THR-9 | Weak cryptographic practices in framework patterns | Framework encourages or contains insecure crypto patterns; deployers copy them. | SEC-1, SEC-4 | `bash scripts/hgm/sast-check.sh` (TODO) |
| THR-10 | Toolchain/config drift causes false-green CI | CI uses different tool versions/config semantics than local; checks are skipped silently. | COM-1..5, DOC-4 | `bash scripts/hgm/toolchain-check.sh` |
| THR-11 | Error-handling flaws leak internals or misreport failures | Incorrect status codes, verbose internal errors, or error swallowing. | QUA-1, CON-2, CON-3 | `./scripts/ci-check.sh` |
| THR-12 | “Root module builds” but examples/modules are broken | Nested modules (examples) silently rot, reducing confidence and increasing misuse risk. | COM-1 | `bash scripts/hgm/modules-check.sh` (TODO) |

## Parity rule (fail closed)
- Every `THR-*` listed above must appear at least once in the controls matrix Threat IDs column.
- Every `THR-*` referenced in the controls matrix must exist here.
- Enforced by: `bash scripts/hgm/threat-controls-parity.sh`

## Notes / constraints
- This is a **custom** domain overlay (no licensed framework text is stored in-repo).
- Prefer threats phrased as failure modes that can be verified by deterministic gates.
