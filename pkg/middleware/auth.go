package middleware

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/security"
)

// JWTClaims represents the claims in a JWT token
type JWTClaims struct {
	jwt.RegisteredClaims
	TenantID  string   `json:"tenant_id"`
	AccountID string   `json:"account_id"`
	Roles     []string `json:"roles"`
	Scopes    []string `json:"scopes"`
}

// JWTValidator handles JWT token validation
type JWTValidator struct {
	config    security.JWTConfig
	publicKey *rsa.PublicKey
	secretKey []byte
}

// NewJWTValidator creates a new JWT validator
func NewJWTValidator(config security.JWTConfig) (*JWTValidator, error) {
	validator := &JWTValidator{
		config: config,
	}

	// Load keys based on signing method
	switch config.SigningMethod {
	case "RS256":
		if config.PublicKeyPath == "" {
			return nil, fmt.Errorf("public key path is required for RS256")
		}

		publicKey, err := loadRSAPublicKey(config.PublicKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load public key: %w", err)
		}
		validator.publicKey = publicKey

	case "HS256":
		if config.SecretKey == "" {
			return nil, fmt.Errorf("secret key is required for HS256")
		}
		validator.secretKey = []byte(config.SecretKey)

	default:
		return nil, fmt.Errorf("unsupported signing method: %s", config.SigningMethod)
	}

	return validator, nil
}

// ValidateToken validates a JWT token and returns the claims
func (v *JWTValidator) ValidateToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (any, error) {
		// Verify signing method
		switch v.config.SigningMethod {
		case "RS256":
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return v.publicKey, nil
		case "HS256":
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return v.secretKey, nil
		default:
			return nil, fmt.Errorf("unsupported signing method: %s", v.config.SigningMethod)
		}
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Validate standard claims
	if err := v.validateStandardClaims(claims); err != nil {
		return nil, err
	}

	// Validate custom claims
	if err := v.validateCustomClaims(claims); err != nil {
		return nil, err
	}

	return claims, nil
}

// validateStandardClaims validates the standard JWT claims
func (v *JWTValidator) validateStandardClaims(claims *JWTClaims) error {
	now := time.Now()

	// Check expiration
	if claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(now) {
		return fmt.Errorf("token has expired")
	}

	// Check not before
	if claims.NotBefore != nil && claims.NotBefore.Time.After(now) {
		return fmt.Errorf("token not valid yet")
	}

	// Check issued at (with max age)
	if claims.IssuedAt != nil && v.config.MaxAge > 0 {
		maxAge := claims.IssuedAt.Time.Add(v.config.MaxAge)
		if now.After(maxAge) {
			return fmt.Errorf("token exceeds maximum age")
		}
	}

	// Check issuer
	if v.config.Issuer != "" && claims.Issuer != v.config.Issuer {
		return fmt.Errorf("invalid issuer: expected %s, got %s", v.config.Issuer, claims.Issuer)
	}

	// Check audience
	if len(v.config.Audience) > 0 {
		validAudience := false
		for _, aud := range v.config.Audience {
			for _, claimAud := range claims.Audience {
				if aud == claimAud {
					validAudience = true
					break
				}
			}
			if validAudience {
				break
			}
		}
		if !validAudience {
			return fmt.Errorf("invalid audience")
		}
	}

	return nil
}

// validateCustomClaims validates custom claims specific to Pay Theory
func (v *JWTValidator) validateCustomClaims(claims *JWTClaims) error {
	// Validate tenant ID if required
	if v.config.RequireTenantID && claims.TenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}

	// Custom tenant validation
	if v.config.ValidateTenant != nil && claims.TenantID != "" {
		if err := v.config.ValidateTenant(claims.TenantID); err != nil {
			return fmt.Errorf("tenant validation failed: %w", err)
		}
	}

	// Validate subject (user ID)
	if claims.Subject == "" {
		return fmt.Errorf("subject (user_id) is required")
	}

	return nil
}

// JWT creates JWT authentication middleware
func JWT(config security.JWTConfig) lift.Middleware {
	validator, err := NewJWTValidator(config)
	if err != nil {
		// This is a configuration error, panic is appropriate
		panic(fmt.Sprintf("Failed to create JWT validator: %v", err))
	}

	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			// Create security context wrapper
			secCtx := lift.WithSecurity(ctx)

			// Extract token from Authorization header
			token := extractBearerToken(ctx)
			if token == "" {
				return lift.Unauthorized("Missing or invalid authorization token")
			}

			// Validate token
			claims, err := validator.ValidateToken(token)
			if err != nil {
				return lift.Unauthorized(fmt.Sprintf("Invalid token: %v", err))
			}

			// Multi-tenant validation
			if config.RequireTenantID && claims.TenantID == "" {
				return lift.AuthorizationError("Tenant ID is required")
			}

			// Create principal from claims
			principal := createPrincipalFromClaims(claims, ctx)

			// Set principal in security context
			secCtx.SetPrincipal(principal)

			// Add authentication info to logger
			if ctx.Logger != nil {
				ctx.Logger = ctx.Logger.WithField("user_id", principal.UserID).
					WithField("tenant_id", principal.TenantID).
					WithField("auth_method", "jwt")
			}

			return next.Handle(ctx)
		})
	}
}

// JWTOptional creates optional JWT authentication middleware
// If no token is provided, continues with anonymous principal
func JWTOptional(config security.JWTConfig) lift.Middleware {
	validator, err := NewJWTValidator(config)
	if err != nil {
		panic(fmt.Sprintf("Failed to create JWT validator: %v", err))
	}

	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			// Create security context wrapper
			secCtx := lift.WithSecurity(ctx)

			// Extract token from Authorization header
			token := extractBearerToken(ctx)

			var principal *security.Principal

			if token == "" {
				// No token provided, use anonymous principal
				principal = security.AnonymousPrincipal()
			} else {
				// Validate token
				claims, err := validator.ValidateToken(token)
				if err != nil {
					// Invalid token, use anonymous principal
					principal = security.AnonymousPrincipal()
				} else {
					// Valid token, create principal from claims
					principal = createPrincipalFromClaims(claims, ctx)
				}
			}

			// Set principal in security context
			secCtx.SetPrincipal(principal)

			// Add authentication info to logger
			if ctx.Logger != nil {
				ctx.Logger = ctx.Logger.WithField("user_id", principal.UserID).
					WithField("tenant_id", principal.TenantID).
					WithField("auth_method", principal.AuthMethod)
			}

			return next.Handle(ctx)
		})
	}
}

// RequireRole creates middleware that requires specific roles
func RequireRole(roles ...string) lift.Middleware {
	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			secCtx := lift.WithSecurity(ctx)
			principal := secCtx.GetPrincipal()
			if principal == nil {
				return lift.Unauthorized("Authentication required")
			}

			if !principal.HasAnyRole(roles...) {
				return lift.AuthorizationError(fmt.Sprintf("Required roles: %v", roles))
			}

			return next.Handle(ctx)
		})
	}
}

// RequireScope creates middleware that requires specific scopes
func RequireScope(scopes ...string) lift.Middleware {
	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			secCtx := lift.WithSecurity(ctx)
			principal := secCtx.GetPrincipal()
			if principal == nil {
				return lift.Unauthorized("Authentication required")
			}

			for _, scope := range scopes {
				if !principal.HasScope(scope) {
					return lift.AuthorizationError(fmt.Sprintf("Required scope: %s", scope))
				}
			}

			return next.Handle(ctx)
		})
	}
}

// RequireTenant creates middleware that validates tenant access
func RequireTenant(tenantID string) lift.Middleware {
	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			secCtx := lift.WithSecurity(ctx)
			principal := secCtx.GetPrincipal()
			if principal == nil {
				return lift.Unauthorized("Authentication required")
			}

			if !principal.IsValidForTenant(tenantID) {
				return lift.AuthorizationError("Access denied for this tenant")
			}

			return next.Handle(ctx)
		})
	}
}

// extractBearerToken extracts the bearer token from the Authorization header
func extractBearerToken(ctx *lift.Context) string {
	authHeader := ctx.Header("Authorization")
	if authHeader == "" {
		return ""
	}

	// Check for Bearer token
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return ""
	}

	return strings.TrimPrefix(authHeader, bearerPrefix)
}

// createPrincipalFromClaims creates a Principal from JWT claims
func createPrincipalFromClaims(claims *JWTClaims, ctx *lift.Context) *security.Principal {
	var expiresAt time.Time
	if claims.ExpiresAt != nil {
		expiresAt = claims.ExpiresAt.Time
	}

	var issuedAt time.Time
	if claims.IssuedAt != nil {
		issuedAt = claims.IssuedAt.Time
	} else {
		issuedAt = time.Now()
	}

	return &security.Principal{
		UserID:     claims.Subject,
		TenantID:   claims.TenantID,
		AccountID:  claims.AccountID,
		Roles:      claims.Roles,
		Scopes:     claims.Scopes,
		AuthMethod: "jwt",
		IssuedAt:   issuedAt,
		ExpiresAt:  expiresAt,
		IPAddress:  ctx.Header("X-Real-IP"),
		UserAgent:  ctx.Header("User-Agent"),
		RequestID:  ctx.RequestID,
	}
}

// loadRSAPublicKey loads an RSA public key from a PEM file
func loadRSAPublicKey(path string) (*rsa.PublicKey, error) {
	keyData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("key is not an RSA public key")
	}

	return rsaPub, nil
}
