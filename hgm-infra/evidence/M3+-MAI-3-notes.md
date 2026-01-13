# M3+-MAI-3: Canonical Semantics / Duplication Control Enforcement

## Date
2026-01-13

## Summary
Implemented deterministic duplication control enforcement by wiring the `dupl` linter (already configured in `.golangci.yml`) into a fail-closed verifier check. This completes all Maintainability (MAI) category requirements.

## Status Change
- **Before**: MAI-3 = BLOCKED (22 PASS, 0 FAIL, 5 BLOCKED)
- **After**: MAI-3 = PASS (23 PASS, 0 FAIL, 4 BLOCKED)
- **Category Achievement**: All MAI-1, MAI-2, MAI-3 checks now PASSING ✅

## Changes Made

### 1. Implemented Duplication Control Verifier
**File**: `hgm-infra/verifiers/hgm-verify-rubric.sh`

**Function Added**: `canonical_semantics_duplication_check()` (lines 385-411)

**Check Logic** (fail-closed):
1. **Tooling availability**: Validates `golangci-lint` is available in PATH
   - If missing → returns exit code 2 (BLOCKED status)
   - Ensures verifier fails safely if tooling is not present

2. **Anti-drift check**: Verifies `dupl` linter is enabled in `.golangci.yml`
   - Searches for `- dupl` entry under linters.enable section
   - If missing → returns exit code 1 (FAIL status)
   - Prevents configuration dilution/drift

3. **Duplication scan**: Runs dupl-only scan to detect duplicate code
   - Command: `golangci-lint run --enable-only=dupl --config .golangci.yml ./...`
   - Uses `--enable-only` to keep MAI-3 semantically distinct from CON-2 (full lint)
   - Enforces configured threshold: 100 tokens (from `.golangci.yml` line 142)
   - If issues found → returns exit code 1 (FAIL status)

**Verifier Integration**: Updated line 526 from:
```bash
run_check "MAI-3" "Maintainability" "TODO: add canonical semantics/duplication verifier"
```
To:
```bash
run_check "MAI-3" "Maintainability" "canonical_semantics_duplication_check"
```

### 2. Updated Planning Docs (Anti-Drift Alignment)

**Updated Files**:

1. **hgm-infra/planning/lift-10of10-rubric.md** (line 84)
   - **Before**: `TODO: add duplication/singleton checks (currently BLOCKED)`
   - **After**: `` `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh` (MAI-3) ``

2. **hgm-infra/planning/lift-evidence-plan.md** (line 48)
   - **Before**: `TODO (BLOCKED)`
   - **After**: `` `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh` ``

3. **hgm-infra/planning/lift-maintainability-roadmap.md** (lines 55-77)
   - Moved MAI-3 from "Milestones (Planned Work)" to "Guardrails (Currently Enforced)"
   - **Status**: Changed from 🔜 PLANNED → ✅ ENFORCED (as of 2026-01-13)
   - Added verification details, evidence path, and rationale
   - Documents current threshold: 100 tokens

4. **hgm-infra/planning/lift-10of10-roadmap.md** (lines 158-170)
   - Added M3+-MAI-3 progress log entry with implementation details and results

## Verification

### Command Run
```bash
bash ./hgm-infra/verifiers/hgm-verify-rubric.sh
```

### Results
- **MAI-3 Status**: PASS ✅
- **Evidence Log**: `hgm-infra/evidence/MAI-3-output.log`
  - Output: "0 issues."
  - Output: "Canonical semantics OK (duplication within limits)"
- **Rubric Report**: `hgm-infra/evidence/hgm-rubric-report.json` shows MAI-3 = PASS

### Duplication Scan Details
- **Linter**: dupl (via golangci-lint v2.5.0)
- **Threshold**: 100 tokens (configured in `.golangci.yml`)
- **Result**: 0 duplicate code issues found
- **Interpretation**: Codebase currently has no duplication clusters exceeding the 100-token threshold

### Overall Rubric Status
- **Passing**: 23/27 checks (up from 22)
- **Failing**: 0 checks
- **Blocked**: 4 checks (down from 5)
- **Overall**: BLOCKED (no active failures, only TODO items for future features)

## Maintainability Category - Complete! 🎯

All three Maintainability (MAI) checks are now **PASSING**:

| Check | Requirement | Status | Evidence |
|-------|-------------|--------|----------|
| MAI-1 | File-size/complexity budgets enforced | ✅ PASS | File budget OK (<= 1500 lines) |
| MAI-2 | Maintainability roadmap current | ✅ PASS | Maintainability roadmap OK |
| MAI-3 | Canonical implementations (no duplicate semantics) | ✅ PASS | Canonical semantics OK (0 issues) |

This represents a complete, enforced maintainability guardrail system:
- **File budgets** prevent large-file drift
- **Roadmap tracking** ensures maintainability goals are documented and current
- **Duplication control** prevents divergent implementations

## Acceptance Criteria Met
✅ MAI-3 is PASS in hgm-infra/evidence/hgm-rubric-report.json
✅ MAI-3 verifier is deterministic and fails closed
✅ No application code changes (governance-only)
✅ Planning docs aligned with verifier (no drift)
✅ No verifier dilution (dupl enabled, threshold unchanged)

## Files Modified

### Updated
- `hgm-infra/verifiers/hgm-verify-rubric.sh` (added canonical_semantics_duplication_check function, updated MAI-3 check)
- `hgm-infra/planning/lift-10of10-rubric.md` (MAI-3 verification command)
- `hgm-infra/planning/lift-evidence-plan.md` (MAI-3 refresh command)
- `hgm-infra/planning/lift-maintainability-roadmap.md` (MAI-3 moved to Guardrails, marked ENFORCED)
- `hgm-infra/planning/lift-10of10-roadmap.md` (added M3+-MAI-3 progress log entry)

### Created
- `hgm-infra/evidence/M3+-MAI-3-notes.md` (this file)

### Refreshed Evidence
- `hgm-infra/evidence/MAI-3-output.log` (now contains dupl scan output)
- `hgm-infra/evidence/hgm-rubric-report.json` (MAI-3 status updated to PASS)

## Configuration Details

### Duplication Threshold
From `.golangci.yml` line 142:
```yaml
dupl:
  threshold: 100
```

This threshold means:
- Duplicate code blocks with 100+ tokens are flagged
- Token = semantic code unit (not lines or characters)
- Threshold is intentionally strict for AI-heavy development
- No exceptions or exclusions configured

### Linter Configuration
The `dupl` linter is:
- ✅ Enabled in `.golangci.yml` line 39
- ❌ Explicitly disabled for test files (line 60) - test duplication is acceptable
- 🔍 Run in isolation via `--enable-only=dupl` for MAI-3 check
- 🔍 Also included in full CON-2 lint run for comprehensive checking

## Remaining Blockers (4)
- **QUA-2**: Contract/integration tests (requires contract testing framework)
- **CON-3**: Contract parity checks (requires contract testing implementation)
- **COM-6**: Logging/operational standards (requires logging standards doc)
- **SEC-4**: Domain P0 regression tests (requires P0 test suite)

These are TODO placeholders for features not yet implemented. **All implementable rubric checks are now passing!**

## Future Work (If Needed)
If duplication is found in future development:
1. Review dupl findings in evidence log
2. Identify highest-signal duplication clusters
3. Extract shared abstractions or utilities
4. Consider threshold adjustment only if justified (requires rubric version bump)
5. Document intentional duplication with architectural justification

## Notes
- No application code changes were required for this step (codebase already clean)
- Threshold of 100 tokens is appropriate for this codebase size and AI-assisted development
- Dupl scan completed cleanly with 0 issues - excellent baseline
