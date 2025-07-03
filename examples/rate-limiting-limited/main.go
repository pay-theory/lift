package main

import (
	"fmt"
	"time"

	"github.com/pay-theory/dynamorm"
	"github.com/pay-theory/dynamorm/pkg/session"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/limited"
	"go.uber.org/zap"
)

// RateLimitMiddleware creates a middleware using the limited library
func RateLimitMiddleware(limiter *limited.DynamoRateLimiter) lift.Middleware {
	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			// Generate rate limit key
			key := generateRateLimitKey(ctx)
			
			// Check rate limit
			decision, err := limiter.CheckAndIncrement(ctx.Context, key)
			if err != nil {
				// Log error but allow request on failure
				if ctx.Logger != nil {
					ctx.Logger.Error("Rate limit check failed", map[string]any{
						"error": err.Error(),
						"key":   key,
					})
				}
				return next.Handle(ctx)
			}
			
			// Set rate limit headers
			remaining := decision.Limit - decision.CurrentCount
			if remaining < 0 {
				remaining = 0
			}
			ctx.Response.Header("X-RateLimit-Limit", fmt.Sprintf("%d", decision.Limit))
			ctx.Response.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
			ctx.Response.Header("X-RateLimit-Reset", fmt.Sprintf("%d", decision.ResetsAt.Unix()))
			
			if !decision.Allowed {
				retryAfter := 60 // default to 60 seconds
				if decision.RetryAfter != nil {
					retryAfter = int(decision.RetryAfter.Seconds())
				}
				ctx.Response.Header("Retry-After", fmt.Sprintf("%d", retryAfter))
				
				return ctx.Response.Status(429).JSON(map[string]any{
					"error":       "Rate limit exceeded",
					"limit":       decision.Limit,
					"remaining":   remaining,
					"reset_at":    decision.ResetsAt.Unix(),
					"retry_after": retryAfter,
				})
			}
			
			return next.Handle(ctx)
		})
	}
}

// generateRateLimitKey creates a rate limit key based on the request context
func generateRateLimitKey(ctx *lift.Context) limited.RateLimitKey {
	key := limited.RateLimitKey{
		Resource:  ctx.Request.Path,
		Operation: ctx.Request.Method,
		Metadata:  make(map[string]string),
	}
	
	// Use user ID if authenticated
	if userID := ctx.UserID(); userID != "" {
		key.Identifier = fmt.Sprintf("user:%s", userID)
		key.Metadata["user_id"] = userID
	} else {
		// Fall back to IP address
		ip := ctx.Header("X-Forwarded-For")
		if ip == "" {
			ip = ctx.Header("X-Real-IP")
		}
		if ip == "" {
			ip = "unknown"
		}
		key.Identifier = fmt.Sprintf("ip:%s", ip)
		key.Metadata["ip"] = ip
	}
	
	// Add tenant ID if present
	if tenantID := ctx.TenantID(); tenantID != "" {
		key.Metadata["tenant_id"] = tenantID
	}
	
	return key
}

func main() {
	app := lift.New()
	
	// Initialize DynamoDB connection using DynamORM's NewBasic
	db, err := dynamorm.NewBasic(session.Config{
		Region: "us-east-1",
		// For local testing:
		// Endpoint: "http://localhost:8000",
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize DynamoDB: %v", err))
	}
	
	// Create logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	
	// Create rate limiting strategies
	// Public endpoints: 1000 requests per hour
	publicStrategy := limited.NewFixedWindowStrategy(time.Hour, 1000)
	publicLimiter := limited.NewDynamoRateLimiter(
		db,
		nil, // Use default config
		publicStrategy,
		logger,
	)
	
	// Note: In a real implementation, you would create different limiters
	// for different endpoints and apply them selectively
	
	// Apply global rate limiting (you can be more specific with different limiters)
	app.Use(RateLimitMiddleware(publicLimiter))
	
	// Public endpoints
	public := app.Group("/public")
	public.POST("/signup", handleSignup)
	public.POST("/login", handleLogin)
	public.POST("/forgot-password", handleForgotPassword)
	
	// API endpoints
	api := app.Group("/api")
	api.GET("/users", handleListUsers)
	api.POST("/users", handleCreateUser)
	api.GET("/users/:id", handleGetUser)
	api.PUT("/users/:id", handleUpdateUser)
	api.DELETE("/users/:id", handleDeleteUser)
	api.POST("/expensive-operation", handleExpensiveOperation)
	api.POST("/data-export", handleDataExport)
	
	// Health check (no rate limiting)
	app.GET("/health", handleHealth)
	
	app.Start()
}

// Handler implementations

func handleSignup(ctx *lift.Context) error {
	var req SignupRequest
	if err := ctx.ParseRequest(&req); err != nil {
		return err
	}
	
	// Process signup...
	return ctx.Status(201).JSON(map[string]any{
		"message": "Account created successfully",
		"user_id": "new-user-id",
	})
}

func handleLogin(ctx *lift.Context) error {
	var req LoginRequest
	if err := ctx.ParseRequest(&req); err != nil {
		return err
	}
	
	// Process login...
	return ctx.JSON(map[string]any{
		"token": "jwt-token-here",
		"user": map[string]any{
			"id":    "user-id",
			"email": req.Email,
		},
	})
}

func handleForgotPassword(ctx *lift.Context) error {
	var req ForgotPasswordRequest
	if err := ctx.ParseRequest(&req); err != nil {
		return err
	}
	
	// Process password reset...
	return ctx.JSON(map[string]any{
		"message": "Password reset email sent",
	})
}

func handleExpensiveOperation(ctx *lift.Context) error {
	// This operation is rate limited more strictly
	if ctx.Logger != nil {
		ctx.Logger.Info("Starting expensive operation", map[string]any{
			"tenant_id": ctx.TenantID(),
			"user_id":   ctx.UserID(),
		})
	}
	
	// Simulate expensive operation
	time.Sleep(100 * time.Millisecond)
	
	return ctx.JSON(map[string]any{
		"result": "Operation completed",
		"cost":   "high",
	})
}

func handleDataExport(ctx *lift.Context) error {
	exportType := ctx.Query("type")
	if exportType == "" {
		return ctx.BadRequest("Export type is required", nil)
	}
	
	return ctx.JSON(map[string]any{
		"export_id": "export-123",
		"type":      exportType,
		"status":    "processing",
	})
}

func handleListUsers(ctx *lift.Context) error {
	return ctx.JSON(map[string]any{
		"users": []map[string]any{
			{"id": "1", "name": "User 1"},
			{"id": "2", "name": "User 2"},
		},
		"total": 2,
	})
}

func handleCreateUser(ctx *lift.Context) error {
	var req CreateUserRequest
	if err := ctx.ParseRequest(&req); err != nil {
		return err
	}
	
	return ctx.Status(201).JSON(map[string]any{
		"id":   "new-user-id",
		"name": req.Name,
	})
}

func handleGetUser(ctx *lift.Context) error {
	userID := ctx.Param("id")
	return ctx.JSON(map[string]any{
		"id":   userID,
		"name": "User Name",
	})
}

func handleUpdateUser(ctx *lift.Context) error {
	userID := ctx.Param("id")
	var req UpdateUserRequest
	if err := ctx.ParseRequest(&req); err != nil {
		return err
	}
	
	return ctx.JSON(map[string]any{
		"id":   userID,
		"name": req.Name,
	})
}

func handleDeleteUser(ctx *lift.Context) error {
	userID := ctx.Param("id")
	return ctx.JSON(map[string]any{
		"message": fmt.Sprintf("User %s deleted", userID),
	})
}

func handleHealth(ctx *lift.Context) error {
	return ctx.JSON(map[string]any{
		"status": "healthy",
		"time":   time.Now().Unix(),
	})
}

// Request types
type SignupRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	Name     string `json:"name" validate:"required"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type CreateUserRequest struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
}

type UpdateUserRequest struct {
	Name string `json:"name" validate:"required"`
}