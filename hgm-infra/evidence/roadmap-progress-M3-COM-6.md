# Roadmap Progress: M3-COM-6

Date: 2026-01-13T19:33:28Z

## COM-6 policy (v0.1.0)
- Scope: Go files in `pkg/**` and `cmd/**`
- Ignore: `*_test.go`
- Forbidden: fmt.Print/Printf/Println, log.Print/Printf/Println/Fatal*/Panic*, builtin println
- Output: file path + line number + rule name

## Files changed (hgm-infra only)
- `hgm-infra/verifiers/com6-logging-standards-check.go`
- `hgm-infra/verifiers/hgm-verify-rubric.sh`
- `hgm-infra/planning/lift-10of10-rubric.md`
- `hgm-infra/planning/lift-evidence-plan.md`

## COM-6 status
- Status: FAIL
- Evidence:
  - `hgm-infra/evidence/COM-6-output.log`
  - `hgm-infra/evidence/hgm-rubric-report.json`

## Top violations (paths only)
- `pkg/cli/commands.go`
- `pkg/cli/cdk_commands.go`
- `pkg/cli/new_command.go`
- `pkg/compliance/gdpr_complete.go`
- `pkg/dev/server.go`
- `pkg/security/audit.go`
- `pkg/testing/scenarios.go`
- `pkg/observability/cloudwatch/metrics.go`

## Follow-up
- Proposed remediation step: M3-COM-6-REM-1 (replace direct fmt/log/println usage with structured logging/redaction APIs).
