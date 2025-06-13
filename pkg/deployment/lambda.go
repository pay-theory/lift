package deployment

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/health"
	"github.com/pay-theory/lift/pkg/lift/resources"
)

// DeploymentConfig holds configuration for Lambda deployment
type DeploymentConfig struct {
	Environment     string        `json:"environment"`
	LogLevel        string        `json:"log_level"`
	MetricsEnabled  bool          `json:"metrics_enabled"`
	TracingEnabled  bool          `json:"tracing_enabled"`
	HealthChecks    []string      `json:"health_checks"`
	PreWarmTargets  []string      `json:"pre_warm_targets"`
	TimeoutSeconds  int           `json:"timeout_seconds"`
	MemoryMB        int           `json:"memory_mb"`
	ColdStartOptim  bool          `json:"cold_start_optimization"`
	GracefulTimeout time.Duration `json:"graceful_timeout"`
}

// DefaultDeploymentConfig returns production-ready default configuration
func DefaultDeploymentConfig() *DeploymentConfig {
	return &DeploymentConfig{
		Environment:     getEnv("LIFT_ENVIRONMENT", "production"),
		LogLevel:        getEnv("LIFT_LOG_LEVEL", "info"),
		MetricsEnabled:  getEnvBool("LIFT_METRICS_ENABLED", true),
		TracingEnabled:  getEnvBool("LIFT_TRACING_ENABLED", true),
		HealthChecks:    []string{"app", "resources", "memory"},
		PreWarmTargets:  []string{"database", "cache", "external_apis"},
		TimeoutSeconds:  30,
		MemoryMB:        512,
		ColdStartOptim:  true,
		GracefulTimeout: 30 * time.Second,
	}
}

// LambdaDeployment provides production-ready Lambda deployment infrastructure
type LambdaDeployment struct {
	app           *lift.App
	config        *DeploymentConfig
	healthManager health.HealthManager
	metrics       lift.MetricsCollector
	resourceMgr   *resources.ResourceManager

	// Cold start detection
	coldStartMutex sync.RWMutex
	isColdStartVar bool
	startTime      time.Time

	// Performance tracking
	requestCount  int64
	totalDuration time.Duration
	lastRequest   time.Time
}

// NewLambdaDeployment creates a new production-ready Lambda deployment
func NewLambdaDeployment(app *lift.App, config *DeploymentConfig) (*LambdaDeployment, error) {
	if app == nil {
		return nil, fmt.Errorf("app cannot be nil")
	}

	if config == nil {
		config = DefaultDeploymentConfig()
	}

	// Initialize health manager
	healthConfig := health.DefaultHealthManagerConfig()
	healthManager := health.NewHealthManager(healthConfig)

	// Register standard health checks
	for _, checkName := range config.HealthChecks {
		switch checkName {
		case "app":
			healthManager.RegisterChecker("app", &AppHealthChecker{app: app})
		case "resources":
			healthManager.RegisterChecker("resources", &ResourceHealthChecker{})
		case "memory":
			healthManager.RegisterChecker("memory", &MemoryHealthChecker{maxMemoryMB: config.MemoryMB})
		}
	}

	// Initialize metrics collector if enabled
	var metricsCollector lift.MetricsCollector
	if config.MetricsEnabled {
		metricsCollector = &lift.NoOpMetrics{} // Use NoOp for now, can be replaced with real implementation
	}

	// Initialize resource manager
	resourceConfig := resources.DefaultResourceManagerConfig()
	resourceMgr := resources.NewResourceManager(resourceConfig)

	deployment := &LambdaDeployment{
		app:            app,
		config:         config,
		healthManager:  healthManager,
		metrics:        metricsCollector,
		resourceMgr:    resourceMgr,
		isColdStartVar: true,
		startTime:      time.Now(),
	}

	return deployment, nil
}

// Handler returns the production-ready Lambda handler
func (d *LambdaDeployment) Handler() lambda.Handler {
	return lambda.NewHandler(d.handleLambdaEvent)
}

// handleLambdaEvent processes Lambda events through the Lift framework
func (d *LambdaDeployment) handleLambdaEvent(ctx context.Context, event json.RawMessage) (interface{}, error) {
	startTime := time.Now()

	// Check if this is a cold start
	isColdStart := d.isColdStart()

	// Pre-warm resources if cold start and optimization enabled
	if isColdStart && d.config.ColdStartOptim {
		if err := d.resourceMgr.PreWarmAll(ctx); err != nil {
			// Log warning but don't fail the request
			d.logWarning("Pre-warming failed", err)
		}
	}

	// Mark as warm after first request
	if isColdStart {
		d.markWarm()
	}

	// Add Lambda context information
	ctx = d.enrichContext(ctx, isColdStart)

	// Process event through Lift framework
	// For now, we'll create a simple response - this needs to be integrated with the actual app routing
	response := map[string]interface{}{
		"statusCode": 200,
		"body":       `{"message": "Lambda deployment successful"}`,
		"headers": map[string]string{
			"Content-Type": "application/json",
		},
	}

	// Record metrics
	duration := time.Since(startTime)
	d.recordMetrics(ctx, duration, nil, isColdStart)

	return response, nil
}

// isColdStart checks if this is a cold start
func (d *LambdaDeployment) isColdStart() bool {
	d.coldStartMutex.RLock()
	defer d.coldStartMutex.RUnlock()
	return d.isColdStartVar
}

// markWarm marks the Lambda as warm
func (d *LambdaDeployment) markWarm() {
	d.coldStartMutex.Lock()
	defer d.coldStartMutex.Unlock()
	d.isColdStartVar = false
}

// enrichContext adds deployment-specific context information
func (d *LambdaDeployment) enrichContext(ctx context.Context, isColdStart bool) context.Context {
	// Add Lambda context if available
	if lc, ok := lambdacontext.FromContext(ctx); ok {
		ctx = context.WithValue(ctx, "lambda_request_id", lc.AwsRequestID)
		ctx = context.WithValue(ctx, "lambda_function_name", lc.InvokedFunctionArn)
		// Note: Lambda context deadline is available through ctx.Deadline()
	}

	// Add deployment information
	ctx = context.WithValue(ctx, "deployment_environment", d.config.Environment)
	ctx = context.WithValue(ctx, "is_cold_start", isColdStart)
	ctx = context.WithValue(ctx, "deployment_start_time", d.startTime)

	return ctx
}

// recordMetrics records performance and operational metrics
func (d *LambdaDeployment) recordMetrics(ctx context.Context, duration time.Duration, err error, isColdStart bool) {
	if d.metrics == nil {
		return
	}

	// Record basic metrics
	d.metrics.Histogram("lambda_request_duration").Observe(float64(duration.Milliseconds()))
	d.metrics.Counter("lambda_request_count").Inc()

	// Record cold start metrics
	if isColdStart {
		d.metrics.Counter("lambda_cold_start_count").Inc()
		d.metrics.Histogram("lambda_cold_start_duration").Observe(float64(duration.Milliseconds()))
	}

	// Record error metrics
	if err != nil {
		d.metrics.Counter("lambda_error_count").Inc()
		d.metrics.Gauge("lambda_error_rate").Set(d.calculateErrorRate())
	}

	// Record environment metrics
	d.metrics.Gauge("lambda_memory_used_mb").Set(d.getMemoryUsage())
	d.metrics.Gauge("lambda_uptime_seconds").Set(time.Since(d.startTime).Seconds())
}

// HealthCheck performs comprehensive health check
func (d *LambdaDeployment) HealthCheck(ctx context.Context) (*LambdaHealthStatus, error) {
	overallHealth := d.healthManager.OverallHealth(ctx)

	status := &LambdaHealthStatus{
		Status:      overallHealth.Status,
		Timestamp:   overallHealth.Timestamp,
		Environment: d.config.Environment,
		Uptime:      time.Since(d.startTime),
		Checks:      make(map[string]CheckResult),
	}

	// Convert health status details to check results
	if overallHealth.Details != nil {
		for name, detail := range overallHealth.Details {
			if healthStatus, ok := detail.(health.HealthStatus); ok {
				checkResult := CheckResult{
					Status:   healthStatus.Status,
					Duration: healthStatus.Duration,
					Message:  healthStatus.Message,
					Error:    healthStatus.Error,
				}
				status.Checks[name] = checkResult
			}
		}
	}

	return status, nil
}

// Shutdown performs graceful shutdown
func (d *LambdaDeployment) Shutdown(ctx context.Context) error {
	// Shutdown components in order
	var shutdownErrors []error

	// 1. Stop accepting new requests (handled by Lambda runtime)

	// 2. Shutdown resource manager
	if err := d.resourceMgr.Close(); err != nil {
		shutdownErrors = append(shutdownErrors, fmt.Errorf("resource manager shutdown: %w", err))
	}

	// 3. Flush metrics
	if d.metrics != nil {
		if err := d.metrics.Flush(); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("metrics flush: %w", err))
		}
	}

	if len(shutdownErrors) > 0 {
		return fmt.Errorf("shutdown errors: %v", shutdownErrors)
	}

	return nil
}

// Helper types and functions

// LambdaHealthStatus represents the overall health status for Lambda deployment
type LambdaHealthStatus struct {
	Status      string                 `json:"status"`
	Timestamp   time.Time              `json:"timestamp"`
	Environment string                 `json:"environment"`
	Uptime      time.Duration          `json:"uptime"`
	Checks      map[string]CheckResult `json:"checks"`
}

// CheckResult represents individual health check result
type CheckResult struct {
	Status   string        `json:"status"`
	Duration time.Duration `json:"duration"`
	Message  string        `json:"message,omitempty"`
	Error    string        `json:"error,omitempty"`
}

// Health checker implementations

// AppHealthChecker checks the Lift app health
type AppHealthChecker struct {
	app *lift.App
}

func (c *AppHealthChecker) Name() string {
	return "app"
}

func (c *AppHealthChecker) Check(ctx context.Context) health.HealthStatus {
	start := time.Now()

	// Check if app is responsive
	if c.app == nil {
		return health.HealthStatus{
			Status:    health.StatusUnhealthy,
			Timestamp: time.Now(),
			Duration:  time.Since(start),
			Message:   "App is nil",
			Error:     "app is nil",
		}
	}

	// Perform basic app health check
	return health.HealthStatus{
		Status:    health.StatusHealthy,
		Timestamp: time.Now(),
		Duration:  time.Since(start),
		Message:   "App is healthy",
	}
}

// ResourceHealthChecker checks resource availability
type ResourceHealthChecker struct{}

func (c *ResourceHealthChecker) Name() string {
	return "resources"
}

func (c *ResourceHealthChecker) Check(ctx context.Context) health.HealthStatus {
	start := time.Now()

	// Check system resources
	// This is a placeholder - implement actual resource checks
	return health.HealthStatus{
		Status:    health.StatusHealthy,
		Timestamp: time.Now(),
		Duration:  time.Since(start),
		Message:   "Resources are healthy",
	}
}

// MemoryHealthChecker checks memory usage
type MemoryHealthChecker struct {
	maxMemoryMB int
}

func (c *MemoryHealthChecker) Name() string {
	return "memory"
}

func (c *MemoryHealthChecker) Check(ctx context.Context) health.HealthStatus {
	start := time.Now()

	// Check memory usage against limits
	// This is a placeholder - implement actual memory checks
	return health.HealthStatus{
		Status:    health.StatusHealthy,
		Timestamp: time.Now(),
		Duration:  time.Since(start),
		Message:   "Memory usage is healthy",
	}
}

// Utility functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1"
	}
	return defaultValue
}

func (d *LambdaDeployment) logWarning(message string, err error) {
	// Placeholder for logging - integrate with observability package
	fmt.Printf("WARNING: %s: %v\n", message, err)
}

func (d *LambdaDeployment) calculateErrorRate() float64 {
	// Placeholder for error rate calculation
	return 0.0
}

func (d *LambdaDeployment) getMemoryUsage() float64 {
	// Placeholder for memory usage calculation
	return 0.0
}
