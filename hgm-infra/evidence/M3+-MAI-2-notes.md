# M3+-MAI-2: Maintainability Roadmap Enforcement

## Date
2026-01-13

## Summary
Unblocked MAI-2 by creating a versioned maintainability roadmap planning artifact and wiring a deterministic verifier check for it. This is a governance-only step with no application code changes.

## Status Change
- **Before**: MAI-2 = BLOCKED (22 PASS, 0 FAIL, 6 BLOCKED)
- **After**: MAI-2 = PASS (22 PASS, 0 FAIL, 5 BLOCKED)

## Changes Made

### 1. Created Maintainability Roadmap
**File**: `hgm-infra/planning/lift-maintainability-roadmap.md`

**Content**:
- Title: "Lift: Maintainability Roadmap (Rubric v0.1.0)"
- Explicitly references active rubric version: "Rubric v0.1.0"
- Required sections:
  - **Guardrails** (Currently Enforced):
    - MAI-1: File-size/complexity budgets (1500 line limit)
    - MAI-2: Maintainability roadmap current (this document)
  - **Milestones** (Planned Work):
    - MAI-3: Canonical semantics (duplication control) - PLANNED
    - Future work: complexity budgets, nesting limits, naming conventions
  - **Progress log**: Records all maintainability milestone completions

**Purpose**: Provides explicit documentation of maintainability guardrails, prevents drift between enforcement and planning, ensures stakeholder visibility.

### 2. Wired Deterministic Verifier Check
**File**: `hgm-infra/verifiers/hgm-verify-rubric.sh`

**Function Added**: `maintainability_roadmap_check()` (lines 350-383)

**Check Logic** (fail-closed):
- ✅ File exists: `hgm-infra/planning/lift-maintainability-roadmap.md`
- ✅ Contains active rubric version: "Rubric v0.1.0"
- ✅ Contains required sections: "Guardrails", "Milestones", "Progress log"
- ❌ Fails if any condition is not met

**Verifier Integration**: Updated line 497 from:
```bash
run_check "MAI-2" "Maintainability" "TODO: add maintainability roadmap/verifier"
```
To:
```bash
run_check "MAI-2" "Maintainability" "maintainability_roadmap_check"
```

### 3. Updated Planning Docs (Anti-Drift Alignment)

**Updated Files**:
1. `hgm-infra/planning/lift-10of10-rubric.md` (line 83)
   - **Before**: `TODO: add maintainability plan + verifier (currently BLOCKED)`
   - **After**: `` `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh` (MAI-2) ``

2. `hgm-infra/planning/lift-evidence-plan.md` (line 47)
   - **Before**: `TODO (BLOCKED)`
   - **After**: `` `bash ./hgm-infra/verifiers/hgm-verify-rubric.sh` ``

3. `hgm-infra/planning/lift-10of10-roadmap.md` (line 63)
   - Added to workstream tracking docs list:
     - `Maintainability: hgm-infra/planning/lift-maintainability-roadmap.md`

## Verification

### Command Run
```bash
bash ./hgm-infra/verifiers/hgm-verify-rubric.sh
```

### Results
- **MAI-2 Status**: PASS ✅
- **Evidence Log**: `hgm-infra/evidence/MAI-2-output.log` shows "Maintainability roadmap OK"
- **Rubric Report**: `hgm-infra/evidence/hgm-rubric-report.json` shows MAI-2 = PASS

### Overall Rubric Status
- **Passing**: 22/27 checks (up from 21)
- **Failing**: 0 checks
- **Blocked**: 5 checks (down from 6)
- **Overall**: BLOCKED (no active failures, only TODO items)

## Acceptance Criteria Met
✅ MAI-2 is PASS in hgm-infra/evidence/hgm-rubric-report.json
✅ MAI-2 verifier is deterministic and fails closed if roadmap is missing/mismatched
✅ No code changes outside hgm-infra/** (governance-only)
✅ Planning docs updated to reference actual verifier (no drift)

## Files Modified

### Created
- `hgm-infra/planning/lift-maintainability-roadmap.md`
- `hgm-infra/evidence/M3+-MAI-2-notes.md` (this file)

### Updated
- `hgm-infra/verifiers/hgm-verify-rubric.sh` (added maintainability_roadmap_check function, updated MAI-2 check)
- `hgm-infra/planning/lift-10of10-rubric.md` (MAI-2 verification command)
- `hgm-infra/planning/lift-evidence-plan.md` (MAI-2 refresh command)
- `hgm-infra/planning/lift-10of10-roadmap.md` (added maintainability to workstream tracking docs)

### Refreshed
- `hgm-infra/evidence/MAI-2-output.log` (now contains actual check output)
- `hgm-infra/evidence/hgm-rubric-report.json` (MAI-2 status updated)

## Scope Compliance
✅ Only files under hgm-infra/** were modified
✅ No application code changes (pkg/**, cmd/**, internal/**, examples/**)
✅ No rubric weakening or dilution
✅ Verifier remains deterministic and fail-closed

## Next Steps
Remaining BLOCKED items (5):
- QUA-2: Contract/integration tests
- CON-3: Contract parity checks
- COM-6: Logging/operational standards
- SEC-4: Domain P0 regression tests
- MAI-3: Canonical semantics (duplication control)

These require additional tooling, configuration, or future milestone work.
