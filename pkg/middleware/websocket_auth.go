package middleware

import (
	"strings"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/security"
)

// WebSocketAuthConfig configures WebSocket authentication
type WebSocketAuthConfig struct {
	JWTConfig      security.JWTConfig
	TokenExtractor func(ctx *lift.Context) string
	OnError        func(ctx *lift.Context, err error) error
	SkipRoutes     []string // Routes to skip authentication (e.g., health checks)
}

// WebSocketAuth creates authentication middleware for WebSocket connections
func WebSocketAuth(config WebSocketAuthConfig) lift.Middleware {
	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			// Check if this is a WebSocket context
			wsCtx, err := ctx.AsWebSocket()
			if err != nil {
				// Not a WebSocket context, pass through
				return next.Handle(ctx)
			}

			// Check if route should be skipped
			routeKey := wsCtx.RouteKey()
			for _, skip := range config.SkipRoutes {
				if routeKey == skip {
					return next.Handle(ctx)
				}
			}

			// Only validate on connect events
			if !wsCtx.IsConnectEvent() {
				// For non-connect events, check if user is already authenticated
				if ctx.UserID() == "" {
					if config.OnError != nil {
						return config.OnError(ctx, lift.NewLiftError("UNAUTHORIZED", "User not authenticated", 401))
					}
					return ctx.Status(401).JSON(map[string]string{
						"error": "User not authenticated",
					})
				}
				return next.Handle(ctx)
			}

			// Extract token
			token := ""
			if config.TokenExtractor != nil {
				token = config.TokenExtractor(ctx)
			} else {
				// Default extraction from query params and headers
				token = ctx.Query("Authorization")
				if token == "" {
					token = ctx.Query("authorization")
				}
				if token == "" {
					token = ctx.Query("token")
				}
				if token == "" {
					token = ctx.Header("Authorization")
				}
				if token == "" {
					token = ctx.Header("authorization")
				}
			}

			// Remove "Bearer " prefix if present
			token = strings.TrimPrefix(token, "Bearer ")
			token = strings.TrimPrefix(token, "bearer ")

			if token == "" {
				if config.OnError != nil {
					return config.OnError(ctx, lift.NewLiftError("MISSING_TOKEN", "Missing authentication token", 401))
				}
				return ctx.Status(401).JSON(map[string]string{
					"error": "Missing authentication token",
				})
			}

			// Create JWT validator
			validator, err := NewJWTValidator(config.JWTConfig)
			if err != nil {
				if config.OnError != nil {
					return config.OnError(ctx, err)
				}
				return ctx.Status(500).JSON(map[string]string{
					"error": "Failed to create JWT validator: " + err.Error(),
				})
			}

			// Validate token
			claims, err := validator.ValidateToken(token)
			if err != nil {
				if config.OnError != nil {
					return config.OnError(ctx, err)
				}
				return ctx.Status(401).JSON(map[string]string{
					"error": "Invalid token: " + err.Error(),
				})
			}

			// Create security context
			secCtx := lift.WithSecurity(ctx)

			// Create principal from claims
			principal := &security.Principal{
				UserID:     claims.Subject,
				TenantID:   claims.TenantID,
				AccountID:  claims.AccountID,
				Roles:      claims.Roles,
				Scopes:     claims.Scopes,
				AuthMethod: "jwt",
				IssuedAt:   time.Now(),
				IPAddress:  ctx.Header("X-Real-IP"),
				UserAgent:  ctx.Header("User-Agent"),
				RequestID:  ctx.RequestID,
			}

			// Set principal in security context
			secCtx.SetPrincipal(principal)

			// Also set in regular context for backward compatibility
			ctx.SetUserID(claims.Subject)
			ctx.Set("claims", claims)
			ctx.Set("tenant_id", claims.TenantID)
			ctx.Set("roles", claims.Roles)

			// Log successful authentication
			if ctx.Logger != nil {
				ctx.Logger.Info("WebSocket authenticated", map[string]interface{}{
					"user_id":       claims.Subject,
					"connection_id": wsCtx.ConnectionID(),
					"route_key":     routeKey,
				})
			}

			return next.Handle(ctx)
		})
	}
}

// WebSocketAuthFromQuery is a simple token extractor that gets the token from query parameters
func WebSocketAuthFromQuery(paramName string) func(ctx *lift.Context) string {
	return func(ctx *lift.Context) string {
		return ctx.Query(paramName)
	}
}

// WebSocketAuthFromHeader is a token extractor that gets the token from headers
func WebSocketAuthFromHeader(headerName string) func(ctx *lift.Context) string {
	return func(ctx *lift.Context) string {
		return ctx.Header(headerName)
	}
}
