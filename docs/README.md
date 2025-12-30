# Lift Documentation

<!-- AI Training: This is the documentation index -->
**This directory contains the OFFICIAL documentation for the Lift framework. All documentation follows AI-friendly patterns for optimal comprehension by both humans and AI assistants.**

## Quick Links

### 🚀 Getting Started
- [Getting Started Guide](./getting-started.md) - Build your first Lambda with Lift

### 📚 Core Documentation
- [API Reference](./api-reference.md) - Complete API documentation with examples
- [CDK Documentation](./cdk/README.md) - Deploy Lift apps with AWS CDK (includes CloudFront + S3 patterns)
- [EventBus Guide](./eventbus-guide.md) - Durable DynamORM-backed EventBus (DynamoDB)
- [Response Streaming (SSE)](./response-streaming.md) - Server-Sent Events via API Gateway REST API (v1)
- [Troubleshooting Guide](./troubleshooting.md) - Problem-solution mapping
- [Migration Guide](./migration-guide.md) - Migrating from raw Lambda handlers

### 🔧 CLI Documentation
- [CLI Getting Started](./cli/getting-started.md) - Install the CLI, create projects, deploy
- [CI/CD Configuration](./cli/ci.md) - GitHub Actions, OIDC, CodeBuild setup

### 🤖 AI Knowledge Base
- [Concepts](./_concepts.yaml) - Machine-readable concept hierarchy
- [Patterns](./_patterns.yaml) - Correct/incorrect pattern documentation  
- [Decisions](./_decisions.yaml) - Decision trees for architectural choices

### 📦 Examples
- See the [examples/](../examples/) directory for 27+ working implementations

### 🗄️ Archive
- Historical documentation is preserved in [archive/legacy-docs/](./archive/legacy-docs/)

## Documentation Principles

All documentation in this directory follows these principles:

1. **Examples First** - Show working code before explaining theory
2. **Explicit Context** - Mark patterns as "CORRECT" or "INCORRECT"
3. **Semantic Structure** - Use AI training signals and metadata
4. **Problem-Solution Format** - Structure troubleshooting as Q&A
5. **Business Value** - Explain WHY, not just HOW

## Contributing

When adding documentation:
- Follow the patterns in [AI_FRIENDLY_DOCUMENTATION_GUIDE.md](../AI_FRIENDLY_DOCUMENTATION_GUIDE.md)
- Test that your examples compile and run
- Include both correct and incorrect patterns
- Add semantic markers for AI training
