# Roadmap Progress: M3-MAI-1

Date: 2026-01-13T17:49:53Z

Baseline MAI-1 violations (from initial MAI-1-output.log):
- pkg/cli/dynamorm_commands.go:2311
- pkg/lift/app.go:1992
- pkg/testing/enterprise/types.go:1824
- pkg/testing/mocks.go:1736

Files split and new files created:
- pkg/cli/dynamorm_commands.go split into:
  - pkg/cli/dynamorm_scaffold.go
  - pkg/cli/dynamorm_migrate.go
  - pkg/cli/dynamorm_migrate_templates.go
  - pkg/cli/dynamorm_commands.go (stub placeholder)
- pkg/lift/app.go split into:
  - pkg/lift/app_handlers.go
- pkg/testing/enterprise/types.go split into:
  - pkg/testing/enterprise/types_contracts.go
  - pkg/testing/enterprise/types_chaos_helpers.go
- pkg/testing/mocks.go split into:
  - pkg/testing/mocks_cloudwatch.go

Notes:
- Changes are refactor-only (file splits and moves within the same packages; no intended behavior changes).

Final verifier summary:
- Status: BLOCKED
- Pass: 21
- Fail: 0
- Blocked: 6
- Report: hgm-infra/evidence/hgm-rubric-report.json
- MAI-1 evidence: hgm-infra/evidence/MAI-1-output.log
