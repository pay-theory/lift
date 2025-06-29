package main

import (
	"log"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/middleware"
	"github.com/pay-theory/lift/pkg/security"
)

func main() {
	// Create a new Lift application
	app := lift.New()

	// Configure JWT authentication
	jwtConfig := security.JWTConfig{
		SigningMethod:   "HS256",
		SecretKey:       "your-secret-key-here", // In production, load from AWS Secrets Manager
		Issuer:          "pay-theory",
		Audience:        []string{"lift-api"},
		MaxAge:          time.Hour,
		RequireTenantID: true,
		ValidateTenant: func(tenantID string) error {
			// Custom tenant validation logic
			validTenants := []string{"tenant1", "tenant2", "premium-tenant"}
			for _, valid := range validTenants {
				if tenantID == valid {
					return nil
				}
			}
			return security.NewSecurityError("INVALID_TENANT", "Tenant not found")
		},
	}

	// Public endpoints (no authentication required)
	app.GET("/health", healthHandler)
	app.GET("/public", publicHandler)

	// Protected endpoints with JWT authentication
	app.GET("/api/profile", protectedHandler(jwtConfig, profileHandler))
	app.GET("/api/users", adminHandler(jwtConfig, usersHandler))
	app.GET("/api/payments", paymentsAccessHandler(jwtConfig, paymentsHandler))
	app.GET("/api/tenant/:id/data", protectedHandler(jwtConfig, tenantDataHandler))

	// Optional authentication endpoints
	app.GET("/mixed/content", optionalAuthHandler(jwtConfig, mixedContentHandler))

	// Start the application
	log.Println("Starting JWT authentication example...")
	app.Start()
}

// protectedHandler wraps a handler with JWT authentication
func protectedHandler(jwtConfig security.JWTConfig, handler func(*lift.Context) error) func(*lift.Context) error {
	return func(ctx *lift.Context) error {
		return middleware.JWT(jwtConfig)(lift.HandlerFunc(handler)).Handle(ctx)
	}
}

// adminHandler wraps a handler with JWT authentication and admin role requirement
func adminHandler(jwtConfig security.JWTConfig, handler func(*lift.Context) error) func(*lift.Context) error {
	return func(ctx *lift.Context) error {
		// First apply JWT authentication
		jwtMiddleware := middleware.JWT(jwtConfig)
		// Then apply role requirement
		roleMiddleware := middleware.RequireRole("admin", "manager")

		// Chain them together
		return jwtMiddleware(roleMiddleware(lift.HandlerFunc(handler))).Handle(ctx)
	}
}

// paymentsAccessHandler wraps a handler with JWT authentication and payments scope requirement
func paymentsAccessHandler(jwtConfig security.JWTConfig, handler func(*lift.Context) error) func(*lift.Context) error {
	return func(ctx *lift.Context) error {
		// First apply JWT authentication
		jwtMiddleware := middleware.JWT(jwtConfig)
		// Then apply scope requirement
		scopeMiddleware := middleware.RequireScope("payments:read")

		// Chain them together
		return jwtMiddleware(scopeMiddleware(lift.HandlerFunc(handler))).Handle(ctx)
	}
}

// optionalAuthHandler wraps a handler with optional JWT authentication
func optionalAuthHandler(jwtConfig security.JWTConfig, handler func(*lift.Context) error) func(*lift.Context) error {
	return func(ctx *lift.Context) error {
		return middleware.JWTOptional(jwtConfig)(lift.HandlerFunc(handler)).Handle(ctx)
	}
}

// healthHandler provides a health check endpoint
func healthHandler(ctx *lift.Context) error {
	return ctx.JSON(map[string]any{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"service":   "jwt-auth-example",
	})
}

// publicHandler demonstrates a public endpoint
func publicHandler(ctx *lift.Context) error {
	return ctx.JSON(map[string]any{
		"message": "This is a public endpoint",
		"access":  "no authentication required",
	})
}

// profileHandler demonstrates accessing user profile with JWT
func profileHandler(ctx *lift.Context) error {
	secCtx := lift.WithSecurity(ctx)
	principal := secCtx.GetPrincipal()

	if principal == nil {
		return lift.NewLiftError("UNAUTHORIZED", "Authentication required", 401)
	}

	return ctx.JSON(map[string]any{
		"user_id":     principal.UserID,
		"tenant_id":   principal.TenantID,
		"account_id":  principal.AccountID,
		"roles":       principal.Roles,
		"scopes":      principal.Scopes,
		"auth_method": principal.AuthMethod,
		"expires_at":  principal.ExpiresAt.Unix(),
	})
}

// usersHandler demonstrates role-based access control
func usersHandler(ctx *lift.Context) error {
	secCtx := lift.WithSecurity(ctx)
	principal := secCtx.GetPrincipal()

	// This handler requires admin or manager role (enforced by middleware)
	users := []map[string]any{
		{
			"id":     "user1",
			"name":   "John Doe",
			"tenant": principal.TenantID,
			"role":   "user",
		},
		{
			"id":     "user2",
			"name":   "Jane Smith",
			"tenant": principal.TenantID,
			"role":   "manager",
		},
	}

	return ctx.JSON(map[string]any{
		"users":       users,
		"accessed_by": principal.UserID,
		"tenant":      principal.TenantID,
	})
}

// paymentsHandler demonstrates scope-based access control
func paymentsHandler(ctx *lift.Context) error {
	secCtx := lift.WithSecurity(ctx)
	principal := secCtx.GetPrincipal()

	// This handler requires payments:read scope (enforced by middleware)
	payments := []map[string]any{
		{
			"id":     "pay1",
			"amount": 100.00,
			"status": "completed",
			"tenant": principal.TenantID,
		},
		{
			"id":     "pay2",
			"amount": 250.50,
			"status": "pending",
			"tenant": principal.TenantID,
		},
	}

	return ctx.JSON(map[string]any{
		"payments":    payments,
		"accessed_by": principal.UserID,
		"tenant":      principal.TenantID,
	})
}

// tenantDataHandler demonstrates tenant-specific data access
func tenantDataHandler(ctx *lift.Context) error {
	secCtx := lift.WithSecurity(ctx)
	principal := secCtx.GetPrincipal()
	requestedTenantID := ctx.Param("id")

	// Validate tenant access
	if !principal.IsValidForTenant(requestedTenantID) {
		return lift.NewLiftError("FORBIDDEN", "Access denied for this tenant", 403)
	}

	// Return tenant-specific data
	return ctx.JSON(map[string]any{
		"tenant_id":   requestedTenantID,
		"data":        "sensitive tenant data",
		"accessed_by": principal.UserID,
		"timestamp":   time.Now().Unix(),
	})
}

// mixedContentHandler demonstrates optional authentication
func mixedContentHandler(ctx *lift.Context) error {
	secCtx := lift.WithSecurity(ctx)
	principal := secCtx.GetPrincipal()

	response := map[string]any{
		"message": "This endpoint works with or without authentication",
	}

	if principal != nil && principal.AuthMethod != "none" {
		// User is authenticated
		response["authenticated"] = true
		response["user_id"] = principal.UserID
		response["tenant_id"] = principal.TenantID
		response["personalized_content"] = "Welcome back, " + principal.UserID + "!"
	} else {
		// User is anonymous
		response["authenticated"] = false
		response["generic_content"] = "Welcome, anonymous user!"
	}

	return ctx.JSON(response)
}
