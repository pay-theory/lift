# Lift: Maintainability Roadmap (Rubric v0.1.0)

This roadmap tracks maintainability guardrails and convergence work for the Lift serverless framework against **Rubric v0.1.0**.

## Purpose

Maintainability guardrails prevent technical debt accumulation and ensure the codebase remains navigable for AI-assisted development and human review. These constraints are enforced via deterministic verifiers that fail closed when budgets are exceeded.

## Guardrails (Currently Enforced)

### MAI-1: File-Size/Complexity Budgets
**Status**: ✅ ENFORCED (as of 2026-01-13)

**Rule**: No non-test Go source file under `pkg/**`, `cmd/**`, or `internal/**` may exceed 1500 lines.

**Rationale**:
- Large files are difficult to navigate and comprehend for both humans and AI systems
- Splitting large files forces clearer module boundaries and separation of concerns
- Budget serves as an early warning for single-responsibility violations

**Verification**:
```bash
bash ./hgm-infra/verifiers/hgm-verify-rubric.sh
```

The MAI-1 verifier check scans all non-test Go files and fails if any file exceeds the 1500 line threshold.

**Recent Actions**:
- 2026-01-13: Resolved 4 file budget violations (7863 lines across 4 files) by splitting into 9 files
  - `pkg/cli/dynamorm_commands.go` (2311→3 files)
  - `pkg/lift/app.go` (1992→2 files)
  - `pkg/testing/enterprise/types.go` (1824→2 files)
  - `pkg/testing/mocks.go` (1736→2 files)

### MAI-2: Maintainability Roadmap Current
**Status**: ✅ ENFORCED (as of 2026-01-13)

**Rule**: A versioned maintainability roadmap document (this file) must exist and must reference the active rubric version ("Rubric v0.1.0").

**Rationale**:
- Maintainability guardrails require explicit documentation and stakeholder visibility
- Roadmap versioning prevents drift between enforcement and planning artifacts
- Fail-closed verification ensures the roadmap stays current as the rubric evolves

**Verification**:
```bash
bash ./hgm-infra/verifiers/hgm-verify-rubric.sh
```

The MAI-2 verifier check validates:
- This file (`hgm-infra/planning/lift-maintainability-roadmap.md`) exists
- File contains the active rubric version string: "Rubric v0.1.0"
- File contains required section markers: "Guardrails", "Milestones", "Progress log"

### MAI-3: Canonical Semantics (Duplication Control)
**Status**: ✅ ENFORCED (as of 2026-01-13)

**Rule**: Code duplication must remain within configured thresholds to prevent divergent implementations of similar logic.

**Rationale**:
- Copy-paste code creates multiple sources of truth for similar logic
- Duplicated code increases cognitive load and makes refactoring risky
- AI coding assistants can amplify duplication patterns if not constrained
- Enforcing duplication limits promotes the creation of shared abstractions

**Verification**:
```bash
bash ./hgm-infra/verifiers/hgm-verify-rubric.sh
```

The MAI-3 verifier check:
- Validates `golangci-lint` is available (fails with BLOCKED if missing)
- Ensures `dupl` linter is enabled in `.golangci.yml` (anti-drift check)
- Runs dupl-only scan: `golangci-lint run --enable-only=dupl --config .golangci.yml ./...`
- Fails if duplication exceeds configured threshold (currently 100 tokens in `.golangci.yml`)

**Evidence**: `hgm-infra/evidence/MAI-3-output.log`

## Milestones (Planned Work)

### Future Maintainability Work
Additional maintainability constraints may be added in future rubric versions as the project matures:
- Cyclomatic complexity budgets per function
- Maximum nesting depth limits
- Naming convention enforcement (linter-based)
- Module dependency graph acyclicity checks

## Progress Log

### 2026-01-13: Roadmap Established (MAI-2)
- Created this maintainability roadmap document
- Wired MAI-2 deterministic verifier check
- Status: MAI-2 moved from BLOCKED → PASS

### 2026-01-13: File Budgets Enforced (MAI-1)
- Resolved 4 file budget violations by splitting into 9 smaller files
- All files now under 1500 line threshold
- Status: MAI-1 moved from FAIL → PASS
- Evidence: `hgm-infra/evidence/MAI-1-output.log`, `hgm-infra/evidence/M3+-MAI-1-notes.md`

## References

- Primary rubric: `hgm-infra/planning/lift-10of10-rubric.md`
- Evidence plan: `hgm-infra/planning/lift-evidence-plan.md`
- Verifier entrypoint: `hgm-infra/verifiers/hgm-verify-rubric.sh`
- Roadmap tracker: `hgm-infra/planning/lift-10of10-roadmap.md`
