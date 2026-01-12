# Lift: Supply Chain Hardening Roadmap (Rubric v0.1)

This workstream exists because supply-chain integrity is security-critical for Lift and is partially addressed (SEC-3
is currently only a baseline).

## Scope and blockers
- **Workstream:** supply-chain hardening
- **Goal:** ensure build/release inputs are pinned and outputs are verifiable
- **Blocking rubric IDs:** SEC-3 (and indirectly COM-2)
- **Primary verifier:** `bash scripts/hgm/supply-chain-check.sh`
- **Primary evidence:** script output + CI logs + release artifacts

## Baseline (2026-01-12)
- Current status: **partial**
  - GitHub Actions are pinned to commit SHAs in workflows (verified by `supply-chain-check.sh`).
  - Release workflow produces checksums (`dist/checksums.txt`).
- Gaps:
  - No provenance/attestation (SLSA) wiring yet
  - No artifact signing policy yet (cosign, etc.)

## Guardrails
- Pins must be immutable (commit SHAs or digest-pinned containers).
- No “best effort” security gates that pass on errors; fail closed.

## Milestones
### WS-1 — Enforce immutable pins
Acceptance criteria:
- `bash scripts/hgm/supply-chain-check.sh` runs in CI.
- No workflow uses `@vX` or `@main` for actions.

### WS-2 — Provenance and release integrity
Acceptance criteria:
- Release pipeline produces a verifiable provenance artifact.
- Release checksums are generated deterministically.

### WS-3 — Signing and verification
Acceptance criteria:
- Implement CI-gated signing (`hgm sign` later) and verify in release or downstream consumption.

## Notes
- Keep this roadmap aligned with the main `docs/planning/lift-10of10-roadmap.md`.
