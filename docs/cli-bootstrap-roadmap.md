# Lift CLI Bootstrap & Deployment Roadmap

Status: draft (for alignment)

Contract (v1): `./cli-contract-v1.md`

## Problem Statement

Lift applications currently require developers to repeatedly:

- Create a filesystem layout (Go services + `cdk/` + `dist/` outputs)
- Wire stage/environment configuration
- Set up CI/CD for builds + CDK deploys

The goal is for Lift to provide a first-class `lift` CLI that bootstraps projects and deploys them consistently, using a stage-centric workflow (`dev`, `staging`, `live`) and a domain model that is stable once deployed.

## Goals

1. **Bootstrap**: `lift new` creates a deployable Lift + CDK project with a consistent structure and CI tooling.
2. **Stage-first deployments**: `lift up --stage dev|staging|live` builds and deploys the app for a specific stage.
3. **Domain management contract**:
   - `live` uses the apex `base_domain`
   - stage prefixes apply for non-live (`dev.<base_domain>`, `staging.<base_domain>`)
   - service subdomains prefix stage roots (`api.<stage-root>`, e.g. `api.dev.example.com`)
   - domains are configurable before first deploy, but **immutable per-stage after deploy** (domain changes require teardown)
4. **CI defaults**:
   - default: GitHub Actions using OIDC (AWS role ARNs live in GitHub Environment vars)
   - optional `--pt`: Pay Theory mode that matches `reference/merchant-application` devops (CodeBuild `buildspec.yml` + `shell/*` scripts)
5. **Templates/blueprints**: multiple bootstrappable project types (start with a “basic API” and a “merchant-application-like” multi-Lambda template).

## Non-Goals (Early Phases)

- Designing a full application framework generator (beyond a small set of curated templates).
- Automatically migrating existing repositories into the Lift contract (may be a later tool).
- Supporting “deploy from outside the project folder” (`lift up` runs from inside the bootstrapped project root).

## Canonical Project Layout (Generated)

Align with `reference/merchant-application` conventions:

- `cmd/<function>/...` Go entrypoints for each Lambda
- `cdk/` Go CDK app (stacks, `cdk.json`, `go.mod`)
- `dist/<function>/bootstrap` build outputs
- `lift.yaml` (Lift project configuration; stages, domains, build/deploy metadata)
- CI tooling:
  - default: `.github/workflows/*` (OIDC)
  - `--pt`: `buildspec.yml` and `shell/build.sh`, `shell/deploy.sh`, etc.

## Configuration: `lift.yaml` (Contract)

`lift.yaml` is required for any bootstrapped project and is the single source of truth for:

- Stage names and stage root domains
- Base domain and service subdomain prefixes
- Lambda build graph (which `cmd/*` build to which `dist/*` outputs)
- CDK app location and deploy ordering

### Stage & Domain Model

Stage names are fixed:

- `dev`
- `staging`
- `live`

`base_domain` is the apex domain (example: `example.com`) and is required for templates that use domains.

Derived stage root domain defaults:

- `dev`: `dev.<base_domain>` (example: `dev.example.com`)
- `staging`: `staging.<base_domain>` (example: `staging.example.com`)
- `live`: `<base_domain>` (example: `example.com`)

Service subdomains prefix the stage root domain:

- `api` in `dev` -> `api.dev.example.com`
- `api` in `staging` -> `api.staging.example.com`
- `api` in `live` -> `api.example.com`

### Example `lift.yaml`

```yaml
version: 1

app:
  name: my-new-app
  template: basic-api

domains:
  base_domain: example.com

stages:
  dev:
    root_domain: dev.example.com
  staging:
    root_domain: staging.example.com
  live:
    root_domain: example.com

services:
  api:
    subdomain: api

build:
  goos: linux
  goarch: arm64
  cgo: 0
  ldflags: "-s -w"
  tags: ["lambda.norpc"]

functions:
  api:
    cmd: ./cmd/api
    out: ./dist/api/bootstrap

cdk:
  path: ./cdk
  # Stack names should be stage-scoped (and partner-scoped in --pt templates)
  deploy_order:
    - data
    - service
  stacks:
    data:
      name_template: "{{.AppName}}-data-{{.Stage}}"
    service:
      name_template: "{{.AppName}}-service-{{.Stage}}"
```

## CLI UX & Command Behavior

### `lift new [<app>]`

Purpose: bootstrap a new Lift project.

- `lift new` bootstraps the current directory (app name defaults from directory name).
- `lift new <app>` creates `./<app>` and bootstraps within it.
- For templates requiring domains, require `--base-domain <apex>`.

Flags (proposed):

- `--template <name>`: `basic-api` (default), `merchant-app`, `microservice`, `event-driven`, etc.
- `--base-domain <apex>`: required for domain templates
- `--pt`: generate Pay Theory devops assets (`buildspec.yml`, `shell/*`) instead of GitHub Actions defaults
- `--no-ci`: opt out of generating CI files (local-only scaffolding)

### `lift up --stage <dev|staging|live>`

Purpose: build + deploy for a stage.

Rules:

- Must run from a project root containing `lift.yaml`.
- Stage is required (default: `dev`).
- Reads `lift.yaml`, builds all declared `functions` into their `dist/*` outputs.
- Executes CDK deploy in the declared stack order, passing required context:
  - `stage`
  - `baseDomain`
  - computed domains (e.g. `apiDomain`)
  - plus template-specific context (e.g. `partner` in `--pt` mode)

Domain immutability enforcement:

- On first successful deploy per stage, record a stage lock file (example: `.lift/state/<stage>.json`) containing:
  - stage root domain
  - computed service domains
  - stack name templates and resolved names
- On subsequent deploys, if `lift.yaml` domain fields for that stage differ, fail with an actionable error:
  - “domain configuration changed; run `lift down --stage <stage>` before re-deploying with new domains”

### `lift down --stage <dev|staging|live>`

Purpose: destroy all stacks for a stage and clear the domain lock.

- Executes CDK destroy in reverse deploy order.
- Removes `.lift/state/<stage>.json` to allow future domain changes.

### Additional Commands (phased)

- `lift diff --stage ...`: `cdk diff` wrapper with consistent context
- `lift synth --stage ...`: `cdk synth` wrapper with consistent context
- `lift status --stage ...`: show stacks + key outputs (best-effort)

## CI/CD Generation

### Default: GitHub Actions (OIDC)

Generate:

- `.github/workflows/deploy.yml`: deploys a chosen stage
- `.github/workflows/pr.yml`: runs `go test` (and optional `cdk synth`)

GitHub Environments:

- Create environments: `dev`, `staging`, `live`
- Each environment sets vars:
  - `AWS_ROLE_ARN` (required)
  - `AWS_REGION` (recommended; can be repo-level if shared)

Workflow requirements:

- Uses `aws-actions/configure-aws-credentials` with OIDC to assume `vars.AWS_ROLE_ARN`
- Uses `environment: ${{ inputs.stage }}` so environment-scoped vars apply
- Runs `lift up --stage ${{ inputs.stage }}`

### `--pt`: Pay Theory Mode (merchant-application devops)

Generate assets patterned after `reference/merchant-application`:

- `buildspec.yml` with:
  - Go + Node runtime setup
  - build all lambdas to `dist/*`
  - `cdk synth`
  - `cdk deploy` stacks with `--require-approval never`
- `shell/build.sh` and `shell/deploy.sh`:
  - accept required flags (partner, stage, targetMode)
  - perform build + deploy in correct order

In this mode:

- Stage is still required and maps to Lift stages.
- Partner is required (to match Pay Theory naming and isolation).

## Technical Architecture (Implementation Notes)

### CLI Implementation (Go)

Suggested structure:

- `cmd/lift/` executable entrypoint
- `pkg/liftcli/` for CLI command implementations (or evolve the existing `pkg/cli`)
- `internal/liftconfig/` for:
  - `lift.yaml` schema (versioned)
  - validation (stages, domains, required fields)
  - computed domain helpers
- `internal/templates/` embedded templates (using `go:embed`)

### Template Rendering

- Ship templates in-tree and embed into the CLI binary to avoid runtime network needs.
- Use `text/template` for file content templating and a small “copy tree” renderer for the template directory.

### CDK Invocation Strategy

- `lift up` shells out to `cdk` and relies on:
  - node + `aws-cdk` installed (developer machine or CI)
  - AWS credentials present (local) or OIDC (CI)
- The generated CDK app should support a “bootstrap placeholder” synth when required context is missing (mirroring `reference/merchant-application/cdk/main.go`).

## Roadmap (Milestones)

Milestones are designed to ship value early while keeping the contract stable.

### Milestone 0: Spec & Contract Lock

Deliverables:

- `lift.yaml` v1 schema definition (documented) including:
  - fixed stages (`dev`, `staging`, `live`)
  - base domain + stage root domains
  - functions build graph
  - CDK path + deploy ordering
- Template requirements and naming conventions (dist outputs, CDK context keys).

Acceptance:

- A reviewer can answer: “What files exist after `lift new`?” and “What does `lift up` do?” without ambiguity.

### Milestone 1: Real `lift` Binary + Project Root Detection

Deliverables:

- `cmd/lift` builds a working binary.
- `lift` can locate and parse `lift.yaml`.
- `lift up` is blocked outside a project root with an actionable error.

Acceptance:

- `go build ./cmd/lift` succeeds.
- `lift up` from outside a project prints “no lift.yaml found” and exits non-zero.

### Milestone 2: `lift new` (basic template) + `lift.yaml` generation

Deliverables:

- `lift new` scaffolds a `basic-api` template:
  - `cmd/api`
  - `cdk/` app (single stack ok)
  - `dist/` output path used by CDK
  - `lift.yaml` created and validated
- Domain-supporting template behavior:
  - requires `--base-domain` and writes stage root defaults

Acceptance:

- A freshly generated project builds locally (`lift build` or `lift up --stage dev` after prerequisites installed).

### Milestone 3: Build System (`lift build`) for multi-Lambda projects

Deliverables:

- `lift build` reads `functions` from `lift.yaml` and produces `dist/<fn>/bootstrap`.
- Consistent build flags: linux/arm64, `CGO_ENABLED=0`, `-trimpath`, `-ldflags "-s -w"`, optional tags.

Acceptance:

- Adding a second function in config builds both outputs correctly.

### Milestone 4: Deploy Orchestration (`lift up/down`) + Domain Locking

Deliverables:

- `lift up --stage ...`:
  - computes domains (`api.<stage-root>`, etc.)
  - runs `cdk deploy` in order with consistent context
  - writes `.lift/state/<stage>.json` lock metadata
- `lift down --stage ...`:
  - destroys in reverse order
  - clears the stage lock
- Domain immutability enforcement for each stage.

Acceptance:

- Changing `stages.dev.root_domain` after a successful deploy causes `lift up --stage dev` to fail until `lift down --stage dev` is run.

### Milestone 5: GitHub Actions Generator (OIDC default)

Deliverables:

- `lift new` generates:
  - `.github/workflows/deploy.yml` with input `stage` in `{dev,staging,live}`
  - `.github/workflows/pr.yml` to run tests/build checks
- Documentation snippet in generated README:
  - “Create GitHub Environments dev/staging/live”
  - “Set vars.AWS_ROLE_ARN (and AWS_REGION)”

Acceptance:

- Workflow uses `environment: ${{ inputs.stage }}` and reads `vars.AWS_ROLE_ARN`.

### Milestone 6: `--pt` Mode (merchant-application devops parity)

Deliverables:

- `lift new --pt` generates:
  - `buildspec.yml` aligned to `reference/merchant-application`
  - `shell/build.sh` and `shell/deploy.sh` aligned to `reference/merchant-application`
- `lift up` supports additional context requirements in PT templates (e.g., `partner`).

Acceptance:

- Generated artifacts are recognizable as “merchant-application style” and can be used as-is in CodeBuild.

### Milestone 7: Additional Templates

Deliverables:

- Add curated templates:
  - `microservice` (single Lambda + API)
  - `event-driven` (API + processor + queue/bus)
  - `merchant-app` (multi-Lambda, multi-stack)
- Each template includes:
  - `lift.yaml` compatible outputs
  - CI defaults (GitHub Actions) or `--pt` mode variants

Acceptance:

- Each template can `lift build` and `cdk synth` without manual edits (beyond required vars/base domain).

### Milestone 8: Incremental Scaffolding (`lift add ...`)

Deliverables (initial set):

- `lift add function <name>`:
  - creates `cmd/<name>` with a Lift handler
  - updates `lift.yaml` `functions`
  - updates CDK to deploy the new function (template-dependent)

Acceptance:

- After adding a function, `lift build` produces the new `dist/<name>/bootstrap` and `lift up` deploys it.

### Milestone 9: Distribution & Developer Experience

Deliverables:

- Install story for the CLI:
  - `go install ...` (baseline)
  - optional: GitHub Releases with binaries
- Clear prerequisite checks in `lift` (`cdk` present, Go present, etc.) with actionable errors.
- Docs: “Getting started with `lift new` and `lift up`” and “CI setup” sections.

Acceptance:

- A developer can follow docs end-to-end without tribal knowledge.

## Risks & Mitigations

- **CDK toolchain drift**: Pin CDK version in templates (or provide a known-good range) and keep templates updated.
- **Domain immutability complexity**: Start with local lock files; add optional AWS-based verification later if needed.
- **Template explosion**: Keep templates curated; require each template to meet the contract and have an owner.
- **PT vs OSS divergence**: Keep `--pt` as an additive variant and avoid leaking PT-only assumptions into default templates.

## Next Step (Suggested)

Confirm:

1. Minimum template set for MVP (`basic-api` + `merchant-app`).
2. Whether domain-enabled templates should create Route53/ACM resources automatically or accept existing infrastructure via config.
3. The canonical GitHub vars list (`AWS_ROLE_ARN`, `AWS_REGION`, and any others required by CDK/app).
