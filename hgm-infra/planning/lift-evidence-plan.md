# Lift Evidence Plan (Rubric v0.4)

Defines where evidence for rubric items is produced and how to regenerate it. Evidence should be reproducible from a
commit SHA (no hand-assembled screenshots unless unavoidable).

## Evidence sources

### CI artifacts (preferred)
- Unit tests: `./scripts/ci-check.sh` → `hgm-infra/evidence/QUA-1-output.log`
- Coverage: `./hgm-infra/verifiers/hgm-verify-rubric.sh` (QUA-3/COM-4) →
  - `hgm-infra/evidence/coverage.out`
  - `hgm-infra/evidence/coverage.core.out`
  - `hgm-infra/evidence/QUA-3-output.log`
- Lint: `make lint` → `hgm-infra/evidence/CON-2-output.log`
- SAST: verifier SEC-1 (gosec via pinned golangci-lint) → `hgm-infra/evidence/SEC-1-output.log`
- Supply-chain checks: verifier SEC-3 → `hgm-infra/evidence/SEC-3-output.log`

### Deterministic in-repo artifacts
- Controls matrix: `hgm-infra/planning/lift-controls-matrix.md`
- Rubric: `hgm-infra/planning/lift-10of10-rubric.md`
- Roadmap: `hgm-infra/planning/lift-10of10-roadmap.md`
- Evidence plan: `hgm-infra/planning/lift-evidence-plan.md`
- Threat model: `hgm-infra/planning/lift-threat-model.md`
- AI drift recovery: `hgm-infra/planning/lift-ai-drift-recovery.md`
- Signature bundle (local certification; not CI): `hgm-infra/signatures/hgm-signature-bundle.json`

## Rubric-to-evidence map
Every rubric ID maps to exactly one verifier and one primary evidence location.

| Rubric ID | Primary evidence | Evidence path | How to refresh |
| --- | --- | --- | --- |
| QUA-1 | Unit test output | `hgm-infra/evidence/QUA-1-output.log` | `./hgm-infra/verifiers/hgm-verify-rubric.sh` |
| QUA-2 | Integration/contract output | `hgm-infra/evidence/QUA-2-output.log` | `./hgm-infra/verifiers/hgm-verify-rubric.sh` (runs `go test -tags=contract -count=1 ./pkg/contract/...`) |
| QUA-3 | Coverage profile + summary | `hgm-infra/evidence/QUA-3-output.log` | `./hgm-infra/verifiers/hgm-verify-rubric.sh` |
| CON-1 | Formatter diff list | `hgm-infra/evidence/CON-1-output.log` | `./hgm-infra/verifiers/hgm-verify-rubric.sh` |
| CON-2 | Lint output | `hgm-infra/evidence/CON-2-output.log` | `./hgm-infra/verifiers/hgm-verify-rubric.sh` |
| CON-3 | Contract verification output | `hgm-infra/evidence/CON-3-output.log` | `./hgm-infra/verifiers/hgm-verify-rubric.sh` (runs `go test -tags=contract -count=1 ./pkg/contract/...`) |
| COM-1 | Module compile check | `hgm-infra/evidence/COM-1-output.log` | `./hgm-infra/verifiers/hgm-verify-rubric.sh` |
| COM-2 | Toolchain pin verification | `hgm-infra/evidence/COM-2-output.log` | `./hgm-infra/verifiers/hgm-verify-rubric.sh` |
| COM-3 | Lint config validation | `hgm-infra/evidence/COM-3-output.log` | `./hgm-infra/verifiers/hgm-verify-rubric.sh` |
| COM-4 | Coverage threshold check | `hgm-infra/evidence/COM-4-output.log` | `./hgm-infra/verifiers/hgm-verify-rubric.sh` |
| COM-5 | Security config validation | `hgm-infra/evidence/COM-5-output.log` | `./hgm-infra/verifiers/hgm-verify-rubric.sh` |
| COM-6 | Logging standards check | `hgm-infra/evidence/COM-6-output.log` | `./hgm-infra/verifiers/hgm-verify-rubric.sh` (runs `go test -count=1 -run '^TestOps_' ./pkg/observability/zap ./pkg/middleware`) |
| SEC-1 | SAST scan output | `hgm-infra/evidence/SEC-1-output.log` | `./hgm-infra/verifiers/hgm-verify-rubric.sh` |
| SEC-2 | Vulnerability scan output | `hgm-infra/evidence/SEC-2-output.log` | TODO |
| SEC-3 | Supply-chain verification | `hgm-infra/evidence/SEC-3-output.log` | `./hgm-infra/verifiers/hgm-verify-rubric.sh` |
| SEC-4 | Domain P0 regression tests | `hgm-infra/evidence/SEC-4-output.log` | `./hgm-infra/verifiers/hgm-verify-rubric.sh` (runs `go test -count=1 -run '^TestP0_' ./pkg/observability/zap ./pkg/middleware`) |
| CMP-1 | Controls matrix exists | `hgm-infra/planning/lift-controls-matrix.md` | File existence check |
| CMP-2 | Evidence plan exists | `hgm-infra/planning/lift-evidence-plan.md` | File existence check |
| CMP-3 | Threat model exists | `hgm-infra/planning/lift-threat-model.md` | File existence check |
| MAI-1 | File budget check | `hgm-infra/evidence/MAI-1-output.log` | `./hgm-infra/verifiers/hgm-verify-rubric.sh` |
| MAI-2 | Maintainability roadmap check | `hgm-infra/evidence/MAI-2-output.log` | `./hgm-infra/verifiers/hgm-verify-rubric.sh` |
| MAI-3 | Duplicate semantics check | `hgm-infra/evidence/MAI-3-output.log` | `./hgm-infra/verifiers/hgm-verify-rubric.sh` |
| DOC-1 | Threat model present | `hgm-infra/planning/lift-threat-model.md` | File existence check |
| DOC-2 | Evidence plan present | `hgm-infra/planning/lift-evidence-plan.md` | File existence check |
| DOC-3 | Rubric + roadmap present | `hgm-infra/planning/lift-10of10-rubric.md` | File existence check |
| DOC-4 | Doc integrity output | `hgm-infra/evidence/DOC-4-output.log` | `./hgm-infra/verifiers/hgm-verify-rubric.sh` |
| DOC-5 | Threat ↔ controls parity | `hgm-infra/evidence/DOC-5-parity.log` | `./hgm-infra/verifiers/hgm-verify-rubric.sh` |

## Rubric Report (Fixed Location)
The deterministic verifier produces a machine-readable report at:
- `hgm-infra/evidence/hgm-rubric-report.json`

This report contains per-rubric-ID status (PASS/FAIL/BLOCKED), evidence paths, and pack provenance.

## Notes
- All evidence paths are relative to repo root and must live under `hgm-infra/`.
- Missing verifiers are BLOCKED; do not accept “manual green” for CI gates.
- When adding new gates, update the rubric and roadmap (and bump the rubric version if requirements change).
