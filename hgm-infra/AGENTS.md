# Agent Instructions (Hypergenium)

Scope: this file applies to `hgm-infra/**`.

## Start Here

1) Read `hgm-infra/README.md`.
2) Run the deterministic verifier from repo root:
   - `bash hgm-infra/verifiers/hgm-verify-rubric.sh`
3) Inspect results:
   - `hgm-infra/evidence/hgm-rubric-report.json`
   - `hgm-infra/evidence/*-output.log`

## Constraints

- Keep changes under `hgm-infra/` unless explicitly asked to modify application code.
- Do not weaken gates (no threshold reductions, no excludes, no disabling checks).
- If a verifier cannot be executed deterministically, return `BLOCKED` rather than guessing.
- Do not make scripts executable automatically; run them via `bash`.
- Do not introduce secrets.

