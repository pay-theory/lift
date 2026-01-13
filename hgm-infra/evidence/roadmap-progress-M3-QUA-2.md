# Roadmap Progress: M3-QUA-2

Date: 2026-01-13T19:18:07Z

## Contract tests added
- Contract doc invariants: file exists, includes `version: 1`, and stage keys `dev`, `staging`, `live`.
- Template invariants for required templates and PT variants: `version: 1`, `app:` with `template: <name>`, `stages:` with `dev/staging/live`, and `domains:` with `base_domain:`.

## Verifier command
- `go test ./internal/contracttests -v`

## Final status
- QUA-2: PASS
- Evidence:
  - `hgm-infra/evidence/QUA-2-output.log`
  - `hgm-infra/evidence/hgm-rubric-report.json`

## Follow-up ideas
- Expand contract tests to cover additional CLI contract keys (domains/root_domain invariants, services map schema).
