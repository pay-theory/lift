package testing

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScenario represents a complete test scenario
type TestScenario struct {
	Name        string
	Description string
	Setup       func(*TestApp) error
	Request     func(*TestApp) *TestResponse
	Assertions  func(*testing.T, *TestResponse)
	Cleanup     func(*TestApp) error
	Skip        bool
	SkipReason  string
}

// TestApp represents a test application instance
type TestApp struct {
	app     *lift.App
	context context.Context
	headers map[string]string
	auth    *AuthConfig
}

// AuthConfig holds authentication configuration for tests
type AuthConfig struct {
	Token    string
	TenantID string
	UserID   string
	Scopes   []string
}

// NewTestApp creates a new test application with a default Lift app
func NewTestApp() *TestApp {
	app := lift.New()
	return &TestApp{
		app:     app,
		context: context.Background(),
		headers: make(map[string]string),
	}
}

// App returns the underlying Lift application for configuration
func (ta *TestApp) App() *lift.App {
	return ta.app
}

// WithAuth sets authentication for subsequent requests
func (ta *TestApp) WithAuth(auth *AuthConfig) *TestApp {
	ta.auth = auth
	if auth.Token != "" {
		ta.headers["Authorization"] = "Bearer " + auth.Token
	}
	if auth.TenantID != "" {
		ta.headers["X-Tenant-ID"] = auth.TenantID
	}
	if auth.UserID != "" {
		ta.headers["X-User-ID"] = auth.UserID
	}
	return ta
}

// WithHeader adds a header for subsequent requests
func (ta *TestApp) WithHeader(key, value string) *TestApp {
	ta.headers[key] = value
	return ta
}

// WithHeaders adds multiple headers for subsequent requests
func (ta *TestApp) WithHeaders(headers map[string]string) *TestApp {
	for k, v := range headers {
		ta.headers[k] = v
	}
	return ta
}

// ClearHeaders clears all headers
func (ta *TestApp) ClearHeaders() *TestApp {
	ta.headers = make(map[string]string)
	ta.auth = nil
	return ta
}

// GET performs a GET request with optional query parameters
func (ta *TestApp) GET(path string, query ...map[string]string) *TestResponse {
	var queryParams map[string]string
	if len(query) > 0 {
		queryParams = query[0]
	}
	return ta.request("GET", path, nil, queryParams)
}

// POST performs a POST request
func (ta *TestApp) POST(path string, body interface{}) *TestResponse {
	return ta.request("POST", path, body, nil)
}

// PUT performs a PUT request
func (ta *TestApp) PUT(path string, body interface{}) *TestResponse {
	return ta.request("PUT", path, body, nil)
}

// PATCH performs a PATCH request
func (ta *TestApp) PATCH(path string, body interface{}) *TestResponse {
	return ta.request("PATCH", path, body, nil)
}

// DELETE performs a DELETE request
func (ta *TestApp) DELETE(path string) *TestResponse {
	return ta.request("DELETE", path, nil, nil)
}

// request is the internal method for making requests
func (ta *TestApp) request(method, path string, body interface{}, query map[string]string) *TestResponse {
	// This is a placeholder implementation
	// In a real implementation, this would invoke the Lift app with the request
	// For now, we'll return a mock response
	return &TestResponse{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       `{"status": "ok"}`,
		err:        nil,
	}
}

// RunScenarios executes a collection of test scenarios
func RunScenarios(t *testing.T, app *TestApp, scenarios []TestScenario) {
	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			if scenario.Skip {
				t.Skip(scenario.SkipReason)
				return
			}

			// Setup
			if scenario.Setup != nil {
				err := scenario.Setup(app)
				require.NoError(t, err, "Scenario setup failed")
			}

			// Execute request
			resp := scenario.Request(app)

			// Run assertions
			scenario.Assertions(t, resp)

			// Cleanup
			if scenario.Cleanup != nil {
				err := scenario.Cleanup(app)
				assert.NoError(t, err, "Scenario cleanup failed")
			}
		})
	}
}

// Rate Limiting Test Scenarios

// RateLimitingScenarios returns common rate limiting test scenarios
func RateLimitingScenarios(endpoint string, limit int) []TestScenario {
	return []TestScenario{
		{
			Name:        "requests_within_limit_allowed",
			Description: "Requests within rate limit should be allowed",
			Request: func(app *TestApp) *TestResponse {
				// Make requests up to limit - 1
				for i := 0; i < limit-1; i++ {
					resp := app.GET(endpoint, nil)
					if resp.GetStatusCode() != 200 {
						return resp
					}
				}
				// Return the last successful request
				return app.GET(endpoint, nil)
			},
			Assertions: func(t *testing.T, resp *TestResponse) {
				resp.AssertStatus(200)
				resp.AssertRateLimitHeaders()
				resp.AssertRateLimitRemaining(0)
			},
		},
		{
			Name:        "requests_exceeding_limit_blocked",
			Description: "Requests exceeding rate limit should be blocked",
			Setup: func(app *TestApp) error {
				// Exhaust the rate limit
				for i := 0; i < limit; i++ {
					app.GET(endpoint, nil)
				}
				return nil
			},
			Request: func(app *TestApp) *TestResponse {
				return app.GET(endpoint, nil)
			},
			Assertions: func(t *testing.T, resp *TestResponse) {
				resp.AssertRateLimitExceeded()
			},
		},
		{
			Name:        "rate_limit_headers_present",
			Description: "Rate limit headers should be present in all responses",
			Request: func(app *TestApp) *TestResponse {
				return app.GET(endpoint, nil)
			},
			Assertions: func(t *testing.T, resp *TestResponse) {
				resp.AssertRateLimitHeaders()
				resp.AssertRateLimitLimit(limit)
			},
		},
	}
}

// TestRateLimiting is a helper function for testing rate limiting
func TestRateLimiting(t *testing.T, app *TestApp, endpoint string, limit int) {
	scenarios := RateLimitingScenarios(endpoint, limit)
	RunScenarios(t, app, scenarios)
}

// Multi-tenant Test Scenarios

// MultiTenantScenarios returns common multi-tenant test scenarios
func MultiTenantScenarios(endpoint string) []TestScenario {
	return []TestScenario{
		{
			Name:        "tenant_isolation_enforced",
			Description: "Data should be isolated between tenants",
			Request: func(app *TestApp) *TestResponse {
				return app.WithAuth(&AuthConfig{
					TenantID: "tenant-1",
					Token:    "valid-token",
				}).GET(endpoint, nil)
			},
			Assertions: func(t *testing.T, resp *TestResponse) {
				resp.AssertStatus(200)
				resp.AssertTenantIsolation("tenant-1")
			},
		},
		{
			Name:        "missing_tenant_rejected",
			Description: "Requests without tenant ID should be rejected",
			Request: func(app *TestApp) *TestResponse {
				return app.WithAuth(&AuthConfig{
					Token: "valid-token",
					// No TenantID
				}).GET(endpoint, nil)
			},
			Assertions: func(t *testing.T, resp *TestResponse) {
				resp.AssertStatus(400)
				resp.AssertJSONPath("$.error", "Tenant ID required")
			},
		},
		{
			Name:        "cross_tenant_access_blocked",
			Description: "Users should not access other tenant's data",
			Setup: func(app *TestApp) error {
				// Create data for tenant-1
				app.WithAuth(&AuthConfig{
					TenantID: "tenant-1",
					Token:    "valid-token",
				}).POST(endpoint, map[string]interface{}{
					"name": "tenant-1-data",
				})
				return nil
			},
			Request: func(app *TestApp) *TestResponse {
				// Try to access as tenant-2
				return app.WithAuth(&AuthConfig{
					TenantID: "tenant-2",
					Token:    "valid-token",
				}).GET(endpoint, nil)
			},
			Assertions: func(t *testing.T, resp *TestResponse) {
				resp.AssertStatus(200)
				// Should return empty results, not tenant-1's data
				resp.AssertJSONPathCount("$.data", 0)
			},
		},
	}
}

// Authentication Test Scenarios

// AuthenticationScenarios returns common authentication test scenarios
func AuthenticationScenarios(endpoint string) []TestScenario {
	return []TestScenario{
		{
			Name:        "valid_token_accepted",
			Description: "Valid authentication token should be accepted",
			Request: func(app *TestApp) *TestResponse {
				return app.WithAuth(&AuthConfig{
					Token:    "valid-token",
					TenantID: "test-tenant",
				}).GET(endpoint, nil)
			},
			Assertions: func(t *testing.T, resp *TestResponse) {
				resp.AssertStatus(200)
			},
		},
		{
			Name:        "missing_token_rejected",
			Description: "Requests without authentication should be rejected",
			Request: func(app *TestApp) *TestResponse {
				return app.ClearHeaders().GET(endpoint, nil)
			},
			Assertions: func(t *testing.T, resp *TestResponse) {
				resp.AssertUnauthorized()
			},
		},
		{
			Name:        "invalid_token_rejected",
			Description: "Invalid authentication token should be rejected",
			Request: func(app *TestApp) *TestResponse {
				return app.WithAuth(&AuthConfig{
					Token: "invalid-token",
				}).GET(endpoint, nil)
			},
			Assertions: func(t *testing.T, resp *TestResponse) {
				resp.AssertUnauthorized()
			},
		},
		{
			Name:        "expired_token_rejected",
			Description: "Expired authentication token should be rejected",
			Request: func(app *TestApp) *TestResponse {
				return app.WithAuth(&AuthConfig{
					Token: "expired-token",
				}).GET(endpoint, nil)
			},
			Assertions: func(t *testing.T, resp *TestResponse) {
				resp.AssertUnauthorized()
			},
		},
	}
}

// CRUD Test Scenarios

// CRUDScenarios returns common CRUD operation test scenarios
func CRUDScenarios(basePath string, createData, updateData map[string]interface{}) []TestScenario {
	var createdID string

	return []TestScenario{
		{
			Name:        "create_resource",
			Description: "Should create a new resource",
			Request: func(app *TestApp) *TestResponse {
				return app.POST(basePath, createData)
			},
			Assertions: func(t *testing.T, resp *TestResponse) {
				resp.AssertStatus(201)
				resp.AssertJSONPathExists("$.id")
				// Store the created ID for subsequent tests
				createdID = resp.GetJSONPath("$.id").(string)
			},
		},
		{
			Name:        "read_resource",
			Description: "Should read the created resource",
			Request: func(app *TestApp) *TestResponse {
				return app.GET(fmt.Sprintf("%s/%s", basePath, createdID), nil)
			},
			Assertions: func(t *testing.T, resp *TestResponse) {
				resp.AssertStatus(200)
				resp.AssertJSONPath("$.id", createdID)
			},
		},
		{
			Name:        "update_resource",
			Description: "Should update the created resource",
			Request: func(app *TestApp) *TestResponse {
				return app.PUT(fmt.Sprintf("%s/%s", basePath, createdID), updateData)
			},
			Assertions: func(t *testing.T, resp *TestResponse) {
				resp.AssertStatus(200)
				resp.AssertJSONPath("$.id", createdID)
				// Verify update data
				for key, value := range updateData {
					resp.AssertJSONPath(fmt.Sprintf("$.%s", key), value)
				}
			},
		},
		{
			Name:        "list_resources",
			Description: "Should list resources including the created one",
			Request: func(app *TestApp) *TestResponse {
				return app.GET(basePath, nil)
			},
			Assertions: func(t *testing.T, resp *TestResponse) {
				resp.AssertStatus(200)
				resp.AssertJSONPathExists("$.data")
				// Should contain at least one item
				resp.AssertJSONPathCount("$.data", 1)
			},
		},
		{
			Name:        "delete_resource",
			Description: "Should delete the created resource",
			Request: func(app *TestApp) *TestResponse {
				return app.DELETE(fmt.Sprintf("%s/%s", basePath, createdID))
			},
			Assertions: func(t *testing.T, resp *TestResponse) {
				resp.AssertStatus(204)
			},
		},
		{
			Name:        "read_deleted_resource_fails",
			Description: "Should not find the deleted resource",
			Request: func(app *TestApp) *TestResponse {
				return app.GET(fmt.Sprintf("%s/%s", basePath, createdID), nil)
			},
			Assertions: func(t *testing.T, resp *TestResponse) {
				resp.AssertStatus(404)
			},
		},
	}
}

// Validation Test Scenarios

// ValidationScenarios returns common validation test scenarios
func ValidationScenarios(endpoint string, invalidData map[string]interface{}, expectedErrors []string) []TestScenario {
	return []TestScenario{
		{
			Name:        "validation_errors_returned",
			Description: "Invalid data should return validation errors",
			Request: func(app *TestApp) *TestResponse {
				return app.POST(endpoint, invalidData)
			},
			Assertions: func(t *testing.T, resp *TestResponse) {
				resp.AssertValidationErrors(expectedErrors)
			},
		},
		{
			Name:        "empty_body_rejected",
			Description: "Empty request body should be rejected",
			Request: func(app *TestApp) *TestResponse {
				return app.POST(endpoint, nil)
			},
			Assertions: func(t *testing.T, resp *TestResponse) {
				resp.AssertStatus(400)
			},
		},
		{
			Name:        "malformed_json_rejected",
			Description: "Malformed JSON should be rejected",
			Request: func(app *TestApp) *TestResponse {
				// This would need to be implemented to send raw malformed JSON
				return app.POST(endpoint, "invalid-json")
			},
			Assertions: func(t *testing.T, resp *TestResponse) {
				resp.AssertStatus(400)
				resp.AssertJSONPath("$.error", "Invalid JSON")
			},
		},
	}
}

// Performance Test Scenarios

// PerformanceScenarios returns performance-related test scenarios
func PerformanceScenarios(endpoint string, maxResponseTime time.Duration) []TestScenario {
	return []TestScenario{
		{
			Name:        "response_time_acceptable",
			Description: "Response time should be within acceptable limits",
			Request: func(app *TestApp) *TestResponse {
				start := time.Now()
				resp := app.GET(endpoint, nil)
				duration := time.Since(start)

				// Add timing to headers for assertion
				if resp.Headers == nil {
					resp.Headers = make(map[string]string)
				}
				resp.Headers["X-Test-Duration"] = duration.String()

				return resp
			},
			Assertions: func(t *testing.T, resp *TestResponse) {
				resp.AssertStatus(200)
				durationStr := resp.GetHeader("X-Test-Duration")
				if durationStr != "" {
					duration, err := time.ParseDuration(durationStr)
					require.NoError(t, err)
					assert.True(t, duration <= maxResponseTime,
						"Response time %v exceeded maximum %v", duration, maxResponseTime)
				}
			},
		},
	}
}

// Pagination Test Scenarios

// PaginationScenarios returns pagination test scenarios
func PaginationScenarios(endpoint string, totalItems int) []TestScenario {
	return []TestScenario{
		{
			Name:        "first_page_returned",
			Description: "First page should be returned by default",
			Request: func(app *TestApp) *TestResponse {
				return app.GET(endpoint, nil)
			},
			Assertions: func(t *testing.T, resp *TestResponse) {
				resp.AssertStatus(200)
				resp.AssertPagination(totalItems, 1, 10) // Assuming default page size of 10
			},
		},
		{
			Name:        "custom_page_size_respected",
			Description: "Custom page size should be respected",
			Request: func(app *TestApp) *TestResponse {
				return app.GET(endpoint, map[string]string{
					"per_page": "5",
				})
			},
			Assertions: func(t *testing.T, resp *TestResponse) {
				resp.AssertStatus(200)
				resp.AssertJSONPath("$.pagination.per_page", 5)
				resp.AssertJSONPathCount("$.data", 5)
			},
		},
		{
			Name:        "next_page_navigation",
			Description: "Should be able to navigate to next page",
			Request: func(app *TestApp) *TestResponse {
				return app.GET(endpoint, map[string]string{
					"page":     "2",
					"per_page": "5",
				})
			},
			Assertions: func(t *testing.T, resp *TestResponse) {
				resp.AssertStatus(200)
				resp.AssertJSONPath("$.pagination.page", 2)
				resp.AssertHasPrevPage()
			},
		},
	}
}

// Error Handling Test Scenarios

// ErrorHandlingScenarios returns error handling test scenarios
func ErrorHandlingScenarios() []TestScenario {
	return []TestScenario{
		{
			Name:        "not_found_error",
			Description: "Non-existent resource should return 404",
			Request: func(app *TestApp) *TestResponse {
				return app.GET("/api/nonexistent/12345", nil)
			},
			Assertions: func(t *testing.T, resp *TestResponse) {
				resp.AssertStatus(404)
				resp.AssertJSONPath("$.error", "Not found")
			},
		},
		{
			Name:        "method_not_allowed",
			Description: "Unsupported HTTP method should return 405",
			Request: func(app *TestApp) *TestResponse {
				return app.PATCH("/api/readonly-endpoint", map[string]interface{}{})
			},
			Assertions: func(t *testing.T, resp *TestResponse) {
				resp.AssertStatus(405)
				resp.AssertHeaderExists("Allow")
			},
		},
	}
}

// Utility Functions

// CreateTestData creates test data for scenarios
func CreateTestData(app *TestApp, endpoint string, data map[string]interface{}) (string, error) {
	resp := app.POST(endpoint, data)
	if resp.GetStatusCode() != 201 {
		return "", fmt.Errorf("failed to create test data: status %d", resp.GetStatusCode())
	}

	id := resp.GetJSONPath("$.id")
	if id == nil {
		return "", fmt.Errorf("created resource has no ID")
	}

	return id.(string), nil
}

// CleanupTestData removes test data
func CleanupTestData(app *TestApp, endpoint, id string) error {
	resp := app.DELETE(fmt.Sprintf("%s/%s", endpoint, id))
	if resp.GetStatusCode() != 204 && resp.GetStatusCode() != 404 {
		return fmt.Errorf("failed to cleanup test data: status %d", resp.GetStatusCode())
	}
	return nil
}
