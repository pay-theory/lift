package enterprise

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestContractTestingFramework(t *testing.T) {
	config := &ContractTestConfig{
		Environment:    "test",
		Timeout:        30 * time.Second,
		RetryAttempts:  3,
		RetryDelay:     1 * time.Second,
		StrictMode:     true,
		Parallel:       true,
		MaxConcurrency: 5,
	}

	framework := NewContractTestingFramework(config)
	if framework == nil {
		t.Fatal("Failed to create contract testing framework")
	}

	// Test framework initialization
	if framework.config == nil {
		t.Error("Framework config not set correctly")
	}

	if framework.registry == nil {
		t.Error("Registry not initialized")
	}

	if framework.validator == nil {
		t.Error("Validator not initialized")
	}
}

func TestServiceContract(t *testing.T) {
	contract := &ServiceContract{
		ID:      "test-contract-001",
		Name:    "User Service Contract",
		Version: "1.0.0",
		Provider: ServiceInfo{
			Name:        "user-service",
			Version:     "1.0.0",
			BaseURL:     "https://api.example.com",
			Environment: "production",
		},
		Consumer: ServiceInfo{
			Name:        "web-app",
			Version:     "2.1.0",
			BaseURL:     "https://app.example.com",
			Environment: "production",
		},
		Status:    ContractActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Test contract creation
	if contract.ID != "test-contract-001" {
		t.Error("Contract ID not set correctly")
	}

	if contract.Status != ContractActive {
		t.Error("Contract status not set correctly")
	}

	// Test JSON serialization
	data, err := json.Marshal(contract)
	if err != nil {
		t.Fatalf("Failed to marshal contract: %v", err)
	}

	var unmarshaled ServiceContract
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal contract: %v", err)
	}

	if unmarshaled.ID != contract.ID {
		t.Error("Contract ID not preserved after serialization")
	}
}

func TestContractInteraction(t *testing.T) {
	interaction := ContractInteraction{
		ID:          "get-user-001",
		Description: "Get user by ID",
		Request: &InteractionRequest{
			Method: "GET",
			Path:   "/users/{id}",
			Headers: map[string]string{
				"Accept":        "application/json",
				"Authorization": "Bearer {token}",
			},
		},
		Response: &InteractionResponse{
			Status: 200,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: map[string]interface{}{
				"id":    123,
				"name":  "John Doe",
				"email": "john@example.com",
			},
		},
	}

	// Test interaction structure
	if interaction.Request.Method != "GET" {
		t.Error("Request method not set correctly")
	}

	if interaction.Response.Status != 200 {
		t.Error("Response status not set correctly")
	}

	// Test headers
	if interaction.Request.Headers["Accept"] != "application/json" {
		t.Error("Request headers not set correctly")
	}
}

func TestSchemaValidation(t *testing.T) {
	framework := NewContractTestingFramework(&ContractTestConfig{})

	// Test string schema validation
	stringSchema := &SchemaDefinition{
		Type:      "string",
		MinLength: &[]int{3}[0],
		MaxLength: &[]int{50}[0],
	}

	// Valid string
	check, err := framework.validateSchema("valid string", stringSchema)
	if err != nil {
		t.Fatalf("Schema validation failed: %v", err)
	}
	if !check.Valid {
		t.Error("Valid string should pass validation")
	}

	// Invalid string (too short)
	check, err = framework.validateSchema("ab", stringSchema)
	if err != nil {
		t.Fatalf("Schema validation failed: %v", err)
	}
	if check.Valid {
		t.Error("Short string should fail validation")
	}

	// Test object schema validation
	objectSchema := &SchemaDefinition{
		Type: "object",
		Properties: map[string]*SchemaProperty{
			"name": {
				Type:      "string",
				Required:  true,
				MinLength: &[]int{1}[0],
			},
			"age": {
				Type:    "integer",
				Minimum: &[]float64{0}[0],
				Maximum: &[]float64{150}[0],
			},
		},
		Required: []string{"name"},
	}

	// Valid object
	validObject := map[string]interface{}{
		"name": "John Doe",
		"age":  float64(30),
	}

	check, err = framework.validateSchema(validObject, objectSchema)
	if err != nil {
		t.Fatalf("Schema validation failed: %v", err)
	}
	if !check.Valid {
		t.Error("Valid object should pass validation")
	}

	// Invalid object (missing required field)
	invalidObject := map[string]interface{}{
		"age": float64(30),
	}

	check, err = framework.validateSchema(invalidObject, objectSchema)
	if err != nil {
		t.Fatalf("Schema validation failed: %v", err)
	}
	if check.Valid {
		t.Error("Object missing required field should fail validation")
	}
}

func TestContractValidation(t *testing.T) {
	framework := NewContractTestingFramework(&ContractTestConfig{})

	// Create test contract with interactions
	contract := &ServiceContract{
		ID:      "test-validation-001",
		Name:    "Test Validation Contract",
		Version: "1.0.0",
		Interactions: []ContractInteraction{
			{
				ID:          "test-interaction-001",
				Description: "Test GET endpoint",
				Request: &InteractionRequest{
					Method: "GET",
					Path:   "/api/test",
					Headers: map[string]string{
						"Accept": "application/json",
					},
					Schema: &SchemaDefinition{
						Type: "object",
						Properties: map[string]*SchemaProperty{
							"query": {Type: "string"},
						},
					},
				},
				Response: &InteractionResponse{
					Status: 200,
					Headers: map[string]string{
						"Content-Type": "application/json",
					},
					Body: map[string]interface{}{
						"status": "success",
						"data":   []interface{}{},
					},
					Schema: &SchemaDefinition{
						Type: "object",
						Properties: map[string]*SchemaProperty{
							"status": {Type: "string"},
							"data":   {Type: "array"},
						},
						Required: []string{"status"},
					},
				},
			},
		},
	}

	ctx := context.Background()
	result, err := framework.ValidateContract(ctx, contract)
	if err != nil {
		t.Fatalf("Contract validation failed: %v", err)
	}

	if result == nil {
		t.Fatal("Validation result is nil")
	}

	if result.ContractID != contract.ID {
		t.Error("Contract ID not set correctly in result")
	}

	if result.Status == "" {
		t.Error("Validation status not set")
	}

	if len(result.Validations) == 0 {
		t.Error("No validations generated")
	}

	// Check validation details
	if len(result.Validations) == 0 {
		t.Error("No interaction validations found")
	}

	for _, validation := range result.Validations {
		if validation.InteractionID == "" {
			t.Error("Interaction ID not set in validation")
		}

		if len(validation.Checks) == 0 {
			t.Error("No validation checks found for interaction")
		}
	}
}

func TestHTTPMethodValidation(t *testing.T) {
	framework := NewContractTestingFramework(&ContractTestConfig{})

	// Test valid HTTP methods
	validMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	for _, method := range validMethods {
		check := framework.validateHTTPMethod(method)
		if !check.Valid {
			t.Errorf("Valid HTTP method %s should pass validation", method)
		}
		if check.Status != "passed" {
			t.Errorf("Valid HTTP method %s should have passed status", method)
		}
	}

	// Test invalid HTTP method
	check := framework.validateHTTPMethod("INVALID")
	if check.Valid {
		t.Error("Invalid HTTP method should fail validation")
	}
	if check.Status != "failed" {
		t.Error("Invalid HTTP method should have failed status")
	}
	if len(check.Errors) == 0 {
		t.Error("Invalid HTTP method should have error messages")
	}
}

func TestHTTPPathValidation(t *testing.T) {
	framework := NewContractTestingFramework(&ContractTestConfig{})

	// Test valid HTTP paths
	validPaths := []string{"/", "/api", "/api/users", "/api/users/{id}", "/api/v1/users?filter=active"}
	for _, path := range validPaths {
		check := framework.validateHTTPPath(path)
		if !check.Valid {
			t.Errorf("Valid HTTP path %s should pass validation", path)
		}
	}

	// Test invalid HTTP paths
	invalidPaths := []string{"", "api", "api/users", "relative/path"}
	for _, path := range invalidPaths {
		check := framework.validateHTTPPath(path)
		if check.Valid {
			t.Errorf("Invalid HTTP path %s should fail validation", path)
		}
	}
}

func TestHeaderValidation(t *testing.T) {
	framework := NewContractTestingFramework(&ContractTestConfig{})

	// Test valid headers
	validHeaders := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer token123",
		"Accept":        "application/json",
	}

	check := framework.validateHeaders(validHeaders)
	if !check.Valid {
		t.Error("Valid headers should pass validation")
	}
	if check.Status != "passed" {
		t.Error("Valid headers should have passed status")
	}

	// Test invalid headers (empty name)
	invalidHeaders := map[string]string{
		"":           "some-value",
		"Valid-Name": "valid-value",
	}

	check = framework.validateHeaders(invalidHeaders)
	if check.Valid {
		t.Error("Headers with empty name should fail validation")
	}

	// Test invalid headers (empty value)
	invalidHeaders2 := map[string]string{
		"Content-Type": "",
		"Accept":       "application/json",
	}

	check = framework.validateHeaders(invalidHeaders2)
	if check.Valid {
		t.Error("Headers with empty value should fail validation")
	}
}

func TestTypeValidation(t *testing.T) {
	framework := NewContractTestingFramework(&ContractTestConfig{})

	// Test string type
	if !framework.validateType("hello", "string") {
		t.Error("String should validate as string type")
	}
	if framework.validateType(123, "string") {
		t.Error("Number should not validate as string type")
	}

	// Test number type
	if !framework.validateType(float64(123), "number") {
		t.Error("Float64 should validate as number type")
	}
	if framework.validateType("123", "number") {
		t.Error("String should not validate as number type")
	}

	// Test integer type
	if !framework.validateType(float64(123), "integer") {
		t.Error("Whole number should validate as integer type")
	}
	if framework.validateType(float64(123.5), "integer") {
		t.Error("Decimal number should not validate as integer type")
	}

	// Test boolean type
	if !framework.validateType(true, "boolean") {
		t.Error("Boolean should validate as boolean type")
	}
	if framework.validateType("true", "boolean") {
		t.Error("String should not validate as boolean type")
	}

	// Test array type
	if !framework.validateType([]interface{}{1, 2, 3}, "array") {
		t.Error("Slice should validate as array type")
	}
	if framework.validateType("array", "array") {
		t.Error("String should not validate as array type")
	}

	// Test object type
	if !framework.validateType(map[string]interface{}{"key": "value"}, "object") {
		t.Error("Map should validate as object type")
	}
	if framework.validateType("object", "object") {
		t.Error("String should not validate as object type")
	}

	// Test null type
	if !framework.validateType(nil, "null") {
		t.Error("Nil should validate as null type")
	}
	if framework.validateType("null", "null") {
		t.Error("String should not validate as null type")
	}
}

func TestValidationSummaryGeneration(t *testing.T) {
	framework := NewContractTestingFramework(&ContractTestConfig{})

	// Create mock validation results
	validations := map[string]*InteractionValidation{
		"interaction1": {
			InteractionID: "interaction1",
			Status:        "passed",
			Checks: map[string]*ValidationCheck{
				"check1": {Status: "passed"},
				"check2": {Status: "passed"},
			},
		},
		"interaction2": {
			InteractionID: "interaction2",
			Status:        "failed",
			Checks: map[string]*ValidationCheck{
				"check1": {Status: "passed"},
				"check2": {Status: "failed"},
			},
		},
	}

	summary := framework.generateValidationSummary(validations)

	if summary.TotalInteractions != 2 {
		t.Errorf("Expected 2 total interactions, got %d", summary.TotalInteractions)
	}

	if summary.ValidInteractions != 1 {
		t.Errorf("Expected 1 valid interaction, got %d", summary.ValidInteractions)
	}

	if summary.InvalidInteractions != 1 {
		t.Errorf("Expected 1 invalid interaction, got %d", summary.InvalidInteractions)
	}

	if summary.TotalChecks != 4 {
		t.Errorf("Expected 4 total checks, got %d", summary.TotalChecks)
	}

	if summary.PassedChecks != 3 {
		t.Errorf("Expected 3 passed checks, got %d", summary.PassedChecks)
	}

	if summary.FailedChecks != 1 {
		t.Errorf("Expected 1 failed check, got %d", summary.FailedChecks)
	}

	expectedSuccessRate := float64(1) / float64(2) * 100
	if summary.SuccessRate != expectedSuccessRate {
		t.Errorf("Expected success rate %.2f, got %.2f", expectedSuccessRate, summary.SuccessRate)
	}
}

func TestContractStatusCalculation(t *testing.T) {
	framework := NewContractTestingFramework(&ContractTestConfig{})

	// Test all passed validations
	allPassedValidations := map[string]*InteractionValidation{
		"interaction1": {Status: "passed"},
		"interaction2": {Status: "passed"},
	}

	status := framework.calculateValidationStatus(allPassedValidations)
	if status != "passed" {
		t.Errorf("Expected passed status, got %s", status)
	}

	// Test mixed validations
	mixedValidations := map[string]*InteractionValidation{
		"interaction1": {Status: "passed"},
		"interaction2": {Status: "failed"},
	}

	status = framework.calculateValidationStatus(mixedValidations)
	if status != "failed" {
		t.Errorf("Expected failed status, got %s", status)
	}

	// Test empty validations
	emptyValidations := map[string]*InteractionValidation{}
	status = framework.calculateValidationStatus(emptyValidations)
	if status != "unknown" {
		t.Errorf("Expected unknown status, got %s", status)
	}
}

func TestInteractionStatusCalculation(t *testing.T) {
	framework := NewContractTestingFramework(&ContractTestConfig{})

	// Test all passed checks
	allPassedChecks := map[string]*ValidationCheck{
		"check1": {Status: "passed"},
		"check2": {Status: "passed"},
	}

	status := framework.calculateInteractionStatus(allPassedChecks)
	if status != "passed" {
		t.Errorf("Expected passed status, got %s", status)
	}

	// Test mixed checks
	mixedChecks := map[string]*ValidationCheck{
		"check1": {Status: "passed"},
		"check2": {Status: "failed"},
	}

	status = framework.calculateInteractionStatus(mixedChecks)
	if status != "failed" {
		t.Errorf("Expected failed status, got %s", status)
	}

	// Test empty checks
	emptyChecks := map[string]*ValidationCheck{}
	status = framework.calculateInteractionStatus(emptyChecks)
	if status != "unknown" {
		t.Errorf("Expected unknown status, got %s", status)
	}
}

func TestContractPerformanceValidation(t *testing.T) {
	framework := NewContractTestingFramework(&ContractTestConfig{})

	// Create a large contract for performance testing
	contract := &ServiceContract{
		ID:           "performance-test-001",
		Name:         "Performance Test Contract",
		Version:      "1.0.0",
		Interactions: make([]ContractInteraction, 100),
	}

	// Generate 100 test interactions
	for i := 0; i < 100; i++ {
		contract.Interactions[i] = ContractInteraction{
			ID:          fmt.Sprintf("interaction-%d", i),
			Description: fmt.Sprintf("Test interaction %d", i),
			Request: &InteractionRequest{
				Method: "GET",
				Path:   fmt.Sprintf("/api/test/%d", i),
				Headers: map[string]string{
					"Accept": "application/json",
				},
			},
			Response: &InteractionResponse{
				Status: 200,
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
				Body: map[string]interface{}{
					"id":   i,
					"data": "test data",
				},
			},
		}
	}

	// Measure validation performance
	start := time.Now()
	ctx := context.Background()
	result, err := framework.ValidateContract(ctx, contract)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Performance validation failed: %v", err)
	}

	if result == nil {
		t.Fatal("Performance validation result is nil")
	}

	// Performance should be under 5 seconds for 100 interactions
	if duration > 5*time.Second {
		t.Errorf("Validation took too long: %v (expected < 5s)", duration)
	}

	// Verify all interactions were validated
	if len(result.Validations) != 100 {
		t.Errorf("Expected 100 validations, got %d", len(result.Validations))
	}

	t.Logf("Performance test completed in %v for %d interactions", duration, len(contract.Interactions))
}

func BenchmarkContractValidation(b *testing.B) {
	framework := NewContractTestingFramework(&ContractTestConfig{})

	contract := &ServiceContract{
		ID:      "benchmark-test-001",
		Name:    "Benchmark Test Contract",
		Version: "1.0.0",
		Interactions: []ContractInteraction{
			{
				ID:          "benchmark-interaction-001",
				Description: "Benchmark test interaction",
				Request: &InteractionRequest{
					Method: "GET",
					Path:   "/api/benchmark",
					Headers: map[string]string{
						"Accept": "application/json",
					},
				},
				Response: &InteractionResponse{
					Status: 200,
					Headers: map[string]string{
						"Content-Type": "application/json",
					},
					Body: map[string]interface{}{
						"status": "success",
						"data":   "benchmark data",
					},
				},
			},
		},
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := framework.ValidateContract(ctx, contract)
		if err != nil {
			b.Fatalf("Benchmark validation failed: %v", err)
		}
	}
}

func BenchmarkSchemaValidation(b *testing.B) {
	framework := NewContractTestingFramework(&ContractTestConfig{})

	schema := &SchemaDefinition{
		Type: "object",
		Properties: map[string]*SchemaProperty{
			"name": {
				Type:      "string",
				MinLength: &[]int{1}[0],
				MaxLength: &[]int{100}[0],
			},
			"age": {
				Type:    "integer",
				Minimum: &[]float64{0}[0],
				Maximum: &[]float64{150}[0],
			},
			"email": {
				Type:    "string",
				Pattern: "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$",
			},
		},
		Required: []string{"name", "email"},
	}

	data := map[string]interface{}{
		"name":  "John Doe",
		"age":   float64(30),
		"email": "john@example.com",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := framework.validateSchema(data, schema)
		if err != nil {
			b.Fatalf("Benchmark schema validation failed: %v", err)
		}
	}
}
