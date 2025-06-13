package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/pay-theory/lift/pkg/lift/health"
	"github.com/pay-theory/lift/pkg/lift/resources"
)

// MockResource implements the resources.Resource interface for demonstration
type MockResource struct {
	id       string
	lastUsed time.Time
	valid    bool
}

func (r *MockResource) Initialize(ctx context.Context) error {
	r.lastUsed = time.Now()
	r.valid = true
	return nil
}

func (r *MockResource) HealthCheck(ctx context.Context) error {
	if !r.valid {
		return fmt.Errorf("resource %s is invalid", r.id)
	}
	return nil
}

func (r *MockResource) Cleanup() error {
	r.valid = false
	return nil
}

func (r *MockResource) IsValid() bool {
	return r.valid && time.Since(r.lastUsed) < 5*time.Minute
}

func (r *MockResource) LastUsed() time.Time {
	return r.lastUsed
}

func (r *MockResource) MarkUsed() {
	r.lastUsed = time.Now()
}

// MockResourceFactory creates mock resources
type MockResourceFactory struct{}

func (f *MockResourceFactory) Create(ctx context.Context) (resources.Resource, error) {
	resource := &MockResource{
		id:       fmt.Sprintf("resource-%d", time.Now().UnixNano()),
		lastUsed: time.Now(),
		valid:    true,
	}
	return resource, nil
}

func (f *MockResourceFactory) Validate(resource resources.Resource) bool {
	return resource.IsValid()
}

func main() {
	fmt.Println("🏥 Lift Health Monitoring Example")
	fmt.Println("==================================")

	// Create health manager
	healthConfig := health.DefaultHealthManagerConfig()
	healthConfig.ParallelChecks = true
	healthConfig.CacheEnabled = true
	healthConfig.CacheDuration = 30 * time.Second
	healthManager := health.NewHealthManager(healthConfig)

	// 1. Memory Health Checker
	fmt.Println("\n📊 Setting up Memory Health Checker...")
	memoryChecker := health.NewMemoryHealthChecker("memory")
	healthManager.RegisterChecker("memory", memoryChecker)

	// 2. Connection Pool Health Checker
	fmt.Println("🔗 Setting up Connection Pool Health Checker...")
	poolConfig := resources.DefaultPoolConfig()
	poolConfig.MaxActive = 10
	poolConfig.MaxIdle = 5
	poolConfig.MinIdle = 2

	factory := &MockResourceFactory{}
	pool := resources.NewConnectionPool(poolConfig, factory)

	poolChecker := health.NewPoolHealthChecker("connection-pool", pool)
	healthManager.RegisterChecker("connection-pool", poolChecker)

	// 3. HTTP Service Health Checker
	fmt.Println("🌐 Setting up HTTP Service Health Checker...")
	httpChecker := health.NewHTTPHealthChecker("google", "https://www.google.com")
	healthManager.RegisterChecker("external-service", httpChecker)

	// 4. Custom Business Logic Health Checker
	fmt.Println("⚙️  Setting up Custom Business Logic Health Checker...")
	businessChecker := health.NewCustomHealthChecker("business-logic", func(ctx context.Context) health.HealthStatus {
		// Simulate business logic check
		start := time.Now()

		// Check if it's business hours (simplified)
		hour := time.Now().Hour()
		isBusinessHours := hour >= 9 && hour <= 17

		status := health.StatusHealthy
		message := "Business logic is healthy"

		if !isBusinessHours {
			status = health.StatusDegraded
			message = "Outside business hours - reduced capacity"
		}

		return health.HealthStatus{
			Status:    status,
			Timestamp: time.Now(),
			Duration:  time.Since(start),
			Message:   message,
			Details: map[string]interface{}{
				"business_hours": isBusinessHours,
				"current_hour":   hour,
			},
		}
	})
	healthManager.RegisterChecker("business-logic", businessChecker)

	// 5. Database Health Checker (mock)
	fmt.Println("🗄️  Setting up Database Health Checker...")
	// Note: This would normally connect to a real database
	// For demo purposes, we'll create a mock database connection
	mockDB := &sql.DB{} // This is just for demonstration
	dbChecker := health.NewDatabaseHealthChecker("database", mockDB)
	// We'll skip registering this one since it would fail without a real DB
	_ = dbChecker

	// Create health endpoints
	fmt.Println("\n🌐 Setting up Health Endpoints...")
	endpointsConfig := health.DefaultHealthEndpointsConfig()
	endpointsConfig.EnableDetailedErrors = true // Enable for demo
	endpoints := health.NewHealthEndpoints(healthManager, endpointsConfig)

	// Create health middleware
	middlewareConfig := health.DefaultHealthMiddlewareConfig()
	healthMiddleware := health.NewHealthMiddleware(healthManager, middlewareConfig)

	// Set up HTTP server
	mux := http.NewServeMux()

	// Register health endpoints
	endpoints.RegisterRoutes(mux)

	// Add a demo endpoint with health middleware
	demoHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{
			"message": "Hello from Lift Health Monitoring Demo!",
			"timestamp": "%s",
			"path": "%s"
		}`, time.Now().Format(time.RFC3339), r.URL.Path)
	})

	// Wrap demo handler with health middleware
	mux.Handle("/demo", healthMiddleware.Handler(demoHandler))

	// Add a simple index page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>Lift Health Monitoring Demo</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .endpoint { margin: 10px 0; padding: 10px; background: #f5f5f5; border-radius: 5px; }
        .endpoint a { text-decoration: none; color: #0066cc; font-weight: bold; }
        .endpoint .desc { color: #666; margin-top: 5px; }
    </style>
</head>
<body>
    <h1>🏥 Lift Health Monitoring Demo</h1>
    <p>This demo showcases the comprehensive health monitoring system built into the Lift framework.</p>
    
    <h2>Health Endpoints</h2>
    <div class="endpoint">
        <a href="/health">/health</a>
        <div class="desc">Overall health status (JSON)</div>
    </div>
    <div class="endpoint">
        <a href="/health/ready">/health/ready</a>
        <div class="desc">Kubernetes readiness probe</div>
    </div>
    <div class="endpoint">
        <a href="/health/live">/health/live</a>
        <div class="desc">Kubernetes liveness probe</div>
    </div>
    <div class="endpoint">
        <a href="/health/components">/health/components</a>
        <div class="desc">Individual component health status</div>
    </div>
    <div class="endpoint">
        <a href="/health/components?component=memory">/health/components?component=memory</a>
        <div class="desc">Specific component health (memory)</div>
    </div>
    
    <h2>Demo Endpoints</h2>
    <div class="endpoint">
        <a href="/demo">/demo</a>
        <div class="desc">Demo endpoint with health middleware (check X-Health-Status header)</div>
    </div>
    
    <h2>Health Checkers Configured</h2>
    <ul>
        <li><strong>Memory:</strong> System memory usage monitoring</li>
        <li><strong>Connection Pool:</strong> Resource pool health and statistics</li>
        <li><strong>External Service:</strong> HTTP service dependency check</li>
        <li><strong>Business Logic:</strong> Custom business rules validation</li>
    </ul>
    
    <h2>Features Demonstrated</h2>
    <ul>
        <li>✅ Multiple health checker types</li>
        <li>✅ Parallel health checking</li>
        <li>✅ Health check caching</li>
        <li>✅ Kubernetes-compatible probes</li>
        <li>✅ JSON and plain text responses</li>
        <li>✅ Health status middleware</li>
        <li>✅ CORS support</li>
        <li>✅ Detailed error reporting</li>
    </ul>
</body>
</html>`)
	})

	// Demonstrate health checks
	fmt.Println("\n🔍 Running Initial Health Checks...")
	ctx := context.Background()

	// Check individual components
	checkers := healthManager.ListCheckers()
	fmt.Printf("Registered health checkers: %v\n", checkers)

	for _, name := range checkers {
		status, err := healthManager.CheckComponent(ctx, name)
		if err != nil {
			fmt.Printf("❌ %s: ERROR - %v\n", name, err)
			continue
		}

		emoji := "✅"
		if status.Status == health.StatusDegraded {
			emoji = "⚠️"
		} else if status.Status == health.StatusUnhealthy {
			emoji = "❌"
		}

		fmt.Printf("%s %s: %s (%v) - %s\n",
			emoji, name, status.Status, status.Duration, status.Message)
	}

	// Check overall health
	fmt.Println("\n📋 Overall Health Status:")
	overall := healthManager.OverallHealth(ctx)
	overallEmoji := "✅"
	if overall.Status == health.StatusDegraded {
		overallEmoji = "⚠️"
	} else if overall.Status == health.StatusUnhealthy {
		overallEmoji = "❌"
	}

	fmt.Printf("%s Overall: %s (%v) - %s\n",
		overallEmoji, overall.Status, overall.Duration, overall.Message)

	// Start HTTP server
	port := ":8080"
	fmt.Printf("\n🚀 Starting HTTP server on http://localhost%s\n", port)
	fmt.Println("\nTry these endpoints:")
	fmt.Printf("  • http://localhost%s/           - Demo homepage\n", port)
	fmt.Printf("  • http://localhost%s/health     - Overall health\n", port)
	fmt.Printf("  • http://localhost%s/health/ready - Readiness probe\n", port)
	fmt.Printf("  • http://localhost%s/health/live  - Liveness probe\n", port)
	fmt.Printf("  • http://localhost%s/health/components - Component health\n", port)
	fmt.Printf("  • http://localhost%s/demo       - Demo with health header\n", port)

	fmt.Println("\n💡 Tips:")
	fmt.Println("  • Check response headers for X-Health-Status")
	fmt.Println("  • Try Accept: text/plain for plain text responses")
	fmt.Println("  • Health checks are cached for 30 seconds")
	fmt.Println("  • Business logic checker shows degraded status outside 9-5")

	log.Fatal(http.ListenAndServe(port, mux))
}
