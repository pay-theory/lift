package lift

import (
	"context"
	"encoding/json"
	"time"
)

// Validator interface for request validation
type Validator interface {
	Validate(any) error
}

// Context represents the enhanced context for Lambda handlers
type Context struct {
	context.Context

	// Request/Response cycle
	Request  *Request
	Response *Response

	// Observability
	Logger  Logger
	Metrics MetricsCollector

	// Utilities
	validator Validator
	params    map[string]string
	values    map[string]any

	// Optional database connection
	DB any

	// Lambda-specific
	RequestID string

	// Performance tracking
	startTime time.Time

	// Authentication
	claims          map[string]any
	isAuthenticated bool

	// Response buffering
	responseBuffer   *ResponseBuffer
	bufferingEnabled bool
}

// NewContext creates a new enhanced context
func NewContext(baseCtx context.Context, req *Request) *Context {
	return &Context{
		Context:         baseCtx,
		Request:         req,
		Response:        NewResponse(),
		params:          make(map[string]string),
		values:          make(map[string]any),
		claims:          nil, // Initialize as nil, will be set when claims are provided
		startTime:       time.Now(),
		isAuthenticated: false,
	}
}

// Param retrieves a path parameter
func (c *Context) Param(key string) string {
	return c.params[key]
}

// Query retrieves a query parameter
func (c *Context) Query(key string) string {
	if c.Request == nil || c.Request.QueryParams == nil {
		return ""
	}
	return c.Request.QueryParams[key]
}

// Header retrieves a request header
func (c *Context) Header(key string) string {
	if c.Request == nil || c.Request.Headers == nil {
		return ""
	}
	return c.Request.Headers[key]
}

// Set stores a value in the context
func (c *Context) Set(key string, value any) {
	if c.values == nil {
		c.values = make(map[string]any)
	}
	c.values[key] = value
}

// Get retrieves a value from the context
func (c *Context) Get(key string) any {
	if c.values == nil {
		return nil
	}
	return c.values[key]
}

// UserID retrieves the current user ID from context
func (c *Context) UserID() string {
	if userID, ok := c.values["user_id"].(string); ok {
		return userID
	}
	return ""
}

// TenantID retrieves the current tenant ID from context
func (c *Context) TenantID() string {
	if tenantID, ok := c.values["tenant_id"].(string); ok {
		return tenantID
	}
	return ""
}

// AccountID retrieves the current account ID from context (Partner or Kernel)
func (c *Context) AccountID() string {
	if accountID, ok := c.values["account_id"].(string); ok {
		return accountID
	}
	return ""
}

// SetParam sets a path parameter (used by router)
func (c *Context) SetParam(key, value string) {
	c.params[key] = value
}

// EnableResponseBuffering enables response buffering for this context
// This allows middleware to intercept and access response data after handler execution
func (c *Context) EnableResponseBuffering() {
	if !c.bufferingEnabled {
		c.responseBuffer = NewResponseBuffer()
		c.bufferingEnabled = true
	}
}

// GetResponseBuffer returns the response buffer if buffering is enabled
func (c *Context) GetResponseBuffer() *ResponseBuffer {
	if c.bufferingEnabled {
		return c.responseBuffer
	}
	return nil
}

// FlushResponse is a no-op since buffering just captures data
func (c *Context) FlushResponse() error {
	// No explicit flush needed as we're just capturing data
	return nil
}

// captureResponseData captures response data in the buffer if enabled
func (c *Context) captureResponseData() {
	if c.bufferingEnabled && c.responseBuffer != nil {
		c.responseBuffer.SetBody(c.Response.Body, c.Response.Body)
		c.responseBuffer.SetStatusCode(c.Response.StatusCode)
		for k, v := range c.Response.Headers {
			c.responseBuffer.SetHeader(k, v)
		}
	}
}

// JSON sets the response body as JSON
func (c *Context) JSON(data any) error {
	err := c.Response.JSON(data)
	if err == nil {
		c.captureResponseData()
	}
	return err
}

// Text sends a text response
func (c *Context) Text(text string) error {
	err := c.Response.Text(text)
	if err == nil {
		c.captureResponseData()
	}
	return err
}

// HTML sends an HTML response
func (c *Context) HTML(html string) error {
	err := c.Response.HTML(html)
	if err == nil {
		c.captureResponseData()
	}
	return err
}

// Status sets the response status code
func (c *Context) Status(code int) *Context {
	c.Response.StatusCode = code
	return c
}

// ParseRequest parses the request body into the provided interface
func (c *Context) ParseRequest(v any) error {
	if c.Request == nil || len(c.Request.Body) == 0 {
		return NewLiftError("EMPTY_BODY", "Request body is empty", 400)
	}

	// Parse JSON
	if err := json.Unmarshal(c.Request.Body, v); err != nil {
		return NewLiftError("INVALID_JSON", "Invalid JSON in request body", 400).WithCause(err)
	}

	// Validate if validator is available
	if c.validator != nil {
		if err := c.validator.Validate(v); err != nil {
			return NewLiftError("VALIDATION_ERROR", "Validation failed", 400).WithCause(err)
		}
	}

	return nil
}

// WithTimeout executes a function with a timeout
func (c *Context) WithTimeout(duration time.Duration, fn func() (any, error)) (any, error) {
	ctx, cancel := context.WithTimeout(c.Context, duration)
	defer cancel()

	type result struct {
		value any
		err   error
	}

	ch := make(chan result, 1)
	go func() {
		value, err := fn()
		ch <- result{value, err}
	}()

	select {
	case res := <-ch:
		return res.value, res.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Duration returns the time elapsed since the request started
func (c *Context) Duration() time.Duration {
	return time.Since(c.startTime)
}

// SetValidator sets the validator for request validation
func (c *Context) SetValidator(validator Validator) {
	c.validator = validator
}

// SetRequestID sets the request ID in the context
func (c *Context) SetRequestID(requestID string) {
	c.RequestID = requestID
	c.Set("request_id", requestID)
}

// GetRequestID returns the request ID from the context
func (c *Context) GetRequestID() string {
	if c.RequestID != "" {
		return c.RequestID
	}
	if requestID, ok := c.values["request_id"].(string); ok {
		return requestID
	}
	return ""
}

// SetTenantID sets the tenant ID in the context
func (c *Context) SetTenantID(tenantID string) {
	c.Set("tenant_id", tenantID)
}

// GetTenantID returns the tenant ID from the context
func (c *Context) GetTenantID() string {
	return c.TenantID()
}

// SetUserID sets the user ID in the context
func (c *Context) SetUserID(userID string) {
	c.Set("user_id", userID)
}

// GetUserID returns the user ID from the context
func (c *Context) GetUserID() string {
	return c.UserID()
}

// HTTP Response convenience methods

// OK sends a 200 OK response with JSON data
func (c *Context) OK(data any) error {
	c.Response.StatusCode = 200
	err := c.Response.JSON(data)
	if err == nil {
		c.captureResponseData()
	}
	return err
}

// Created sends a 201 Created response with JSON data
func (c *Context) Created(data any) error {
	c.Response.StatusCode = 201
	err := c.Response.JSON(data)
	if err == nil {
		c.captureResponseData()
	}
	return err
}

// BadRequest sends a 400 Bad Request response
// Deprecated: Use ValidationError instead
func (c *Context) BadRequest(message string, err error) error {
	c.Response.StatusCode = 400
	response := map[string]any{
		"error":   "Bad Request",
		"message": message,
	}
	if err != nil {
		response["details"] = err.Error()
	}
	respErr := c.Response.JSON(response)
	if respErr == nil {
		c.captureResponseData()
	}
	return respErr
}

// NotFound sends a 404 Not Found response
func (c *Context) NotFound(message string, err error) error {
	c.Response.StatusCode = 404
	response := map[string]any{
		"error":   "Not Found",
		"message": message,
	}
	if err != nil {
		response["details"] = err.Error()
	}
	respErr := c.Response.JSON(response)
	if respErr == nil {
		c.captureResponseData()
	}
	return respErr
}

// Forbidden sends a 403 Forbidden response
// Deprecated: Use AuthorizationError from the errors package instead
func (c *Context) Forbidden(message string, err error) error {
	c.Response.StatusCode = 403
	response := map[string]any{
		"error":   "Forbidden",
		"message": message,
	}
	if err != nil {
		response["details"] = err.Error()
	}
	respErr := c.Response.JSON(response)
	if respErr == nil {
		c.captureResponseData()
	}
	return respErr
}

// SystemError sends a 500 Internal Server Error response
func (c *Context) SystemError(message string, err error) error {
	c.Response.StatusCode = 500
	response := map[string]any{
		"error":   "Internal Server Error",
		"message": message,
	}
	if err != nil {
		response["details"] = err.Error()
	}
	respErr := c.Response.JSON(response)
	if respErr == nil {
		c.captureResponseData()
	}
	return respErr
}

// Unauthorized sends a 401 Unauthorized response
func (c *Context) Unauthorized(message string, err error) error {
	c.Response.StatusCode = 401
	response := map[string]any{
		"error":   "Unauthorized",
		"message": message,
	}
	if err != nil {
		response["details"] = err.Error()
	}
	respErr := c.Response.JSON(response)
	if respErr == nil {
		c.captureResponseData()
	}
	return respErr
}

// PathParam retrieves a path parameter (alias for Param)
func (c *Context) PathParam(key string) string {
	return c.Param(key)
}

// QueryParam retrieves a query parameter (alias for Query)
func (c *Context) QueryParam(key string) string {
	return c.Query(key)
}

// Authentication methods

// SetClaims sets JWT claims in the context and extracts user/tenant information
func (c *Context) SetClaims(claims map[string]any) {
	// Initialize claims map if nil
	if c.claims == nil {
		c.claims = make(map[string]any)
	}

	// Copy claims to avoid external modifications
	for key, value := range claims {
		c.claims[key] = value
	}

	c.isAuthenticated = true

	// Extract user_id from claims (prefer user_id, fallback to sub)
	if userID, ok := claims["user_id"].(string); ok && userID != "" {
		c.SetUserID(userID)
	} else if sub, ok := claims["sub"].(string); ok && sub != "" {
		c.SetUserID(sub)
	}

	// Extract tenant_id from claims
	if tenantID, ok := claims["tenant_id"].(string); ok && tenantID != "" {
		c.SetTenantID(tenantID)
	}

	// Extract account_id from claims if present
	if accountID, ok := claims["account_id"].(string); ok && accountID != "" {
		c.Set("account_id", accountID)
	}
}

// Claims returns the JWT claims from the context
func (c *Context) Claims() map[string]any {
	return c.claims
}

// GetClaim retrieves a specific claim from the JWT
func (c *Context) GetClaim(key string) any {
	if c.claims == nil {
		return nil
	}
	return c.claims[key]
}

// IsAuthenticated returns whether the context has valid authentication
func (c *Context) IsAuthenticated() bool {
	return c.isAuthenticated
}
