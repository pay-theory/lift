# M3-CON-3 Iteration Notes

## Changes
1. **Implemented CON-3 Verifier**:
   - Added `check_cli_template_contract_parity` to `hgm-infra/verifiers/hgm-verify-rubric.sh`.
   - This function enforces the `docs/cli-contract-v1.md` requirements for all templates in `internal/templates/`.
   - It checks for the existence of `lift.yaml`, `go.mod`, `cdk/`, `cmd/`, and CI assets (GitHub Actions or PT Buildspec/Shell), allowing for `.tmpl` extensions.

2. **Verifier Wiring**:
   - Updated `hgm-infra/verifiers/hgm-verify-rubric.sh` to use `check_cli_template_contract_parity` for the `CON-3` check, replacing the "TODO".

## Status Updates
- **CON-3 (Public boundary contract parity):** BLOCKED → FAIL.
  - The verifier is now configured (unblocked), but it correctly reports failures for the Pay Theory (`*-pt`) templates.
  - **Standard Templates (basic-api, event-driven, etc.):** PASSED.
  - **PT Templates (*-pt):** FAILED.
    - Missing `go.mod.tmpl` (Project Root contract).
    - Missing `cmd/` directory (Filesystem Layout contract).

## Next Steps
- Propose a follow-up task to fix the `*-pt` templates to comply with the v1 contract (add missing `go.mod.tmpl` and `cmd/` scaffold).
- Alternatively, if these templates are intended to be partial overlays, update the contract `docs/cli-contract-v1.md` to reflect that.
