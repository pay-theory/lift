# M1-VER-1 Iteration Notes

## Changes
1. **Verifier Stability (`hgm-infra/verifiers/hgm-verify-rubric.sh`)**:
   - Updated `CON-2` (Lint) to use `--allow-parallel-runners`. This fixes the "parallel golangci-lint is running" error caused by cache/lock contention.
   - Updated `SEC-1` (Security) to use `--enable-only=gosec` instead of `--disable-all --enable=gosec` (which failed with "unknown flag"). Also added `--allow-parallel-runners`.

2. **Planning Alignment**:
   - Updated `hgm-infra/planning/lift-10of10-rubric.md` and `hgm-infra/planning/lift-evidence-plan.md` to match the corrected verifier commands. This ensures documentation accurately reflects the source of truth.

## Status Updates
- **CON-2 (Lint):** FAIL (Execution Error) → PASS (0 issues).
- **SEC-1 (Security):** FAIL (Flag Error) → PASS (0 issues).
- **Overall Report:** PASS count increased from 16 to 18.

## Remaining Failures (Out of Scope)
- `CON-1` (gofmt): Files need formatting.
- `COM-1` (Compile): Examples fail to compile (dependency issues).
- `MAI-1` (File Budget): Some files exceed line limits.
