package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/pay-theory/lift/pkg/observability"
	"github.com/pay-theory/lift/pkg/observability/zap"
	"github.com/pay-theory/lift/pkg/services/kernel"
)

// Example demonstrates how to use the kernel client to call various kernel services
func main() {
	// Set required environment variables for the example
	// In production, these would be set by your Lambda environment
	os.Setenv("PARTNER", "qakernel")
	os.Setenv("STAGE", "dev")
	os.Setenv("TARGET_REGION", "us-east-1")

	// Create context
	ctx := context.Background()

	// Create logger (in production, this would be your singleton logger from internal/logger)
	globalLogger, err := zap.NewZapLogger(observability.LoggerConfig{
		Level:  "info",
		Format: "json",
	})
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer globalLogger.Close()

	// Logger getter function (mimics internal/logger.GetLiftLogger pattern)
	getLogger := func() observability.StructuredLogger {
		return globalLogger
	}

	// Create kernel client with logger getter function
	// In production: kernel.NewClient(ctx, pkglogger.GetLiftLogger)
	client, err := kernel.NewClient(ctx, getLogger)
	if err != nil {
		log.Fatalf("Failed to create kernel client: %v", err)
	}

	// Example 1: Call Paze Wallet Service
	fmt.Println("\n=== Example 1: Paze Wallet Service ===")
	if err := examplePazeWalletCall(ctx, client); err != nil {
		log.Printf("Paze wallet example failed: %v", err)
	}

	// Example 2: Call K3 API
	fmt.Println("\n=== Example 2: K3 API ===")
	if err := exampleK3Call(ctx, client); err != nil {
		log.Printf("K3 example failed: %v", err)
	}

	// Example 3: Call Bin Lookup Service
	fmt.Println("\n=== Example 3: Bin Lookup Service ===")
	if err := exampleBinLookupCall(ctx, client); err != nil {
		log.Printf("Bin lookup example failed: %v", err)
	}

	// Example 4: Call Apple Wallet Service
	fmt.Println("\n=== Example 4: Apple Wallet Service ===")
	if err := exampleAppleWalletCall(ctx, client); err != nil {
		log.Printf("Apple wallet example failed: %v", err)
	}

	// Example 5: Generic call with custom options
	fmt.Println("\n=== Example 5: Generic Call with Custom Options ===")
	if err := exampleGenericCall(ctx, client); err != nil {
		log.Printf("Generic call example failed: %v", err)
	}

	// Example 6: Error handling
	fmt.Println("\n=== Example 6: Error Handling ===")
	exampleErrorHandling(ctx, client)
}

// examplePazeWalletCall demonstrates calling the Paze wallet service
func examplePazeWalletCall(ctx context.Context, client *kernel.Client) error {
	fmt.Println("Calling Paze Wallet Service to decode token...")

	// Prepare request data
	requestData := map[string]any{
		"token":          "encrypted_paze_token_here",
		"transaction_id": "txn_123456789",
	}

	// Make the call
	response, err := client.PazeWalletCall(ctx, "decode-token", requestData, "POST")
	if err != nil {
		return fmt.Errorf("paze wallet call failed: %w", err)
	}

	// Parse response
	var result map[string]any
	if err := response.Unmarshal(&result); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	fmt.Printf("Response status: %d\n", response.StatusCode)
	fmt.Printf("Response duration: %v\n", response.Duration)
	fmt.Printf("Decoded data: %+v\n", result)

	return nil
}

// exampleK3Call demonstrates calling the K3 API
func exampleK3Call(ctx context.Context, client *kernel.Client) error {
	fmt.Println("Calling K3 API to tokenize card...")

	// Prepare tokenization request
	tokenizeData := map[string]any{
		"card_number": "4111111111111111",
		"exp_month":   "12",
		"exp_year":    "2025",
		"cvv":         "123",
		"merchant_id": "merchant_123",
	}

	// Make the call
	response, err := client.K3Call(ctx, "v1/tokenize", tokenizeData, "POST")
	if err != nil {
		return fmt.Errorf("k3 call failed: %w", err)
	}

	// Parse response
	var tokenResult struct {
		Token     string `json:"token"`
		LastFour  string `json:"last_four"`
		CardBrand string `json:"card_brand"`
	}
	if err := response.Unmarshal(&tokenResult); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	fmt.Printf("Token: %s\n", tokenResult.Token)
	fmt.Printf("Last Four: %s\n", tokenResult.LastFour)
	fmt.Printf("Card Brand: %s\n", tokenResult.CardBrand)

	return nil
}

// exampleBinLookupCall demonstrates calling the bin lookup service
func exampleBinLookupCall(ctx context.Context, client *kernel.Client) error {
	fmt.Println("Calling Bin Lookup Service...")

	// Make the call with query parameters
	response, err := client.BinLookupCall(ctx, "?card_bin=411111", "GET", nil)
	if err != nil {
		return fmt.Errorf("bin lookup call failed: %w", err)
	}

	// Parse response
	var binData struct {
		CardBrand string `json:"card_brand"`
		CardType  string `json:"card_type"`
		BankName  string `json:"bank_name"`
		Country   string `json:"country"`
	}
	if err := response.Unmarshal(&binData); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	fmt.Printf("Card Brand: %s\n", binData.CardBrand)
	fmt.Printf("Card Type: %s\n", binData.CardType)
	fmt.Printf("Bank Name: %s\n", binData.BankName)
	fmt.Printf("Country: %s\n", binData.Country)

	return nil
}

// exampleAppleWalletCall demonstrates calling the Apple Wallet service
func exampleAppleWalletCall(ctx context.Context, client *kernel.Client) error {
	fmt.Println("Calling Apple Wallet Service to decode token...")

	// Prepare Apple Pay token data
	applePayData := map[string]any{
		"ephemeral_public_key": "BFxF...",
		"signature":            "MIAGCSqG...",
		"token":                "encrypted_token_data",
		"transaction_id":       "txn_apple_123",
	}

	// Make the call
	response, err := client.AppleWalletCall(ctx, "decode-token", applePayData, "POST")
	if err != nil {
		return fmt.Errorf("apple wallet call failed: %w", err)
	}

	// Parse response
	var result struct {
		CardNumber string `json:"card_number"`
		ExpMonth   string `json:"exp_month"`
		ExpYear    string `json:"exp_year"`
	}
	if err := response.Unmarshal(&result); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	fmt.Printf("Card Number: %s\n", result.CardNumber)
	fmt.Printf("Expiration: %s/%s\n", result.ExpMonth, result.ExpYear)

	return nil
}

// exampleGenericCall demonstrates using the generic Call method with custom options
func exampleGenericCall(ctx context.Context, client *kernel.Client) error {
	fmt.Println("Making generic call with custom options...")

	// Custom call options
	opts := &kernel.CallOptions{
		ServicePrefix: "custom-service",
		Endpoint:      "api/v1/process",
		Method:        "POST",
		Body: map[string]any{
			"data":   "custom_value",
			"type":   "processing",
			"amount": 1000,
		},
		IncludeRegion: true,
		Subsystem:     "CUSTOM SERVICE",
		Headers: map[string]string{
			"X-Custom-Header":  "custom-value",
			"X-Request-Source": "example-app",
		},
	}

	// Make the call
	response, err := client.Call(ctx, opts)
	if err != nil {
		return fmt.Errorf("custom call failed: %w", err)
	}

	fmt.Printf("Response status: %d\n", response.StatusCode)
	fmt.Printf("Response duration: %v\n", response.Duration)
	fmt.Printf("Response body: %s\n", string(response.Body))

	return nil
}

// exampleErrorHandling demonstrates proper error handling patterns
func exampleErrorHandling(ctx context.Context, client *kernel.Client) {
	fmt.Println("Demonstrating error handling...")

	// Example 1: Handle network errors
	_, err := client.K3Call(ctx, "invalid-endpoint", nil, "POST")
	if err != nil {
		fmt.Printf("Expected error (invalid endpoint): %v\n", err)
	}

	// Example 2: Handle HTTP error status codes
	invalidData := map[string]any{
		"invalid_field": "this will cause validation error",
	}
	response, err := client.K3Call(ctx, "v1/tokenize", invalidData, "POST")
	if err != nil {
		fmt.Printf("Expected error (validation): %v\n", err)
		if response != nil {
			fmt.Printf("Error response status: %d\n", response.StatusCode)
			fmt.Printf("Error response body: %s\n", string(response.Body))
		}
	}

	// Example 3: Handle unmarshal errors
	response = &kernel.Response{
		StatusCode: 200,
		Body:       []byte("invalid json {{{"),
	}
	var invalidResult map[string]any
	if err := response.Unmarshal(&invalidResult); err != nil {
		fmt.Printf("Expected error (invalid JSON): %v\n", err)
	}

	fmt.Println("Error handling examples completed")
}

// exampleWithLiftContext demonstrates usage within a Lift Lambda handler
// This would typically be in your actual Lambda handler file
/*
func ExampleLiftHandler(ctx *lift.Context) error {
	// Get kernel client from context (set by middleware)
	kernelClient := kernel.GetKernelClient(ctx)
	if kernelClient == nil {
		return lift.SystemError("Kernel client not available")
	}

	// Use kernel client to call Paze service
	response, err := kernelClient.PazeWalletCall(ctx, "decode-token", map[string]any{
		"token": ctx.Query("token"),
	}, "POST")
	if err != nil {
		ctx.Logger().Error("Failed to decode Paze token", map[string]any{
			"error": err.Error(),
		})
		return lift.SystemError("Failed to decode token").WithCause(err)
	}

	// Parse and return result
	var result map[string]any
	if err := response.Unmarshal(&result); err != nil {
		return lift.SystemError("Failed to parse response").WithCause(err)
	}

	return ctx.OK(result)
}

// In main.go Lambda setup
func main() {
	app := lift.New()

	// Create logger and set as singleton
	logger, err := zap.NewZapLogger(observability.LoggerConfig{
		Level:  "info",
		Format: "json",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer logger.Close()

	app.WithLogger(logger)

	// Set singleton logger (in your internal/logger package)
	// pkglogger.SetLiftLogger(logger)

	// Create kernel client with logger getter function
	// This uses your service's singleton logger pattern
	kernelClient, err := kernel.NewClient(context.Background(), pkglogger.GetLiftLogger)
	if err != nil {
		log.Fatalf("Failed to create kernel client: %v", err)
	}

	// Register middleware
	app.Use(kernel.KernelClientMiddleware(kernelClient))

	// Register handlers
	app.POST("/paze/decode", ExampleLiftHandler)

	// Start Lambda
	lambda.Start(app.HandleRequest)
}
*/
