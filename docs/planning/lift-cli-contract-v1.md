# Lift CLI Contract v1 (`lift.yaml`, stages, domains, templates)

Status: v1 draft (intended to be stable for Milestones 1–4)

This document defines the **contract** between:

- Lift CLI (`lift new`, `lift up`, `lift down`, …)
- Bootstrapped projects on disk (filesystem layout)
- The project configuration file (`lift.yaml`)
- Template authors (what a template must produce/expect)
- CI generators (GitHub Actions default, Pay Theory `--pt` optional)

## Core Principles

1. **Stage-first**: every deployment happens into a specific stage: `dev`, `staging`, or `live`.
2. **Stage isolates everything**: stack names, domains, and deploy targets are stage-scoped.
3. **Domain stability**: stage domains may be edited **before** first deploy, but **cannot** change for that stage after deploy without teardown.
4. **Reproducible structure**: generated projects have a consistent, tooling-friendly layout aligned with `reference/merchant-application`.

## Project Root

A Lift CLI project root is any directory that contains `lift.yaml`.

Rules:

- `lift up`/`lift down` must be run **inside** a project root (or a subdirectory of one).
- The CLI must locate the root by walking upward from the current working directory until it finds `lift.yaml`.

## Stages (Fixed)

Stage keys are fixed and case-sensitive:

- `dev`
- `staging`
- `live`

The CLI and templates must not introduce additional stage keys in v1.

## Domains (Model & Defaults)

### Base Domain

`domains.base_domain` is the apex domain (example: `example.com`).

- Required for templates that need domains (APIs, frontends, anything that creates DNS/certificates/custom domains).
- Optional for templates that do not use domains.

### Stage Root Domains

Each stage may define a **stage root domain**.

Defaults (when `domains.base_domain` is set and no overrides are provided):

- `dev.root_domain = dev.<base_domain>` (example: `dev.example.com`)
- `staging.root_domain = staging.<base_domain>` (example: `staging.example.com`)
- `live.root_domain = <base_domain>` (example: `example.com`) **(apex)**

### Service Subdomains

Applications may define service subdomains (example: `api`).

Full service domains are derived as:

`<service_subdomain>.<stage_root_domain>`

Examples (`base_domain=example.com`, service subdomain `api`):

- `dev`: `api.dev.example.com`
- `staging`: `api.staging.example.com`
- `live`: `api.example.com`

### Domain Immutability (Per Stage)

Once a stage has been deployed successfully, its domain configuration is considered **locked**.

- Any change to `stages.<stage>.root_domain` or to derived service domains must require teardown.
- CLI behavior (contract):
  - `lift up --stage <stage>` must fail with an actionable error if domains differ from the last deployed state.
  - The error must instruct the user to run `lift down --stage <stage>` before re-deploying with new domains.

How this is enforced (implementation detail, but recommended v1):

- The CLI writes `.lift/state/<stage>.json` after a successful deploy containing the resolved domains used for that deploy.

## Filesystem Layout (Template Output Contract)

Templates should follow the `reference/merchant-application` conventions:

- `lift.yaml` at repo root
- `cmd/<function>/...` per Lambda entrypoint
- `cdk/` contains the CDK app (Go CDK)
- `dist/<function>/bootstrap` build outputs (generated; should not be committed)
- `.github/workflows/*` (default CI) OR `buildspec.yml` + `shell/*` (`--pt` CI)

Recommended `.gitignore` entries:

- `dist/`
- `cdk/cdk.out/` or `cdk.out/`
- `.lift/state/`

## `lift.yaml` Schema (v1)

### Minimal Required Fields

All Lift CLI projects must have:

- `version` (integer) — must be `1`
- `app.name` (string)
- `app.template` (string)
- `stages.dev`, `stages.staging`, `stages.live` objects present (may be empty if domains not used)

If domains are used by the template:

- `domains.base_domain` (string)
- `stages.<stage>.root_domain` (string) for each stage

### Recommended Fields

Templates that can be deployed should define:

- `functions` map describing what to build
- `cdk.path` and deploy ordering metadata
- `services` subdomain map for domain derivation
- `build` defaults (GOOS/GOARCH/flags/tags)

### Reference Shape (YAML)

```yaml
version: 1

app:
  name: my-new-app
  template: basic-api

domains:
  # REQUIRED for domain-enabled templates
  base_domain: example.com

stages:
  dev:
    root_domain: dev.example.com
  staging:
    root_domain: staging.example.com
  live:
    root_domain: example.com

services:
  # Optional; used to derive per-stage service domains.
  api:
    subdomain: api

build:
  goos: linux
  goarch: arm64
  cgo: 0
  trimpath: true
  ldflags: "-s -w"
  tags: ["lambda.norpc"]

functions:
  # name must match the dist folder name: dist/<name>/bootstrap
  api:
    cmd: ./cmd/api
    out: ./dist/api/bootstrap

cdk:
  path: ./cdk
  deploy_order: [data, service]
  stacks:
    data:
      name_template: "{{.AppName}}-data-{{.Stage}}"
    service:
      name_template: "{{.AppName}}-service-{{.Stage}}"
```

## CDK Context Contract (v1)

The CLI must pass a consistent set of CDK context values when running `cdk` commands.

Baseline context keys:

- `stage`: one of `dev|staging|live`
- `appName`: from `app.name` (optional but recommended)

If domains are enabled:

- `baseDomain`: `domains.base_domain`
- `stageRootDomain`: `stages.<stage>.root_domain`
- `serviceDomain.<service>`: derived full domain for each `services.<service>.subdomain`
  - example key: `serviceDomain.api = api.dev.example.com`

Pay Theory mode (`lift new --pt`) adds:

- `partner`: partner name (required for deploy)
- `targetMode`: deployment mode (default `standard`, template-specific)

Templates may add additional context keys, but they should be namespaced/predictable.

## CI Generation Contract

### Default: GitHub Actions + OIDC

Generated projects should default to GitHub Actions.

Requirements:

- Define GitHub **Environments** named `dev`, `staging`, `live`.
- Store AWS role ARNs in **GitHub Environment vars** (not in `lift.yaml`):
  - `vars.AWS_ROLE_ARN` (required)
  - `vars.AWS_REGION` (recommended)
- Workflow must set `environment: ${{ inputs.stage }}` so environment-scoped vars apply.
- Workflow should assume role via OIDC and run:
  - `lift up --stage ${{ inputs.stage }}`

### Optional: Pay Theory `--pt` Mode (merchant-application devops)

If `lift new --pt` is used, generate assets patterned after `reference/merchant-application`:

- `buildspec.yml`
- `shell/build.sh`
- `shell/deploy.sh`

In this mode, partner/stage are explicit deploy inputs and stack naming follows the merchant-application patterns.

## `lift new` Behavior (v1)

`lift new` bootstraps either:

- the current directory (`lift new`), or
- a new directory named `<app>` (`lift new <app>`)

`lift up` must always be run inside the bootstrapped project directory.

