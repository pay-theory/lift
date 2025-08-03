package lift

import (
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// JWTAuthConfig holds configuration for JWT authentication
type JWTAuthConfig struct {
	PublicKey    any
	ErrorHandler func(ctx *Context, err error) error
	Validator    func(claims jwt.MapClaims) error
	Secret       string
	Algorithm    string
	TokenLookup  string
	SkipPaths    []string
}

// WithJWTAuth adds JWT authentication middleware to the application
func WithJWTAuth(config JWTAuthConfig) AppOption {
	return func(app *App) {
		// Apply defaults
		if config.Algorithm == "" {
			config.Algorithm = "HS256"
		}
		if config.TokenLookup == "" {
			config.TokenLookup = "header:Authorization"
		}
		if config.ErrorHandler == nil {
			config.ErrorHandler = func(ctx *Context, err error) error {
				return ctx.Unauthorized("Invalid or missing token", err)
			}
		}

		// Create the middleware
		jwtMiddleware := createJWTMiddleware(config)
		app.Use(jwtMiddleware)
	}
}

// WithSimpleJWTAuth adds JWT authentication with just a secret key
func WithSimpleJWTAuth(secret string) AppOption {
	return WithJWTAuth(JWTAuthConfig{
		Secret:    secret,
		Algorithm: "HS256",
	})
}

// SecurityConfig holds configuration for security middleware
// Memory optimized: 96 → 88 bytes (8 bytes saved)
type SecurityConfig struct {
	// 8-byte aligned fields (functions, slices)
	Handler       func(ctx *Context) error                              // Custom security handler
	AuditLogger   func(ctx *Context, event string, data map[string]any) // Audit logger
	IPWhitelist   []string                                              // IP whitelist (empty means allow all)
	RequiredRoles []string                                              // Required roles for all endpoints (can be overridden per route)

	// Boolean flags (1 byte each)
	EnableSecurityHeaders bool // Enable security headers
	EnableCSRF            bool // Enable CSRF protection
	EnableRateLimiting    bool // Enable rate limiting
}

// WithSecurityMiddleware adds security middleware to the application
func WithSecurityMiddleware(config SecurityConfig) AppOption {
	return func(app *App) {
		// Create security middleware
		securityMiddleware := createSecurityMiddleware(config)
		app.Use(securityMiddleware)
	}
}

// createJWTMiddleware creates the JWT middleware
func createJWTMiddleware(config JWTAuthConfig) Middleware {
	// Create token extractor
	extractor := createTokenExtractor(config.TokenLookup)

	return func(next Handler) Handler {
		return HandlerFunc(func(ctx *Context) error {
			// Check if path should be skipped
			path := ctx.Request.Path
			for _, skipPath := range config.SkipPaths {
				if path == skipPath || strings.HasPrefix(path, skipPath) {
					return next.Handle(ctx)
				}
			}

			// Extract token
			tokenString, err := extractor(ctx)
			if err != nil {
				return config.ErrorHandler(ctx, err)
			}

			// Parse token
			token, err := parseJWTToken(tokenString, config)
			if err != nil {
				return config.ErrorHandler(ctx, err)
			}

			// Validate token
			if !token.Valid {
				return config.ErrorHandler(ctx, fmt.Errorf("invalid token"))
			}

			// Extract claims
			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				return config.ErrorHandler(ctx, fmt.Errorf("invalid claims type"))
			}

			// Validate claims if validator provided
			if config.Validator != nil {
				if err := config.Validator(claims); err != nil {
					return config.ErrorHandler(ctx, err)
				}
			}

			// Set claims in context
			ctx.SetClaims(claims)

			// Continue to next handler
			return next.Handle(ctx)
		})
	}
}

// createTokenExtractor creates a token extractor based on the lookup string
func createTokenExtractor(lookup string) func(*Context) (string, error) {
	parts := strings.Split(lookup, ":")
	if len(parts) != 2 {
		// Return an extractor that always returns an error
		return func(_ *Context) (string, error) {
			return "", fmt.Errorf("invalid token lookup format: expected 'type:field', got '%s'", lookup)
		}
	}

	switch parts[0] {
	case "header":
		return func(ctx *Context) (string, error) {
			auth := ctx.Header(parts[1])
			if auth == "" {
				return "", fmt.Errorf("missing %s header", parts[1])
			}
			// Handle Bearer token
			if parts[1] == "Authorization" && strings.HasPrefix(auth, "Bearer ") {
				return strings.TrimPrefix(auth, "Bearer "), nil
			}
			return auth, nil
		}
	case "query":
		return func(ctx *Context) (string, error) {
			token := ctx.Query(parts[1])
			if token == "" {
				return "", fmt.Errorf("missing %s query parameter", parts[1])
			}
			return token, nil
		}
	default:
		// Return an extractor that always returns an error
		return func(_ *Context) (string, error) {
			return "", fmt.Errorf("unsupported token lookup type: %s (supported: header, query)", parts[0])
		}
	}
}

// parseJWTToken parses and validates a JWT token
func parseJWTToken(tokenString string, config JWTAuthConfig) (*jwt.Token, error) {
	// Parse with appropriate method based on algorithm
	switch config.Algorithm {
	case "HS256", "HS384", "HS512":
		return jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
			// Validate algorithm
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(config.Secret), nil
		})
	case "RS256", "RS384", "RS512":
		return jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
			// Validate algorithm
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return config.PublicKey, nil
		})
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", config.Algorithm)
	}
}

// createSecurityMiddleware creates the security middleware
func createSecurityMiddleware(config SecurityConfig) Middleware {
	processor := newSecurityProcessor(config)
	
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx *Context) error {
			return processor.process(ctx, next)
		})
	}
}

// securityProcessor handles security processing logic
type securityProcessor struct {
	config          SecurityConfig
	ipValidator     *ipValidator
	headerApplier   *securityHeaderApplier
	roleValidator   *roleValidator
	auditLogger     *securityAuditLogger
}

// newSecurityProcessor creates a new security processor
func newSecurityProcessor(config SecurityConfig) *securityProcessor {
	processor := &securityProcessor{
		config: config,
	}
	
	// Initialize components based on configuration
	if len(config.IPWhitelist) > 0 {
		processor.ipValidator = newIPValidator(config.IPWhitelist)
	}
	
	if config.EnableSecurityHeaders {
		processor.headerApplier = newSecurityHeaderApplier()
	}
	
	if len(config.RequiredRoles) > 0 {
		processor.roleValidator = newRoleValidator(config.RequiredRoles)
	}
	
	if config.AuditLogger != nil {
		processor.auditLogger = newSecurityAuditLogger(config.AuditLogger)
	}
	
	return processor
}

// process handles the security processing
func (sp *securityProcessor) process(ctx *Context, next Handler) error {
	// Convert to security context
	secCtx := NewSecurityContext(ctx)
	
	// Validate IP if configured
	if sp.ipValidator != nil {
		if err := sp.ipValidator.validate(secCtx); err != nil {
			return err
		}
	}
	
	// Apply security headers if configured
	if sp.headerApplier != nil {
		sp.headerApplier.apply(ctx)
	}
	
	// Validate roles if configured
	if sp.roleValidator != nil {
		if err := sp.roleValidator.validate(ctx, secCtx); err != nil {
			return err
		}
	}
	
	// Setup audit logging if configured
	if sp.auditLogger != nil {
		defer sp.auditLogger.logRequest(ctx, secCtx)
	}
	
	// Call custom handler if provided
	if sp.config.Handler != nil {
		if err := sp.config.Handler(ctx); err != nil {
			return err
		}
	}
	
	// Continue to next handler
	return next.Handle(ctx)
}

// ipValidator validates IP addresses against a whitelist
type ipValidator struct {
	whitelist []string
}

// newIPValidator creates a new IP validator
func newIPValidator(whitelist []string) *ipValidator {
	return &ipValidator{whitelist: whitelist}
}

// validate checks if the request IP is in the whitelist
func (iv *ipValidator) validate(secCtx *SecurityContext) error {
	if !secCtx.ValidateIP(iv.whitelist) {
		return AuthorizationError("Access denied")
	}
	return nil
}

// securityHeaderApplier applies security headers to responses
type securityHeaderApplier struct {
	headers map[string]string
}

// newSecurityHeaderApplier creates a new security header applier
func newSecurityHeaderApplier() *securityHeaderApplier {
	return &securityHeaderApplier{
		headers: map[string]string{
			"X-Content-Type-Options":    "nosniff",
			"X-Frame-Options":           "DENY",
			"X-XSS-Protection":          "1; mode=block",
			"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
		},
	}
}

// apply adds security headers to the response
func (sha *securityHeaderApplier) apply(ctx *Context) {
	for key, value := range sha.headers {
		ctx.Response.Headers[key] = value
	}
}

// roleValidator validates required roles
type roleValidator struct {
	requiredRoles []string
}

// newRoleValidator creates a new role validator
func newRoleValidator(requiredRoles []string) *roleValidator {
	return &roleValidator{requiredRoles: requiredRoles}
}

// validate checks if the user has required roles
func (rv *roleValidator) validate(ctx *Context, secCtx *SecurityContext) error {
	if !ctx.IsAuthenticated() {
		return nil // Skip role check for unauthenticated requests
	}
	
	for _, role := range rv.requiredRoles {
		if secCtx.HasRole(role) {
			return nil // User has at least one required role
		}
	}
	
	return AuthorizationError("Insufficient permissions")
}

// securityAuditLogger handles security audit logging
type securityAuditLogger struct {
	logger func(ctx *Context, event string, data map[string]any)
}

// newSecurityAuditLogger creates a new security audit logger
func newSecurityAuditLogger(logger func(ctx *Context, event string, data map[string]any)) *securityAuditLogger {
	return &securityAuditLogger{logger: logger}
}

// logRequest logs the request for audit purposes
func (sal *securityAuditLogger) logRequest(ctx *Context, secCtx *SecurityContext) {
	sal.logger(ctx, "request", secCtx.ToAuditMap())
}
