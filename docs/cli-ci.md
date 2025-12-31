# Lift CLI: CI/CD Configuration

This guide covers setting up continuous integration and deployment for Lift projects.

## GitHub Actions (Default)

Projects scaffolded with `lift new` include GitHub Actions workflows in `.github/workflows/`.

### Generated Workflows

| File | Purpose |
|------|---------|
| `deploy.yml` | Manual deployment via `workflow_dispatch` (select stage) |
| `pr.yml` | Validate PRs (run tests, build Lambda) |

### Workflow Triggers

- **deploy.yml**: Uses `workflow_dispatch` with a stage input selector (dev/staging/live). Deployments are triggered manually from the Actions UI.
- **pr.yml**: Runs on pull requests to `main` or `master`. Runs tests and builds the Lambda binary.

### OIDC Authentication

The generated workflows use **OpenID Connect (OIDC)** for AWS authentication—no long-lived credentials needed.

```yaml
# From deploy.yml
permissions:
  id-token: write
  contents: read

steps:
  - name: Configure AWS credentials
    uses: aws-actions/configure-aws-credentials@v4
    with:
      role-to-assume: ${{ vars.AWS_ROLE_ARN }}
      aws-region: ${{ vars.AWS_REGION }}
```

### Required GitHub Environment Variables

Configure these in **GitHub Settings → Environments** for each deployment stage:

| Variable | Description | Example |
|----------|-------------|---------|
| `AWS_ROLE_ARN` | IAM role ARN with OIDC trust policy | `arn:aws:iam::123456789012:role/GitHubActionsRole` |
| `AWS_REGION` | AWS region for deployment (optional, defaults to us-east-1) | `us-east-1` |

#### Setting Up Environments

1. Go to **Repository Settings → Environments**
2. Create three environments: `dev`, `staging`, `live`
3. For each environment, add the required variables

**Recommended Environment Protection:**

| Environment | Protection Rules |
|-------------|------------------|
| `dev` | None (allow manual dispatch freely) |
| `staging` | Require reviewers or wait timer |
| `live` | Require reviewers, restrict to `main` branch |

### Creating the OIDC IAM Role

Create an IAM role with a trust policy for GitHub Actions:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::123456789012:oidc-provider/token.actions.githubusercontent.com"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "token.actions.githubusercontent.com:aud": "sts.amazonaws.com"
        },
        "StringLike": {
          "token.actions.githubusercontent.com:sub": "repo:your-org/your-repo:*"
        }
      }
    }
  ]
}
```

**Minimum IAM Permissions:**
- CloudFormation (full access)
- S3 (CDK staging bucket)
- Lambda, API Gateway, IAM (for deployed resources)
- SSM, Secrets Manager (if used)
- Route53, ACM (if using custom domains)

### Generated deploy.yml Structure

The generated `deploy.yml` uses GitHub Environments with manual dispatch:

```yaml
name: Deploy

on:
  workflow_dispatch:
    inputs:
      stage:
        description: 'Stage to deploy to'
        required: true
        type: choice
        options:
          - dev
          - staging
          - live

permissions:
  id-token: write
  contents: read

jobs:
  deploy:
    runs-on: ubuntu-latest
    environment: ${{ inputs.stage }}
    
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      
      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '20'
      
      - name: Install AWS CDK
        run: npm install -g aws-cdk
      
      - name: Install Lift CLI
        run: go install github.com/pay-theory/lift/cmd/lift@latest
      
      - name: Configure AWS Credentials
        uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: ${{ vars.AWS_ROLE_ARN }}
          aws-region: ${{ vars.AWS_REGION || 'us-east-1' }}
      
      - name: Deploy
        run: lift up --stage ${{ inputs.stage }}
```

### Generated pr.yml Structure

The generated `pr.yml` validates PRs with tests and builds:

```yaml
name: PR Check

on:
  pull_request:
    branches: [ main, master ]

jobs:
  test:
    runs-on: ubuntu-latest
    
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      
      - name: Run tests
        run: go test -mod=mod ./...
      
      - name: Build
        run: |
          mkdir -p dist/api
          GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -mod=mod -trimpath -ldflags="-s -w" -o dist/api/bootstrap ./cmd/api
```

### Adding Automated Deployments

To add automated deployment on push to main, modify `deploy.yml`:

```yaml
on:
  workflow_dispatch:
    inputs:
      stage:
        description: 'Stage to deploy to'
        required: true
        type: choice
        options:
          - dev
          - staging
          - live
  push:
    branches: [ main ]
```

Then add conditional logic to auto-deploy to dev on push:

```yaml
jobs:
  deploy:
    runs-on: ubuntu-latest
    environment: ${{ inputs.stage || 'dev' }}
    # ... rest of job
    
    - name: Deploy
      run: lift up --stage ${{ inputs.stage || 'dev' }}
```

---

## Pay Theory Mode (CodeBuild)

For PT-mode projects (`lift new --base-domain example.com --pt`), CI uses **AWS CodeBuild** with a CodePipeline or manual trigger.

### Generated Files

| File | Purpose |
|------|---------|
| `buildspec.yml` | CodeBuild specification (inline commands) |
| `shell/build.sh` | Optional build script for local use |
| `shell/deploy.sh` | Optional deploy script for local use |

### Required Environment Variables

Configure these in the CodeBuild project or via CodePipeline:

| Variable | Description | Required |
|----------|-------------|----------|
| `PARTNER` | Partner name for stack naming | ✅ Yes |
| `STAGE` | Deployment stage (`dev`, `staging`, `live`) | ✅ Yes |
| `TARGET_MODE` | Target mode (default: `standard`) | ❌ Optional |
| `AWS_REGION` | AWS region (default: `us-east-1`) | ❌ Optional |

**Note:** `AWS_ACCOUNT_ID` is **not required**—the buildspec derives it via `aws sts get-caller-identity`.

### buildspec.yml Structure

The generated buildspec uses **inline commands** (not shell script delegation), matching the merchant-application pattern:

```yaml
version: 0.2

phases:
  install:
    runtime-versions:
      golang: 1.25
      nodejs: 20
    commands:
      - npm install -g aws-cdk
  pre_build:
    commands:
      - apt-get update -y
  build:
    commands:
      - export PARTNER=$PARTNER
      - export STAGE=$STAGE
      - export TARGET_MODE=${TARGET_MODE:-standard}
      - export CDK_DEFAULT_ACCOUNT=$(aws sts get-caller-identity --query 'Account' --output text)
      - export CDK_DEFAULT_REGION=${AWS_REGION:-us-east-1}
      - echo "Building Lambda functions..."
      - mkdir -p dist/api
      - echo "Building api..."
      - GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -mod=mod -ldflags="-s -w" -tags lambda.norpc -trimpath -o dist/api/bootstrap ./cmd/api
      # LIFT:ADD_FUNCTION_BUILDS (marker for lift add function)
      - echo "Synthesizing CDK stacks..."
      - cd cdk
      - go mod tidy
      - cdk synth
      - echo "Deploying service stack..."
      - cdk deploy {{.AppName}}-service-$PARTNER-$STAGE --require-approval never --context partner=$PARTNER --context stage=$STAGE --context targetMode=$TARGET_MODE
  post_build:
    commands:
      - echo Build completed on `date`
```

### Key Pattern: Inline Commands

The PT buildspec follows the merchant-application pattern of **inline build commands** rather than shell script delegation. This provides:
- Full visibility in CodeBuild logs
- Easier debugging of individual steps
- Direct use of CodeBuild environment variables
- Automatic account detection via STS

### Shell Scripts (Optional)

The generated `shell/` scripts are for **local use** or alternative workflows:

**shell/build.sh:**
```bash
#!/bin/bash
set -euo pipefail

echo "Building functions..."
lift build

echo "Building CDK..."
cd cdk && go build -mod=mod -o /dev/null main.go
cd ..
```

**shell/deploy.sh:**
```bash
#!/bin/bash
set -euo pipefail

if [[ -z "${PARTNER:-}" ]]; then
  echo "Error: PARTNER environment variable is required"
  exit 1
fi

if [[ -z "${STAGE:-}" ]]; then
  echo "Error: STAGE environment variable is required"
  exit 1
fi

echo "Deploying ${PARTNER} to ${STAGE}..."
lift up --stage "${STAGE}" --partner "${PARTNER}" --target-mode "${TARGET_MODE:-standard}"
```

### CodePipeline Configuration

For automated deployments, configure CodePipeline with:

1. **Source Stage:** GitHub/CodeCommit source
2. **Build Stage:** CodeBuild project with buildspec.yml
3. **Environment Variables:** Set `PARTNER` and `STAGE` per-stage

**Stage-Specific Configuration:**

| Stage | PARTNER | STAGE |
|-------|---------|-------|
| Dev | `{partner}` | `dev` |
| Staging | `{partner}` | `staging` |
| Production | `{partner}` | `live` |

### Manual Deployment

For manual deployments from a local machine:

```bash
# Set required variables
export PARTNER=mypartner
export STAGE=dev
export TARGET_MODE=standard

# Run deployment
lift up --stage $STAGE --partner $PARTNER --target-mode $TARGET_MODE
```

### Cross-Account Deployment

For cross-account deployments:

1. Ensure CodeBuild role can assume a role in the target account
2. The buildspec automatically sets `CDK_DEFAULT_ACCOUNT` via STS
3. If you need explicit control, override in your CodePipeline:

```yaml
# In CodePipeline environment variables
CDK_DEFAULT_ACCOUNT: "123456789012"  # Explicit target account
CDK_DEFAULT_REGION: "us-east-1"
```

---

## Troubleshooting

### "cdk not found" in CI

Ensure Node.js and CDK are installed in your workflow:

```yaml
- name: Set up Node.js
  uses: actions/setup-node@v4
  with:
    node-version: '20'

- name: Install CDK
  run: npm install -g aws-cdk
```

### "go not found" in CI

Ensure Go is set up:

```yaml
- name: Set up Go
  uses: actions/setup-go@v5
  with:
    go-version: '1.25'
```

### OIDC Authentication Fails

Check:
1. OIDC provider is configured in AWS account
2. IAM role trust policy matches your repository
3. `AWS_ROLE_ARN` is correctly set in GitHub Environment

### "domain configuration changed" Error

This means the lift.yaml domains differ from the deployed state. Either:
1. Restore the original domain configuration, OR
2. Run `lift down --stage {stage}` to clear the lock, then redeploy

### Go Module Issues

For Go 1.25 compatibility, the generated workflows use `-mod=mod`:

```yaml
- name: Run tests
  run: go test -mod=mod ./...

- name: Build
  run: |
    GOOS=linux GOARCH=arm64 go build -mod=mod ...
```

---

## Best Practices

1. **Use Environment Protection** - Require reviews for staging/live
2. **Pin CDK Version** - Use `aws-cdk@2.x.x` for reproducibility
3. **Test Locally First** - Run `lift build` and `cdk synth` before pushing
4. **Use Branch Protection** - Require PR reviews before merge to main
5. **Manual Deploy Workflow** - The generated workflow uses manual dispatch for safety; add auto-deploy only after testing
