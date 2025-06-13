# Production Guide

This guide covers best practices for deploying and operating Lift applications in production environments. Learn how to optimize performance, ensure reliability, and maintain security at scale.

## Pre-Production Checklist

### Code Quality

```bash
# Run all tests
go test ./... -race -cover

# Check test coverage (aim for 80%+)
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Static analysis
golangci-lint run
gosec ./...

# Dependency vulnerabilities
go list -json -m all | nancy sleuth

# Build verification
GOOS=linux GOARCH=amd64 go build -o bootstrap main.go
```

### Configuration Review

```go
// production.config.go
func GetProductionConfig() lift.Config {
    return lift.Config{
        AppName:     "order-service",
        Environment: "production",
        
        // Logging
        LogLevel: "info", // Not debug in production
        
        // Timeouts
        DefaultTimeout: 29 * time.Second, // Lambda limit is 30s
        
        // Security
        EnableAuth:       true,
        RequireHTTPS:     true,
        
        // Performance
        EnableMetrics:    true,
        EnableTracing:    true,
        EnableCaching:    true,
        
        // Error handling
        DetailedErrors:   false, // Hide internal errors
        
        // Rate limiting
        RateLimitEnabled: true,
        RateLimitWindow:  time.Minute,
        RateLimitMax:     1000,
    }
}
```

## Deployment Strategies

### Blue-Green Deployment

```yaml
# serverless.yml
service: my-service

provider:
  name: aws
  runtime: provided.al2
  architecture: arm64 # Graviton2 for better price/performance

functions:
  api:
    handler: bootstrap
    events:
      - httpApi: '*'
    environment:
      LIFT_ENV: production
    memorySize: 512
    timeout: 30
    reservedConcurrency: 100
    
    # Enable gradual deployment
    deploymentSettings:
      type: Linear10PercentEvery5Minutes
      alias: live
      alarms:
        - HighErrorRateAlarm
        - HighLatencyAlarm
```

### Canary Deployment

```go
// Canary deployment configuration
type CanaryConfig struct {
    Percentage int
    Duration   time.Duration
    Metrics    []string
}

func setupCanary() {
    config := CanaryConfig{
        Percentage: 10, // 10% of traffic to new version
        Duration:   30 * time.Minute,
        Metrics: []string{
            "errors",
            "latency_p99",
            "success_rate",
        },
    }
    
    // Monitor canary metrics
    go monitorCanary(config)
}
```

### Infrastructure as Code

```hcl
# terraform/lambda.tf
resource "aws_lambda_function" "api" {
  function_name = "${var.service_name}-${var.environment}"
  role          = aws_iam_role.lambda.arn
  handler       = "bootstrap"
  runtime       = "provided.al2"
  
  # Code
  s3_bucket = aws_s3_bucket.deployments.id
  s3_key    = "lambda/${var.version}/function.zip"
  
  # Performance
  memory_size                    = var.memory_size
  timeout                       = 30
  reserved_concurrent_executions = var.reserved_concurrency
  
  # Environment
  environment {
    variables = {
      LIFT_ENV           = var.environment
      JWT_SECRET_ARN     = aws_secretsmanager_secret.jwt.arn
      DYNAMODB_TABLE     = aws_dynamodb_table.main.name
      METRICS_NAMESPACE  = var.metrics_namespace
    }
  }
  
  # VPC (if needed)
  vpc_config {
    subnet_ids         = var.private_subnet_ids
    security_group_ids = [aws_security_group.lambda.id]
  }
  
  # Tracing
  tracing_config {
    mode = "Active"
  }
  
  # Dead letter queue
  dead_letter_config {
    target_arn = aws_sqs_queue.dlq.arn
  }
  
  tags = var.tags
}

# Auto-scaling
resource "aws_lambda_provisioned_concurrency_config" "api" {
  function_name                     = aws_lambda_function.api.function_name
  provisioned_concurrent_executions = var.provisioned_concurrency
  qualifier                         = aws_lambda_alias.live.name
  
  # Scale based on schedule
  dynamic "provisioned_concurrency_config" {
    for_each = var.scaling_schedule
    content {
      schedule_expression = provisioned_concurrency_config.value.schedule
      min_capacity       = provisioned_concurrency_config.value.min
      max_capacity       = provisioned_concurrency_config.value.max
    }
  }
}
```

## Performance Optimization

### Lambda Configuration

```go
// Optimal memory configuration based on profiling
const (
    // CPU-bound operations
    CPUIntensiveMemory = 3008 // Max CPU allocation
    
    // I/O-bound operations  
    IOBoundMemory = 1024 // Balanced
    
    // Light operations
    LightweightMemory = 512 // Cost-optimized
)

// Choose based on workload
func getOptimalMemory(handlerType string) int {
    switch handlerType {
    case "data-processing":
        return CPUIntensiveMemory
    case "api-gateway":
        return IOBoundMemory
    case "webhook":
        return LightweightMemory
    default:
        return IOBoundMemory
    }
}
```

### Cold Start Optimization

```go
// Global initialization outside handler
var (
    db        *dynamodb.Client
    s3Client  *s3.Client
    initOnce  sync.Once
    initError error
)

func init() {
    // Pre-warm connections
    initOnce.Do(func() {
        cfg, err := config.LoadDefaultConfig(context.Background())
        if err != nil {
            initError = err
            return
        }
        
        // Initialize clients
        db = dynamodb.NewFromConfig(cfg)
        s3Client = s3.NewFromConfig(cfg)
        
        // Pre-compile regex
        compileRegexPatterns()
        
        // Load static data
        loadStaticData()
    })
}

// Handler checks initialization
func handler(ctx *lift.Context) error {
    if initError != nil {
        return lift.InternalError("Initialization failed")
    }
    
    // Use pre-initialized clients
    return processRequest(ctx, db, s3Client)
}
```

### Connection Pooling

```go
// HTTP client with connection pooling
var httpClient = &http.Client{
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
        TLSHandshakeTimeout: 10 * time.Second,
        ExpectContinueTimeout: 1 * time.Second,
        
        // Reuse connections
        DisableKeepAlives: false,
        
        // Connection limits
        MaxConnsPerHost:     10,
        ResponseHeaderTimeout: 10 * time.Second,
    },
    Timeout: 30 * time.Second,
}

// Database connection pool
type DBPool struct {
    connections chan *sql.DB
    maxSize     int
}

func NewDBPool(dsn string, maxSize int) (*DBPool, error) {
    pool := &DBPool{
        connections: make(chan *sql.DB, maxSize),
        maxSize:     maxSize,
    }
    
    // Pre-create connections
    for i := 0; i < maxSize; i++ {
        db, err := sql.Open("postgres", dsn)
        if err != nil {
            return nil, err
        }
        
        db.SetMaxIdleConns(1)
        db.SetMaxOpenConns(1)
        db.SetConnMaxLifetime(5 * time.Minute)
        
        pool.connections <- db
    }
    
    return pool, nil
}
```

### Caching Strategies

```go
// In-memory cache for Lambda container
var cache = &MemoryCache{
    data: make(map[string]CacheEntry),
    mu:   &sync.RWMutex{},
    ttl:  5 * time.Minute,
}

type MemoryCache struct {
    data map[string]CacheEntry
    mu   *sync.RWMutex
    ttl  time.Duration
}

type CacheEntry struct {
    Value      interface{}
    Expiration time.Time
}

func (c *MemoryCache) Get(key string) (interface{}, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    
    entry, exists := c.data[key]
    if !exists || time.Now().After(entry.Expiration) {
        return nil, false
    }
    
    return entry.Value, true
}

// Use in handler
func getCachedUser(ctx *lift.Context, userID string) (*User, error) {
    // Check cache first
    if cached, ok := cache.Get("user:" + userID); ok {
        return cached.(*User), nil
    }
    
    // Load from database
    user, err := loadUser(userID)
    if err != nil {
        return nil, err
    }
    
    // Cache for next request
    cache.Set("user:"+userID, user)
    
    return user, nil
}
```

## Reliability

### Error Recovery

```go
// Retry with exponential backoff
func withRetry(operation func() error, maxAttempts int) error {
    var err error
    
    for attempt := 0; attempt < maxAttempts; attempt++ {
        err = operation()
        if err == nil {
            return nil
        }
        
        // Check if error is retryable
        if !isRetryable(err) {
            return err
        }
        
        // Exponential backoff with jitter
        delay := time.Duration(math.Pow(2, float64(attempt))) * time.Second
        jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
        time.Sleep(delay + jitter)
    }
    
    return fmt.Errorf("max retries exceeded: %w", err)
}

// Circuit breaker pattern
type CircuitBreaker struct {
    failureThreshold int
    resetTimeout     time.Duration
    
    mu           sync.Mutex
    failures     int
    lastFailTime time.Time
    state        string // "closed", "open", "half-open"
}

func (cb *CircuitBreaker) Call(fn func() error) error {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    
    // Check state
    if cb.state == "open" {
        if time.Since(cb.lastFailTime) > cb.resetTimeout {
            cb.state = "half-open"
            cb.failures = 0
        } else {
            return errors.New("circuit breaker open")
        }
    }
    
    // Execute function
    err := fn()
    
    if err != nil {
        cb.failures++
        cb.lastFailTime = time.Now()
        
        if cb.failures >= cb.failureThreshold {
            cb.state = "open"
        }
        return err
    }
    
    // Success - reset
    cb.failures = 0
    cb.state = "closed"
    return nil
}
```

### Dead Letter Queue

```go
// DLQ handler for failed messages
func handleDLQ(ctx *lift.Context) error {
    for _, record := range ctx.Request.Records {
        // Parse failed message
        var failedMsg FailedMessage
        json.Unmarshal([]byte(record["body"].(string)), &failedMsg)
        
        // Log failure details
        ctx.Logger.Error("Processing DLQ message", map[string]interface{}{
            "message_id":     failedMsg.MessageID,
            "error":         failedMsg.Error,
            "attempt_count": failedMsg.Attempts,
            "original_time": failedMsg.Timestamp,
        })
        
        // Attempt recovery or alert
        if err := attemptRecovery(failedMsg); err != nil {
            // Send to permanent failure storage
            storeFailure(failedMsg)
            
            // Alert operations team
            sendAlert(AlertConfig{
                Severity: "high",
                Title:    "Message processing failed permanently",
                Details:  failedMsg,
            })
        }
    }
    
    return nil
}
```

### Health Checks

```go
// Comprehensive health check
func healthCheck(ctx *lift.Context) error {
    health := &HealthStatus{
        Service:   "order-service",
        Version:   version,
        Timestamp: time.Now(),
        Checks:    make([]HealthCheck, 0),
    }
    
    // Check database
    dbCheck := checkDatabase()
    health.Checks = append(health.Checks, dbCheck)
    
    // Check external services
    for _, service := range externalServices {
        check := checkService(service)
        health.Checks = append(health.Checks, check)
    }
    
    // Check resource usage
    resourceCheck := checkResources()
    health.Checks = append(health.Checks, resourceCheck)
    
    // Determine overall status
    health.Status = "healthy"
    for _, check := range health.Checks {
        if check.Status == "unhealthy" {
            health.Status = "unhealthy"
            ctx.Response.StatusCode = 503
            break
        }
    }
    
    return ctx.JSON(health)
}

func checkDatabase() HealthCheck {
    start := time.Now()
    err := db.Ping()
    
    return HealthCheck{
        Name:     "database",
        Status:   statusFromError(err),
        Duration: time.Since(start),
        Error:    errorString(err),
    }
}
```

## Security in Production

### Secrets Management

```go
// Load secrets from AWS Secrets Manager
func loadSecrets() error {
    secretsClient := secretsmanager.NewFromConfig(cfg)
    
    // Get secret
    result, err := secretsClient.GetSecretValue(context.Background(), 
        &secretsmanager.GetSecretValueInput{
            SecretId: aws.String("prod/myapp/secrets"),
        })
    if err != nil {
        return fmt.Errorf("failed to get secrets: %w", err)
    }
    
    // Parse secrets
    var secrets map[string]string
    if err := json.Unmarshal([]byte(*result.SecretString), &secrets); err != nil {
        return fmt.Errorf("failed to parse secrets: %w", err)
    }
    
    // Set environment variables
    for key, value := range secrets {
        os.Setenv(key, value)
    }
    
    return nil
}

// Rotate secrets
func setupSecretRotation() {
    // Check secret age periodically
    go func() {
        ticker := time.NewTicker(24 * time.Hour)
        for range ticker.C {
            if shouldRotateSecret() {
                rotateSecret()
            }
        }
    }()
}
```

### Security Headers

```go
// Production security headers
app.Use(middleware.SecurityHeaders(middleware.SecurityConfig{
    // Strict Transport Security
    HSTSMaxAge:            63072000, // 2 years
    HSTSIncludeSubdomains: true,
    HSTSPreload:          true,
    
    // Content Security Policy
    ContentSecurityPolicy: strings.Join([]string{
        "default-src 'none'",
        "script-src 'self'",
        "style-src 'self'",
        "img-src 'self' data: https:",
        "font-src 'self'",
        "connect-src 'self'",
        "frame-ancestors 'none'",
        "base-uri 'self'",
        "form-action 'self'",
    }, "; "),
    
    // Other headers
    XFrameOptions:        "DENY",
    XContentTypeOptions:  "nosniff",
    ReferrerPolicy:      "strict-origin-when-cross-origin",
    PermissionsPolicy:   "geolocation=(), microphone=(), camera=()",
}))
```

## Monitoring and Alerting

### CloudWatch Alarms

```go
// Create comprehensive alarms
func setupAlarms() error {
    cloudwatch := cloudwatch.NewFromConfig(cfg)
    
    alarms := []types.PutMetricAlarmInput{
        // Error rate alarm
        {
            AlarmName:          aws.String("high-error-rate"),
            MetricName:         aws.String("Errors"),
            Namespace:          aws.String("AWS/Lambda"),
            Statistic:          types.StatisticSum,
            Period:             aws.Int32(300),
            EvaluationPeriods:  aws.Int32(2),
            Threshold:          aws.Float64(10),
            ComparisonOperator: types.ComparisonOperatorGreaterThanThreshold,
            AlarmActions:       []string{snsTopicArn},
        },
        // Latency alarm
        {
            AlarmName:          aws.String("high-latency"),
            MetricName:         aws.String("Duration"),
            Namespace:          aws.String("AWS/Lambda"),
            Statistic:          types.StatisticAverage,
            Period:             aws.Int32(60),
            EvaluationPeriods:  aws.Int32(3),
            Threshold:          aws.Float64(1000), // 1 second
            ComparisonOperator: types.ComparisonOperatorGreaterThanThreshold,
            AlarmActions:       []string{snsTopicArn},
        },
        // Throttling alarm
        {
            AlarmName:          aws.String("throttling"),
            MetricName:         aws.String("Throttles"),
            Namespace:          aws.String("AWS/Lambda"),
            Statistic:          types.StatisticSum,
            Period:             aws.Int32(60),
            EvaluationPeriods:  aws.Int32(1),
            Threshold:          aws.Float64(1),
            ComparisonOperator: types.ComparisonOperatorGreaterThanThreshold,
            AlarmActions:       []string{snsTopicArn},
        },
    }
    
    for _, alarm := range alarms {
        _, err := cloudwatch.PutMetricAlarm(context.Background(), &alarm)
        if err != nil {
            return err
        }
    }
    
    return nil
}
```

### Custom Metrics

```go
// Business metrics for production monitoring
func recordBusinessMetrics(ctx *lift.Context, order Order) {
    // Order metrics
    ctx.Metrics.Count("orders.completed", 1, map[string]string{
        "product_type": order.ProductType,
        "payment_method": order.PaymentMethod,
    })
    
    ctx.Metrics.Record("orders.value", map[string]interface{}{
        "amount": order.Amount,
        "currency": order.Currency,
    })
    
    // Performance metrics
    if order.ProcessingTime > 5*time.Second {
        ctx.Metrics.Count("sla.violations", 1, map[string]string{
            "type": "slow_processing",
        })
    }
    
    // Customer metrics
    ctx.Metrics.Gauge("customers.active", float64(getActiveCustomers()))
}
```

### Distributed Tracing

```go
// Enhanced X-Ray tracing for production
func setupTracing() {
    xray.Configure(xray.Config{
        DaemonAddr:     "127.0.0.1:2000",
        ServiceVersion: version,
        
        // Sampling rules for production
        SamplingRules: []xray.SamplingRule{
            {
                ServiceName: "order-service",
                HTTPMethod:  "GET",
                URLPath:     "/health",
                FixedRate:   0.01, // 1% for health checks
            },
            {
                ServiceName: "order-service",
                HTTPMethod:  "*",
                URLPath:     "*",
                FixedRate:   0.1, // 10% for other requests
            },
        },
    })
}

// Add detailed segments
func traceOperation(ctx *lift.Context, operation string) func() {
    segment := ctx.StartSegment(operation)
    
    // Add metadata
    segment.AddMetadata("service", map[string]interface{}{
        "version":     version,
        "environment": environment,
        "region":      region,
    })
    
    // Return close function
    return func() {
        segment.End()
    }
}
```

## Cost Optimization

### Right-Sizing Lambda

```go
// Monitor and optimize Lambda configuration
func analyzeLambdaPerformance() {
    metrics := getLambdaMetrics()
    
    // Check memory utilization
    if metrics.AvgMemoryUsed < metrics.MemorySize*0.5 {
        recommend("Reduce memory size to", metrics.MemorySize/2)
    }
    
    // Check duration vs timeout
    if metrics.P99Duration < metrics.Timeout*0.3 {
        recommend("Reduce timeout to", metrics.P99Duration*3)
    }
    
    // Check concurrent executions
    if metrics.AvgConcurrent < metrics.ReservedConcurrency*0.7 {
        recommend("Reduce reserved concurrency")
    }
}
```

### Request Optimization

```go
// Batch operations to reduce Lambda invocations
func batchProcessor(ctx *lift.Context) error {
    var batch []Request
    
    // Collect requests
    for _, record := range ctx.Request.Records {
        var req Request
        json.Unmarshal([]byte(record["body"].(string)), &req)
        batch = append(batch, req)
    }
    
    // Process in bulk
    results, err := processBatch(batch)
    if err != nil {
        return err
    }
    
    // Return batch results
    return ctx.JSON(results)
}

// Compress large responses
func compressionMiddleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Check if client accepts compression
            acceptEncoding := ctx.Header("Accept-Encoding")
            if !strings.Contains(acceptEncoding, "gzip") {
                return next.Handle(ctx)
            }
            
            // Execute handler
            err := next.Handle(ctx)
            if err != nil {
                return err
            }
            
            // Compress response if large
            if body, ok := ctx.Response.Body.([]byte); ok && len(body) > 1024 {
                compressed := gzipCompress(body)
                ctx.Response.Body = compressed
                ctx.Header("Content-Encoding", "gzip")
            }
            
            return nil
        })
    }
}
```

## Disaster Recovery

### Backup Strategies

```go
// Automated backups
func setupBackups() {
    // DynamoDB point-in-time recovery
    enablePITR("users-table")
    enablePITR("orders-table")
    
    // S3 versioning and replication
    enableVersioning("data-bucket")
    setupCrossRegionReplication("data-bucket", "us-west-2")
    
    // Lambda function versioning
    enableFunctionVersioning("order-service")
}

// Data export for compliance
func exportData(ctx *lift.Context) error {
    tenantID := ctx.TenantID()
    
    // Export all tenant data
    data := exportTenantData(tenantID)
    
    // Encrypt and store
    encrypted := encryptData(data)
    key := fmt.Sprintf("exports/%s/%s.json", tenantID, time.Now().Format("2006-01-02"))
    
    err := s3Client.PutObject(context.Background(), &s3.PutObjectInput{
        Bucket: aws.String("backup-bucket"),
        Key:    aws.String(key),
        Body:   bytes.NewReader(encrypted),
        ServerSideEncryption: types.ServerSideEncryptionAes256,
    })
    
    return err
}
```

### Multi-Region Deployment

```go
// Active-active multi-region setup
type MultiRegionConfig struct {
    Regions []string
    Primary string
    Routing RoutingStrategy
}

func setupMultiRegion(config MultiRegionConfig) {
    // Deploy to all regions
    for _, region := range config.Regions {
        deployToRegion(region)
    }
    
    // Setup Route53 routing
    setupRoute53(config)
    
    // Configure data replication
    setupDynamoDBGlobalTables()
    setupS3CrossRegionReplication()
}
```

## Production Checklist

### Pre-Deployment

- [ ] All tests passing with >80% coverage
- [ ] Security scan completed (no high/critical issues)
- [ ] Performance benchmarks meet SLAs
- [ ] Documentation updated
- [ ] Runbook created for operations team
- [ ] Rollback plan documented

### Deployment

- [ ] Blue-green or canary deployment configured
- [ ] CloudWatch alarms active
- [ ] Dead letter queues configured
- [ ] Secrets rotated
- [ ] Function versioning enabled
- [ ] Reserved concurrency set appropriately

### Post-Deployment

- [ ] Monitor error rates for 24 hours
- [ ] Check cold start performance
- [ ] Verify all integrations working
- [ ] Review CloudWatch logs for issues
- [ ] Update capacity planning based on metrics
- [ ] Schedule post-mortem if issues occurred

## Summary

Production deployment requires:

- **Performance**: Optimized configuration and caching
- **Reliability**: Error recovery and health checks
- **Security**: Secrets management and security headers
- **Monitoring**: Comprehensive metrics and alerting
- **Cost**: Right-sized resources and optimization
- **Recovery**: Backup and multi-region strategies

Following these practices ensures your Lift applications run reliably and efficiently in production. 