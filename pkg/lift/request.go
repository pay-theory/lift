package lift

import (
	"context"
	"strings"

	"github.com/pay-theory/lift/pkg/lift/adapters"
)

// Re-export types from adapters for backward compatibility
type TriggerType = adapters.TriggerType

// Request wraps the adapter Request with additional methods and exposes fields directly
type Request struct {
	*adapters.Request

	// ctx holds the Lambda context for proper cancellation and timeout propagation
	ctx context.Context

	// Expose adapter fields directly for backward compatibility
	Method      string            `json:"method,omitempty"`
	Path        string            `json:"path,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	QueryParams map[string]string `json:"query_params,omitempty"`
	Body        []byte            `json:"body,omitempty"`
}

// NewRequest creates a new Request from an adapter Request
func NewRequest(adapterReq *adapters.Request) *Request {
	if adapterReq == nil {
		return &Request{
			Request:     &adapters.Request{},
			ctx:         context.Background(),
			Headers:     make(map[string]string),
			QueryParams: make(map[string]string),
		}
	}

	return &Request{
		Request:     adapterReq,
		ctx:         context.Background(),
		Method:      adapterReq.Method,
		Path:        adapterReq.Path,
		Headers:     adapterReq.Headers,
		QueryParams: adapterReq.QueryParams,
		Body:        adapterReq.Body,
	}
}

// NewRequestWithContext creates a new Request from an adapter Request with the provided context.
// This is the preferred constructor when you have access to the Lambda context.
func NewRequestWithContext(ctx context.Context, adapterReq *adapters.Request) *Request {
	if ctx == nil {
		ctx = context.Background()
	}

	req := NewRequest(adapterReq)
	req.ctx = ctx
	return req
}

// SetContext sets the context for the request.
// This should be called by the app to propagate the Lambda context.
func (r *Request) SetContext(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	r.ctx = ctx
}

// Re-export constants from adapters
const (
	TriggerAPIGateway   = adapters.TriggerAPIGateway
	TriggerAPIGatewayV2 = adapters.TriggerAPIGatewayV2
	TriggerSQS          = adapters.TriggerSQS
	TriggerS3           = adapters.TriggerS3
	TriggerEventBridge  = adapters.TriggerEventBridge
	TriggerWebSocket    = adapters.TriggerWebSocket
	TriggerUnknown      = adapters.TriggerUnknown
)

// RequestContext provides backward compatibility for accessing request context
func (r *Request) RequestContext() map[string]any {
	if r.RawEvent == nil {
		return make(map[string]any)
	}

	// Try to extract request context from raw event
	if eventMap, ok := r.RawEvent.(map[string]any); ok {
		if requestContext, ok := eventMap["requestContext"].(map[string]any); ok {
			return requestContext
		}
	}

	return make(map[string]any)
}

// GetHeader retrieves a header value (case-insensitive)
func (r *Request) GetHeader(key string) string {
	if r.Headers == nil {
		return ""
	}

	// Try exact match first
	if value, exists := r.Headers[key]; exists {
		return value
	}

	// Try case-insensitive match
	for k, v := range r.Headers {
		if equalFold(k, key) {
			return v
		}
	}

	return ""
}

// GetQuery retrieves a query parameter value
func (r *Request) GetQuery(key string) string {
	if r.QueryParams == nil {
		return ""
	}
	return r.QueryParams[key]
}

// GetParam retrieves a path parameter value
func (r *Request) GetParam(key string) string {
	if r.PathParams == nil {
		return ""
	}
	return r.PathParams[key]
}

// equalFold is a simple case-insensitive string comparison
func equalFold(s1, s2 string) bool {
	if len(s1) != len(s2) {
		return false
	}
	for i := 0; i < len(s1); i++ {
		c1, c2 := s1[i], s2[i]
		if c1 >= 'A' && c1 <= 'Z' {
			c1 += 'a' - 'A'
		}
		if c2 >= 'A' && c2 <= 'Z' {
			c2 += 'a' - 'A'
		}
		if c1 != c2 {
			return false
		}
	}
	return true
}

// Context returns the request context.
// This returns the actual Lambda context passed from the handler, enabling proper
// cancellation, timeouts, and access to AWS Lambda metadata (request ID, function info, etc.).
// Middleware and handlers should use this context for operations that need to respect
// Lambda execution deadlines.
func (r *Request) Context() context.Context {
	if r.ctx != nil {
		return r.ctx
	}
	return context.Background()
}

// Header returns the request headers (for compatibility)
func (r *Request) Header() map[string]string {
	if r.Headers == nil {
		return make(map[string]string)
	}
	return r.Headers
}

// RemoteAddr returns the remote client IP address.
// It extracts the IP using the following precedence:
// 1. API Gateway V2 requestContext.http.sourceIp
// 2. API Gateway V1 requestContext.identity.sourceIp
// 3. X-Forwarded-For header (first IP in comma-separated list)
// 4. X-Real-IP header
// 5. CF-Connecting-IP header (Cloudflare)
//
// Returns an empty string if no valid IP can be determined.
// Note: For accurate IP extraction with validation, use the security.ExtractClientIP function.
func (r *Request) RemoteAddr() string {
	// First, try to get IP from requestContext (most reliable source from API Gateway)
	reqCtx := r.RequestContext()
	if len(reqCtx) > 0 {
		// API Gateway V2 format: requestContext.http.sourceIp
		if http, ok := reqCtx["http"].(map[string]any); ok {
			if sourceIP, ok := http["sourceIp"].(string); ok && sourceIP != "" {
				return sourceIP
			}
		}

		// API Gateway V1 format: requestContext.identity.sourceIp
		if identity, ok := reqCtx["identity"].(map[string]any); ok {
			if sourceIP, ok := identity["sourceIp"].(string); ok && sourceIP != "" {
				return sourceIP
			}
		}

		// Direct sourceIp on requestContext (some WebSocket configurations)
		if sourceIP, ok := reqCtx["sourceIp"].(string); ok && sourceIP != "" {
			return sourceIP
		}
	}

	// Fall back to headers
	if r.Headers != nil {
		// X-Forwarded-For: may contain multiple IPs (client, proxies...)
		// The first IP is typically the original client
		if xForwardedFor := r.GetHeader("X-Forwarded-For"); xForwardedFor != "" {
			// Parse first IP from comma-separated list
			if idx := strings.Index(xForwardedFor, ","); idx > 0 {
				return strings.TrimSpace(xForwardedFor[:idx])
			}
			return strings.TrimSpace(xForwardedFor)
		}

		// X-Real-IP: single IP set by reverse proxies
		if xRealIP := r.GetHeader("X-Real-IP"); xRealIP != "" {
			return xRealIP
		}

		// CF-Connecting-IP: Cloudflare's client IP header
		if cfIP := r.GetHeader("CF-Connecting-IP"); cfIP != "" {
			return cfIP
		}
	}

	// Return empty string instead of localhost placeholder
	// This makes it clear when IP extraction fails rather than masking the issue
	return ""
}

// UserAgent returns the user agent string (for compatibility)
func (r *Request) UserAgent() string {
	if r.Headers == nil {
		return ""
	}
	return r.Headers["User-Agent"]
}

// URL returns a simple URL structure (for compatibility)
func (r *Request) URL() *SimpleURL {
	return &SimpleURL{
		Path: r.Path,
	}
}

// SimpleURL provides basic URL functionality for compatibility
type SimpleURL struct {
	Path string
}
