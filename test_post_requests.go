package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pay-theory/lift/pkg/lift"
)

// Test request structure
type TestRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
}

func main() {
	fmt.Println("=== Testing Lift POST Request Handling ===\n")

	// Create app with debug mode enabled
	app := lift.New(lift.WithDebug())

	// Register a POST handler
	app.POST("/test", func(ctx *lift.Context) error {
		fmt.Printf("Handler called! Method: %s, Path: %s\n", ctx.Request.Method, ctx.Request.Path)
		fmt.Printf("Raw Body: %s\n", string(ctx.Request.Body))
		fmt.Printf("Body Length: %d\n", len(ctx.Request.Body))

		// Try to parse the request
		var req TestRequest
		if err := ctx.ParseRequest(&req); err != nil {
			fmt.Printf("Parse error: %v\n", err)
			return err
		}

		fmt.Printf("Parsed successfully: %+v\n", req)
		return ctx.JSON(map[string]interface{}{
			"message": "Success",
			"data":    req,
		})
	})

	// Test 1: API Gateway v2 POST with JSON body
	fmt.Println("\n--- Test 1: API Gateway v2 POST with JSON ---")
	testAPIGatewayV2Post(app)

	// Test 2: API Gateway v1 POST with JSON body
	fmt.Println("\n--- Test 2: API Gateway v1 POST with JSON ---")
	testAPIGatewayV1Post(app)

	// Test 3: Empty body
	fmt.Println("\n--- Test 3: Empty Body ---")
	testEmptyBody(app)

	// Test 4: Base64 encoded body
	fmt.Println("\n--- Test 4: Base64 Encoded Body ---")
	testBase64Body(app)

	// Test 5: Invalid JSON
	fmt.Println("\n--- Test 5: Invalid JSON ---")
	testInvalidJSON(app)

	// Test 6: Missing path field (should trigger our error)
	fmt.Println("\n--- Test 6: API Gateway v1 Missing Path ---")
	testV1MissingPath(app)
}

func testAPIGatewayV2Post(app *lift.App) {
	event := map[string]interface{}{
		"version": "2.0",
		"routeKey": "POST /test",
		"rawPath": "/test",
		"headers": map[string]string{
			"content-type": "application/json",
		},
		"requestContext": map[string]interface{}{
			"http": map[string]interface{}{
				"method": "POST",
				"path":   "/test",
			},
			"stage": "$default",
		},
		"body": `{"name":"John Doe","email":"john@example.com","age":30}`,
		"isBase64Encoded": false,
	}

	ctx := context.Background()
	response, err := app.HandleRequest(ctx, event)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		respJSON, _ := json.MarshalIndent(response, "", "  ")
		fmt.Printf("Response: %s\n", respJSON)
	}
}

func testAPIGatewayV1Post(app *lift.App) {
	event := map[string]interface{}{
		"resource": "/test",
		"path": "/test",
		"httpMethod": "POST",
		"headers": map[string]string{
			"Content-Type": "application/json",
		},
		"requestContext": map[string]interface{}{
			"stage": "prod",
		},
		"body": `{"name":"Jane Smith","email":"jane@example.com","age":25}`,
		"isBase64Encoded": false,
	}

	ctx := context.Background()
	response, err := app.HandleRequest(ctx, event)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		respJSON, _ := json.MarshalIndent(response, "", "  ")
		fmt.Printf("Response: %s\n", respJSON)
	}
}

func testEmptyBody(app *lift.App) {
	event := map[string]interface{}{
		"version": "2.0",
		"routeKey": "POST /test",
		"rawPath": "/test",
		"requestContext": map[string]interface{}{
			"http": map[string]interface{}{
				"method": "POST",
				"path":   "/test",
			},
			"stage": "$default",
		},
		"body": "",
		"isBase64Encoded": false,
	}

	ctx := context.Background()
	response, err := app.HandleRequest(ctx, event)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		respJSON, _ := json.MarshalIndent(response, "", "  ")
		fmt.Printf("Response: %s\n", respJSON)
	}
}

func testBase64Body(app *lift.App) {
	// Base64 encoded: {"name":"Base64 User","email":"base64@example.com","age":40}
	event := map[string]interface{}{
		"version": "2.0",
		"routeKey": "POST /test",
		"rawPath": "/test",
		"requestContext": map[string]interface{}{
			"http": map[string]interface{}{
				"method": "POST",
				"path":   "/test",
			},
			"stage": "$default",
		},
		"body": "eyJuYW1lIjoiQmFzZTY0IFVzZXIiLCJlbWFpbCI6ImJhc2U2NEBleGFtcGxlLmNvbSIsImFnZSI6NDB9",
		"isBase64Encoded": true,
	}

	ctx := context.Background()
	response, err := app.HandleRequest(ctx, event)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		respJSON, _ := json.MarshalIndent(response, "", "  ")
		fmt.Printf("Response: %s\n", respJSON)
	}
}

func testInvalidJSON(app *lift.App) {
	event := map[string]interface{}{
		"version": "2.0",
		"routeKey": "POST /test",
		"rawPath": "/test",
		"requestContext": map[string]interface{}{
			"http": map[string]interface{}{
				"method": "POST",
				"path":   "/test",
			},
			"stage": "$default",
		},
		"body": `{"name": "Invalid JSON", "email": }`,
		"isBase64Encoded": false,
	}

	ctx := context.Background()
	response, err := app.HandleRequest(ctx, event)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		respJSON, _ := json.MarshalIndent(response, "", "  ")
		fmt.Printf("Response: %s\n", respJSON)
	}
}

func testV1MissingPath(app *lift.App) {
	event := map[string]interface{}{
		"resource": "/test",
		// Missing "path" field - should trigger our new error
		"httpMethod": "POST",
		"headers": map[string]string{
			"Content-Type": "application/json",
		},
		"requestContext": map[string]interface{}{
			"stage": "prod",
		},
		"body": `{"name":"Test","email":"test@example.com","age":30}`,
		"isBase64Encoded": false,
	}

	ctx := context.Background()
	response, err := app.HandleRequest(ctx, event)
	if err != nil {
		fmt.Printf("Error (expected): %v\n", err)
	} else {
		respJSON, _ := json.MarshalIndent(response, "", "  ")
		fmt.Printf("Response: %s\n", respJSON)
	}
}