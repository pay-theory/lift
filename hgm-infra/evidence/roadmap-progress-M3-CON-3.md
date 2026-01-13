# Roadmap Progress: M3-CON-3

Date: 2026-01-13T18:26:30Z

## Previous status
- CON-3 was BLOCKED because the verifier was TODO and emitted a placeholder log.

## Contract invariants enforced (v1)
- Contract doc exists at `docs/cli-contract-v1.md`.
- Contract doc explicitly lists stage keys `dev`, `staging`, `live`.
- Contract doc includes a required `version: 1` statement/example.
- Each required template has both standard and PT lift templates.
- Each `lift.yaml.tmpl` includes `version: 1`, `app:` with `template: <name>`, `stages:` with `dev/staging/live`, and `domains:` with `base_domain:`.

## Paths checked
- `docs/cli-contract-v1.md`
- `internal/templates/basic-api/lift.yaml.tmpl`
- `internal/templates/basic-api-pt/lift.yaml.tmpl`
- `internal/templates/microservice/lift.yaml.tmpl`
- `internal/templates/microservice-pt/lift.yaml.tmpl`
- `internal/templates/event-driven/lift.yaml.tmpl`
- `internal/templates/event-driven-pt/lift.yaml.tmpl`
- `internal/templates/merchant-app/lift.yaml.tmpl`
- `internal/templates/merchant-app-pt/lift.yaml.tmpl`
- `internal/templates/sns-processor/lift.yaml.tmpl`
- `internal/templates/sns-processor-pt/lift.yaml.tmpl`

## Verifier summary
- Status: BLOCKED
- Pass: 23
- Fail: 0
- Blocked: 4

Evidence:
- `hgm-infra/evidence/hgm-rubric-report.json`
- `hgm-infra/evidence/CON-3-output.log`
