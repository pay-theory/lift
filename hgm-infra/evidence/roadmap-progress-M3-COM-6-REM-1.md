# Roadmap Progress: M3-COM-6-REM-1

Date: 2026-01-13T20:26:48Z

## COM-6 remediation summary
- Baseline violations: 387 (from prior COM-6 checker output).
- Final violations: 0 (COM-6 checker reports no violations; 288 files checked).
- Converted direct fmt.Print*/log.* usage to structured stdout/stderr helpers and removed stdlib log imports.
- Added a small stdio helper to handle stdout/stderr writes with checked errors.

## Changes applied
- Replaced fmt.Print*/Printf/Println and log.* usages in pkg/** and cmd/** with stdio helpers (stdout/stderr).
- Added `pkg/utils/stdio/stdio.go` to centralize stdout/stderr writes with error handling.
- Left response-writer output (e.g., HTTP handlers) intact but ensured stdout/stderr logging uses the helper.

## Sensitive logging decisions
- Preserved existing message content and routing to stdout/stderr; no new sensitive fields introduced.
- Avoided adding structured logging fields that could inadvertently include secrets.

## Evidence
- `hgm-infra/evidence/COM-6-output.log`
- `hgm-infra/evidence/hgm-rubric-report.json`
