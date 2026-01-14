# Lift Threat Model (custom — v0.1)

This document enumerates the highest-risk threats for the in-scope system and assigns stable IDs (`THR-*`) that must map
to controls in `hgm-infra/planning/lift-controls-matrix.md`.

## Scope (must be explicit)
- **System:** Lift (Go framework + runtime) for building AWS Lambda functions.
- **In-scope data:** authentication tokens (JWTs), tenant/user identifiers, secrets (config, signing keys), PII, and
  potentially CHD-adjacent payloads because Lift is used in cardholder data environments.
- **Environments:** dev, stage, prod (define “prod-like”: same guardrails/middleware chain, same observability settings,
  and same redaction/logging policies).
- **Third parties:** AWS services (Lambda, API Gateway, CloudWatch Logs, X-Ray, DynamoDB, Secrets Manager), GitHub
  Actions, npm registry (CDK install), Go module ecosystem.
- **Out of scope:** downstream application code built with Lift, customer AWS account/IAM configuration, and the
  correctness/availability of AWS services.
- **Assurance target:** audit-ready engineering gates with deterministic evidence generation; fail closed for missing
  verifiers.

## Assets and Trust Boundaries (high level)
- **Primary assets**
  - A1: Authentication/authorization state (JWT claims, user ID, tenant ID).
  - A2: Secrets (JWT signing keys, API keys, database credentials, KMS/materialized secrets).
  - A3: Production logs/telemetry (CloudWatch Logs, X-Ray traces, metrics) which may contain sensitive data.
  - A4: Request/response payloads (may include PII; may include CHD-adjacent data when Lift is used in PCI contexts).
  - A5: Release artifacts (binaries) and the build pipeline.

- **Trust boundaries**
  - TB1: Internet/client → API Gateway / ALB.
  - TB2: AWS API Gateway → Lambda runtime (event translation boundary).
  - TB3: Lambda runtime → AWS services (AWS SDK calls, IAM boundary).
  - TB4: Developer workstation/CI runner → build/release outputs.

- **Entry points**
  - EP1: HTTP routes / handlers.
  - EP2: WebSocket routes / handlers.
  - EP3: Middleware configuration (auth, logging, rate limiting, load shedding).
  - EP4: CI workflows (test/lint/release), dependency downloads, and CDK synthesis.

## Top Threats (stable IDs)

| Threat ID | Title | What can go wrong | Primary controls (Control IDs) | Verification (gate) |
| --- | --- | --- | --- | --- |
| THR-1 | Authentication/authorization bypass | Misconfigured or vulnerable middleware allows requests to execute handlers without valid auth, or with forged claims. | SEC-4, CON-3 | P0 tests + contract tests |
| THR-2 | Sensitive data exposure via logs/telemetry | Secrets, JWTs, PII, or CHD-adjacent payloads are written to logs/traces/metrics; attackers or insiders can retrieve them. | COM-6, SEC-4 | Logging standards verifier + P0 tests |
| THR-3 | Injection/path traversal via unsafe input handling | Framework helpers make it easy to pass attacker-controlled values into exec/file/network calls without validation or allowlisting. | SEC-1, SEC-4 | gosec + targeted P0 tests |
| THR-4 | JWT algorithm/key confusion | Algorithm mismatch (HS vs RS), weak validation, or unsafe defaults lead to token forgery. | SEC-4, CON-3 | P0 tests + contract tests |
| THR-5 | Supply-chain compromise | Unpinned CI actions, dependency compromise, or tampered release artifacts introduce malicious code. | SEC-3, SEC-2 | Supply-chain verifier + govulncheck (pinned) |
| THR-6 | Denial-of-service via payload size/timeouts | Large requests, expensive parsing/validation, or unbounded loops lead to timeouts/cost spikes/availability impact. | SEC-4, QUA-2 | P0 tests + contract tests |
| THR-7 | Control drift / false-green verification | Checks are loosened (exclusions, lowered thresholds, unpinned tools), producing a “green” signal that is not meaningful. | COM-2, COM-3, COM-4, COM-7, DOC-4 | `make rubric` |
| THR-8 | WebSocket misuse / routing confusion | Incorrect route selection or handler mapping can leak data cross-tenant or enable unauthorized actions. | SEC-4, QUA-2 | P0 + contract tests |
| THR-9 | Insecure secret handling defaults | Secrets are stored in code/config, or helper functions encourage unsafe local secret storage. | SEC-1, SEC-4 | gosec + P0 tests |

## Parity Rule (no “named threat without control”)
- Every `THR-*` listed above must appear at least once in the controls matrix “Threat IDs” column.
- The repo must have a deterministic parity check that fails if any threat is unmapped.

## Notes
- Keep raw standards text out of the repo when licensing is uncertain; reference KBs by ID/path.
- Prefer threats phrased as failure modes the repo can actually prevent or detect.
