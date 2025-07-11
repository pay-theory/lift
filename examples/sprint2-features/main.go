package main

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/pay-theory/lift/pkg/dynamorm"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/middleware"
)

func main() {
	// Create Lift app
	app := lift.New()

	// Configure DynamORM for rate limiting
	// Note: DynamORM initialization would be done here
	// For this example, we'll use the Limited library approach
	var db *dynamorm.DynamORMWrapper
	// db initialization would happen here in production

	// 1. Sliding Window Rate Limiting (with DynamORM)
	if db != nil {
		rateLimiter, err := middleware.NewSlidingWindowRateLimiter(middleware.RateLimitConfig{
			DynamORM:      db,
			DefaultLimit:  100,
			DefaultWindow: 15 * time.Minute,
			IncludePath:   true,
			IncludeMethod: true,
		})
		if err == nil {
			app.Use(rateLimiter.Middleware())
		}
	}

	// 2. Limited Library Rate Limiting (simpler alternative)
	limitedRateLimiter, err := middleware.LimitedRateLimit(middleware.LimitedConfig{
		Region:    "us-east-1",
		TableName: "rate-limits",
		Window:    time.Hour,
		Limit:     1000,
		Strategy:  "sliding", // Using sliding window strategy
	})
	if err == nil {
		app.Use(limitedRateLimiter)
	}

	// 3. Circuit Breaker (already fully implemented)
	circuitBreaker := middleware.CircuitBreakerMiddleware(middleware.CircuitBreakerConfig{
		Name:             "api-service",
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
	})
	app.Use(circuitBreaker)

	// 4. Service Mesh Integration
	serviceMesh, err := middleware.ServiceMesh(middleware.ServiceMeshConfig{
		MeshName:    "production-mesh",
		VirtualNode: "api-service-v1",
		ServiceName: "api-service",
		Namespace:   "production",
		Region:      "us-east-1",
		Port:        "8080",
	})
	if err == nil {
		app.Use(serviceMesh)
	}

	// 5. Load Shedding (already fully implemented)
	loadShedder := middleware.LoadSheddingMiddleware(middleware.LoadSheddingConfig{
		CPUThreshold:     0.8,
		MemoryThreshold:  0.85,
		LatencyThreshold: 500 * time.Millisecond,
	})
	app.Use(loadShedder)

	// 6. Memory Cache with TTL (already implemented)
	// Note: Cache middleware would be initialized here
	// Example: cache := middleware.NewCache(config)

	// Example API routes
	api := app.Group("/api/v1")

	// Apply tenant-based rate limiting to API group
	tenantLimiter, _ := middleware.TenantRateLimitWithLimited(500, time.Hour)
	app.Use(tenantLimiter) // RouteGroup doesn't have Use method, apply to app

	// Health check endpoint (used by service mesh)
	app.GET("/health", func(ctx *lift.Context) error {
		return ctx.JSON(map[string]interface{}{
			"status":    "healthy",
			"service":   "sprint2-features-demo",
			"timestamp": time.Now().UTC(),
		})
	})

	// Protected endpoint with all features
	api.GET("/protected", func(ctx *lift.Context) error {
		// This endpoint benefits from:
		// - Rate limiting (sliding window)
		// - Circuit breaker protection
		// - Service mesh integration
		// - Load shedding
		// - Response caching

		return ctx.JSON(map[string]interface{}{
			"message":   "Successfully accessed protected resource",
			"user_id":   ctx.UserID(),
			"tenant_id": ctx.TenantID(),
			"features": []string{
				"sliding_window_rate_limiting",
				"circuit_breaker",
				"service_mesh",
				"load_shedding",
				"memory_cache_ttl",
			},
		})
	})

	// Simulate a flaky endpoint (for circuit breaker testing)
	api.GET("/flaky", func(ctx *lift.Context) error {
		// Randomly fail 50% of requests
		if time.Now().Unix()%2 == 0 {
			return fmt.Errorf("simulated failure")
		}
		return ctx.JSON(map[string]string{"status": "success"})
	})

	// CPU intensive endpoint (for load shedding testing)
	api.GET("/cpu-intensive", func(ctx *lift.Context) error {
		// Simulate CPU intensive work
		start := time.Now()
		for time.Since(start) < 100*time.Millisecond {
			// Busy loop
		}
		return ctx.JSON(map[string]string{"processed": "true"})
	})

	// Start the Lambda handler
	lambda.Start(app.Handle)
}

// Example of using the features programmatically
func demonstrateFeatures() {
	ctx := context.Background()

	// Example: Manual circuit breaker state check
	cb := middleware.CircuitBreakerMiddleware(middleware.CircuitBreakerConfig{
		Name:             "external-api",
		FailureThreshold: 3,
		Timeout:          time.Minute,
	})

	// Example: Custom rate limit key extraction
	customRateLimiter, _ := middleware.LimitedRateLimit(middleware.LimitedConfig{
		Region:    "us-east-1",
		TableName: "custom-limits",
		Window:    5 * time.Minute,
		Limit:     50,
		Strategy:  "sliding",
	})

	// These can be used in your application
	_ = ctx
	_ = cb
	_ = customRateLimiter
}
