package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pay-theory/lift/pkg/lift"
)

func main() {
	// Get JWT secret from environment or generate a secure one
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		// In production, this should fail or use a secure secret management service
		fmt.Println("WARNING: JWT_SECRET not set. Using a default value for demo purposes only.")
		jwtSecret = "demo-secret-key-change-in-production"
	}

	// Create app with JWT authentication
	app := lift.New(
		lift.WithJWTAuth(lift.JWTAuthConfig{
			Secret:    jwtSecret,
			Algorithm: "HS256",
			SkipPaths: []string{"/health", "/login"},
			Validator: func(claims jwt.MapClaims) error {
				// Validate token expiration
				if exp, ok := claims["exp"].(float64); ok {
					if time.Now().Unix() > int64(exp) {
						return fmt.Errorf("token expired")
					}
				}
				return nil
			},
		}),
		lift.WithSecurityMiddleware(lift.SecurityConfig{
			EnableSecurityHeaders: true,
			AuditLogger: func(_ *lift.Context, event string, data map[string]any) {
				fmt.Printf("Audit: %s - %v\n", event, data)
			},
		}),
	)

	// Public endpoints
	if err := app.GET("/health", func(ctx *lift.Context) error {
		return ctx.OK(map[string]string{"status": "healthy"})
	}); err != nil {
		log.Fatalf("Failed to register GET /health: %v", err)
	}

	if err := app.POST("/login", func(ctx *lift.Context) error {
		// In real app, validate credentials
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}

		if err := ctx.ParseRequest(&req); err != nil {
			return err
		}

		// Create JWT token
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id":   "user-123",
			"tenant_id": "tenant-456",
			"username":  req.Username,
			"roles":     []string{"user", "admin"},
			"exp":       time.Now().Add(24 * time.Hour).Unix(),
		})

		tokenString, err := token.SignedString([]byte(jwtSecret))
		if err != nil {
			return ctx.SystemError("Failed to create token", err)
		}

		return ctx.OK(map[string]string{
			"token": tokenString,
		})
	}); err != nil {
		log.Fatalf("Failed to register POST /login: %v", err)
	}

	// Protected endpoints
	if err := app.GET("/me", func(ctx *lift.Context) error {
		// Access JWT claims
		claims := ctx.Claims()
		if claims == nil {
			return ctx.Unauthorized("No claims found", nil)
		}

		return ctx.OK(map[string]any{
			"user_id":          ctx.UserID(),
			"tenant_id":        ctx.TenantID(),
			"username":         claims["username"],
			"roles":            claims["roles"],
			"is_authenticated": ctx.IsAuthenticated(),
		})
	}); err != nil {
		log.Fatalf("Failed to register GET /me: %v", err)
	}

	if err := app.GET("/admin", func(ctx *lift.Context) error {
		// Check for admin role
		claims := ctx.Claims()
		if claims == nil {
			return ctx.Unauthorized("Authentication required", nil)
		}

		roles, ok := claims["roles"].([]any)
		if !ok {
			return ctx.Forbidden("No roles found", nil)
		}

		hasAdmin := false
		for _, role := range roles {
			if role == "admin" {
				hasAdmin = true
				break
			}
		}

		if !hasAdmin {
			return ctx.Forbidden("Admin role required", nil)
		}

		return ctx.OK(map[string]string{
			"message": "Welcome admin!",
			"user_id": ctx.UserID(),
		})
	}); err != nil {
		log.Fatalf("Failed to register GET /admin: %v", err)
	}

	// Multi-tenant endpoint
	if err := app.GET("/tenant/:tenantId/data", func(ctx *lift.Context) error {
		requestedTenant := ctx.Param("tenantId")
		userTenant := ctx.TenantID()

		if userTenant != requestedTenant {
			return ctx.Forbidden("Cannot access other tenant's data", nil)
		}

		return ctx.OK(map[string]any{
			"tenant_id": requestedTenant,
			"data":      "Tenant specific data",
			"user_id":   ctx.UserID(),
		})
	}); err != nil {
		log.Fatalf("Failed to register GET /tenant/:tenantId/data: %v", err)
	}

	// In Lambda, you would start with:
	// lambda.Start(app.HandleRequest)
}
