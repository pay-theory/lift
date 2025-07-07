package main

import (
	"context"
	"fmt"

	"github.com/pay-theory/lift/pkg/lift"
)

func main() {
	fmt.Println("=== Testing Lift Adapter Error Reporting ===\n")

	// Create app with debug mode
	app := lift.New(lift.WithDebug())

	// Register a handler (won't be reached for unknown events)
	app.POST("/test", func(ctx *lift.Context) error {
		return ctx.JSON(map[string]string{"message": "Success"})
	})

	// Test 1: Unknown event format
	fmt.Println("--- Test 1: Unknown Event Format ---")
	unknownEvent := map[string]interface{}{
		"someField": "value",
		"anotherField": 123,
		"nested": map[string]interface{}{
			"data": "test",
		},
	}
	
	ctx := context.Background()
	_, err := app.HandleRequest(ctx, unknownEvent)
	if err != nil {
		fmt.Printf("Error (enhanced): %v\n\n", err)
	}

	// Test 2: Event that looks like API Gateway but missing required fields
	fmt.Println("--- Test 2: Partial API Gateway Event ---")
	partialEvent := map[string]interface{}{
		"httpMethod": "POST",
		"path": "/test",
		// Missing requestContext and resource - won't match v1
	}
	
	_, err = app.HandleRequest(ctx, partialEvent)
	if err != nil {
		fmt.Printf("Error (enhanced): %v\n\n", err)
	}

	// Test 3: Event with v2 version but missing required fields
	fmt.Println("--- Test 3: Partial API Gateway v2 Event ---")
	partialV2Event := map[string]interface{}{
		"version": "2.0",
		"routeKey": "POST /test",
		// Missing requestContext - won't match v2
	}
	
	_, err = app.HandleRequest(ctx, partialV2Event)
	if err != nil {
		fmt.Printf("Error (enhanced): %v\n\n", err)
	}

	// Test 4: Complete test with debug logging enabled
	fmt.Println("--- Test 4: Valid Event with Debug Logging ---")
	validEvent := map[string]interface{}{
		"version": "2.0",
		"routeKey": "POST /test",
		"rawPath": "/test",
		"requestContext": map[string]interface{}{
			"http": map[string]interface{}{
				"method": "POST",
				"path":   "/test",
			},
		},
		"body": `{"test": true}`,
	}
	
	response, err := app.HandleRequest(ctx, validEvent)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Success! Response status: %v\n", response)
	}
}