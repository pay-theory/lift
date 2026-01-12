# Lift: Dependency Vulnerability Scan Roadmap (Rubric v0.1)

This workstream exists because dependency vulnerability scanning is currently a **blocking gate** for SEC-2.

## Scope and blockers
- **Workstream:** dependency vulnerability scanning
- **Goal:** implement a pinned, deterministic vulnerability scan that runs in CI
- **Blocking rubric IDs:** SEC-2
- **Primary verifier:** `bash scripts/hgm/vuln-scan.sh`
- **Primary evidence:** CI logs + (preferred) machine-readable report artifact

## Baseline (2026-01-12)
- Current status: **BLOCKED** (`scripts/hgm/vuln-scan.sh` is a placeholder)
- Failure mode: no pinned scanner/version, no CI wiring, no evidence artifact

## Guardrails
- Do not treat “no output” as success; the scan must fail on findings.
- Pin a specific scanner version (no `latest`).
- Keep the scan’s scope stable (no hiding packages).

## Milestones
### WS-1 — Choose and pin scanner
Acceptance criteria:
- Pick a scanner appropriate for Go modules (recommended: `govulncheck`).
- Pin an explicit version (document it in the script).

### WS-2 — Make it deterministic and CI-friendly
Acceptance criteria:
- The scan runs from a clean checkout.
- Uses the repo’s module cache settings as needed.
- Produces stable output (no flaky network dependencies beyond module download).

### WS-3 — Wire into protected-branch CI + archive evidence
Acceptance criteria:
- CI runs the scan on PRs and pushes.
- Evidence is retained as an artifact.

## Suggested implementation sketch (not executed here)
- Update `scripts/hgm/vuln-scan.sh` to run something like:
  - `go run golang.org/x/vuln/cmd/govulncheck@<PINNED_VERSION> ./...`

(Once implemented, bump rubric version if thresholds/scope change.)
