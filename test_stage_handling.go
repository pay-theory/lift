package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pay-theory/lift/pkg/lift"
)

func main() {
	fmt.Println("=== Testing Stage Prefix Handling ===\n")

	// Create app
	app := lift.New()

	// Register handlers for different paths
	app.GET("/health", func(ctx *lift.Context) error {
		return ctx.JSON(map[string]string{
			"status": "healthy",
			"path": ctx.Request.Path,
		})
	})

	app.POST("/api/users", func(ctx *lift.Context) error {
		return ctx.JSON(map[string]string{
			"message": "User endpoint",
			"path": ctx.Request.Path,
		})
	})

	// Test 1: API Gateway v2 with stage prefix
	fmt.Println("--- Test 1: API Gateway v2 with Stage Prefix ---")
	testV2WithStage(app, "prod", "/prod/health")

	// Test 2: API Gateway v1 with stage prefix
	fmt.Println("\n--- Test 2: API Gateway v1 with Stage Prefix ---")
	testV1WithStage(app, "dev", "/dev/api/users")

	// Test 3: $default stage (should not strip)
	fmt.Println("\n--- Test 3: $default Stage ---")
	testV2WithStage(app, "$default", "/health")

	// Test 4: Path that looks like stage but isn't
	fmt.Println("\n--- Test 4: Path with Stage-like Prefix ---")
	testV2FalseStage(app, "prod", "/production/test")
}

func testV2WithStage(app *lift.App, stage, fullPath string) {
	event := map[string]interface{}{
		"version": "2.0",
		"routeKey": fmt.Sprintf("GET %s", fullPath),
		"rawPath": fullPath,
		"requestContext": map[string]interface{}{
			"http": map[string]interface{}{
				"method": "GET",
				"path":   fullPath,
			},
			"stage": stage,
		},
	}

	ctx := context.Background()
	response, err := app.HandleRequest(ctx, event)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		// Extract the body to see the path
		if resp, ok := response.(*lift.Response); ok {
			bodyStr, _ := json.Marshal(resp.Body)
			fmt.Printf("Stage: %s, Full Path: %s\n", stage, fullPath)
			fmt.Printf("Response: %s\n", bodyStr)
		}
	}
}

func testV1WithStage(app *lift.App, stage, fullPath string) {
	event := map[string]interface{}{
		"resource": "/api/users",
		"path": fullPath,
		"httpMethod": "POST",
		"requestContext": map[string]interface{}{
			"stage": stage,
		},
		"body": `{"name": "test"}`,
	}

	ctx := context.Background()
	response, err := app.HandleRequest(ctx, event)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		// Extract the body to see the path
		if resp, ok := response.(*lift.Response); ok {
			bodyStr, _ := json.Marshal(resp.Body)
			fmt.Printf("Stage: %s, Full Path: %s\n", stage, fullPath)
			fmt.Printf("Response: %s\n", bodyStr)
		}
	}
}

func testV2FalseStage(app *lift.App, stage, fullPath string) {
	// Register a handler for this specific path
	app.GET("/production/test", func(ctx *lift.Context) error {
		return ctx.JSON(map[string]string{
			"message": "This is not a stage prefix",
			"path": ctx.Request.Path,
		})
	})

	event := map[string]interface{}{
		"version": "2.0",
		"routeKey": fmt.Sprintf("GET %s", fullPath),
		"rawPath": fullPath,
		"requestContext": map[string]interface{}{
			"http": map[string]interface{}{
				"method": "GET",
				"path":   fullPath,
			},
			"stage": stage,
		},
	}

	ctx := context.Background()
	response, err := app.HandleRequest(ctx, event)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		// Extract the body to see the path
		if resp, ok := response.(*lift.Response); ok {
			bodyStr, _ := json.Marshal(resp.Body)
			fmt.Printf("Stage: %s, Path: %s (should not strip 'production')\n", stage, fullPath)
			fmt.Printf("Response: %s\n", bodyStr)
		}
	}
}