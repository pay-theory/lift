package lift

import (
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// JWTAuthConfig holds configuration for JWT authentication
type JWTAuthConfig struct {
	// Secret key for HMAC algorithms
	Secret string

	// Public key for RSA/ECDSA algorithms
	PublicKey interface{}

	// Algorithm to use (HS256, RS256, etc)
	Algorithm string

	// Token lookup string (e.g., "header:Authorization,query:token")
	TokenLookup string

	// Skip authentication for these paths
	SkipPaths []string

	// Custom error handler
	ErrorHandler func(ctx *Context, err error) error

	// Custom claims validator
	Validator func(claims jwt.MapClaims) error
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
type SecurityConfig struct {
	// Enable security headers
	EnableSecurityHeaders bool

	// Enable CSRF protection
	EnableCSRF bool

	// Enable rate limiting
	EnableRateLimiting bool

	// Custom security handler
	Handler func(ctx *Context) error

	// IP whitelist (empty means allow all)
	IPWhitelist []string

	// Required roles for all endpoints (can be overridden per route)
	RequiredRoles []string

	// Audit logger
	AuditLogger func(ctx *Context, event string, data map[string]interface{})
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
		panic("invalid token lookup format")
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
		panic(fmt.Sprintf("unsupported token lookup: %s", parts[0]))
	}
}

// parseJWTToken parses and validates a JWT token
func parseJWTToken(tokenString string, config JWTAuthConfig) (*jwt.Token, error) {
	// Parse with appropriate method based on algorithm
	switch config.Algorithm {
	case "HS256", "HS384", "HS512":
		return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// Validate algorithm
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(config.Secret), nil
		})
	case "RS256", "RS384", "RS512":
		return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
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
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx *Context) error {
			// Convert to security context
			secCtx := NewSecurityContext(ctx)

			// Check IP whitelist if configured
			if len(config.IPWhitelist) > 0 {
				if !secCtx.ValidateIP(config.IPWhitelist) {
					return ctx.Forbidden("Access denied", nil)
				}
			}

			// Add security headers
			if config.EnableSecurityHeaders {
				ctx.Response.Headers["X-Content-Type-Options"] = "nosniff"
				ctx.Response.Headers["X-Frame-Options"] = "DENY"
				ctx.Response.Headers["X-XSS-Protection"] = "1; mode=block"
				ctx.Response.Headers["Strict-Transport-Security"] = "max-age=31536000; includeSubDomains"
			}

			// Check required roles if any
			if len(config.RequiredRoles) > 0 && ctx.IsAuthenticated() {
				hasRequiredRole := false
				for _, role := range config.RequiredRoles {
					if secCtx.HasRole(role) {
						hasRequiredRole = true
						break
					}
				}
				if !hasRequiredRole {
					return ctx.Forbidden("Insufficient permissions", nil)
				}
			}

			// Log audit event
			if config.AuditLogger != nil {
				defer func() {
					config.AuditLogger(ctx, "request", secCtx.ToAuditMap())
				}()
			}

			// Call custom handler if provided
			if config.Handler != nil {
				if err := config.Handler(ctx); err != nil {
					return err
				}
			}

			// Continue to next handler
			return next.Handle(ctx)
		})
	}
}
