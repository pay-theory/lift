package middleware

import (
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pay-theory/lift/pkg/lift"
)

// JWTConfig holds configuration for JWT middleware
type JWTConfig struct {
	// Secret key for HMAC algorithms
	Secret string

	// Public key for RSA/ECDSA algorithms
	PublicKey any

	// Algorithm to use (HS256, RS256, etc)
	Algorithm string

	// Token lookup string (e.g., "header:Authorization,query:token")
	TokenLookup string

	// Claims validator function
	Validator func(claims jwt.MapClaims) error

	// Error handler
	ErrorHandler func(ctx *lift.Context, err error) error

	// Skip authentication for these paths
	SkipPaths []string

	// Optional: custom claims type
	Claims jwt.Claims

	// Optional: custom token extractor
	Extractor func(ctx *lift.Context) (string, error)
}

// DefaultJWTConfig returns a default JWT configuration
func DefaultJWTConfig() JWTConfig {
	return JWTConfig{
		Algorithm:   "HS256",
		TokenLookup: "header:Authorization",
		ErrorHandler: func(ctx *lift.Context, err error) error {
			return ctx.Unauthorized("Invalid or missing token", err)
		},
	}
}

// JWTAuth creates a JWT authentication middleware
func JWTAuth(config JWTConfig) lift.Middleware {
	// Apply defaults
	if config.Algorithm == "" {
		config.Algorithm = "HS256"
	}
	if config.TokenLookup == "" {
		config.TokenLookup = "header:Authorization"
	}
	if config.ErrorHandler == nil {
		config.ErrorHandler = DefaultJWTConfig().ErrorHandler
	}

	// Create token extractor if not provided
	if config.Extractor == nil {
		config.Extractor = createExtractor(config.TokenLookup)
	}

	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			// Check if path should be skipped
			path := ctx.Request.Path
			for _, skipPath := range config.SkipPaths {
				if path == skipPath || strings.HasPrefix(path, skipPath) {
					return next.Handle(ctx)
				}
			}

			// Extract token
			tokenString, err := config.Extractor(ctx)
			if err != nil {
				return config.ErrorHandler(ctx, err)
			}

			// Parse token
			token, err := parseToken(tokenString, config)
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
				// Try to handle custom claims
				if config.Claims != nil {
					claims = jwt.MapClaims{}
					// Convert custom claims to MapClaims
					// This is a simplified version - in production you'd want more robust conversion
				} else {
					return config.ErrorHandler(ctx, fmt.Errorf("invalid claims type"))
				}
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

// createExtractor creates a token extractor based on the lookup string
func createExtractor(lookup string) func(*lift.Context) (string, error) {
	parts := strings.Split(lookup, ":")
	if len(parts) != 2 {
		panic("invalid token lookup format")
	}

	switch parts[0] {
	case "header":
		return func(ctx *lift.Context) (string, error) {
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
		return func(ctx *lift.Context) (string, error) {
			token := ctx.Query(parts[1])
			if token == "" {
				return "", fmt.Errorf("missing %s query parameter", parts[1])
			}
			return token, nil
		}
	case "cookie":
		return func(ctx *lift.Context) (string, error) {
			return extractJWTFromCookie(ctx, parts[1])
		}
	default:
		panic(fmt.Sprintf("unsupported token lookup: %s", parts[0]))
	}
}

// parseToken parses and validates a JWT token
func parseToken(tokenString string, config JWTConfig) (*jwt.Token, error) {
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

// WithJWTAuth is a convenience function for creating JWT middleware with minimal config
func WithJWTAuth(secret string) lift.Middleware {
	return JWTAuth(JWTConfig{
		Secret:    secret,
		Algorithm: "HS256",
	})
}

// extractJWTFromCookie extracts JWT token from HTTP cookies with security validation
func extractJWTFromCookie(ctx *lift.Context, cookieName string) (string, error) {
	// Parse cookies from the Cookie header
	cookieHeader := ctx.Header("Cookie")
	if cookieHeader == "" {
		return "", fmt.Errorf("no cookies found in request")
	}

	// Parse individual cookies
	cookies := parseCookies(cookieHeader)

	// Find the JWT cookie
	tokenCookie, exists := cookies[cookieName]
	if !exists {
		return "", fmt.Errorf("JWT cookie '%s' not found", cookieName)
	}

	// Validate the cookie token
	if err := validateJWTCookie(tokenCookie); err != nil {
		return "", fmt.Errorf("invalid JWT cookie: %w", err)
	}

	return tokenCookie.Value, nil
}

// CookieToken represents a parsed HTTP cookie
type CookieToken struct {
	Name     string
	Value    string
	HttpOnly bool
	Secure   bool
	SameSite string
	Path     string
	Domain   string
	MaxAge   int
}

// parseCookies parses the Cookie header value into individual cookies
func parseCookies(cookieHeader string) map[string]*CookieToken {
	cookies := make(map[string]*CookieToken)

	// Split by semicolon to get individual cookies
	parts := strings.Split(cookieHeader, ";")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Find the first equals sign to separate name from value
		equalIndex := strings.Index(part, "=")
		if equalIndex == -1 {
			continue // Skip malformed cookies
		}

		name := strings.TrimSpace(part[:equalIndex])
		value := strings.TrimSpace(part[equalIndex+1:])

		// Remove quotes if present
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1 : len(value)-1]
		}

		cookies[name] = &CookieToken{
			Name:  name,
			Value: value,
			// Note: Cookie header only contains name=value pairs
			// Attributes like HttpOnly, Secure, etc. are only in Set-Cookie response headers
		}
	}

	return cookies
}

// validateJWTCookie performs security validation on the JWT cookie
func validateJWTCookie(cookie *CookieToken) error {
	// Validate cookie name
	if cookie.Name == "" {
		return fmt.Errorf("cookie name cannot be empty")
	}

	// Validate token value is not empty
	if cookie.Value == "" {
		return fmt.Errorf("JWT token value cannot be empty")
	}

	// Basic JWT format validation (header.payload.signature)
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 3 {
		return fmt.Errorf("invalid JWT format: expected 3 parts separated by dots, got %d", len(parts))
	}

	// Validate each part is base64-encoded (basic check)
	for i, part := range parts {
		if part == "" {
			return fmt.Errorf("JWT part %d is empty", i+1)
		}

		// Check for valid base64url characters
		for _, char := range part {
			if !isValidBase64URLChar(char) {
				return fmt.Errorf("JWT part %d contains invalid base64url character: %c", i+1, char)
			}
		}
	}

	// Validate token length (prevent extremely long tokens)
	if len(cookie.Value) > 8192 { // 8KB limit
		return fmt.Errorf("JWT token too long: %d bytes (max 8192)", len(cookie.Value))
	}

	return nil
}

// isValidBase64URLChar checks if a character is valid in base64url encoding
func isValidBase64URLChar(c rune) bool {
	return (c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') ||
		c == '-' || c == '_' // base64url uses - and _ instead of + and /
}
