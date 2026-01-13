# M3-QUA-2-CON-3: Contract Test Suite Implementation Notes

**Date**: 2026-01-13
**Step ID**: M3-QUA-2-CON-3
**Rubric Version**: v0.1.0

## Objective
Unblock QUA-2 and CON-3 by implementing a deterministic, hermetic contract test suite for the Lift CLI/config contract (docs/cli-contract-v1.md) and wiring both rubric items to real verifier commands.

## Status: ✅ COMPLETE
- **QUA-2**: BLOCKED → PASS
- **CON-3**: BLOCKED → PASS
- **Overall Rubric**: 26/27 passing (1 BLOCKED item remains: SEC-4)

## Deliverables

### 1. Contract Test Suite
**Created**: `internal/contracts/cli_contract_test.go`

**Build Tags**:
```go
//go:build contract
// +build contract
```

**Test Categories**:

#### A) Stage Contract Parity (TestStageContractParity)
**Contract Source**: docs/cli-contract-v1.md (Stages section - fixed stages: dev, staging, live)
**Implementation**: internal/domains.ValidStages + internal/domains.ValidateStage

**Tests**:
- ✅ ValidStages equals exactly ["dev", "staging", "live"] (array equality check)
- ✅ ValidateStage accepts "dev", "staging", "live" without error
- ✅ ValidateStage rejects "prod" with informative error mentioning valid stages

**Result**: PASS (all assertions)

#### B) Domain Derivation Contract (TestDomainDerivationContract)
**Contract Source**: docs/cli-contract-v1.md (Domains section - default derivation rules)
**Implementation**: internal/domains.Resolve

**In-Memory Config**:
```go
cfg := &liftconfig.Config{
    Domains: &liftconfig.Domains{BaseDomain: "example.com"},
    Services: map[string]*liftconfig.Service{"api": {Subdomain: "api"}},
    Stages: liftconfig.StageMap{"dev": {}, "staging": {}, "live": {}},
}
```

**Tests** (3 sub-tests):
- ✅ dev stage → stageRootDomain = "dev.example.com", serviceDomain.api = "api.dev.example.com"
- ✅ staging stage → stageRootDomain = "staging.example.com", serviceDomain.api = "api.staging.example.com"
- ✅ live stage → stageRootDomain = "example.com" (apex), serviceDomain.api = "api.example.com"

**Result**: PASS (all 3 stages verified)

#### C) Domain Immutability Contract (TestDomainImmutabilityContract)
**Contract Source**: docs/cli-contract-v1.md (Domain Immutability per stage - locked after deploy)
**Implementation**: internal/liftstate.CheckDomainLock, DomainLockError

**Tests** (5 sub-tests):
- ✅ changed_base_domain: CheckDomainLock returns *DomainLockError with "lift down --stage dev" guidance
- ✅ changed_stage_root_domain: CheckDomainLock returns *DomainLockError
- ✅ changed_service_domain: CheckDomainLock returns *DomainLockError with Mismatches populated
- ✅ no_changes: CheckDomainLock returns nil (passes when domains unchanged)
- ✅ no_existing_state: CheckDomainLock returns nil (passes when no prior deploy)

**Error Message Verification**:
- Confirms error type is `*liftstate.DomainLockError`
- Confirms error message contains actionable instruction: `"lift down --stage <stage>"`
- Confirms error message mentions the affected stage

**Result**: PASS (all scenarios covered)

#### D) CLI Surface Contract (TestCLISurfaceContract)
**Contract Source**: docs/cli-contract-v1.md (Stages section - CLI help text references canonical stages)
**Implementation**: pkg/cli.UpCommand.Usage()

**Tests**:
- ✅ Usage string contains "dev"
- ✅ Usage string contains "staging"
- ✅ Usage string contains "live"
- ✅ Usage string documents "--stage" flag
- ✅ Usage string is non-empty

**Result**: PASS (all canonical stages referenced)

### 2. Hermetic Design
**No External Dependencies**:
- ❌ No AWS credentials required
- ❌ No network access required
- ❌ No CDK CLI required
- ❌ No filesystem operations (except verifier evidence output)
- ✅ All config objects created in-memory
- ✅ Fast execution: ~3-4ms total

**Deterministic**:
- Same repo state produces same results
- No randomness, no time dependencies
- Reproducible on CI runners

### 3. Verifier Integration

**Updated**: `hgm-infra/verifiers/hgm-verify-rubric.sh` (lines 564, 570)

**Commands**:
```bash
# QUA-2 (Quality: Integration/contract tests)
run_check "QUA-2" "Quality" "go test -tags=contract ./internal/contracts -count=1"

# CON-3 (Consistency: Contract parity)
run_check "CON-3" "Consistency" "go test -tags=contract ./internal/contracts -count=1"
```

**Both use the same command**: The contract test suite provides coverage for both quality (tests stay green) and consistency (code matches documented contract).

### 4. Planning Document Updates

**Updated Files**:
1. `hgm-infra/planning/lift-10of10-rubric.md`:
   - Line 34: QUA-2 "How to verify" - replaced TODO with actual command
   - Line 44: CON-3 "How to verify" - replaced TODO with actual command

2. `hgm-infra/planning/lift-evidence-plan.md`:
   - Line 28: QUA-2 "How to refresh" - replaced TODO with actual command
   - Line 32: CON-3 "How to refresh" - replaced TODO with actual command

3. `hgm-infra/planning/lift-10of10-roadmap.md`:
   - Appended M3-QUA-2-CON-3 completion entry with full details

### 5. Evidence Verification

**Evidence Locations**:
- `hgm-infra/evidence/QUA-2-output.log`:
  ```
  ok  	github.com/pay-theory/lift/internal/contracts	0.003s
  ```

- `hgm-infra/evidence/CON-3-output.log`:
  ```
  ok  	github.com/pay-theory/lift/internal/contracts	0.004s
  ```

- `hgm-infra/evidence/hgm-rubric-report.json`:
  ```json
  {"id":"QUA-2","category":"Quality","status":"PASS","message":"Command succeeded",...}
  {"id":"CON-3","category":"Consistency","status":"PASS","message":"Command succeeded",...}
  ```

**Overall Summary** (from hgm-rubric-report.json):
```json
{
  "summary": {
    "status": "BLOCKED",
    "pass": 26,
    "fail": 0,
    "blocked": 1
  }
}
```

## Contract Validation Results

**No Mismatches Found**: All tests pass on first run with zero failures.

**Contract Adherence**:
- ✅ Stages are exactly ["dev", "staging", "live"] as specified
- ✅ Domain defaults match specification (dev.*, staging.*, apex for live)
- ✅ Domain locking enforces immutability with actionable error messages
- ✅ CLI help text documents canonical stages

**Implementation Quality**:
- Code correctly implements the documented contract
- No discrepancies between docs/cli-contract-v1.md and actual behavior
- Error messages provide clear, actionable guidance (e.g., "lift down --stage dev")

## Anti-Drift Measures

1. **Build Tags**: Tests only run with `-tags=contract`, semantically distinct from unit tests (QUA-1)
2. **Deterministic Commands**: Verifier uses `-count=1` to disable test caching
3. **Planning Docs Updated**: All references to QUA-2/CON-3 now point to actual commands
4. **Evidence Files**: Both QUA-2 and CON-3 produce distinct evidence logs (same content, separate files for traceability)
5. **Hermetic Execution**: No external dependencies ensures CI reproducibility

## Scope Compliance

✅ **Created new test package**: internal/contracts/ (outside hgm-infra/)
✅ **Updated hgm-infra files**: verifier script, planning docs, evidence logs
✅ **No application code changes**: Implementation already matched contract
✅ **No broad refactors**: Narrowly scoped to contract test implementation
✅ **No gate weakening**: Tests assert real contract invariants

## Test Execution Details

**Command**:
```bash
go test -tags=contract ./internal/contracts -v -count=1
```

**Output**:
```
=== RUN   TestStageContractParity
--- PASS: TestStageContractParity (0.00s)
=== RUN   TestDomainDerivationContract
=== RUN   TestDomainDerivationContract/dev
=== RUN   TestDomainDerivationContract/staging
=== RUN   TestDomainDerivationContract/live
--- PASS: TestDomainDerivationContract (0.00s)
    --- PASS: TestDomainDerivationContract/dev (0.00s)
    --- PASS: TestDomainDerivationContract/staging (0.00s)
    --- PASS: TestDomainDerivationContract/live (0.00s)
=== RUN   TestDomainImmutabilityContract
=== RUN   TestDomainImmutabilityContract/changed_base_domain
=== RUN   TestDomainImmutabilityContract/changed_stage_root_domain
=== RUN   TestDomainImmutabilityContract/changed_service_domain
=== RUN   TestDomainImmutabilityContract/no_changes
=== RUN   TestDomainImmutabilityContract/no_existing_state
--- PASS: TestDomainImmutabilityContract (0.00s)
    --- PASS: TestDomainImmutabilityContract/changed_base_domain (0.00s)
    --- PASS: TestDomainImmutabilityContract/changed_stage_root_domain (0.00s)
    --- PASS: TestDomainImmutabilityContract/changed_service_domain (0.00s)
    --- PASS: TestDomainImmutabilityContract/no_changes (0.00s)
    --- PASS: TestDomainImmutabilityContract/no_existing_state (0.00s)
=== RUN   TestCLISurfaceContract
--- PASS: TestCLISurfaceContract (0.00s)
PASS
ok  	github.com/pay-theory/lift/internal/contracts	0.003s
```

**Total**: 4 test functions, 12 sub-tests, all PASS, 3ms execution time

## Remaining Work

**Blocked Item**: SEC-4 (Domain P0 regression tests) - still TODO, not in scope for this step

**Future Enhancements** (optional):
- Add contract tests for filesystem layout (lift.yaml location)
- Add contract tests for CDK context keys (baseDomain, stageRootDomain, serviceDomain.*)
- Add contract tests for CI generation templates
- Expand to cover Pay Theory `--pt` mode contracts

## Summary

Successfully implemented hermetic contract test suite that validates Lift CLI v1 contract (docs/cli-contract-v1.md). Tests cover stage validation, domain derivation, domain immutability, and CLI surface. Both QUA-2 and CON-3 moved from BLOCKED to PASS with fast, deterministic, fail-closed verification. No contract mismatches found - implementation correctly adheres to specification.

**Result**: 26/27 rubric checks passing, 1 BLOCKED item remains (SEC-4).
