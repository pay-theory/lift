# AI / Agent Drift: Failure Modes + Recovery (Lift)

This document captures how AI assistance can accidentally degrade controls, and the guardrails Lift uses to prevent
“green by dilution.”

## Common drift failure modes
- **Scope shrink to pass checks:** excluding packages, skipping tests, or weakening config so CI “goes green.”
- **Toolchain drift:** local runs use one Go/lint version while CI uses another; results aren’t comparable.
- **Silent config invalidation:** a mis-indented YAML block or unsupported config key makes a tool skip checks.
- **Evidence drift:** docs claim a control exists, but no verifier and no reproducible evidence are present.
- **Security affordance without semantics:** tags/flags that look like security controls but are not enforced.
- **Public API drift:** exported helper APIs diverge from canonical semantics without tests that detect it.
- **Maintainability erosion:** growing complexity/duplication increases risk of security fixes being incomplete.

## Guardrails we enforce (fail closed)
- The rubric (`docs/planning/lift-10of10-rubric.md`) is versioned and is the source of truth.
- Anti-drift checks exist for:
  - toolchain alignment (`scripts/hgm/toolchain-check.sh`)
  - coverage threshold enforcement (`scripts/hgm/coverage-threshold-check.sh`)
  - security config drift (`scripts/hgm/sec-config-check.sh`)
  - supply-chain hygiene (`scripts/hgm/supply-chain-check.sh`)
  - threat ↔ controls parity (`scripts/hgm/threat-controls-parity.sh`)
- Avoid blanket exclusions. If a suppression is required, keep it narrow and justify it.

## Recovery playbook
1. Re-run the full rubric surface (locally or in CI):
   - `./scripts/ci-check.sh`
   - `bash scripts/hgm/fmt-check.sh`
   - `golangci-lint run --timeout=10m --config .golangci.yml ./...`
   - `bash scripts/hgm/toolchain-check.sh`
   - `bash scripts/hgm/sec-config-check.sh`
   - `bash scripts/hgm/supply-chain-check.sh`
   - `bash scripts/hgm/threat-controls-parity.sh`
2. Fix failing gates by changing code/tests/config — not by shrinking scope.
3. If a verifier is flaky/wrong, fix the verifier and bump rubric version (no silent loosening).
4. Update the roadmap + evidence plan when new threats or controls are introduced.

## Turning discoveries into durable gates
When a new drift failure is discovered:
1) define the failure mode as a threat or rubric item,
2) implement a deterministic verifier (script/command),
3) wire it into CI,
4) update evidence plan + roadmap, and bump rubric version.
