package lift

import (
	"context"
	"testing"
)

func TestNew(t *testing.T) {
	app := New()
	if app == nil {
		t.Fatal("New() returned nil")
	}

	if app.router == nil {
		t.Error("App router is nil")
	}

	if app.config == nil {
		t.Error("App config is nil")
	}

	if app.started {
		t.Error("App should not be started initially")
	}
}

func TestAppRoutes(t *testing.T) {
	app := New()

	// Test route registration
	app.GET("/test", func(ctx *Context) error {
		return ctx.JSON(map[string]string{"message": "test"})
	})

	app.POST("/users", func(ctx *Context) error {
		return ctx.JSON(map[string]string{"created": "user"})
	})

	// Test method chaining
	result := app.PUT("/update", func(ctx *Context) error {
		return nil
	}).DELETE("/delete", func(ctx *Context) error {
		return nil
	})

	if result != app {
		t.Error("Method chaining should return the same app instance")
	}
}

func TestAppStart(t *testing.T) {
	app := New()

	// Start should succeed
	err := app.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	if !app.started {
		t.Error("App should be marked as started")
	}

	// Starting again should be safe
	err = app.Start()
	if err != nil {
		t.Fatalf("Second Start() failed: %v", err)
	}
}

func TestAppWithConfig(t *testing.T) {
	config := &Config{
		LogLevel: "DEBUG",
		Timeout:  60,
	}

	app := New().WithConfig(config)

	if app.config.LogLevel != "DEBUG" {
		t.Errorf("Expected LogLevel DEBUG, got %s", app.config.LogLevel)
	}

	if app.config.Timeout != 60 {
		t.Errorf("Expected Timeout 60, got %d", app.config.Timeout)
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.MaxRequestSize <= 0 {
		t.Error("MaxRequestSize should be positive")
	}

	if config.MaxResponseSize <= 0 {
		t.Error("MaxResponseSize should be positive")
	}

	if config.LogLevel == "" {
		t.Error("LogLevel should not be empty")
	}

	if !config.MetricsEnabled {
		t.Error("MetricsEnabled should be true by default")
	}
}

func TestAppHandleRequest(t *testing.T) {
	app := New()

	// Add a simple route
	app.GET("/test", func(ctx *Context) error {
		return ctx.JSON(map[string]string{"status": "ok"})
	})

	err := app.Start()
	if err != nil {
		t.Fatalf("Failed to start app: %v", err)
	}

	// Create a proper API Gateway V1 event
	event := map[string]interface{}{
		"resource":   "/test",
		"httpMethod": "GET",
		"path":       "/test",
		"requestContext": map[string]interface{}{
			"requestId": "test-request-id",
		},
		"headers": map[string]interface{}{
			"Content-Type": "application/json",
		},
	}

	// Test HandleRequest
	resp, err := app.HandleRequest(context.Background(), event)
	if err != nil {
		t.Fatalf("HandleRequest failed: %v", err)
	}

	if resp == nil {
		t.Error("Response should not be nil")
	}
}
