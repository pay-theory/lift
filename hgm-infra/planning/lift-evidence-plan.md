# Lift Evidence Plan (Rubric v0.1.0)

Defines where evidence for rubric items is produced and how to regenerate it. Evidence should be reproducible from a
commit SHA (no hand-assembled screenshots unless unavoidable).

## Evidence sources
### CI artifacts (preferred)
- Coverage: `make test-coverage` → `hgm-infra/evidence/coverage.out` (copied by verifier)
- Lint: `golangci-lint run --config .golangci.yml ./...` output (pinned in CI workflow)
- Security: `golangci-lint ... --enable=gosec` output (pinned in CI workflow)
- Rubric report: `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh` → `hgm-infra/evidence/hgm-rubric-report.json`

### Deterministic in-repo artifacts
- Controls matrix: `hgm-infra/planning/lift-controls-matrix.md`
- Rubric: `hgm-infra/planning/lift-10of10-rubric.md`
- Roadmap: `hgm-infra/planning/lift-10of10-roadmap.md`
- Evidence plan: `hgm-infra/planning/lift-evidence-plan.md`
- Threat model: `hgm-infra/planning/lift-threat-model.md`
- AI drift recovery: `hgm-infra/planning/lift-ai-drift-recovery.md`
- Signature bundle (local certification, optional): `hgm-infra/signatures/hgm-signature-bundle.json`

## Rubric-to-evidence map
Every rubric ID maps to exactly one verifier and one primary evidence location.

| Rubric ID | Primary evidence | Evidence path | How to refresh |
| --- | --- | --- | --- |
| QUA-1 | Unit test output | `hgm-infra/evidence/QUA-1-output.log` | `make test` |
| QUA-2 | Integration/contract output | `hgm-infra/evidence/QUA-2-output.log` | `go test ./internal/contracttests -v` |
| QUA-3 | Coverage profile + summary | `hgm-infra/evidence/QUA-3-output.log` + `hgm-infra/evidence/coverage.out` | `make test-coverage` |
| CON-1 | Formatter diff list | `hgm-infra/evidence/CON-1-output.log` | `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh` |
| CON-2 | Lint output | `hgm-infra/evidence/CON-2-output.log` | `golangci-lint run --config .golangci.yml ./...` |
| CON-3 | Contract verification output | `hgm-infra/evidence/CON-3-output.log` | `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh` |
| COM-1 | Module compile check | `hgm-infra/evidence/COM-1-output.log` | `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh` |
| COM-2 | Toolchain pin verification | `hgm-infra/evidence/COM-2-output.log` | `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh` |
| COM-3 | Lint config validation | `hgm-infra/evidence/COM-3-output.log` | `golangci-lint config verify --config .golangci.yml` |
| COM-4 | Coverage threshold check | `hgm-infra/evidence/COM-4-output.log` | `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh` |
| COM-5 | Security config validation | `hgm-infra/evidence/COM-5-output.log` | `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh` |
| COM-6 | Logging standards check | `hgm-infra/evidence/COM-6-output.log` | TODO (BLOCKED) |
| SEC-1 | SAST scan output | `hgm-infra/evidence/SEC-1-output.log` | `golangci-lint run --enable-only=gosec --config .golangci.yml ./...` |
| SEC-2 | Vulnerability scan output | `hgm-infra/evidence/SEC-2-output.log` | TODO (BLOCKED) |
| SEC-3 | Supply-chain verification | `hgm-infra/evidence/SEC-3-output.log` | TODO (BLOCKED) |
| SEC-4 | Domain P0 regression tests | `hgm-infra/evidence/SEC-4-output.log` | TODO (BLOCKED) |
| CMP-1 | Controls matrix exists | `hgm-infra/planning/lift-controls-matrix.md` | file existence check |
| CMP-2 | Evidence plan exists | `hgm-infra/planning/lift-evidence-plan.md` | file existence check |
| CMP-3 | Threat model exists | `hgm-infra/planning/lift-threat-model.md` | file existence check |
| MAI-1 | File budget check | `hgm-infra/evidence/MAI-1-output.log` | `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh` |
| MAI-2 | Maintainability roadmap check | `hgm-infra/evidence/MAI-2-output.log` | `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh` |
| MAI-3 | Singleton/duplication check | `hgm-infra/evidence/MAI-3-output.log` | `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh` |
| DOC-1 | Threat model present | `hgm-infra/planning/lift-threat-model.md` | file existence check |
| DOC-2 | Evidence plan present | `hgm-infra/planning/lift-evidence-plan.md` | file existence check |
| DOC-3 | Rubric + roadmap present | `hgm-infra/planning/lift-10of10-rubric.md` | file existence check |
| DOC-4 | Doc integrity (links, versions) | `hgm-infra/evidence/DOC-4-output.log` | `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh` |
| DOC-5 | Threat ↔ controls parity | `hgm-infra/evidence/DOC-5-parity.log` | `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh` |

## Rubric Report (Fixed Location)
The deterministic verifier (`hgm-infra/verifiers/hgm-verify-rubric.sh`) produces a machine-readable report at:
- `hgm-infra/evidence/hgm-rubric-report.json`

## Notes
- Evidence should live under `hgm-infra/`.
- Missing verifiers are **BLOCKED** (not passing).
- If you change what is measured or how it is measured, bump rubric version and update this plan.
