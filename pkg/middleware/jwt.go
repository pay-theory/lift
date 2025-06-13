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
	PublicKey interface{}

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
			// Cookie extraction would go here
			// For now, return error
			return "", fmt.Errorf("cookie extraction not implemented")
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

// WithJWTAuth is a convenience function for creating JWT middleware with minimal config
func WithJWTAuth(secret string) lift.Middleware {
	return JWTAuth(JWTConfig{
		Secret:    secret,
		Algorithm: "HS256",
	})
}
