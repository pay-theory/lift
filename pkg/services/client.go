package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
)

// Tracer interface for distributed tracing (placeholder)
type Tracer interface {
	StartSpan(operationName string) interface{}
	FinishSpan(span interface{})
}

// MetricsCollector interface for metrics collection (placeholder)
type MetricsCollector interface {
	Counter(name string, tags map[string]string) Counter
	Histogram(name string, tags map[string]string) Histogram
	Gauge(name string, tags map[string]string) Gauge
	Flush() error
}

// Counter interface for counter metrics
type Counter interface {
	Inc()
}

// Histogram interface for histogram metrics
type Histogram interface {
	Observe(value float64)
}

// Gauge interface for gauge metrics
type Gauge interface {
	Set(value float64)
}

// ServiceClient provides type-safe inter-service communication
type ServiceClient struct {
	registry       *ServiceRegistry
	circuitBreaker CircuitBreaker
	retryPolicy    *RetryPolicy
	tracer         Tracer
	metrics        MetricsCollector
	httpClient     HTTPClient
	config         ServiceClientConfig
	mu             sync.RWMutex
}

// ServiceClientConfig configures the service client
type ServiceClientConfig struct {
	DefaultTimeout       time.Duration `json:"default_timeout"`
	MaxRetries           int           `json:"max_retries"`
	RetryBackoff         time.Duration `json:"retry_backoff"`
	EnableTracing        bool          `json:"enable_tracing"`
	EnableMetrics        bool          `json:"enable_metrics"`
	EnableCircuitBreaker bool          `json:"enable_circuit_breaker"`
	TenantIsolation      bool          `json:"tenant_isolation"`
	UserAgent            string        `json:"user_agent"`
}

// ServiceRequest represents a service call request
type ServiceRequest struct {
	ServiceName         string                 `json:"service_name"`
	Method              string                 `json:"method"`
	Path                string                 `json:"path"`
	Headers             map[string]string      `json:"headers"`
	Body                interface{}            `json:"body"`
	TenantID            string                 `json:"tenant_id,omitempty"`
	UserID              string                 `json:"user_id,omitempty"`
	RequestID           string                 `json:"request_id,omitempty"`
	LoadBalanceStrategy LoadBalanceStrategy    `json:"load_balance_strategy"`
	Timeout             time.Duration          `json:"timeout"`
	Metadata            map[string]interface{} `json:"metadata"`
}

// ServiceResponse represents a service call response
type ServiceResponse struct {
	StatusCode int                    `json:"status_code"`
	Headers    map[string]string      `json:"headers"`
	Body       []byte                 `json:"body"`
	Metadata   map[string]interface{} `json:"metadata"`
	Duration   time.Duration          `json:"duration"`
	Instance   *ServiceInstance       `json:"instance"`
}

// RetryPolicy defines retry behavior
type RetryPolicy struct {
	MaxRetries           int           `json:"max_retries"`
	InitialBackoff       time.Duration `json:"initial_backoff"`
	MaxBackoff           time.Duration `json:"max_backoff"`
	BackoffMultiplier    float64       `json:"backoff_multiplier"`
	RetryableStatusCodes []int         `json:"retryable_status_codes"`
	RetryableErrors      []string      `json:"retryable_errors"`
}

// HTTPClient defines the interface for HTTP operations
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// NewServiceClient creates a new service client
func NewServiceClient(registry *ServiceRegistry, config ServiceClientConfig) *ServiceClient {
	if config.DefaultTimeout == 0 {
		config.DefaultTimeout = 30 * time.Second
	}

	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}

	if config.RetryBackoff == 0 {
		config.RetryBackoff = 1 * time.Second
	}

	if config.UserAgent == "" {
		config.UserAgent = "lift-service-client/1.0"
	}

	client := &ServiceClient{
		registry: registry,
		config:   config,
		httpClient: &http.Client{
			Timeout: config.DefaultTimeout,
		},
		retryPolicy: &RetryPolicy{
			MaxRetries:           config.MaxRetries,
			InitialBackoff:       config.RetryBackoff,
			MaxBackoff:           30 * time.Second,
			BackoffMultiplier:    2.0,
			RetryableStatusCodes: []int{500, 502, 503, 504},
			RetryableErrors:      []string{"timeout", "connection refused", "connection reset"},
		},
	}

	return client
}

// Call makes a type-safe service call with automatic discovery
func (c *ServiceClient) Call(ctx context.Context, request *ServiceRequest) (*ServiceResponse, error) {
	start := time.Now()

	// Set defaults
	if request.LoadBalanceStrategy == "" {
		request.LoadBalanceStrategy = RoundRobin
	}

	if request.Timeout == 0 {
		request.Timeout = c.config.DefaultTimeout
	}

	// Add request ID if not provided
	if request.RequestID == "" {
		request.RequestID = c.generateRequestID()
	}

	// Discover service instance
	instance, err := c.registry.Discover(ctx, request.ServiceName, DiscoveryOptions{
		TenantID: request.TenantID,
		Strategy: request.LoadBalanceStrategy,
	})
	if err != nil {
		c.recordMetrics(request.ServiceName, "discovery_failed", time.Since(start), err)
		return nil, fmt.Errorf("service discovery failed: %w", err)
	}

	// Execute with circuit breaker if enabled
	if c.config.EnableCircuitBreaker && c.circuitBreaker != nil {
		result, err := c.circuitBreaker.Execute(func() (interface{}, error) {
			return c.executeRequest(ctx, instance, request)
		})

		if err != nil {
			c.recordMetrics(request.ServiceName, "circuit_breaker_failed", time.Since(start), err)
			return nil, err
		}

		response := result.(*ServiceResponse)
		c.recordMetrics(request.ServiceName, "success", time.Since(start), nil)
		return response, nil
	}

	// Execute request directly
	response, err := c.executeRequest(ctx, instance, request)
	if err != nil {
		c.recordMetrics(request.ServiceName, "request_failed", time.Since(start), err)
		return nil, err
	}

	c.recordMetrics(request.ServiceName, "success", time.Since(start), nil)
	return response, nil
}

// executeRequest executes the actual HTTP request
func (c *ServiceClient) executeRequest(ctx context.Context, instance *ServiceInstance, request *ServiceRequest) (*ServiceResponse, error) {
	start := time.Now()

	// Build URL
	url := fmt.Sprintf("%s://%s:%d%s",
		instance.Endpoint.Protocol,
		instance.Endpoint.Host,
		instance.Endpoint.Port,
		request.Path,
	)

	// Prepare request body
	var bodyReader io.Reader
	if request.Body != nil {
		bodyBytes, err := json.Marshal(request.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, request.Method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	c.setRequestHeaders(httpReq, request, instance)

	// Execute with retry policy
	var response *ServiceResponse
	err = c.executeWithRetry(ctx, func() error {
		resp, execErr := c.httpClient.Do(httpReq)
		if execErr != nil {
			return execErr
		}
		defer resp.Body.Close()

		// Read response body
		bodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("failed to read response body: %w", readErr)
		}

		// Build service response
		response = &ServiceResponse{
			StatusCode: resp.StatusCode,
			Headers:    make(map[string]string),
			Body:       bodyBytes,
			Duration:   time.Since(start),
			Instance:   instance,
			Metadata:   make(map[string]interface{}),
		}

		// Copy response headers
		for key, values := range resp.Header {
			if len(values) > 0 {
				response.Headers[key] = values[0]
			}
		}

		// Check if response indicates an error that should be retried
		if c.isRetryableStatusCode(resp.StatusCode) {
			return fmt.Errorf("retryable status code: %d", resp.StatusCode)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return response, nil
}

// setRequestHeaders sets standard and custom headers
func (c *ServiceClient) setRequestHeaders(req *http.Request, request *ServiceRequest, instance *ServiceInstance) {
	// Set standard headers
	req.Header.Set("User-Agent", c.config.UserAgent)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Set request ID for tracing
	if request.RequestID != "" {
		req.Header.Set("X-Request-ID", request.RequestID)
	}

	// Set tenant context
	if request.TenantID != "" {
		req.Header.Set("X-Tenant-ID", request.TenantID)
	}

	if request.UserID != "" {
		req.Header.Set("X-User-ID", request.UserID)
	}

	// Set service metadata
	req.Header.Set("X-Source-Service", "lift-client")
	req.Header.Set("X-Target-Service", request.ServiceName)
	req.Header.Set("X-Instance-ID", instance.ID)

	// Set custom headers
	for key, value := range request.Headers {
		req.Header.Set(key, value)
	}

	// Add tracing headers if enabled
	if c.config.EnableTracing && c.tracer != nil {
		c.addTracingHeaders(req, request)
	}
}

// addTracingHeaders adds distributed tracing headers
func (c *ServiceClient) addTracingHeaders(req *http.Request, request *ServiceRequest) {
	// This would integrate with the existing tracing system
	// For now, this is a placeholder
	req.Header.Set("X-Trace-ID", c.generateTraceID())
	req.Header.Set("X-Span-ID", c.generateSpanID())
}

// executeWithRetry executes a function with retry logic
func (c *ServiceClient) executeWithRetry(ctx context.Context, fn func() error) error {
	var lastErr error
	backoff := c.retryPolicy.InitialBackoff

	for attempt := 0; attempt <= c.retryPolicy.MaxRetries; attempt++ {
		// Execute function
		err := fn()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Check if we should retry
		if attempt == c.retryPolicy.MaxRetries || !c.isRetryableError(err) {
			break
		}

		// Wait before retry
		select {
		case <-time.After(backoff):
			// Continue to next attempt
		case <-ctx.Done():
			return ctx.Err()
		}

		// Increase backoff
		backoff = time.Duration(float64(backoff) * c.retryPolicy.BackoffMultiplier)
		if backoff > c.retryPolicy.MaxBackoff {
			backoff = c.retryPolicy.MaxBackoff
		}
	}

	return lastErr
}

// isRetryableStatusCode checks if a status code is retryable
func (c *ServiceClient) isRetryableStatusCode(statusCode int) bool {
	for _, code := range c.retryPolicy.RetryableStatusCodes {
		if statusCode == code {
			return true
		}
	}
	return false
}

// isRetryableError checks if an error is retryable
func (c *ServiceClient) isRetryableError(err error) bool {
	errStr := err.Error()
	for _, retryableErr := range c.retryPolicy.RetryableErrors {
		if contains(errStr, retryableErr) {
			return true
		}
	}
	return false
}

// recordMetrics records service call metrics
func (c *ServiceClient) recordMetrics(serviceName, status string, duration time.Duration, err error) {
	if !c.config.EnableMetrics || c.metrics == nil {
		return
	}

	tags := map[string]string{
		"service": serviceName,
		"status":  status,
	}

	// Record request count
	c.metrics.Counter("service_client.requests", tags).Inc()

	// Record duration
	c.metrics.Histogram("service_client.duration", tags).Observe(float64(duration.Milliseconds()))

	// Record errors
	if err != nil {
		errorTags := map[string]string{
			"service": serviceName,
			"error":   err.Error(),
		}
		c.metrics.Counter("service_client.errors", errorTags).Inc()
	}
}

// generateRequestID generates a unique request ID
func (c *ServiceClient) generateRequestID() string {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}

// generateTraceID generates a trace ID for distributed tracing
func (c *ServiceClient) generateTraceID() string {
	return fmt.Sprintf("trace_%d", time.Now().UnixNano())
}

// generateSpanID generates a span ID for distributed tracing
func (c *ServiceClient) generateSpanID() string {
	return fmt.Sprintf("span_%d", time.Now().UnixNano())
}

// Typed service client interfaces

// UserService defines the interface for user service operations
type UserService interface {
	GetUser(ctx context.Context, userID string) (*User, error)
	CreateUser(ctx context.Context, user *CreateUserRequest) (*User, error)
	UpdateUser(ctx context.Context, userID string, updates *UpdateUserRequest) (*User, error)
	DeleteUser(ctx context.Context, userID string) error
	ListUsers(ctx context.Context, filters *UserFilters) (*UserList, error)
}

// User represents a user entity
type User struct {
	ID        string            `json:"id"`
	Email     string            `json:"email"`
	Name      string            `json:"name"`
	TenantID  string            `json:"tenant_id"`
	Metadata  map[string]string `json:"metadata"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// CreateUserRequest represents a user creation request
type CreateUserRequest struct {
	Email    string            `json:"email"`
	Name     string            `json:"name"`
	TenantID string            `json:"tenant_id"`
	Metadata map[string]string `json:"metadata"`
}

// UpdateUserRequest represents a user update request
type UpdateUserRequest struct {
	Email    *string           `json:"email,omitempty"`
	Name     *string           `json:"name,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// UserFilters represents user listing filters
type UserFilters struct {
	TenantID string `json:"tenant_id,omitempty"`
	Email    string `json:"email,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	Offset   int    `json:"offset,omitempty"`
}

// UserList represents a list of users
type UserList struct {
	Users  []*User `json:"users"`
	Total  int     `json:"total"`
	Limit  int     `json:"limit"`
	Offset int     `json:"offset"`
}

// UserServiceClient implements the UserService interface
type UserServiceClient struct {
	client   *ServiceClient
	tenantID string
}

// NewUserServiceClient creates a new user service client
func NewUserServiceClient(client *ServiceClient, tenantID string) UserService {
	return &UserServiceClient{
		client:   client,
		tenantID: tenantID,
	}
}

// GetUser retrieves a user by ID
func (u *UserServiceClient) GetUser(ctx context.Context, userID string) (*User, error) {
	request := &ServiceRequest{
		ServiceName: "user-service",
		Method:      "GET",
		Path:        fmt.Sprintf("/users/%s", userID),
		TenantID:    u.tenantID,
	}

	response, err := u.client.Call(ctx, request)
	if err != nil {
		return nil, err
	}

	if response.StatusCode == 404 {
		return nil, fmt.Errorf("user not found: %s", userID)
	}

	if response.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}

	var user User
	if err := json.Unmarshal(response.Body, &user); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user: %w", err)
	}

	return &user, nil
}

// CreateUser creates a new user
func (u *UserServiceClient) CreateUser(ctx context.Context, user *CreateUserRequest) (*User, error) {
	request := &ServiceRequest{
		ServiceName: "user-service",
		Method:      "POST",
		Path:        "/users",
		Body:        user,
		TenantID:    u.tenantID,
	}

	response, err := u.client.Call(ctx, request)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != 201 {
		return nil, fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}

	var createdUser User
	if err := json.Unmarshal(response.Body, &createdUser); err != nil {
		return nil, fmt.Errorf("failed to unmarshal created user: %w", err)
	}

	return &createdUser, nil
}

// UpdateUser updates an existing user
func (u *UserServiceClient) UpdateUser(ctx context.Context, userID string, updates *UpdateUserRequest) (*User, error) {
	request := &ServiceRequest{
		ServiceName: "user-service",
		Method:      "PUT",
		Path:        fmt.Sprintf("/users/%s", userID),
		Body:        updates,
		TenantID:    u.tenantID,
	}

	response, err := u.client.Call(ctx, request)
	if err != nil {
		return nil, err
	}

	if response.StatusCode == 404 {
		return nil, fmt.Errorf("user not found: %s", userID)
	}

	if response.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}

	var updatedUser User
	if err := json.Unmarshal(response.Body, &updatedUser); err != nil {
		return nil, fmt.Errorf("failed to unmarshal updated user: %w", err)
	}

	return &updatedUser, nil
}

// DeleteUser deletes a user
func (u *UserServiceClient) DeleteUser(ctx context.Context, userID string) error {
	request := &ServiceRequest{
		ServiceName: "user-service",
		Method:      "DELETE",
		Path:        fmt.Sprintf("/users/%s", userID),
		TenantID:    u.tenantID,
	}

	response, err := u.client.Call(ctx, request)
	if err != nil {
		return err
	}

	if response.StatusCode == 404 {
		return fmt.Errorf("user not found: %s", userID)
	}

	if response.StatusCode != 204 {
		return fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}

	return nil
}

// ListUsers lists users with optional filters
func (u *UserServiceClient) ListUsers(ctx context.Context, filters *UserFilters) (*UserList, error) {
	request := &ServiceRequest{
		ServiceName: "user-service",
		Method:      "GET",
		Path:        "/users",
		TenantID:    u.tenantID,
	}

	// Add query parameters for filters
	if filters != nil {
		// In a real implementation, we'd build query parameters
		// For now, we'll pass filters in the request body
		request.Body = filters
	}

	response, err := u.client.Call(ctx, request)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}

	var userList UserList
	if err := json.Unmarshal(response.Body, &userList); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user list: %w", err)
	}

	return &userList, nil
}

// Utility functions

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}

// ServiceClientMiddleware creates middleware for service client integration
func ServiceClientMiddleware(client *ServiceClient) lift.Middleware {
	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			// Add service client to context
			ctx.Set("service_client", client)
			return next.Handle(ctx)
		})
	}
}

// GetServiceClient retrieves the service client from context
func GetServiceClient(ctx *lift.Context) *ServiceClient {
	if client, ok := ctx.Get("service_client").(*ServiceClient); ok {
		return client
	}
	return nil
}
