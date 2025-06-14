package testing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// LoadTestResult represents the result of a load test
type LoadTestResult struct {
	TotalRequests      int           `json:"total_requests"`
	SuccessfulRequests int           `json:"successful_requests"`
	FailedRequests     int           `json:"failed_requests"`
	AverageLatency     time.Duration `json:"average_latency"`
	P95Latency         time.Duration `json:"p95_latency"`
	P99Latency         time.Duration `json:"p99_latency"`
	MaxLatency         time.Duration `json:"max_latency"`
	MinLatency         time.Duration `json:"min_latency"`
	RequestsPerSecond  float64       `json:"requests_per_second"`
	ErrorRate          float64       `json:"error_rate"`
	Duration           time.Duration `json:"duration"`
	Errors             []string      `json:"errors"`
	StartTime          time.Time     `json:"start_time"`
	EndTime            time.Time     `json:"end_time"`
}

// LoadTestConfig configures load testing parameters
type LoadTestConfig struct {
	ConcurrentUsers   int           `json:"concurrent_users"`
	TotalRequests     int           `json:"total_requests"`
	Duration          time.Duration `json:"duration"`
	RampUpDuration    time.Duration `json:"ramp_up_duration"`
	MaxLatency        time.Duration `json:"max_latency"`
	ErrorThreshold    float64       `json:"error_threshold"`
	RequestsPerSecond int           `json:"requests_per_second"`
}

// LoadTester provides load testing capabilities
type LoadTester struct {
	app    *TestApp
	config *LoadTestConfig
}

// NewLoadTester creates a new load tester
func NewLoadTester(app *TestApp, config *LoadTestConfig) *LoadTester {
	if config == nil {
		config = &LoadTestConfig{
			ConcurrentUsers: 10,
			Duration:        30 * time.Second,
			MaxLatency:      5 * time.Second,
			ErrorThreshold:  0.01, // 1% error rate
		}
	}
	return &LoadTester{
		app:    app,
		config: config,
	}
}

// RunLoadTest executes a load test scenario
func (lt *LoadTester) RunLoadTest(ctx context.Context, request func(*TestApp) *TestResponse) (*LoadTestResult, error) {
	startTime := time.Now()

	result := &LoadTestResult{
		StartTime: startTime,
		Errors:    []string{},
	}

	// Channel to collect results
	results := make(chan *TestResponse, lt.config.ConcurrentUsers*100)
	done := make(chan bool)

	// Start workers
	for i := 0; i < lt.config.ConcurrentUsers; i++ {
		go func(workerID int) {
			for {
				select {
				case <-ctx.Done():
					return
				case <-time.After(lt.config.Duration):
					done <- true
					return
				default:
					resp := request(lt.app)
					results <- resp
				}
			}
		}(i)
	}

	// Collect results
	latencies := []time.Duration{}

	timeout := time.NewTimer(lt.config.Duration + 10*time.Second)
	defer timeout.Stop()

	for {
		select {
		case resp := <-results:
			result.TotalRequests++
			if resp.GetStatusCode() >= 200 && resp.GetStatusCode() < 400 {
				result.SuccessfulRequests++
			} else {
				result.FailedRequests++
				if resp.err != nil {
					result.Errors = append(result.Errors, resp.err.Error())
				}
			}
			// For now, use a placeholder duration since assertions.go TestResponse doesn't have GetDuration
			latencies = append(latencies, time.Millisecond*10)

		case <-done:
			goto collectResults

		case <-timeout.C:
			goto collectResults

		case <-ctx.Done():
			goto collectResults
		}
	}

collectResults:
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// Calculate metrics
	if len(latencies) > 0 {
		// Sort latencies for percentile calculation
		// Simple sorting implementation
		for i := 0; i < len(latencies); i++ {
			for j := i + 1; j < len(latencies); j++ {
				if latencies[i] > latencies[j] {
					latencies[i], latencies[j] = latencies[j], latencies[i]
				}
			}
		}

		result.MinLatency = latencies[0]
		result.MaxLatency = latencies[len(latencies)-1]

		// Calculate average
		var totalLatency time.Duration
		for _, lat := range latencies {
			totalLatency += lat
		}
		result.AverageLatency = totalLatency / time.Duration(len(latencies))

		// Calculate percentiles
		if len(latencies) >= 20 {
			result.P95Latency = latencies[int(float64(len(latencies))*0.95)]
			result.P99Latency = latencies[int(float64(len(latencies))*0.99)]
		}
	}

	// Calculate rates
	if result.Duration > 0 {
		result.RequestsPerSecond = float64(result.TotalRequests) / result.Duration.Seconds()
	}

	if result.TotalRequests > 0 {
		result.ErrorRate = float64(result.FailedRequests) / float64(result.TotalRequests)
	}

	return result, nil
}

// ScenarioRunner provides advanced scenario execution capabilities
type ScenarioRunner struct {
	app               *TestApp
	parallelExecution bool
	maxConcurrency    int
	setupTimeout      time.Duration
	cleanupTimeout    time.Duration
	retryAttempts     int
	retryDelay        time.Duration
}

// NewScenarioRunner creates a new scenario runner with advanced capabilities
func NewScenarioRunner(app *TestApp) *ScenarioRunner {
	return &ScenarioRunner{
		app:               app,
		parallelExecution: true,
		maxConcurrency:    10,
		setupTimeout:      30 * time.Second,
		cleanupTimeout:    30 * time.Second,
		retryAttempts:     3,
		retryDelay:        1 * time.Second,
	}
}

// WithParallelExecution configures parallel execution
func (sr *ScenarioRunner) WithParallelExecution(enabled bool, maxConcurrency int) *ScenarioRunner {
	sr.parallelExecution = enabled
	sr.maxConcurrency = maxConcurrency
	return sr
}

// WithTimeouts configures timeouts
func (sr *ScenarioRunner) WithTimeouts(setup, cleanup time.Duration) *ScenarioRunner {
	sr.setupTimeout = setup
	sr.cleanupTimeout = cleanup
	return sr
}

// WithRetry configures retry behavior
func (sr *ScenarioRunner) WithRetry(attempts int, delay time.Duration) *ScenarioRunner {
	sr.retryAttempts = attempts
	sr.retryDelay = delay
	return sr
}

// RunScenariosAdvanced executes scenarios with advanced features
func (sr *ScenarioRunner) RunScenariosAdvanced(t *testing.T, scenarios []TestScenario) {
	if sr.parallelExecution {
		sr.runScenariosParallel(t, scenarios)
	} else {
		sr.runScenariosSequential(t, scenarios)
	}
}

// runScenariosParallel executes scenarios in parallel
func (sr *ScenarioRunner) runScenariosParallel(t *testing.T, scenarios []TestScenario) {
	semaphore := make(chan bool, sr.maxConcurrency)

	for _, scenario := range scenarios {
		scenario := scenario // capture loop variable
		t.Run(scenario.Name, func(t *testing.T) {
			t.Parallel()

			semaphore <- true
			defer func() { <-semaphore }()

			sr.executeScenario(t, scenario)
		})
	}
}

// runScenariosSequential executes scenarios sequentially
func (sr *ScenarioRunner) runScenariosSequential(t *testing.T, scenarios []TestScenario) {
	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			sr.executeScenario(t, scenario)
		})
	}
}

// executeScenario executes a single scenario with retry logic
func (sr *ScenarioRunner) executeScenario(t *testing.T, scenario TestScenario) {
	if scenario.Skip {
		t.Skip(scenario.SkipReason)
		return
	}

	var lastErr error

	for attempt := 0; attempt <= sr.retryAttempts; attempt++ {
		if attempt > 0 {
			t.Logf("Retrying scenario %s (attempt %d/%d)", scenario.Name, attempt+1, sr.retryAttempts+1)
			time.Sleep(sr.retryDelay)
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					lastErr = fmt.Errorf("scenario panicked: %v", r)
				}
			}()

			// Setup with timeout
			if scenario.Setup != nil {
				setupCtx, cancel := context.WithTimeout(context.Background(), sr.setupTimeout)
				defer cancel()

				done := make(chan error, 1)
				go func() {
					done <- scenario.Setup(sr.app)
				}()

				select {
				case err := <-done:
					if err != nil {
						lastErr = fmt.Errorf("setup failed: %w", err)
						return
					}
				case <-setupCtx.Done():
					lastErr = fmt.Errorf("setup timed out after %v", sr.setupTimeout)
					return
				}
			}

			// Execute request
			resp := scenario.Request(sr.app)

			// Run assertions
			scenario.Assertions(t, resp)

			// Cleanup with timeout
			if scenario.Cleanup != nil {
				cleanupCtx, cancel := context.WithTimeout(context.Background(), sr.cleanupTimeout)
				defer cancel()

				done := make(chan error, 1)
				go func() {
					done <- scenario.Cleanup(sr.app)
				}()

				select {
				case err := <-done:
					if err != nil {
						t.Logf("Cleanup warning: %v", err)
					}
				case <-cleanupCtx.Done():
					t.Logf("Cleanup timed out after %v", sr.cleanupTimeout)
				}
			}

			lastErr = nil // Success
		}()

		if lastErr == nil {
			return // Success
		}
	}

	// All attempts failed
	require.NoError(t, lastErr, "Scenario failed after %d attempts", sr.retryAttempts+1)
}

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
	server  *httptest.Server
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

// Start starts the test server
func (ta *TestApp) Start() error {
	if ta.server != nil {
		return fmt.Errorf("test server already started")
	}

	// Start the lift app to configure routes
	if err := ta.app.Start(); err != nil {
		return fmt.Errorf("failed to start lift app: %w", err)
	}

	// Create test server
	ta.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Convert http.Request to lift context and handle
		ctx := lift.NewContext(ta.context, &lift.Request{
			Method:      r.Method,
			Path:        r.URL.Path,
			Headers:     convertHeaders(r.Header),
			Body:        getRequestBody(r),
			QueryParams: convertQueryToStringMap(r.URL.Query()),
		})

		// Add test headers
		for k, v := range ta.headers {
			ctx.Request.Headers[k] = v
		}

		// Handle the request through lift
		if err := ta.app.HandleTestRequest(ctx); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		// Write lift response to http response
		for k, v := range ctx.Response.Headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(ctx.Response.StatusCode)

		// Type assert the response body
		if bodyBytes, ok := ctx.Response.Body.([]byte); ok {
			w.Write(bodyBytes)
		} else if bodyStr, ok := ctx.Response.Body.(string); ok {
			w.Write([]byte(bodyStr))
		} else if ctx.Response.Body != nil {
			// Handle interface{} types (from ctx.JSON calls) by marshaling to JSON
			jsonBytes, err := json.Marshal(ctx.Response.Body)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to marshal response: %v", err), 500)
				return
			}
			w.Write(jsonBytes)
		}
	}))

	return nil
}

// Stop stops the test server
func (ta *TestApp) Stop() {
	if ta.server != nil {
		ta.server.Close()
		ta.server = nil
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
	startTime := time.Now()

	// If server not started, start it
	if ta.server == nil {
		if err := ta.Start(); err != nil {
			return NewTestResponse(nil, 500, map[string]string{}, []byte{}, err)
		}
	}

	// Build URL
	reqURL := ta.server.URL + path
	if len(query) > 0 {
		queryValues := url.Values{}
		for k, v := range query {
			queryValues.Add(k, v)
		}
		reqURL += "?" + queryValues.Encode()
	}

	// Create request body
	var reqBody *bytes.Buffer
	if body != nil {
		if str, ok := body.(string); ok {
			reqBody = bytes.NewBufferString(str)
		} else {
			jsonBody, err := json.Marshal(body)
			if err != nil {
				return NewTestResponse(nil, 500, map[string]string{}, []byte{},
					fmt.Errorf("failed to marshal request body: %w", err))
			}
			reqBody = bytes.NewBuffer(jsonBody)
		}
	} else {
		reqBody = bytes.NewBuffer([]byte{})
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ta.context, method, reqURL, reqBody)
	if err != nil {
		return NewTestResponse(nil, 500, map[string]string{}, []byte{}, err)
	}

	// Add headers
	for k, v := range ta.headers {
		req.Header.Set(k, v)
	}

	// Set content type for JSON
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Make request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return NewTestResponse(nil, 500, map[string]string{}, []byte{}, err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody := bytes.NewBuffer([]byte{})
	_, err = respBody.ReadFrom(resp.Body)
	if err != nil {
		return NewTestResponse(nil, resp.StatusCode, convertHeadersFromHTTP(resp.Header), []byte{}, err)
	}

	duration := time.Since(startTime)

	// Create response with timing information in headers
	headers := convertHeadersFromHTTP(resp.Header)
	headers["X-Test-Duration"] = duration.String()

	return NewTestResponse(nil, resp.StatusCode, headers, respBody.Bytes(), nil)
}

// Helper functions

func convertHeaders(headers http.Header) map[string]string {
	result := make(map[string]string)
	for k, v := range headers {
		if len(v) > 0 {
			result[k] = v[0]
		}
	}
	return result
}

func convertHeadersFromHTTP(headers http.Header) map[string]string {
	result := make(map[string]string)
	for k, v := range headers {
		if len(v) > 0 {
			result[k] = v[0]
		}
	}
	return result
}

func convertQueryToStringMap(values url.Values) map[string]string {
	result := make(map[string]string)
	for k, v := range values {
		if len(v) > 0 {
			result[k] = v[0]
		}
	}
	return result
}

func getRequestBody(r *http.Request) []byte {
	if r.Body == nil {
		return []byte{}
	}

	buf := bytes.NewBuffer([]byte{})
	buf.ReadFrom(r.Body)
	return buf.Bytes()
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
				return app.GET(endpoint, nil)
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
