# Lift CDK API Reference

## Table of Contents

- [Core Constructs](#core-constructs)
  - [LiftFunction](#liftfunction)
  - [LiftAPI](#liftapi)
  - [LiftTable](#lifttable)
- [Middleware Constructs](#middleware-constructs)
  - [RateLimitedFunction](#ratelimitedfunction)
  - [IdempotentFunction](#idempotentfunction)
  - [SecureFunction](#securefunction)
  - [MonitoredFunction](#monitoredfunction)
- [Patterns](#patterns)
  - [BasicAPI](#basicapi)
  - [SecureAPI](#secureapi)
  - [LiftApp](#liftapp)
- [Stacks](#stacks)
  - [MicroserviceStack](#microservicestack)
  - [MultiTenantSaaSStack](#multitenantssaasstack)
  - [EventDrivenStack](#eventdrivenstack)

## Core Constructs

### LiftFunction

Base Lambda function construct optimized for Lift applications.

```go
import "github.com/pay-theory/lift/pkg/cdk/constructs"

func := constructs.NewLiftFunction(stack, id, props)
```

#### Properties

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| `CodeAssetPath` | `*string` | Required | Path to compiled bootstrap binary |
| `Handler` | `*string` | `"bootstrap"` | Handler name |
| `Runtime` | `awslambda.Runtime` | `PROVIDED_AL2023` | Lambda runtime |
| `Architecture` | `awslambda.Architecture` | `ARM_64` | CPU architecture |
| `MemorySize` | `*float64` | `512` | Memory in MB (128-10240) |
| `Timeout` | `*float64` | `30` | Timeout in seconds (1-900) |
| `Environment` | `*map[string]*string` | `nil` | Environment variables |
| `TracingEnabled` | `*bool` | `true` | Enable X-Ray tracing |
| `DeadLetterQueue` | `*bool` | `false` | Add DLQ for failed invocations |
| `LogRetentionDays` | `*float64` | `7` | CloudWatch log retention |
| `ReservedConcurrentExecutions` | `*float64` | `nil` | Reserved concurrent executions |
| `MaxEventAge` | `*float64` | `nil` | Maximum event age in seconds |
| `RetryAttempts` | `*float64` | `2` | Maximum retry attempts |

#### Methods

| Method | Description |
|--------|-------------|
| `Function() awslambda.Function` | Get underlying Lambda function |
| `Role() awsiam.Role` | Get function's IAM role |
| `LogGroup() awslogs.LogGroup` | Get CloudWatch log group |
| `DeadLetterQueue() awssqs.Queue` | Get DLQ (if enabled) |

#### Example

```go
liftFunc := constructs.NewLiftFunction(stack, jsii.String("MyFunction"), &constructs.LiftFunctionProps{
    CodeAssetPath: jsii.String("./dist/bootstrap"),
    MemorySize: jsii.Number(1024),
    Timeout: jsii.Number(60),
    Environment: &map[string]*string{
        "LOG_LEVEL": jsii.String("debug"),
        "STAGE": jsii.String("prod"),
    },
    DeadLetterQueue: jsii.Bool(true),
    LogRetentionDays: jsii.Number(30),
})
```

### LiftAPI

API Gateway v2 construct integrated with Lift.

```go
api := constructs.NewLiftAPI(stack, id, props)
```

#### Properties

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| `ApiName` | `*string` | Required | API name |
| `LiftHandler` | `LiftFunction` | Required | Lift function to integrate |
| `Description` | `*string` | `nil` | API description |
| `CorsOrigins` | `[]*string` | `["*"]` | CORS allowed origins |
| `CorsHeaders` | `[]*string` | `["*"]` | CORS allowed headers |
| `CorsMethods` | `[]*string` | All methods | CORS allowed methods |
| `CorsExposeHeaders` | `[]*string` | `nil` | CORS expose headers |
| `CustomDomainName` | `*string` | `nil` | Custom domain |
| `CertificateArn` | `*string` | `nil` | ACM certificate ARN |
| `StageName` | `*string` | `"$default"` | API stage name |
| `ThrottleRateLimit` | `*float64` | `10000` | Requests per second |
| `ThrottleBurstLimit` | `*float64` | `5000` | Burst capacity |
| `DisableExecuteApiEndpoint` | `*bool` | `false` | Disable default endpoint |
| `RouteSelectionExpression` | `*string` | `"$request.method $request.path"` | Route selection |
| `ValidateRequestBody` | `*bool` | `false` | Enable request validation |
| `ValidateRequestParameters` | `*bool` | `false` | Enable parameter validation |
| `EnableAccessLogging` | `*bool` | `false` | Enable access logs |
| `AccessLogFormat` | `*string` | JSON format | Access log format |
| `EnableDetailedMetrics` | `*bool` | `false` | Enable detailed CloudWatch metrics |

#### Methods

| Method | Description |
|--------|-------------|
| `Api() awsapigatewayv2.HttpApi` | Get underlying API Gateway |
| `DefaultStage() awsapigatewayv2.HttpStage` | Get default stage |
| `Url() *string` | Get API endpoint URL |
| `CustomDomain() awsapigatewayv2.DomainName` | Get custom domain (if configured) |

#### Example

```go
api := constructs.NewLiftAPI(stack, jsii.String("API"), &constructs.LiftAPIProps{
    ApiName: jsii.String("my-api"),
    LiftHandler: myFunction,
    
    // CORS configuration
    CorsOrigins: jsii.Strings("https://example.com", "https://app.example.com"),
    CorsHeaders: jsii.Strings("Content-Type", "Authorization"),
    
    // Custom domain
    CustomDomainName: jsii.String("api.example.com"),
    CertificateArn: jsii.String("arn:aws:acm:us-east-1:123456789012:certificate/..."),
    
    // Throttling
    ThrottleRateLimit: jsii.Number(1000),
    ThrottleBurstLimit: jsii.Number(2000),
    
    // Logging
    EnableAccessLogging: jsii.Bool(true),
    EnableDetailedMetrics: jsii.Bool(true),
})
```

### LiftTable

DynamoDB table construct with single-table design support.

```go
table := constructs.NewLiftTable(stack, id, props)
```

#### Properties

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| `TableName` | `*string` | Auto-generated | Table name |
| `PartitionKey` | `*awsdynamodb.Attribute` | Required | Partition key |
| `SortKey` | `*awsdynamodb.Attribute` | `nil` | Sort key |
| `BillingMode` | `awsdynamodb.BillingMode` | `PAY_PER_REQUEST` | Billing mode |
| `PointInTimeRecovery` | `*bool` | `true` | Enable PITR |
| `Stream` | `awsdynamodb.StreamViewType` | `nil` | DynamoDB Streams |
| `Encryption` | `awsdynamodb.TableEncryption` | `AWS_MANAGED` | Encryption type |
| `RemovalPolicy` | `awscdk.RemovalPolicy` | `RETAIN` | Deletion policy |
| `GlobalSecondaryIndexes` | `[]*GlobalSecondaryIndex` | `nil` | GSIs |
| `TimeToLiveAttribute` | `*string` | `nil` | TTL attribute |

#### Example

```go
table := constructs.NewLiftTable(stack, jsii.String("DataTable"), &constructs.LiftTableProps{
    PartitionKey: &awsdynamodb.Attribute{
        Name: jsii.String("PK"),
        Type: awsdynamodb.AttributeType_STRING,
    },
    SortKey: &awsdynamodb.Attribute{
        Name: jsii.String("SK"),
        Type: awsdynamodb.AttributeType_STRING,
    },
    GlobalSecondaryIndexes: &[]*constructs.GlobalSecondaryIndex{
        {
            IndexName: jsii.String("GSI1"),
            PartitionKey: &awsdynamodb.Attribute{
                Name: jsii.String("GSI1PK"),
                Type: awsdynamodb.AttributeType_STRING,
            },
            SortKey: &awsdynamodb.Attribute{
                Name: jsii.String("GSI1SK"),
                Type: awsdynamodb.AttributeType_STRING,
            },
        },
    },
    TimeToLiveAttribute: jsii.String("ttl"),
})
```

## Middleware Constructs

### RateLimitedFunction

Lambda function with built-in rate limiting.

```go
rateLimited := constructs.NewRateLimitedFunction(stack, id, props)
```

#### Properties

Extends [LiftFunctionProps](#liftfunction) with:

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| `LimitType` | `*string` | `"IP"` | Type: "IP", "User", or "Tenant" |
| `RequestLimit` | `*float64` | `1000` | Requests per window |
| `WindowSeconds` | `*float64` | `3600` | Time window in seconds |
| `TableName` | `*string` | Auto-generated | DynamoDB table name |
| `EnableBurstCapacity` | `*bool` | `true` | Allow burst capacity |
| `BurstMultiplier` | `*float64` | `2` | Burst capacity multiplier |

#### Methods

Inherits all [LiftFunction](#liftfunction) methods plus:

| Method | Description |
|--------|-------------|
| `RateLimitTable() awsdynamodb.Table` | Get rate limit table |

#### Example

```go
rateLimited := constructs.NewRateLimitedFunction(stack, jsii.String("API"), &constructs.RateLimitedFunctionProps{
    LiftFunctionProps: constructs.LiftFunctionProps{
        CodeAssetPath: jsii.String("./dist/bootstrap"),
        MemorySize: jsii.Number(1024),
    },
    LimitType: jsii.String("User"),     // Rate limit by user ID
    RequestLimit: jsii.Number(100),     // 100 requests
    WindowSeconds: jsii.Number(900),    // per 15 minutes
})
```

### IdempotentFunction

Lambda function with automatic idempotency.

```go
idempotent := constructs.NewIdempotentFunction(stack, id, props)
```

#### Properties

Extends [LiftFunctionProps](#liftfunction) with:

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| `KeySource` | `*string` | `"header"` | Source: "header", "body", or "path" |
| `KeyPath` | `*string` | `"X-Idempotency-Key"` | Path to extract key |
| `TTLSeconds` | `*float64` | `86400` | Record TTL (24 hours) |
| `TableName` | `*string` | Auto-generated | DynamoDB table name |
| `ResponseSizeLimit` | `*float64` | `400000` | Max response size (bytes) |

#### Methods

Inherits all [LiftFunction](#liftfunction) methods plus:

| Method | Description |
|--------|-------------|
| `IdempotencyTable() awsdynamodb.Table` | Get idempotency table |

#### Example

```go
idempotent := constructs.NewIdempotentFunction(stack, jsii.String("Payment"), &constructs.IdempotentFunctionProps{
    LiftFunctionProps: constructs.LiftFunctionProps{
        CodeAssetPath: jsii.String("./dist/bootstrap"),
        Timeout: jsii.Number(60),
    },
    KeySource: jsii.String("body"),
    KeyPath: jsii.String("paymentId"),
    TTLSeconds: jsii.Number(172800), // 48 hours
})
```

### SecureFunction

Lambda function with enhanced security features.

```go
secure := constructs.NewSecureFunction(stack, id, props)
```

#### Properties

Extends [LiftFunctionProps](#liftfunction) with:

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| `Vpc` | `awsec2.IVpc` | `nil` | Existing VPC |
| `CreateVpc` | `*bool` | `false` | Create new VPC |
| `VpcCidr` | `*string` | `"10.0.0.0/16"` | VPC CIDR (if creating) |
| `UsePrivateSubnets` | `*bool` | `false` | Use private subnets only |
| `SecurityGroupIds` | `[]*string` | `nil` | Security group IDs |
| `SecretArns` | `[]*string` | `nil` | Secrets Manager ARNs |
| `EncryptEnvironmentVariables` | `*bool` | `false` | KMS encryption for env vars |
| `KmsKeyArn` | `*string` | `nil` | Custom KMS key ARN |

#### Methods

Inherits all [LiftFunction](#liftfunction) methods plus:

| Method | Description |
|--------|-------------|
| `Vpc() awsec2.IVpc` | Get VPC |
| `SecurityGroups() []awsec2.ISecurityGroup` | Get security groups |
| `KmsKey() awskms.IKey` | Get KMS key |

#### Example

```go
secure := constructs.NewSecureFunction(stack, jsii.String("Secure"), &constructs.SecureFunctionProps{
    LiftFunctionProps: constructs.LiftFunctionProps{
        CodeAssetPath: jsii.String("./dist/bootstrap"),
        Environment: &map[string]*string{
            "DATABASE_URL": jsii.String("{{resolve:secretsmanager:db-secret:SecretString:url}}"),
        },
    },
    CreateVpc: jsii.Bool(true),
    UsePrivateSubnets: jsii.Bool(true),
    SecretArns: &[]*string{
        jsii.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:db-secret"),
    },
    EncryptEnvironmentVariables: jsii.Bool(true),
})
```

### MonitoredFunction

Lambda function with comprehensive monitoring.

```go
monitored := constructs.NewMonitoredFunction(stack, id, props)
```

#### Properties

Extends [LiftFunctionProps](#liftfunction) with:

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| `CreateDashboard` | `*bool` | `true` | Create CloudWatch dashboard |
| `DashboardName` | `*string` | Auto-generated | Dashboard name |
| `EnableAlarms` | `*bool` | `true` | Enable CloudWatch alarms |
| `AlarmEmail` | `*string` | `nil` | SNS topic email for alarms |
| `ErrorRateThreshold` | `*float64` | `0.01` | Error rate threshold (1%) |
| `LatencyThreshold` | `*float64` | `1000` | Latency threshold (ms) |
| `ThrottleThreshold` | `*float64` | `10` | Throttle threshold |
| `ConcurrentExecutionsThreshold` | `*float64` | `900` | Concurrent executions |
| `LambdaInsights` | `*bool` | `true` | Enable Lambda Insights |
| `EnableLogInsightsQueries` | `*bool` | `false` | Add Log Insights queries |

#### Methods

Inherits all [LiftFunction](#liftfunction) methods plus:

| Method | Description |
|--------|-------------|
| `Dashboard() awscloudwatch.Dashboard` | Get dashboard |
| `AlarmTopic() awssns.Topic` | Get alarm SNS topic |
| `ErrorAlarm() awscloudwatch.Alarm` | Get error rate alarm |
| `LatencyAlarm() awscloudwatch.Alarm` | Get latency alarm |
| `AddMetric(name, unit, value)` | Add custom metric |
| `AddAlarm(name, metric, threshold)` | Add custom alarm |
| `AddLogInsightsQuery(name, query)` | Add Log Insights query |
| `AddCommonLogInsightsQueries()` | Add pre-built queries |

#### Example

```go
monitored := constructs.NewMonitoredFunction(stack, jsii.String("API"), &constructs.MonitoredFunctionProps{
    LiftFunctionProps: constructs.LiftFunctionProps{
        CodeAssetPath: jsii.String("./dist/bootstrap"),
    },
    EnableAlarms: jsii.Bool(true),
    AlarmEmail: jsii.String("ops@example.com"),
    ErrorRateThreshold: jsii.Number(0.05),  // 5% error rate
    LatencyThreshold: jsii.Number(2000),    // 2 seconds
    EnableLogInsightsQueries: jsii.Bool(true),
})

// Add custom metric
monitored.AddMetric("PaymentProcessed", awscloudwatch.Unit_COUNT, jsii.Number(1))

// Add custom query
monitored.AddLogInsightsQuery("HighValuePayments", `
    fields @timestamp, amount, userId
    | filter amount > 1000
    | sort @timestamp desc
`)
```

## Patterns

### BasicAPI

Complete API with Lambda backend.

```go
api := patterns.NewBasicAPI(stack, id, props)
```

#### Properties

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| `ApiName` | `*string` | Required | API name |
| `CodeAssetPath` | `*string` | Required | Lambda code path |
| `EnableCors` | `*bool` | `true` | Enable CORS |
| `EnableLogging` | `*bool` | `true` | Enable CloudWatch logs |
| `MemorySize` | `*float64` | `512` | Lambda memory |
| `Timeout` | `*float64` | `30` | Lambda timeout |
| `Environment` | `*map[string]*string` | `nil` | Environment variables |
| `CustomDomainName` | `*string` | `nil` | Custom domain |
| `CertificateArn` | `*string` | `nil` | ACM certificate |

#### Methods

| Method | Description |
|--------|-------------|
| `Function() *constructs.LiftFunction` | Get Lambda function |
| `Api() *constructs.LiftAPI` | Get API Gateway |

### SecureAPI

API with security features.

```go
secure := patterns.NewSecureAPI(stack, id, props)
```

#### Properties

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| `ApiName` | `*string` | Required | API name |
| `CodeAssetPath` | `*string` | Required | Lambda code path |
| `EnableWAF` | `*bool` | `true` | Enable AWS WAF |
| `EnableRateLimiting` | `*bool` | `true` | Enable rate limiting |
| `RateLimitPerHour` | `*float64` | `10000` | Rate limit |
| `EnableVPC` | `*bool` | `false` | Use VPC |
| `UsePrivateSubnets` | `*bool` | `false` | Private subnets only |
| `EnableMonitoring` | `*bool` | `true` | Enable monitoring |
| `AlarmEmail` | `*string` | `nil` | Alarm notifications |

### LiftApp

Complete Lift application pattern.

```go
app := patterns.NewLiftApp(stack, id, props)
```

#### Properties

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| `AppName` | `*string` | Required | Application name |
| `CodeAssetPath` | `*string` | Required | Lambda code path |
| `EnableDatabase` | `*bool` | `false` | Add DynamoDB table |
| `EnableRateLimiting` | `*bool` | `false` | Enable rate limiting |
| `EnableIdempotency` | `*bool` | `false` | Enable idempotency |
| `EnableMultiTenant` | `*bool` | `false` | Multi-tenant support |
| `EnableCache` | `*bool` | `false` | Add ElastiCache |
| `EnableQueue` | `*bool` | `false` | Add SQS queue |
| `EnableNotifications` | `*bool` | `false` | Add SNS topic |

## Stacks

### MicroserviceStack

Stack for individual microservices.

```go
stack := stacks.NewMicroserviceStack(app, id, props)
```

#### Properties

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| `ServiceName` | `string` | Required | Service name |
| `CodePath` | `string` | Required | Lambda code path |
| `EnableDatabase` | `bool` | `false` | Add DynamoDB |
| `EnableCache` | `bool` | `false` | Add ElastiCache |
| `EnableQueue` | `bool` | `false` | Add SQS queue |
| `MemorySize` | `int` | `512` | Lambda memory |
| `Environment` | `map[string]string` | `nil` | Environment vars |

### MultiTenantSaaSStack

Complete SaaS application stack.

```go
stack := stacks.NewMultiTenantSaaSStack(app, id, props)
```

#### Properties

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| `AppName` | `string` | Required | Application name |
| `CodePath` | `string` | Required | Lambda code path |
| `EnableAuth` | `bool` | `true` | Add Cognito |
| `EnableFileStorage` | `bool` | `false` | Add S3 bucket |
| `CustomDomainName` | `string` | `nil` | Custom domain |
| `CertificateArn` | `string` | `nil` | ACM certificate |
| `EnableAnalytics` | `bool` | `false` | Add analytics |

### EventDrivenStack

Event-driven architecture stack.

```go
stack := stacks.NewEventDrivenStack(app, id, props)
```

#### Properties

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| `AppName` | `string` | Required | Application name |
| `ApiCodePath` | `string` | Required | API Lambda path |
| `EventProcessorCodePath` | `string` | Required | Processor path |
| `EnableDLQ` | `bool` | `true` | Add dead letter queue |
| `EventBusName` | `string` | Auto-generated | EventBridge bus |
| `EventRules` | `[]EventRule` | `nil` | Event routing rules |