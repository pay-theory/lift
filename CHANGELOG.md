# Changelog

## [1.0.85-rc](https://github.com/pay-theory/lift/compare/v1.0.84...v1.0.85-rc) (2026-01-14)


### Bug Fixes

* **ci:** publish release assets via github.token ([996a3bd](https://github.com/pay-theory/lift/commit/996a3bd1bb5d15bbeb510db608c62e7968de0071))
* **ci:** publish release assets via github.token ([084f079](https://github.com/pay-theory/lift/commit/084f0796bca751000618cdd1574a06dd15c9c7d5))

## v1.0.82 - 2026-01-04

### Added

- **Lift CLI**: Full implementation of the `lift` command-line tool for project scaffolding and deployment:
  - `lift new <project-name>` - Bootstrap new Lift projects with pre-configured templates
  - `lift build` - Compile Lambda functions with configurable build settings in `lift.yaml`
  - `lift up` - Build and deploy CDK stacks with domain immutability enforcement
  - `lift down` - Destroy stacks in reverse order and clear state files
  - `lift add function` - Add new Lambda functions to existing projects with dynamic placeholders
  - Prerequisite checks for Go and CDK availability with actionable error messages
  - Partner mode (`--pt`) and target mode (`--partner`, `--target-mode`) support for Pay Theory deployments
- **Project Templates**: New project templates for common use cases:
  - `basic-api` - Standard REST API template with GitHub Actions CI/CD
  - `basic-api-pt` - Pay Theory-specific variant with CodeBuild pipelines
  - `event-driven` - Event-driven architecture template
  - `microservice` - Microservice template with enhanced configuration
  - `merchant-app` - Merchant application template
- **CDK Constructs**:
  - `PathRoutedFrontendDistribution` - CloudFront distribution with path-based routing
  - DynamoDB stream event handling with dedicated registration and context record parsing
  - CloudFront helper functions and enhanced event bus components
  - SNS processor template for error notifications
- **Event Bus**: Comprehensive event bus streaming, scheduling, and processing capabilities with new CDK constructs, DynamoDB durability, and idempotency middleware updates
- **WebSocket**: Action routing support with naming utilities
- **DynamoDB Facade**: New facade for simplified DynamoDB interactions
- **Testing Utilities**: Extensive middleware and enterprise testing utilities, security and circuit breaker test coverage improvements
- **Documentation**: New comprehensive guides for CDK, CLI, IP gating, error notifications, JWT patterns, and getting started

### Changed

- **CloudWatch**: Modularized dashboard row creation for Lambda monitoring with new helper functions
- **Go Modules**: Streamlined Go module handling in CI/CD and CDK workflows; removed `constructs-go/constructs/v10` dependency
- **Documentation**: Complete restructure for clarity; removed outdated development notes
- **CI/CD**: Added release workflow and updated GitHub Actions configurations

### Fixed

- **CLI Tests**: Mock CDK/Go presence in tests to support CI environments without local installations

## v1.0.81 - 2025-12-21

### Added
- **CDK (LiftRestAPI)**: Per-method response streaming overrides via `IntegrationOptions.EnableStreaming` and `IntegrationOptions.StreamingTimeoutSeconds`, enabling mixed buffered + streaming methods on the same REST API.

### Changed
- **Runtime (SSE)**: `lift.SSEResponse` returns `events.APIGatewayProxyStreamingResponse` for API Gateway REST API (v1) triggers (keeps `events.LambdaFunctionURLStreamingResponse` for other trigger types).
- **Runtime (SSE)**: Added `ctx.AddMultiValueHeader(...)` support for API Gateway REST API (v1) streaming responses (`multiValueHeaders` metadata).
- **Dependencies**: Updated `github.com/aws/aws-lambda-go` to include `events.APIGatewayProxyStreamingResponse`.
- **Docs**: Clarified per-method streaming configuration (`ResponseTransferMode=STREAM` + `/response-streaming-invocations`) and documented key platform limits (15m integration timeout, idle timeouts).

## v1.0.80 - 2025-12-21

### Added
- **Response Streaming**: Added Server-Sent Events (SSE) support for Lambda response streaming via `LiftRestAPI` construct. See `docs/response-streaming.md` for usage.
- **LiftRestAPI Construct**: New CDK construct for API Gateway v1 with enhanced configuration options and streaming support.
- **IP Gating Documentation**: Added comprehensive documentation for IP authorization and error handling patterns.

### Changed
- **Observability** (BREAKING): Removed `WithDefaultErrorNotifications` function which contained hardcoded internal account IDs. Replaced with `WithEnvironmentErrorNotifications` which reads the SNS topic ARN from the `ERROR_NOTIFICATION_SNS_TOPIC_ARN` environment variable. See migration guide at `/docs/migration-sns-error-notifications.md`.
- **REST API Configuration**: Refactored REST API configuration into dedicated helper methods for improved maintainability.
- **Streaming Error Handling**: Improved error handling for streaming responses.

## v1.0.71 - 2025-10-27

### Added
- **Event Bus**: Introduced DynamoDB-backed event bus service with CDK constructs, helper utilities, and documentation. Includes DynamoDB integration helpers and comprehensive service tests.
- **Streamer Client**: Added streamer client library with connection lifecycle management, structured errors, mocks, and demo application.
- **Documentation**: Added event bus and streamer guides along with new LLM FAQ entries and planning notes.

### Changed
- **WebSocket Context**: Migrated management client to AWS SDK v2 with connection metadata helpers, thread-safe reuse, and improved region resolution.
- **Testing & Tooling**: Expanded mocks, added load-shedding and observability test coverage, and refreshed Go module dependencies.

## v1.0.70 - 2025-01-15

### Added
- **Kernel Client**: Added `pkg/services/kernel` package for authenticated cross-account calls to kernel services. Provides SigV4-signed API Gateway calls with STS AssumeRole authentication, matching Python's `secure_api_call.py` pattern. Includes convenience functions for K3, Paze Wallet, Apple Wallet, Google Wallet, Bin Lookup, and Bank Data services. Supports both shared role (`kernel-access`) and external partner role (`kernel-access-external`) authentication modes. Uses singleton logger pattern via LoggerFunc. See `pkg/services/kernel/doc.go` and `examples/kernel_client_example.go` for usage.

### Changed
- **Observability**: Updated SNS notification APIs. See v1.0.72+ for the current API.
- **Observability**: Added `WithPartnerErrorNotifications` for services that need partner-specific SNS topics (`cns-{partner}-{stage}`). This includes AWS account ID auto-detection via STS GetCallerIdentity when `AWS_ACCOUNT_ID` environment variable is not set. Most services should use `WithDefaultErrorNotifications` instead.
- Router: Unmatched HTTP routes now return structured 404 `LiftError` instead of a generic error.
- Response: `Binary` responses are correctly base64-encoded and flagged with `isBase64Encoded=true`; JSON marshalling respects base64 mode.
- Middleware (Lift IP Authorization): Stop writing responses via deprecated context helpers; now returns `LiftError`s (`ParameterError`, `SystemError`, `AuthorizationError`).
- Health Endpoints: Added optional structured logging support via `HealthEndpointsConfig.Logger`; falls back to standard logging when not provided.
- Docs sweep for accuracy and consistency:
  - Replace `middleware.JWT(...)` with `middleware.JWTAuth(...)` and remove erroneous error returns.
  - Replace `ctx.Bind(...)` with `ctx.ParseRequest(...)`.
  - Use `lift.NewLiftError(...)` and dedicated error constructors (`ValidationError`, `NotFound`, `SystemError`) instead of `NewError`/`BadRequest` patterns.
  - Fix response examples to use `ctx.JSON(data)` for 200 or `ctx.Status(code).JSON(data)` otherwise.
  - Update training and guidance files: `_patterns.yaml`, `_decisions.yaml`, `troubleshooting.md`, `migration-guide.md`, `api-reference.md`, `core-patterns.md`, `dynamorm-integration.md`, `README.md`, `development-guidelines.md`, `cdk/event-driven-api-pattern.md`.
- Tests: Added tests for router 404 behavior and binary response encoding.

### JWT Consolidation

- Canonical API is `middleware.JWTAuth(middleware.JWTConfig)`.
- `lift.WithJWTAuth` and `lift.WithSimpleJWTAuth` are now deprecated; see `docs/migration-jwt-consolidation.md`.

> Note: `HealthEndpointsConfig` gained an optional `Logger` field; this is backwards-compatible. Existing uses continue to work without changes.
