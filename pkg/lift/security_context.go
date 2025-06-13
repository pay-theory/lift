package lift

import (
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pay-theory/lift/pkg/security"
)

// SecurityContext extends the Context with security functionality
type SecurityContext struct {
	*Context
	principal *security.Principal
	requestID string
}

// NewSecurityContext wraps an existing Context with security features
func NewSecurityContext(ctx *Context) *SecurityContext {
	return &SecurityContext{
		Context:   ctx,
		requestID: generateRequestID(),
	}
}

// SetPrincipal sets the authenticated principal
func (sc *SecurityContext) SetPrincipal(principal *security.Principal) {
	sc.principal = principal

	// Store principal information in context values
	sc.Set("principal", principal)
	sc.Set("user_id", principal.UserID)
	sc.Set("tenant_id", principal.TenantID)
	sc.Set("account_id", principal.AccountID)
	sc.Set("roles", principal.Roles)
	sc.Set("scopes", principal.Scopes)
	sc.Set("auth_method", principal.AuthMethod)

	// Update request tracking
	principal.RequestID = sc.requestID
	principal.IPAddress = sc.GetClientIP()
	principal.UserAgent = sc.GetUserAgent()
}

// GetPrincipal returns the authenticated principal
func (sc *SecurityContext) GetPrincipal() *security.Principal {
	return sc.principal
}

// RequestID returns the unique request ID
func (sc *SecurityContext) RequestID() string {
	return sc.requestID
}

// HasRole checks if the principal has a specific role
func (sc *SecurityContext) HasRole(role string) bool {
	if sc.principal == nil {
		return false
	}
	return sc.principal.HasRole(role)
}

// HasPermission checks if the principal can access a resource
func (sc *SecurityContext) HasPermission(resource, action string) bool {
	if sc.principal == nil {
		return false
	}
	return sc.principal.CanAccessResource(resource, action)
}

// IsAuthenticated checks if the request has an authenticated principal
func (sc *SecurityContext) IsAuthenticated() bool {
	return sc.principal != nil && !sc.principal.IsExpired()
}

// GetClientIP extracts the client IP address from the request
func (sc *SecurityContext) GetClientIP() string {
	// Check X-Forwarded-For header (load balancer)
	xForwardedFor := sc.Header("X-Forwarded-For")
	if xForwardedFor != "" {
		// Take the first IP if multiple are present
		ips := strings.Split(xForwardedFor, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	xRealIP := sc.Header("X-Real-IP")
	if xRealIP != "" {
		return xRealIP
	}

	// Extract from API Gateway context
	requestContext := sc.Request.RequestContext()
	if len(requestContext) > 0 {
		if identity, ok := requestContext["identity"].(map[string]interface{}); ok {
			if sourceIP, ok := identity["sourceIp"].(string); ok {
				return sourceIP
			}
		}
	}

	return "unknown"
}

// GetUserAgent returns the User-Agent header
func (sc *SecurityContext) GetUserAgent() string {
	return sc.Header("User-Agent")
}

// ValidateIP checks if the client IP is in the allowed range
func (sc *SecurityContext) ValidateIP(allowedCIDRs []string) bool {
	clientIP := sc.GetClientIP()
	if clientIP == "unknown" {
		return false
	}

	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false
	}

	for _, cidr := range allowedCIDRs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}

		if network.Contains(ip) {
			return true
		}
	}

	return false
}

// ValidateTenant ensures the request is for the correct tenant
func (sc *SecurityContext) ValidateTenant(expectedTenantID string) bool {
	currentTenantID := sc.TenantID()
	if currentTenantID == "" {
		return false
	}

	return currentTenantID == expectedTenantID
}

// ToAuditMap returns a map suitable for audit logging
func (sc *SecurityContext) ToAuditMap() map[string]interface{} {
	auditData := map[string]interface{}{
		"request_id":  sc.requestID,
		"method":      sc.Request.Method,
		"path":        sc.Request.Path,
		"status_code": sc.Response.StatusCode,
		"elapsed_ms":  sc.Duration().Milliseconds(),
		"client_ip":   sc.GetClientIP(),
		"user_agent":  sc.GetUserAgent(),
		"tenant_id":   sc.TenantID(),
		"user_id":     sc.UserID(),
		"timestamp":   time.Now().Unix(),
	}

	// Add principal information if available
	if sc.principal != nil {
		for k, v := range sc.principal.ToAuditMap() {
			auditData[k] = v
		}
	}

	return auditData
}

// RequireAuthentication returns an error if not authenticated
func (sc *SecurityContext) RequireAuthentication() error {
	if !sc.IsAuthenticated() {
		return NewLiftError("UNAUTHORIZED", "Authentication required", 401)
	}
	return nil
}

// RequireRole returns an error if the principal doesn't have the required role
func (sc *SecurityContext) RequireRole(role string) error {
	if err := sc.RequireAuthentication(); err != nil {
		return err
	}

	if !sc.HasRole(role) {
		return NewLiftError("FORBIDDEN", "Insufficient role permissions", 403)
	}

	return nil
}

// RequirePermission returns an error if the principal doesn't have the required permission
func (sc *SecurityContext) RequirePermission(resource, action string) error {
	if err := sc.RequireAuthentication(); err != nil {
		return err
	}

	if !sc.HasPermission(resource, action) {
		return NewLiftError("FORBIDDEN", "Insufficient permissions", 403)
	}

	return nil
}

// RequireTenant returns an error if the principal doesn't belong to the expected tenant
func (sc *SecurityContext) RequireTenant(expectedTenantID string) error {
	if err := sc.RequireAuthentication(); err != nil {
		return err
	}

	if !sc.ValidateTenant(expectedTenantID) {
		return NewLiftError("FORBIDDEN", "Invalid tenant access", 403)
	}

	return nil
}

// OverrideUserID returns the user ID from the principal if available, otherwise from request
func (sc *SecurityContext) UserID() string {
	if sc.principal != nil {
		return sc.principal.UserID
	}
	return sc.Context.UserID()
}

// OverrideTenantID returns the tenant ID from the principal if available, otherwise from request
func (sc *SecurityContext) TenantID() string {
	if sc.principal != nil {
		return sc.principal.TenantID
	}
	return sc.Context.TenantID()
}

// OverrideAccountID returns the account ID from the principal if available
func (sc *SecurityContext) AccountID() string {
	if sc.principal != nil {
		return sc.principal.AccountID
	}
	return sc.Context.AccountID()
}

// generateRequestID creates a unique request ID
func generateRequestID() string {
	return uuid.New().String()
}

// WithSecurity converts a regular Context to a SecurityContext
func WithSecurity(ctx *Context) *SecurityContext {
	return NewSecurityContext(ctx)
}
