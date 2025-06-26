package context

import (
	"context"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/observability"
)

// Context wraps lift.Context with additional convenience methods
type Context struct {
	*lift.Context
}

// NewContext creates a new context wrapper
func NewContext(ctx *lift.Context) *Context {
	return &Context{Context: ctx}
}

// GoContext returns the underlying Go context
func (c *Context) GoContext() context.Context {
	return c.Context.Context
}

// TenantID returns the tenant ID from the context
func (c *Context) TenantID() string {
	if tenantID, ok := c.Get("tenant_id").(string); ok {
		return tenantID
	}
	return c.Header("X-Tenant-ID")
}

// UserID returns the user ID from the context
func (c *Context) UserID() string {
	if userID, ok := c.Get("user_id").(string); ok {
		return userID
	}
	return c.Header("X-User-ID")
}

// Logger returns a structured logger
func (c *Context) Logger() observability.StructuredLogger {
	// Return a default logger if none is set
	// This would typically be injected via middleware
	if logger, ok := c.Get("logger").(observability.StructuredLogger); ok {
		return logger
	}
	// Return a no-op logger as fallback
	return &noOpLogger{}
}

// PathParam returns a path parameter value
func (c *Context) PathParam(key string) string {
	return c.Param(key)
}

// QueryParam returns a query parameter value with optional default
func (c *Context) QueryParam(key string, defaultValue ...string) string {
	value := c.Query(key)
	if value == "" && len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return value
}

// ParseJSON parses the request body as JSON
func (c *Context) ParseJSON(target any) error {
	return c.ParseRequest(target)
}

// noOpLogger is a fallback logger that does nothing
type noOpLogger struct{}

func (l *noOpLogger) Debug(msg string, fields ...map[string]any) {}
func (l *noOpLogger) Info(msg string, fields ...map[string]any)  {}
func (l *noOpLogger) Warn(msg string, fields ...map[string]any)  {}
func (l *noOpLogger) Error(msg string, fields ...map[string]any) {}
func (l *noOpLogger) Fatal(msg string, fields ...map[string]any) {}

// lift.Logger interface methods
func (l *noOpLogger) WithField(key string, value any) lift.Logger  { return l }
func (l *noOpLogger) WithFields(fields map[string]any) lift.Logger { return l }

// StructuredLogger interface methods
func (l *noOpLogger) WithRequestID(requestID string) observability.StructuredLogger { return l }
func (l *noOpLogger) WithTenantID(tenantID string) observability.StructuredLogger   { return l }
func (l *noOpLogger) WithUserID(userID string) observability.StructuredLogger       { return l }
func (l *noOpLogger) WithTraceID(traceID string) observability.StructuredLogger     { return l }
func (l *noOpLogger) WithSpanID(spanID string) observability.StructuredLogger       { return l }

func (l *noOpLogger) Flush(ctx context.Context) error     { return nil }
func (l *noOpLogger) Close() error                        { return nil }
func (l *noOpLogger) IsHealthy() bool                     { return true }
func (l *noOpLogger) GetStats() observability.LoggerStats { return observability.LoggerStats{} }
