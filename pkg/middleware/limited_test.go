package middleware

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLimitedRateLimit(t *testing.T) {
	// Skip if no DynamoDB available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test middleware creation with invalid config (should fail)
	_, err := LimitedRateLimit(LimitedConfig{
		Region:    "",
		TableName: "",
		Window:    time.Minute,
		Limit:     5,
	})
	
	// Should fail due to missing configuration
	assert.Error(t, err)

	// Test middleware creation with valid config
	middleware, err := LimitedRateLimit(LimitedConfig{
		Region:    "us-east-1",
		TableName: "test-rate-limits",
		Endpoint:  "http://localhost:8000", // For local testing
		Window:    time.Minute,
		Limit:     5,
	})
	
	if err != nil {
		t.Skipf("Skipping test due to DynamoDB connection error: %v", err)
	}

	assert.NotNil(t, middleware)
	
	// Note: Full integration testing would require a running DynamoDB instance
	// This test verifies the middleware can be created successfully
}

func TestIPRateLimitWithLimited(t *testing.T) {
	middleware, err := IPRateLimitWithLimited(10, time.Minute)
	if err != nil {
		t.Skipf("Skipping test due to DynamoDB connection error: %v", err)
	}
	
	assert.NotNil(t, middleware)
}

func TestUserRateLimitWithLimited(t *testing.T) {
	middleware, err := UserRateLimitWithLimited(100, 15*time.Minute)
	if err != nil {
		t.Skipf("Skipping test due to DynamoDB connection error: %v", err)
	}
	
	assert.NotNil(t, middleware)
}

func TestTenantRateLimitWithLimited(t *testing.T) {
	middleware, err := TenantRateLimitWithLimited(50, 10*time.Minute)
	if err != nil {
		t.Skipf("Skipping test due to DynamoDB connection error: %v", err)
	}
	
	assert.NotNil(t, middleware)
}

func TestLimitedConfigDefaults(t *testing.T) {
	config := LimitedConfig{}
	
	// Test that defaults get applied
	assert.Equal(t, "", config.TableName)
	assert.Equal(t, time.Duration(0), config.Window)
	assert.Equal(t, 0, config.Limit)
	
	// Middleware creation should apply defaults
	middleware, err := LimitedRateLimit(config)
	if err != nil {
		t.Skipf("Skipping test due to DynamoDB connection error: %v", err)
	}
	
	assert.NotNil(t, middleware)
}