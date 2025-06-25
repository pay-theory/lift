# Lift Framework Examples

This directory contains working examples demonstrating various features and patterns of the Lift framework.

## 🚀 Getting Started

### Core Examples
- **[hello-world](./hello-world/)** - Minimal Lambda function with type safety
- **[basic-crud-api](./basic-crud-api/)** - Complete CRUD API with middleware and testing
- **[error-handling](./error-handling/)** - Structured error handling patterns

## 🔐 Authentication & Security
- **[jwt-auth](./jwt-auth/)** - JWT authentication with middleware
- **[jwt-auth-demo](./jwt-auth-demo/)** - JWT authentication demonstration
- **[rate-limiting](./rate-limiting/)** - Rate limiting middleware implementation

## 🔄 Real-time & Event-Driven
- **[websocket-demo](./websocket-demo/)** - WebSocket support with connection management
- **[websocket-enhanced](./websocket-enhanced/)** - Advanced WebSocket implementation
- **[event-adapters](./event-adapters/)** - Multiple AWS event source handlers (SQS, S3, EventBridge)
- **[multi-event-handler](./multi-event-handler/)** - Handling multiple event types
- **[eventbridge-wakeup](./eventbridge-wakeup/)** - EventBridge scheduled events
- **[multiple-scheduled-events](./multiple-scheduled-events/)** - Multiple scheduled Lambda triggers

## 🏢 Enterprise Applications
- **[multi-tenant-saas](./multi-tenant-saas/)** - Multi-tenant SaaS patterns
- **[enterprise-banking](./enterprise-banking/)** - Banking application with SOC2 compliance
- **[enterprise-healthcare](./enterprise-healthcare/)** - Healthcare with HIPAA compliance
- **[enterprise-ecommerce](./enterprise-ecommerce/)** - E-commerce platform example

## 🏭 Production Patterns
- **[production-api](./production-api/)** - Production-ready API configuration
- **[multi-service-demo](./multi-service-demo/)** - Microservices with chaos engineering
- **[observability-demo](./observability-demo/)** - Comprehensive logging and monitoring
- **[health-monitoring](./health-monitoring/)** - Health check implementation

## 🧪 Testing & Development
- **[mocking-demo](./mocking-demo/)** - Testing with mocks
- **[cloudwatch-mocking-demo](./cloudwatch-mocking-demo/)** - CloudWatch metrics mocking
- **[dynamorm-integration](./dynamorm-integration/)** - DynamORM database integration
- **[streamer-quickstart](./streamer-quickstart/)** - Quick start with streaming

## 🔧 Special Purpose
- **[sprint6-deployment](./sprint6-deployment/)** - Deployment patterns
- **[test-event-routing-bug](./test-event-routing-bug/)** - Event routing test cases
- **[test-scheduled-fix](./test-scheduled-fix/)** - Scheduled event fixes

## Running Examples

Each example directory contains its own `main.go` file and often a `README.md` with specific instructions. To run an example:

```bash
cd examples/hello-world
go run main.go
```

For Lambda deployment:

```bash
GOOS=linux GOARCH=amd64 go build -o bootstrap main.go
zip function.zip bootstrap
```

## Testing Examples

Some examples include test files. Run tests with:

```bash
go test -v
```

## Contributing

When adding new examples:
1. Create a descriptive directory name
2. Include a `README.md` explaining the example
3. Keep examples focused on demonstrating specific features
4. Include tests where appropriate
5. Update this file to list your example