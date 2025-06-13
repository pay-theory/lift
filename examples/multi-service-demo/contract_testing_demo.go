package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/pay-theory/lift/pkg/testing/enterprise"
)

func runContractTestingDemo() {
	fmt.Println("🔄 Contract Testing Framework Demo")
	fmt.Println("==================================")

	// Initialize contract testing framework
	config := &enterprise.ContractTestConfig{
		Environment:    "demo",
		Timeout:        30 * time.Second,
		RetryAttempts:  3,
		RetryDelay:     1 * time.Second,
		StrictMode:     true,
		Parallel:       true,
		MaxConcurrency: 10,
	}

	framework := enterprise.NewContractTestingFramework(config)
	ctx := context.Background()

	// Demo 1: User Service Contract
	fmt.Println("\n📋 Demo 1: User Service Contract Validation")
	fmt.Println("-------------------------------------------")

	userServiceContract := createUserServiceContract()
	result, err := framework.ValidateContract(ctx, userServiceContract)
	if err != nil {
		log.Fatalf("User service contract validation failed: %v", err)
	}

	displayValidationResult("User Service", result)

	// Demo 2: Payment Service Contract
	fmt.Println("\n💳 Demo 2: Payment Service Contract Validation")
	fmt.Println("----------------------------------------------")

	paymentServiceContract := createPaymentServiceContract()
	result, err = framework.ValidateContract(ctx, paymentServiceContract)
	if err != nil {
		log.Fatalf("Payment service contract validation failed: %v", err)
	}

	displayValidationResult("Payment Service", result)

	// Demo 3: Order Service Contract
	fmt.Println("\n📦 Demo 3: Order Service Contract Validation")
	fmt.Println("--------------------------------------------")

	orderServiceContract := createOrderServiceContract()
	result, err = framework.ValidateContract(ctx, orderServiceContract)
	if err != nil {
		log.Fatalf("Order service contract validation failed: %v", err)
	}

	displayValidationResult("Order Service", result)

	// Demo 4: Multi-Service Integration Contract
	fmt.Println("\n🔗 Demo 4: Multi-Service Integration Contract")
	fmt.Println("---------------------------------------------")

	integrationContract := createIntegrationContract()
	result, err = framework.ValidateContract(ctx, integrationContract)
	if err != nil {
		log.Fatalf("Integration contract validation failed: %v", err)
	}

	displayValidationResult("Multi-Service Integration", result)

	// Demo 5: Schema Evolution Testing
	fmt.Println("\n🔄 Demo 5: Schema Evolution Testing")
	fmt.Println("-----------------------------------")

	demonstrateSchemaEvolution(framework, ctx)

	// Demo 6: Performance Testing
	fmt.Println("\n⚡ Demo 6: Contract Performance Testing")
	fmt.Println("--------------------------------------")

	demonstratePerformanceTesting(framework, ctx)

	// Demo 7: Error Handling and Edge Cases
	fmt.Println("\n🚨 Demo 7: Error Handling and Edge Cases")
	fmt.Println("----------------------------------------")

	demonstrateErrorHandling(framework, ctx)

	fmt.Println("\n✅ Contract Testing Demo Completed Successfully!")
	fmt.Println("===============================================")
}

func createUserServiceContract() *enterprise.ServiceContract {
	return &enterprise.ServiceContract{
		ID:      "user-service-v1",
		Name:    "User Service API",
		Version: "1.0.0",
		Provider: enterprise.ServiceInfo{
			Name:        "user-service",
			Version:     "1.0.0",
			BaseURL:     "https://api.paytheory.com/users",
			Environment: "production",
			Metadata: map[string]interface{}{
				"team":        "platform",
				"maintainer":  "platform-team@paytheory.com",
				"description": "Core user management service",
			},
		},
		Consumer: enterprise.ServiceInfo{
			Name:        "web-application",
			Version:     "2.1.0",
			BaseURL:     "https://app.paytheory.com",
			Environment: "production",
			Metadata: map[string]interface{}{
				"team":        "frontend",
				"maintainer":  "frontend-team@paytheory.com",
				"description": "Main web application",
			},
		},
		Interactions: []enterprise.ContractInteraction{
			{
				ID:          "get-user-by-id",
				Description: "Retrieve user information by user ID",
				Request: &enterprise.InteractionRequest{
					Method: "GET",
					Path:   "/users/{id}",
					Headers: map[string]string{
						"Accept":        "application/json",
						"Authorization": "Bearer {token}",
						"Content-Type":  "application/json",
					},
					Schema: &enterprise.SchemaDefinition{
						Type: "object",
						Properties: map[string]*enterprise.SchemaProperty{
							"id": {
								Type:        "string",
								Description: "User ID",
								Pattern:     "^[a-zA-Z0-9-]+$",
								MinLength:   &[]int{1}[0],
								MaxLength:   &[]int{50}[0],
							},
						},
						Required: []string{"id"},
					},
				},
				Response: &enterprise.InteractionResponse{
					Status: 200,
					Headers: map[string]string{
						"Content-Type":  "application/json",
						"Cache-Control": "max-age=300",
					},
					Body: map[string]interface{}{
						"id":         "user-123",
						"email":      "user@example.com",
						"first_name": "John",
						"last_name":  "Doe",
						"created_at": "2023-01-01T00:00:00Z",
						"updated_at": "2023-01-01T00:00:00Z",
						"status":     "active",
					},
					Schema: &enterprise.SchemaDefinition{
						Type: "object",
						Properties: map[string]*enterprise.SchemaProperty{
							"id": {
								Type:        "string",
								Description: "Unique user identifier",
							},
							"email": {
								Type:        "string",
								Description: "User email address",
								Format:      "email",
							},
							"first_name": {
								Type:        "string",
								Description: "User first name",
								MinLength:   &[]int{1}[0],
								MaxLength:   &[]int{50}[0],
							},
							"last_name": {
								Type:        "string",
								Description: "User last name",
								MinLength:   &[]int{1}[0],
								MaxLength:   &[]int{50}[0],
							},
							"created_at": {
								Type:        "string",
								Description: "User creation timestamp",
								Format:      "date-time",
							},
							"updated_at": {
								Type:        "string",
								Description: "User last update timestamp",
								Format:      "date-time",
							},
							"status": {
								Type:        "string",
								Description: "User account status",
								Enum:        []interface{}{"active", "inactive", "suspended"},
							},
						},
						Required: []string{"id", "email", "first_name", "last_name", "created_at", "status"},
					},
				},
				State: "user exists",
				Metadata: map[string]interface{}{
					"priority":    "high",
					"category":    "core",
					"test_data":   "user-123",
					"description": "Critical user retrieval endpoint",
				},
			},
		},
		Status:    enterprise.ContractActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata: map[string]interface{}{
			"service_type":     "microservice",
			"data_sensitivity": "high",
			"sla_tier":         "tier1",
			"compliance":       []string{"SOC2", "GDPR"},
		},
	}
}

func createPaymentServiceContract() *enterprise.ServiceContract {
	return &enterprise.ServiceContract{
		ID:      "payment-service-v1",
		Name:    "Payment Processing API",
		Version: "1.0.0",
		Provider: enterprise.ServiceInfo{
			Name:        "payment-service",
			Version:     "1.0.0",
			BaseURL:     "https://api.paytheory.com/payments",
			Environment: "production",
		},
		Consumer: enterprise.ServiceInfo{
			Name:        "order-service",
			Version:     "1.2.0",
			BaseURL:     "https://api.paytheory.com/orders",
			Environment: "production",
		},
		Interactions: []enterprise.ContractInteraction{
			{
				ID:          "process-payment",
				Description: "Process a payment transaction",
				Request: &enterprise.InteractionRequest{
					Method: "POST",
					Path:   "/payments",
					Headers: map[string]string{
						"Accept":        "application/json",
						"Authorization": "Bearer {token}",
						"Content-Type":  "application/json",
					},
					Body: map[string]interface{}{
						"amount":      10000,
						"currency":    "USD",
						"customer_id": "user-123",
						"order_id":    "order-789",
					},
				},
				Response: &enterprise.InteractionResponse{
					Status: 200,
					Headers: map[string]string{
						"Content-Type": "application/json",
					},
					Body: map[string]interface{}{
						"id":          "payment-abc123",
						"status":      "succeeded",
						"amount":      10000,
						"currency":    "USD",
						"created_at":  "2023-01-01T00:00:00Z",
						"customer_id": "user-123",
						"order_id":    "order-789",
					},
				},
				State: "valid payment method and sufficient funds",
			},
		},
		Status:    enterprise.ContractActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func createOrderServiceContract() *enterprise.ServiceContract {
	return &enterprise.ServiceContract{
		ID:      "order-service-v1",
		Name:    "Order Management API",
		Version: "1.0.0",
		Provider: enterprise.ServiceInfo{
			Name:        "order-service",
			Version:     "1.2.0",
			BaseURL:     "https://api.paytheory.com/orders",
			Environment: "production",
		},
		Consumer: enterprise.ServiceInfo{
			Name:        "web-application",
			Version:     "2.1.0",
			BaseURL:     "https://app.paytheory.com",
			Environment: "production",
		},
		Interactions: []enterprise.ContractInteraction{
			{
				ID:          "create-order",
				Description: "Create a new order",
				Request: &enterprise.InteractionRequest{
					Method: "POST",
					Path:   "/orders",
					Headers: map[string]string{
						"Accept":        "application/json",
						"Authorization": "Bearer {token}",
						"Content-Type":  "application/json",
					},
					Body: map[string]interface{}{
						"customer_id": "user-123",
						"items": []interface{}{
							map[string]interface{}{
								"product_id": "prod-456",
								"quantity":   2,
								"price":      2500,
							},
						},
					},
				},
				Response: &enterprise.InteractionResponse{
					Status: 201,
					Headers: map[string]string{
						"Content-Type": "application/json",
						"Location":     "/orders/{id}",
					},
					Body: map[string]interface{}{
						"id":          "order-789",
						"customer_id": "user-123",
						"status":      "pending",
						"total":       5000,
						"currency":    "USD",
						"created_at":  "2023-01-01T00:00:00Z",
					},
				},
				State: "valid customer and products",
			},
		},
		Status:    enterprise.ContractActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func createIntegrationContract() *enterprise.ServiceContract {
	return &enterprise.ServiceContract{
		ID:      "integration-workflow-v1",
		Name:    "Multi-Service Integration Workflow",
		Version: "1.0.0",
		Provider: enterprise.ServiceInfo{
			Name:        "integration-orchestrator",
			Version:     "1.0.0",
			BaseURL:     "https://api.paytheory.com/integration",
			Environment: "production",
		},
		Consumer: enterprise.ServiceInfo{
			Name:        "web-application",
			Version:     "2.1.0",
			BaseURL:     "https://app.paytheory.com",
			Environment: "production",
		},
		Interactions: []enterprise.ContractInteraction{
			{
				ID:          "complete-purchase-workflow",
				Description: "Complete end-to-end purchase workflow",
				Request: &enterprise.InteractionRequest{
					Method: "POST",
					Path:   "/workflows/purchase",
					Headers: map[string]string{
						"Accept":        "application/json",
						"Authorization": "Bearer {token}",
						"Content-Type":  "application/json",
					},
					Body: map[string]interface{}{
						"customer_id": "user-123",
						"items": []interface{}{
							map[string]interface{}{
								"product_id": "prod-456",
								"quantity":   1,
								"price":      5000,
							},
						},
					},
				},
				Response: &enterprise.InteractionResponse{
					Status: 200,
					Headers: map[string]string{
						"Content-Type": "application/json",
					},
					Body: map[string]interface{}{
						"workflow_id": "workflow-xyz789",
						"status":      "completed",
						"order": map[string]interface{}{
							"id":     "order-789",
							"status": "confirmed",
							"total":  5000,
						},
						"payment": map[string]interface{}{
							"id":     "payment-abc123",
							"status": "succeeded",
							"amount": 5000,
						},
						"user": map[string]interface{}{
							"id":    "user-123",
							"email": "user@example.com",
						},
						"completed_at": "2023-01-01T00:00:00Z",
					},
				},
				State: "valid customer, products, and payment method",
			},
		},
		Status:    enterprise.ContractActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func displayValidationResult(serviceName string, result *enterprise.ContractValidationResult) {
	fmt.Printf("Service: %s\n", serviceName)
	fmt.Printf("Contract ID: %s\n", result.ContractID)
	fmt.Printf("Status: %s\n", result.Status)
	fmt.Printf("Duration: %v\n", result.Duration)

	// Calculate summary statistics from validations
	totalInteractions := len(result.Validations)
	validInteractions := 0
	totalChecks := 0
	passedChecks := 0

	for _, validation := range result.Validations {
		if validation.Status == "passed" {
			validInteractions++
		}
		for _, check := range validation.Checks {
			totalChecks++
			if check.Valid {
				passedChecks++
			}
		}
	}

	successRate := 0.0
	if totalInteractions > 0 {
		successRate = float64(validInteractions) / float64(totalInteractions) * 100
	}

	fmt.Printf("Summary:\n")
	fmt.Printf("  - Total Interactions: %d\n", totalInteractions)
	fmt.Printf("  - Valid Interactions: %d\n", validInteractions)
	fmt.Printf("  - Invalid Interactions: %d\n", totalInteractions-validInteractions)
	fmt.Printf("  - Success Rate: %.2f%%\n", successRate)
	fmt.Printf("  - Total Checks: %d\n", totalChecks)
	fmt.Printf("  - Passed Checks: %d\n", passedChecks)
	fmt.Printf("  - Failed Checks: %d\n", totalChecks-passedChecks)

	// Display detailed validation results
	for interactionID, validation := range result.Validations {
		fmt.Printf("\nInteraction: %s\n", interactionID)
		fmt.Printf("  Status: %s\n", validation.Status)
		fmt.Printf("  Duration: %v\n", validation.Duration)

		for checkType, check := range validation.Checks {
			status := "✅"
			if !check.Valid {
				status = "❌"
			}
			fmt.Printf("  %s %s: %s\n", status, checkType, check.Status)

			if len(check.Errors) > 0 {
				for _, err := range check.Errors {
					fmt.Printf("    Error: %s\n", err)
				}
			}
		}
	}

	fmt.Println()
}

func demonstrateSchemaEvolution(framework *enterprise.ContractTestingFramework, ctx context.Context) {
	fmt.Println("Testing schema evolution compatibility...")
	fmt.Printf("Schema evolution validation: simulated (would validate backward compatibility)\n")
	fmt.Println("✅ Schema evolution testing completed")
}

func demonstratePerformanceTesting(framework *enterprise.ContractTestingFramework, ctx context.Context) {
	fmt.Println("Running performance tests...")

	// Create a contract with many interactions for performance testing
	performanceContract := &enterprise.ServiceContract{
		ID:           "performance-test-contract",
		Name:         "Performance Test Contract",
		Version:      "1.0.0",
		Interactions: make([]enterprise.ContractInteraction, 25),
	}

	// Generate test interactions
	for i := 0; i < 25; i++ {
		performanceContract.Interactions[i] = enterprise.ContractInteraction{
			ID:          fmt.Sprintf("perf-interaction-%d", i),
			Description: fmt.Sprintf("Performance test interaction %d", i),
			Request: &enterprise.InteractionRequest{
				Method: "GET",
				Path:   fmt.Sprintf("/api/test/%d", i),
				Headers: map[string]string{
					"Accept": "application/json",
				},
			},
			Response: &enterprise.InteractionResponse{
				Status: 200,
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
				Body: map[string]interface{}{
					"id":   i,
					"data": fmt.Sprintf("test data %d", i),
				},
			},
		}
	}

	// Measure validation performance
	start := time.Now()
	result, err := framework.ValidateContract(ctx, performanceContract)
	duration := time.Since(start)

	if err != nil {
		log.Printf("Performance test failed: %v", err)
		return
	}

	fmt.Printf("Performance Test Results:\n")
	fmt.Printf("  - Interactions: %d\n", len(performanceContract.Interactions))
	fmt.Printf("  - Total Duration: %v\n", duration)
	fmt.Printf("  - Average per Interaction: %v\n", duration/time.Duration(len(performanceContract.Interactions)))
	fmt.Printf("  - Validations per Second: %.2f\n", float64(len(performanceContract.Interactions))/duration.Seconds())
	// Calculate success rate from validations
	totalValidations := len(result.Validations)
	passedValidations := 0
	for _, validation := range result.Validations {
		if validation.Status == "passed" {
			passedValidations++
		}
	}
	successRate := 0.0
	if totalValidations > 0 {
		successRate = float64(passedValidations) / float64(totalValidations) * 100
	}
	fmt.Printf("  - Success Rate: %.2f%%\n", successRate)

	if duration > 3*time.Second {
		fmt.Printf("⚠️  Performance warning: Validation took longer than expected\n")
	} else {
		fmt.Printf("✅ Performance test passed\n")
	}
}

func demonstrateErrorHandling(framework *enterprise.ContractTestingFramework, ctx context.Context) {
	fmt.Println("Testing error handling and edge cases...")

	// Test 1: Invalid HTTP method
	invalidMethodContract := &enterprise.ServiceContract{
		ID:      "invalid-method-test",
		Name:    "Invalid Method Test",
		Version: "1.0.0",
		Interactions: []enterprise.ContractInteraction{
			{
				ID:          "invalid-method",
				Description: "Test with invalid HTTP method",
				Request: &enterprise.InteractionRequest{
					Method: "INVALID_METHOD",
					Path:   "/api/test",
				},
				Response: &enterprise.InteractionResponse{
					Status: 200,
				},
			},
		},
	}

	result, err := framework.ValidateContract(ctx, invalidMethodContract)
	if err != nil {
		fmt.Printf("❌ Invalid method test error: %v\n", err)
	} else {
		fmt.Printf("Invalid method test result: %s\n", result.Status)
		if result.Status == "failed" {
			fmt.Printf("✅ Correctly detected invalid HTTP method\n")
		}
	}

	// Test 2: Invalid path
	invalidPathContract := &enterprise.ServiceContract{
		ID:      "invalid-path-test",
		Name:    "Invalid Path Test",
		Version: "1.0.0",
		Interactions: []enterprise.ContractInteraction{
			{
				ID:          "invalid-path",
				Description: "Test with invalid path",
				Request: &enterprise.InteractionRequest{
					Method: "GET",
					Path:   "invalid-path-without-slash",
				},
				Response: &enterprise.InteractionResponse{
					Status: 200,
				},
			},
		},
	}

	result, err = framework.ValidateContract(ctx, invalidPathContract)
	if err != nil {
		fmt.Printf("❌ Invalid path test error: %v\n", err)
	} else {
		fmt.Printf("Invalid path test result: %s\n", result.Status)
		if result.Status == "failed" {
			fmt.Printf("✅ Correctly detected invalid path\n")
		}
	}

	fmt.Println("✅ Error handling tests completed")
}

// Note: Schema validation would be implemented in the framework
