# Lift Evidence Plan (Rubric v0.1)

This plan defines **where evidence is produced** for rubric items and how to regenerate it from a commit SHA.

Principle: evidence should be reproducible via commands, not assembled manually.

## Evidence sources

### CI artifacts (preferred)
Current CI runs tests, lint, and CDK checks. Coverage/vuln/provenance artifacts are not yet consistently uploaded.

Recommended future artifacts:
- Coverage profile(s): `coverage.out` and `coverage.core.out`
- Lint logs + (optional) SARIF output for gosec/staticcheck
- Vulnerability scan report output (govulncheck)
- Supply chain verification logs

### Deterministic in-repo artifacts (planning docs)
- Controls matrix: `docs/planning/lift-controls-matrix.md`
- Threat model: `docs/planning/lift-threat-model.md`
- Rubric: `docs/planning/lift-10of10-rubric.md`
- Roadmap: `docs/planning/lift-10of10-roadmap.md`
- Evidence plan: `docs/planning/lift-evidence-plan.md`
- AI drift recovery: `docs/planning/lift-ai-drift-recovery.md`
- Pack metadata: `docs/planning/lift-hgm-pack.json`

## Rubric → evidence map

| Rubric ID | Primary evidence | How to refresh |
| --- | --- | --- |
| QUA-1 | Test output logs | `./scripts/ci-check.sh` |
| QUA-2 | CDK test + synth output | `go test -v ./pkg/cdk/...` (and CI CDK steps) |
| QUA-3 | Coverage profiles + summary | `bash scripts/hgm/coverage-threshold-check.sh` (writes `coverage.out`, `coverage.core.out`) |
| CON-1 | gofmt diff list | `bash scripts/hgm/fmt-check.sh` |
| CON-2 | golangci-lint output (pinned v2.4.0 in CI) | `golangci-lint run --timeout=10m --config .golangci.yml ./...` |
| CON-3 | Contract test logs | `bash scripts/hgm/contract-parity-check.sh` (TODO) |
| COM-1 | Module compilation sweep logs | `bash scripts/hgm/modules-check.sh` (TODO) |
| COM-2 | Toolchain alignment report | `bash scripts/hgm/toolchain-check.sh` |
| COM-3 | Lint config verify output | `bash scripts/hgm/lint-config-check.sh` (TODO) |
| COM-4 | Coverage threshold output | `bash scripts/hgm/coverage-threshold-check.sh` |
| COM-5 | Security config drift output | `bash scripts/hgm/sec-config-check.sh` |
| COM-6 | Logging/ops invariants output | `bash scripts/hgm/logging-check.sh` (TODO) |
| SEC-1 | SAST output (minimum gosec findings) | `bash scripts/hgm/sast-check.sh` (TODO) |
| SEC-2 | govulncheck output | `bash scripts/hgm/vuln-scan.sh` (pinned via env) |
| SEC-3 | Supply-chain check output | `bash scripts/hgm/supply-chain-check.sh` |
| SEC-4 | P0 regression test output | `bash scripts/hgm/p0-check.sh` (TODO) |
| CMP-1..3 | Planning docs check output | `bash scripts/hgm/planning-docs-check.sh` |
| DOC-4 | Doc integrity check output | `bash scripts/hgm/doc-integrity-check.sh` |
| DOC-5 | Threat/control parity check output | `bash scripts/hgm/threat-controls-parity.sh` |

## Notes
- This repo is under a **custom** domain overlay: do not store licensed standards text in-repo.
- If evidence generation changes (new artifact paths, thresholds, or tool versions), bump the rubric version and update
  this plan.
