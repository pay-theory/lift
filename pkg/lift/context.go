package lift

import (
	"context"
	"encoding/json"
	"time"
)

// Validator interface for request validation
type Validator interface {
	Validate(interface{}) error
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
	values    map[string]interface{}

	// Optional database connection
	DB interface{}

	// Lambda-specific
	RequestID string

	// Performance tracking
	startTime time.Time
}

// NewContext creates a new enhanced context
func NewContext(baseCtx context.Context, req *Request) *Context {
	return &Context{
		Context:   baseCtx,
		Request:   req,
		Response:  NewResponse(),
		params:    make(map[string]string),
		values:    make(map[string]interface{}),
		startTime: time.Now(),
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
func (c *Context) Set(key string, value interface{}) {
	if c.values == nil {
		c.values = make(map[string]interface{})
	}
	c.values[key] = value
}

// Get retrieves a value from the context
func (c *Context) Get(key string) interface{} {
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

// JSON sets the response body as JSON
func (c *Context) JSON(data interface{}) error {
	return c.Response.JSON(data)
}

// Text sends a text response
func (c *Context) Text(text string) error {
	return c.Response.Text(text)
}

// HTML sends an HTML response
func (c *Context) HTML(html string) error {
	return c.Response.HTML(html)
}

// Status sets the response status code
func (c *Context) Status(code int) *Context {
	c.Response.StatusCode = code
	return c
}

// ParseRequest parses the request body into the provided interface
func (c *Context) ParseRequest(v interface{}) error {
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
func (c *Context) WithTimeout(duration time.Duration, fn func() (interface{}, error)) (interface{}, error) {
	ctx, cancel := context.WithTimeout(c.Context, duration)
	defer cancel()

	type result struct {
		value interface{}
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
func (c *Context) OK(data interface{}) error {
	c.Response.StatusCode = 200
	return c.JSON(data)
}

// Created sends a 201 Created response with JSON data
func (c *Context) Created(data interface{}) error {
	c.Response.StatusCode = 201
	return c.JSON(data)
}

// BadRequest sends a 400 Bad Request response
func (c *Context) BadRequest(message string, err error) error {
	c.Response.StatusCode = 400
	response := map[string]interface{}{
		"error":   "Bad Request",
		"message": message,
	}
	if err != nil {
		response["details"] = err.Error()
	}
	return c.JSON(response)
}

// NotFound sends a 404 Not Found response
func (c *Context) NotFound(message string, err error) error {
	c.Response.StatusCode = 404
	response := map[string]interface{}{
		"error":   "Not Found",
		"message": message,
	}
	if err != nil {
		response["details"] = err.Error()
	}
	return c.JSON(response)
}

// Forbidden sends a 403 Forbidden response
func (c *Context) Forbidden(message string, err error) error {
	c.Response.StatusCode = 403
	response := map[string]interface{}{
		"error":   "Forbidden",
		"message": message,
	}
	if err != nil {
		response["details"] = err.Error()
	}
	return c.JSON(response)
}

// InternalError sends a 500 Internal Server Error response
func (c *Context) InternalError(message string, err error) error {
	c.Response.StatusCode = 500
	response := map[string]interface{}{
		"error":   "Internal Server Error",
		"message": message,
	}
	if err != nil {
		response["details"] = err.Error()
	}
	return c.JSON(response)
}

// Unauthorized sends a 401 Unauthorized response
func (c *Context) Unauthorized(message string, err error) error {
	c.Response.StatusCode = 401
	response := map[string]interface{}{
		"error":   "Unauthorized",
		"message": message,
	}
	if err != nil {
		response["details"] = err.Error()
	}
	return c.JSON(response)
}

// PathParam retrieves a path parameter (alias for Param)
func (c *Context) PathParam(key string) string {
	return c.Param(key)
}

// QueryParam retrieves a query parameter (alias for Query)
func (c *Context) QueryParam(key string) string {
	return c.Query(key)
}
