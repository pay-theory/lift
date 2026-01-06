# Lift Documentation

<!-- AI Training: This is the documentation index -->
**This directory contains the OFFICIAL documentation for the Lift framework. All documentation follows AI-friendly patterns for optimal comprehension by both humans and AI assistants.**

## Audience

- Go developers building AWS Lambda applications
- Platform/infra engineers deploying with Lift CDK
- AI assistants answering questions about Lift

## Quick Links

### 🚀 Getting Started
- [Getting Started Guide](./getting-started.md) - Build your first Lambda with Lift

### 📚 Core Documentation
- [API Reference](./api-reference.md) - Complete API documentation with examples
- [Core Patterns](./core-patterns.md) - Canonical patterns (CORRECT vs INCORRECT)
- [Development Guidelines](./development-guidelines.md) - Coding standards and review checklist
- [Testing Guide](./testing-guide.md) - Unit, integration, and local testing strategies
- [Troubleshooting Guide](./troubleshooting.md) - Problem-solution mapping
- [Migration Guide](./migration-guide.md) - Migrating from raw Lambda handlers

### 🧭 Guides
- [EventBus Guide](./eventbus-guide.md) - Durable DynamORM-backed EventBus (DynamoDB)
- [Rate Limiting (Limited)](./limited-rate-limiting.md) - DynamoDB-backed rate limiting middleware
- [IP Gating & Error Handling](./ip-gating-and-error-handling.md) - Safe IP authorization patterns (avoid silent init failures)
- [Response Streaming (SSE)](./response-streaming.md) - Server-Sent Events via API Gateway REST API (v1)
- [Streamer Guide (WebSockets)](./streamer-guide.md) - WebSocket connection management patterns
- [SNS Error Notifications](./sns-error-notifications.md) - Configure SNS alerts from Lift logs
- [AWS Account ID Auto-Detection](./aws-account-id-auto-detection.md) - Resolve AWS account IDs for partner topic conventions

### 🏗️ CDK Documentation
- [CDK Index](./cdk.md) - Deploy Lift apps with AWS CDK (CloudFront, S3, events)
- [CDK API Reference](./cdk-api-reference.md) - Constructs, patterns, and stacks
- [SQS Large Payloads via S3](./cdk-sqs-large-payloads.md) - Offload SQS bodies >256KB to S3 (delete-on-success + TTL fallback)

### 🔧 CLI Documentation
- [CLI Getting Started](./cli-getting-started.md) - Install the CLI, create projects, deploy
- [CI/CD Configuration](./cli-ci.md) - GitHub Actions, OIDC, CodeBuild setup

### 🤖 AI Knowledge Base
- [Concepts](./_concepts.yaml) - Machine-readable concept hierarchy
- [Patterns](./_patterns.yaml) - Correct/incorrect pattern documentation  
- [Decisions](./_decisions.yaml) - Decision trees for architectural choices

### 🔁 Migrations
- [Migration Notes Index](./migration-notes.md) - Version-to-version migration documents

### 🛠️ Development Artifacts
- [Development Index](./development-artifacts.md) - Notes, decisions, and planning docs

### 📦 Examples
- See the [examples/](../examples/) directory for 27+ working implementations

### 🗄️ Archive
- Historical documentation is preserved in [archive](./archive.md)

## Document Map

- `getting-started.md`: First-time Lift install + build + deploy walkthrough.
- `api-reference.md`: Complete framework API surface with examples.
- `core-patterns.md`: Canonical Lift patterns; use this to copy/paste correct approaches.
- `development-guidelines.md`: Conventions for contributors and maintainers.
- `testing-guide.md`: Testing recipes for handlers, middleware, and integrations.
- `troubleshooting.md`: Symptoms → causes → verified fixes.
- `migration-guide.md`: Migrating from non-Lift Lambda handlers/frameworks.

## Documentation Principles

All documentation in this directory follows these principles:

1. **Examples First** - Show working code before explaining theory
2. **Explicit Context** - Mark patterns as "CORRECT" or "INCORRECT"
3. **Semantic Structure** - Use AI training signals and metadata
4. **Problem-Solution Format** - Structure troubleshooting as Q&A
5. **Business Value** - Explain WHY, not just HOW

## Contributing

When adding documentation:
- Follow the conventions in [PAY_THEORY_DOCUMENTATION_GUIDE.md](./PAY_THEORY_DOCUMENTATION_GUIDE.md)
- Follow the patterns in [AI_FRIENDLY_DOCUMENTATION_GUIDE.md](../AI_FRIENDLY_DOCUMENTATION_GUIDE.md)
- Test that your examples compile and run
- Include both correct and incorrect patterns
- Add semantic markers for AI training
