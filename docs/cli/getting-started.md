# Lift CLI: Getting Started

This guide walks you through installing the Lift CLI and deploying your first serverless Go application.

## Prerequisites

Before using the Lift CLI, ensure you have the following installed:

### Go 1.25+

The Lift CLI requires Go to build Lambda functions.

```bash
# Check if Go is installed
go version

# If not installed, download from:
# https://go.dev/doc/install
```

### Node.js 18+

Node.js is required as a dependency for AWS CDK.

```bash
# Check if Node.js is installed
node --version

# If not installed, download from:
# https://nodejs.org/
# Or use nvm: https://github.com/nvm-sh/nvm
```

### AWS CDK

AWS CDK is used to deploy infrastructure.

```bash
# Check if CDK is installed
cdk --version

# Install globally via npm
npm install -g aws-cdk

# Or use npx (no global install needed)
npx aws-cdk --version
```

### AWS Credentials

Ensure your AWS credentials are configured:

```bash
# Check current identity
aws sts get-caller-identity

# Configure credentials if needed
aws configure
```

## Installing the Lift CLI

Install the Lift CLI using `go install`:

```bash
# Install latest version
go install github.com/pay-theory/lift/cmd/lift@latest

# Or install a specific version
go install github.com/pay-theory/lift/cmd/lift@v1.2.3

# Verify installation
lift version
```

The binary will be installed to `$GOPATH/bin` (or `$GOBIN` if set). Ensure this directory is in your `$PATH`.

## Creating a New Project

Use `lift new` to scaffold a new project:

```bash
# Create a new project in a subdirectory
lift new my-api --base-domain example.com

# To bootstrap an EXISTING directory, cd into it first:
cd my-existing-dir
lift new --base-domain example.com
```

**Note:** `lift new .` is not supported. To scaffold in the current directory, run `lift new` without a directory argument.

### Template Options

Lift provides several project templates:

| Template | Description |
|----------|-------------|
| `basic-api` (default) | Single Lambda API with minimal infrastructure |
| `microservice` | Lightweight single Lambda microservice |
| `event-driven` | API + processor with SQS queue for async processing |
| `merchant-app` | Multi-Lambda (api + worker), multi-stack architecture |

Specify a template with `--template`:

```bash
lift new my-app --template microservice --base-domain example.com
```

### The `--base-domain` Flag

The `--base-domain` flag is **required** for all templates:

```bash
lift new my-api --base-domain example.com
```

This generates `lift.yaml` with domain configuration:

```yaml
domains:
  base_domain: example.com
```

## Project Structure

After running `lift new`, you'll have:

```
my-api/
├── cmd/
│   └── api/
│       └── main.go         # Lambda entrypoint
├── cdk/
│   ├── main.go             # CDK infrastructure
│   └── go.mod
├── .github/
│   └── workflows/
│       ├── deploy.yml      # Manual deploy workflow (workflow_dispatch)
│       └── pr.yml          # PR validation (tests + build)
├── go.mod
├── lift.yaml               # Lift configuration
└── README.md
```

## The Stage Model

Lift uses a fixed three-stage deployment model:

| Stage | Purpose | Domain Pattern |
|-------|---------|----------------|
| `dev` | Development and testing | `{subdomain}.dev.{base_domain}` |
| `staging` | Pre-production validation | `{subdomain}.staging.{base_domain}` |
| `live` | Production | `{subdomain}.{base_domain}` |

**Note:** Only `dev`, `staging`, and `live` are valid stages. Other names like `prod` or `test` are not supported.

## Deploying

Use `lift up` to build and deploy:

```bash
# Deploy to dev (default)
lift up

# Deploy to a specific stage
lift up --stage dev
lift up --stage staging
lift up --stage live
```

`lift up` performs the following:
1. Checks prerequisites (Go, CDK)
2. Validates domain configuration
3. Builds all Lambda functions
4. Deploys CDK stacks in order
5. Saves state file for domain locking

## Domain Immutability & Lock Files

**Important:** Once deployed, domain configuration is locked for that stage.

When you run `lift up`, Lift creates a state file at `.lift/state/dev.json` (or `staging.json`, `live.json`) that records the deployed domain configuration.

### Why Domain Locking?

Domain locking prevents accidental misconfiguration:
- Changing `base_domain` after deployment would orphan DNS records
- Changing service subdomains could break existing integrations
- Certificate and Route53 records are tied to specific domains

### If You Need to Change Domains

You **must** tear down the existing deployment first:

```bash
# Attempting to redeploy with changed domains will fail
lift up --stage dev
# Error: domain configuration changed (locked: example.com → new-domain.com)
# Run 'lift down --stage dev' first to clear the domain lock

# Tear down to clear the lock
lift down --stage dev

# Now you can redeploy with new domains
lift up --stage dev
```

### State File Location

State files are stored in `.lift/state/`:

```
.lift/
└── state/
    ├── dev.json
    ├── staging.json
    └── live.json
```

**Note:** The generated `.gitignore` excludes `.lift/state/` by default. If you want to preserve domain locks across team members, remove this line from `.gitignore` and commit the state files.

## Tearing Down

Use `lift down` to destroy a deployment:

```bash
# Destroy dev deployment
lift down --stage dev

# Destroy staging deployment
lift down --stage staging
```

This destroys CDK stacks in reverse order and removes the state file.

## Pay Theory Mode (`--pt`)

For Pay Theory internal projects, use the `--pt` flag:

```bash
lift new my-app --base-domain example.com --pt
```

This generates:
- Partner-aware stack naming (`{{.AppName}}-{{.Partner}}-{{.Stage}}`)
- CodeBuild-compatible `buildspec.yml` instead of GitHub Actions
- Shell scripts in `shell/` for manual operations

Deploy with partner context:

```bash
lift up --stage dev --partner mypartner
lift up --stage staging --partner mypartner --target-mode custom
```

See [CI Documentation](./ci.md) for full PT mode CI guidance.

## Adding to Existing Projects

Use `lift add` to add components to existing projects:

```bash
# Add a new Lambda function
lift add function webhook

# Add another function
lift add function payment_processor
```

This will:
- Create `cmd/<name>/main.go` with a Lift handler skeleton
- Update `lift.yaml` to include the new function
- Update `cdk/main.go` to deploy the new function
- For PT projects: update `buildspec.yml` and `shell/build.sh`

## Next Steps

- [CI/CD Configuration](./ci.md) - Set up GitHub Actions or CodeBuild
- [CDK Reference](../cdk-api-reference.md) - Infrastructure patterns
- [API Reference](../api-reference.md) - Framework documentation
