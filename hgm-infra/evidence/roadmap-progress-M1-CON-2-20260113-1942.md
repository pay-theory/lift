# Roadmap Progress: M1-CON-2

Date: 2026-01-13T19:42:20Z

## Issue
- golangci-lint/gosec/govulncheck failed due to duplicate main/collectGoFiles symbols in `hgm-infra/verifiers` (multiple main packages in one directory).

## Changes
- Moved verifier tools into isolated cmd packages:
  - `hgm-infra/verifiers/mai3-dupcheck.go` -> `hgm-infra/verifiers/cmd/mai3-dupcheck/main.go`
  - `hgm-infra/verifiers/com6-logging-standards-check.go` -> `hgm-infra/verifiers/cmd/com6-logging-standards-check/main.go`
- Updated `hgm-infra/verifiers/hgm-verify-rubric.sh` to run the new cmd paths.
- Cleaned minor govet issues in `hgm-infra/verifiers/cmd/com6-logging-standards-check/main.go` (field alignment, shadowed identifiers) without changing behavior.

## Commands run
- `go run ./hgm-infra/verifiers/cmd/mai3-dupcheck`
- `go run ./hgm-infra/verifiers/cmd/com6-logging-standards-check`
- `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh`

## Evidence
- `hgm-infra/evidence/CON-2-output.log`
- `hgm-infra/evidence/SEC-1-output.log`
- `hgm-infra/evidence/SEC-2-output.log`
- `hgm-infra/evidence/hgm-rubric-report.json`
