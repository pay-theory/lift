# Lift: Branch + Release Policy (main release, premain prerelease)

This document defines the intended branch strategy and release automation for Lift in high-risk usage contexts.

## Branches

- `premain` — prerelease integration branch (source of prereleases).
- `main` — release branch (source of stable releases).

## Merge flow (expected)

- Feature/fix work lands via PRs into `premain`.
- A release is cut by merging `premain` into `main` (via PR).
- Hotfixes may merge directly into `main` (then backported to `premain`).

## Protections (required)

Protect both `premain` and `main`:

- Require PRs (no direct pushes).
- Require CODEOWNERS/review approvals.
- Require CI status checks to pass (at minimum: `Quality Gates (10/10 Rubric)`).
- Restrict force-pushes and deletions.

## Automated releases (required)

This repo should publish:

- **Prereleases** on merges to `premain`.
- **Releases** on merges to `main`.

Versioning note:
- The post-HGM baseline release line starts at `v1.1.0` (prereleases: `v1.1.0-rc.N`).

Recommended approach: **release-please** (merge-driven versioning + changelog updates) with:

- prerelease workflow producing tags like `vX.Y.Z-rc.N`, and
- release workflow producing stable `vX.Y.Z` tags and updating `CHANGELOG.md`.

Publishing approach:
- `release-please` creates the tag + GitHub release (notes) using the built-in `GITHUB_TOKEN`.
- `prerelease.yml` / `release-please.yml` call `.github/workflows/release.yml` (via `workflow_call`) to build and upload
  Lift binaries to the GitHub Release (no tag-push chaining, no manual tokens).
- Repos with "immutable releases" enabled must publish assets while the GitHub Release is still a **draft**, then publish
  the release after assets upload completes.

Commit discipline (required):
- Use Conventional Commits (`fix:`, `feat:`, etc.) so release-please can detect user-facing changes and cut releases.

## Required workflow artifacts (Rubric COM-8)

These files are required to exist and be kept current:

- `.github/workflows/prerelease.yml`
- `.github/workflows/release-please.yml`
- `.github/workflows/release.yml`

Additionally, quality/security workflows should run on PRs to (and/or pushes on) both protected branches:

- `.github/workflows/quality-gates.yml`
- `.github/workflows/codeql.yml`

## Notes

- This policy is intentionally tool-agnostic; the rubric requires automation and pinning, not a specific release tool.
- Branch protection rules are configured in the hosting platform (GitHub settings) and must be treated as part of the supply chain.
