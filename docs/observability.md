# Observability

Lift provides comprehensive observability features to monitor, debug, and optimize your serverless applications. This guide covers logging, metrics, tracing, and monitoring best practices.

## Overview

Lift's observability features include:

- **Structured Logging**: JSON-formatted logs with correlation
- **Metrics Collection**: CloudWatch metrics integration
- **Distributed Tracing**: AWS X-Ray support
- **Health Monitoring**: Built-in health check endpoints
- **Performance Profiling**: Request timing and analysis
- **Error Tracking**: Automatic error capture and reporting

## Logging

### Structured Logging

Lift uses structured logging for better searchability and analysis:

```go
func handler(ctx *lift.Context) error {
    // Basic logging
    ctx.Logger.Info("Processing request")
    
    // With structured fields
    ctx.Logger.Info("User action", map[string]interface{}{
        "action":     "create_order",
        "user_id":    ctx.UserID(),
        "tenant_id":  ctx.TenantID(),
        "order_id":   orderID,
        "amount":     99.99,
        "currency":   "USD",
    })
    
    // Different log levels
    ctx.Logger.Debug("Debug information", fields)
    ctx.Logger.Info("Informational message", fields)
    ctx.Logger.Warn("Warning message", fields)
    ctx.Logger.Error("Error occurred", fields)
    
    return ctx.JSON(response)
}
```

### Log Correlation

All logs are automatically correlated with request IDs:

```go
func processOrder(ctx *lift.Context) error {
    // Request ID is automatically included in all logs
    ctx.Logger.Info("Starting order processing")
    
    // Pass request ID to external services
    result, err := paymentService.Process(ctx.RequestID(), payment)
    if err != nil {
        ctx.Logger.Error("Payment failed", map[string]interface{}{
            "error":      err.Error(),
            "payment_id": payment.ID,
        })
        return err
    }
    
    ctx.Logger.Info("Order completed", map[string]interface{}{
        "order_id": result.OrderID,
        "duration": time.Since(start),
    })
    
    return ctx.JSON(result)
}
```

### Custom Logger Configuration

```go
// Configure logger
logger := zap.NewLogger(zap.Config{
    Level:       zap.InfoLevel,
    Environment: "production",
    ServiceName: "order-service",
    
    // Custom fields added to all logs
    DefaultFields: map[string]interface{}{
        "service":     "order-service",
        "version":     version,
        "environment": environment,
    },
    
    // Sampling for high-volume logs
    Sampling: &zap.SamplingConfig{
        Initial:    100,  // Log first 100 of each level
        Thereafter: 10,   // Then log every 10th
    },
})

app := lift.NewWithConfig(lift.Config{
    Logger: logger,
})
```

### Log Aggregation

```go
// CloudWatch Logs integration
func setupCloudWatchLogs() {
    config := cloudwatch.Config{
        LogGroup:        "/aws/lambda/my-service",
        LogStream:       os.Getenv("AWS_LAMBDA_LOG_STREAM_NAME"),
        RetentionDays:   30,
        SubscriptionARN: "arn:aws:logs:us-east-1:123456789012:destination:central-logging",
    }
    
    logger := cloudwatch.NewLogger(config)
    app.SetLogger(logger)
}
```

## Metrics

### CloudWatch Metrics

Lift integrates seamlessly with CloudWatch Metrics:

```go
func handler(ctx *lift.Context) error {
    // Count metrics
    ctx.Metrics.Count("api.requests", 1, map[string]string{
        "endpoint": ctx.Request.Path,
        "method":   ctx.Request.Method,
        "tenant":   ctx.TenantID(),
    })
    
    // Gauge metrics
    activeConnections := getActiveConnections()
    ctx.Metrics.Gauge("websocket.connections", float64(activeConnections))
    
    // Timing metrics
    start := time.Now()
    result := expensiveOperation()
    ctx.Metrics.Timing("operation.duration", time.Since(start), map[string]string{
        "operation": "data_processing",
    })
    
    // Custom metrics
    ctx.Metrics.Record("business.metric", map[string]interface{}{
        "orders_processed": 10,
        "revenue":         999.99,
        "customer_type":   "premium",
    })
    
    return ctx.JSON(result)
}
```

### Metric Namespaces

```go
// Configure metric namespaces
metricsConfig := cloudwatch.MetricsConfig{
    Namespace: "MyApp/Production",
    
    // Default dimensions for all metrics
    DefaultDimensions: map[string]string{
        "Environment": "production",
        "Service":     "order-service",
    },
    
    // Metric resolution (1 or 60 seconds)
    Resolution: 60,
    
    // Batch settings
    BatchSize:     20,
    BatchInterval: 10 * time.Second,
}

app.Use(middleware.Metrics(metricsConfig))
```

### Custom Metrics

```go
// Business metrics
func recordBusinessMetrics(ctx *lift.Context, order Order) {
    // Revenue metrics
    ctx.Metrics.Record("business.revenue", map[string]interface{}{
        "amount":       order.Amount,
        "currency":     order.Currency,
        "product_type": order.ProductType,
        "customer_id":  order.CustomerID,
    })
    
    // Conversion metrics
    ctx.Metrics.Count("funnel.checkout.completed", 1, map[string]string{
        "source":      order.Source,
        "campaign":    order.Campaign,
        "customer_type": order.CustomerType,
    })
    
    // Performance metrics
    if order.ProcessingTime > 5*time.Second {
        ctx.Metrics.Count("sla.violations", 1, map[string]string{
            "type":     "slow_processing",
            "severity": "warning",
        })
    }
}
```

### Metric Alarms

```go
// Create CloudWatch alarms
func setupMetricAlarms() {
    alarms := []cloudwatch.AlarmConfig{
        {
            Name:        "HighErrorRate",
            MetricName:  "errors",
            Namespace:   "MyApp/Production",
            Statistic:   "Sum",
            Period:      300, // 5 minutes
            Threshold:   10,
            ComparisonOperator: "GreaterThanThreshold",
            Actions:     []string{snsTopicARN},
        },
        {
            Name:        "HighLatency",
            MetricName:  "duration",
            Namespace:   "MyApp/Production",
            Statistic:   "Average",
            Period:      60,
            Threshold:   1000, // 1 second
            ComparisonOperator: "GreaterThanThreshold",
        },
    }
    
    cloudwatch.CreateAlarms(alarms)
}
```

## Distributed Tracing

### AWS X-Ray Integration

Lift provides built-in X-Ray tracing:

```go
// Enable X-Ray tracing
app.Use(middleware.XRay(middleware.XRayConfig{
    ServiceName: "order-service",
    
    // Sampling rules
    SamplingRate: 0.1, // Sample 10% of requests
    
    // Custom segments
    SegmentNamer: func(ctx *lift.Context) string {
        return fmt.Sprintf("%s %s", ctx.Request.Method, ctx.Request.Path)
    },
    
    // Capture errors
    CaptureErrors: true,
    
    // Capture AWS SDK calls
    CaptureAWS: true,
}))
```

### Custom Trace Segments

```go
func handler(ctx *lift.Context) error {
    // Start a subsegment
    segment := ctx.StartSegment("process_order")
    defer segment.End()
    
    // Add metadata
    segment.AddMetadata("order", map[string]interface{}{
        "id":     orderID,
        "amount": amount,
        "items":  len(items),
    })
    
    // Trace external calls
    dbSegment := ctx.StartSegment("database_query")
    user, err := getUser(userID)
    dbSegment.End()
    
    if err != nil {
        segment.AddError(err)
        return err
    }
    
    // Add annotations for filtering
    segment.AddAnnotation("customer_type", user.Type)
    segment.AddAnnotation("order_value", "high")
    
    return ctx.JSON(response)
}
```

### Trace Context Propagation

```go
// Propagate trace context to downstream services
func callDownstream(ctx *lift.Context, request Request) (*Response, error) {
    // Get trace header
    traceHeader := ctx.TraceHeader()
    
    // Add to outgoing request
    httpReq, _ := http.NewRequest("POST", url, body)
    httpReq.Header.Set("X-Amzn-Trace-Id", traceHeader)
    
    // Make request with tracing
    segment := ctx.StartSegment("downstream_call")
    defer segment.End()
    
    resp, err := httpClient.Do(httpReq)
    if err != nil {
        segment.AddError(err)
        return nil, err
    }
    
    segment.AddMetadata("response", map[string]interface{}{
        "status_code": resp.StatusCode,
        "duration":    time.Since(start),
    })
    
    return parseResponse(resp)
}
```

## Health Monitoring

### Health Check Endpoints

```go
// Basic health check
app.GET("/health", func(ctx *lift.Context) error {
    return ctx.JSON(map[string]interface{}{
        "status": "healthy",
        "timestamp": time.Now(),
    })
})

// Detailed health check
app.GET("/health/detailed", func(ctx *lift.Context) error {
    health := performHealthChecks()
    
    status := "healthy"
    if !health.IsHealthy() {
        status = "unhealthy"
        ctx.Response.StatusCode = 503
    }
    
    return ctx.JSON(map[string]interface{}{
        "status": status,
        "checks": health.Checks,
        "version": version,
        "uptime": uptime,
    })
})
```

### Component Health Checks

```go
type HealthChecker struct {
    checks []HealthCheck
}

type HealthCheck struct {
    Name   string
    Check  func() error
}

func (h *HealthChecker) Run() HealthReport {
    report := HealthReport{
        Timestamp: time.Now(),
        Checks:    make([]CheckResult, 0),
    }
    
    for _, check := range h.checks {
        start := time.Now()
        err := check.Check()
        
        result := CheckResult{
            Name:     check.Name,
            Duration: time.Since(start),
            Status:   "healthy",
        }
        
        if err != nil {
            result.Status = "unhealthy"
            result.Error = err.Error()
        }
        
        report.Checks = append(report.Checks, result)
    }
    
    return report
}

// Configure health checks
healthChecker := &HealthChecker{
    checks: []HealthCheck{
        {
            Name: "database",
            Check: func() error {
                return db.Ping()
            },
        },
        {
            Name: "redis",
            Check: func() error {
                return redis.Ping()
            },
        },
        {
            Name: "downstream_service",
            Check: func() error {
                resp, err := http.Get(downstreamURL + "/health")
                if err != nil {
                    return err
                }
                if resp.StatusCode != 200 {
                    return fmt.Errorf("unhealthy: %d", resp.StatusCode)
                }
                return nil
            },
        },
    },
}
```

### Liveness and Readiness

```go
// Liveness probe - is the service alive?
app.GET("/health/live", func(ctx *lift.Context) error {
    // Basic check - can we respond?
    return ctx.JSON(map[string]string{
        "status": "alive",
    })
})

// Readiness probe - is the service ready to handle requests?
app.GET("/health/ready", func(ctx *lift.Context) error {
    // Check critical dependencies
    if !isDatabaseReady() || !isCacheReady() {
        return ctx.Status(503).JSON(map[string]string{
            "status": "not_ready",
        })
    }
    
    return ctx.JSON(map[string]string{
        "status": "ready",
    })
})
```

## Performance Monitoring

### Request Timing

```go
// Automatic request timing middleware
app.Use(middleware.Timing(middleware.TimingConfig{
    // Log slow requests
    SlowRequestThreshold: 1 * time.Second,
    
    // Detailed timing breakdown
    DetailedMetrics: true,
    
    // Custom timing collector
    Collector: func(ctx *lift.Context, timing TimingInfo) {
        // Log timing
        ctx.Logger.Info("Request completed", map[string]interface{}{
            "duration_ms":    timing.Total.Milliseconds(),
            "middleware_ms":  timing.Middleware.Milliseconds(),
            "handler_ms":     timing.Handler.Milliseconds(),
            "serialization_ms": timing.Serialization.Milliseconds(),
        })
        
        // Record metrics
        ctx.Metrics.Timing("request.duration", timing.Total)
        
        // Alert on slow requests
        if timing.Total > 5*time.Second {
            alertSlowRequest(ctx, timing)
        }
    },
}))
```

### Memory Profiling

```go
// Memory usage monitoring
func MemoryMonitor() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            var m runtime.MemStats
            runtime.ReadMemStats(&m)
            
            before := m.Alloc
            
            err := next.Handle(ctx)
            
            runtime.ReadMemStats(&m)
            after := m.Alloc
            
            // Log memory usage
            ctx.Logger.Debug("Memory usage", map[string]interface{}{
                "before_mb":     before / 1024 / 1024,
                "after_mb":      after / 1024 / 1024,
                "allocated_mb":  (after - before) / 1024 / 1024,
                "total_alloc_mb": m.TotalAlloc / 1024 / 1024,
                "sys_mb":        m.Sys / 1024 / 1024,
                "num_gc":        m.NumGC,
            })
            
            // Alert on high memory usage
            if m.Alloc > 100*1024*1024 { // 100MB
                ctx.Logger.Warn("High memory usage detected", map[string]interface{}{
                    "alloc_mb": m.Alloc / 1024 / 1024,
                })
            }
            
            return err
        })
    }
}
```

### CPU Profiling

```go
// CPU profiling for development
func EnableProfiling(app *lift.App) {
    if os.Getenv("ENABLE_PROFILING") == "true" {
        app.GET("/debug/pprof/", pprof.Index)
        app.GET("/debug/pprof/profile", pprof.Profile)
        app.GET("/debug/pprof/heap", pprof.Heap)
        app.GET("/debug/pprof/goroutine", pprof.Goroutine)
    }
}
```

## Dashboards and Visualization

### CloudWatch Dashboard

```go
// Create CloudWatch dashboard
func createDashboard() {
    dashboard := cloudwatch.Dashboard{
        Name: "order-service-production",
        Widgets: []cloudwatch.Widget{
            // Request rate
            {
                Type: "metric",
                Properties: map[string]interface{}{
                    "metrics": [][]string{
                        {"MyApp/Production", "api.requests", "Service", "order-service"},
                    },
                    "period": 300,
                    "stat": "Sum",
                    "region": "us-east-1",
                    "title": "Request Rate",
                },
            },
            // Error rate
            {
                Type: "metric",
                Properties: map[string]interface{}{
                    "metrics": [][]interface{}{
                        {"MyApp/Production", "errors", "Service", "order-service"},
                        {".", "api.requests", ".", ".", {
                            "stat": "Sum",
                            "id": "total",
                            "visible": false,
                        }},
                        {"expression": "100 * errors / total", 
                         "label": "Error Rate %",
                         "id": "error_rate"},
                    },
                    "period": 300,
                    "stat": "Sum",
                    "region": "us-east-1",
                    "title": "Error Rate",
                },
            },
            // Latency percentiles
            {
                Type: "metric",
                Properties: map[string]interface{}{
                    "metrics": [][]interface{}{
                        {"MyApp/Production", "duration", "Service", "order-service", 
                         {"stat": "p50", "label": "p50"}},
                        {"...", {"stat": "p90", "label": "p90"}},
                        {"...", {"stat": "p99", "label": "p99"}},
                    },
                    "period": 60,
                    "region": "us-east-1",
                    "title": "Latency Percentiles",
                },
            },
        },
    }
    
    cloudwatch.CreateDashboard(dashboard)
}
```

### Custom Metrics Dashboard

```go
// Business metrics dashboard
func createBusinessDashboard() {
    widgets := []cloudwatch.Widget{
        // Revenue metrics
        {
            Type: "metric",
            Properties: map[string]interface{}{
                "metrics": [][]string{
                    {"MyApp/Business", "revenue", "ProductType", "subscription"},
                    {".", ".", ".", "one-time"},
                },
                "period": 3600,
                "stat": "Sum",
                "title": "Revenue by Product Type",
            },
        },
        // Conversion funnel
        {
            Type: "metric",
            Properties: map[string]interface{}{
                "metrics": [][]string{
                    {"MyApp/Business", "funnel.visit"},
                    {".", "funnel.signup"},
                    {".", "funnel.checkout"},
                    {".", "funnel.complete"},
                },
                "period": 3600,
                "stat": "Sum",
                "title": "Conversion Funnel",
            },
        },
    }
    
    cloudwatch.CreateDashboard(cloudwatch.Dashboard{
        Name:    "business-metrics",
        Widgets: widgets,
    })
}
```

## Alerting

### CloudWatch Alarms

```go
// Configure comprehensive alarms
func setupAlarms() {
    alarms := []cloudwatch.Alarm{
        // High error rate
        {
            Name:        "high-error-rate",
            Description: "Error rate above 1%",
            MetricName:  "error_rate",
            Threshold:   1.0,
            Actions:     []string{criticalSNSTopic},
        },
        // High latency
        {
            Name:        "high-latency-p99",
            Description: "P99 latency above 1 second",
            MetricName:  "duration",
            Statistic:   "p99",
            Threshold:   1000,
            Actions:     []string{warningSNSTopic},
        },
        // Low success rate
        {
            Name:        "low-success-rate",
            Description: "Success rate below 99%",
            Expression:  "100 * successful_requests / total_requests",
            Threshold:   99,
            ComparisonOperator: "LessThanThreshold",
            Actions:     []string{criticalSNSTopic},
        },
    }
    
    for _, alarm := range alarms {
        cloudwatch.PutMetricAlarm(alarm)
    }
}
```

### Custom Alerting

```go
// Alert manager
type AlertManager struct {
    rules []AlertRule
}

type AlertRule struct {
    Name      string
    Condition func(ctx *lift.Context) bool
    Action    func(ctx *lift.Context)
}

func (am *AlertManager) Check(ctx *lift.Context) {
    for _, rule := range am.rules {
        if rule.Condition(ctx) {
            ctx.Logger.Warn("Alert triggered", map[string]interface{}{
                "alert": rule.Name,
            })
            rule.Action(ctx)
        }
    }
}

// Configure alerts
alertManager := &AlertManager{
    rules: []AlertRule{
        {
            Name: "high_memory_usage",
            Condition: func(ctx *lift.Context) bool {
                var m runtime.MemStats
                runtime.ReadMemStats(&m)
                return m.Alloc > 100*1024*1024 // 100MB
            },
            Action: func(ctx *lift.Context) {
                sendAlert("High memory usage detected", ctx)
            },
        },
        {
            Name: "slow_database_query",
            Condition: func(ctx *lift.Context) bool {
                duration := ctx.Get("db_query_duration").(time.Duration)
                return duration > 5*time.Second
            },
            Action: func(ctx *lift.Context) {
                sendAlert("Slow database query", ctx)
            },
        },
    },
}
```

## Best Practices

### 1. Structured Logging

```go
// GOOD: Structured logging with context
ctx.Logger.Info("Order processed", map[string]interface{}{
    "order_id":     orderID,
    "user_id":      userID,
    "amount":       amount,
    "duration_ms":  duration.Milliseconds(),
    "items_count":  len(items),
})

// AVOID: Unstructured logs
log.Printf("Processed order %s for user %s", orderID, userID)
```

### 2. Meaningful Metrics

```go
// GOOD: Business-relevant metrics
ctx.Metrics.Count("checkout.completed", 1, map[string]string{
    "payment_method": paymentMethod,
    "customer_type":  customerType,
})

// AVOID: Technical-only metrics
ctx.Metrics.Count("function.invocations", 1)
```

### 3. Trace Critical Paths

```go
// GOOD: Trace important operations
segment := ctx.StartSegment("process_payment")
defer segment.End()

result, err := processPayment(payment)
if err != nil {
    segment.AddError(err)
}
segment.AddMetadata("result", result)
```

### 4. Alert on Business Impact

```go
// GOOD: Alert on business metrics
if conversionRate < 0.01 { // Below 1%
    alert("Low conversion rate", map[string]interface{}{
        "rate": conversionRate,
        "expected": 0.03,
    })
}

// AVOID: Alert on every error
if err != nil {
    alert("Error occurred") // Too noisy
}
```

### 5. Use Sampling Wisely

```go
// GOOD: Sample high-volume, low-value logs
if rand.Float64() < 0.01 { // 1% sampling
    ctx.Logger.Debug("Cache hit", map[string]interface{}{
        "key": cacheKey,
    })
}

// Always log important events
ctx.Logger.Info("Payment processed", map[string]interface{}{
    "payment_id": paymentID,
    "amount": amount,
})
```

## Summary

Lift's observability features provide:

- **Complete Visibility**: Logs, metrics, and traces in one place
- **AWS Integration**: Native CloudWatch and X-Ray support
- **Performance Insights**: Detailed timing and profiling
- **Proactive Monitoring**: Health checks and alerting
- **Business Intelligence**: Custom metrics and dashboards

Effective observability helps you maintain reliable, performant serverless applications. 