package main

import (
	"fmt"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
)

func main() {
	app := lift.New()

	// Public endpoints
	app.POST("/public/signup", handleSignup)
	app.POST("/public/login", handleLogin)
	app.POST("/public/forgot-password", handleForgotPassword)

	// API endpoints
	app.POST("/api/v1/expensive-operation", handleExpensiveOperation)
	app.POST("/api/v1/data-export", handleDataExport)
	app.GET("/api/v1/users", handleListUsers)
	app.POST("/api/v1/users", handleCreateUser)
	app.GET("/api/v1/users/:id", handleGetUser)
	app.PUT("/api/v1/users/:id", handleUpdateUser)
	app.DELETE("/api/v1/users/:id", handleDeleteUser)

	// Health check
	app.GET("/health", handleHealth)

	// Start the app (this would be called by Lambda)
	app.Start()
}

// Handler implementations

func handleSignup(ctx *lift.Context) error {
	var req SignupRequest
	if err := ctx.ParseRequest(&req); err != nil {
		return lift.NewLiftError("BAD_REQUEST", "Invalid request body", 400)
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
		return lift.NewLiftError("BAD_REQUEST", "Invalid request body", 400)
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
		return lift.NewLiftError("BAD_REQUEST", "Invalid request body", 400)
	}

	// Process password reset...
	return ctx.JSON(map[string]any{
		"message": "Password reset email sent",
	})
}

func handleExpensiveOperation(ctx *lift.Context) error {
	// This operation is expensive, so it would be rate limited in production
	if ctx.Logger != nil {
		ctx.Logger.Info("Starting expensive operation", map[string]any{
			"tenant_id": ctx.TenantID(),
			"user_id":   ctx.UserID(),
		})
	}

	// Simulate expensive operation
	time.Sleep(100 * time.Millisecond) // Reduced for demo

	return ctx.JSON(map[string]any{
		"result": "Operation completed",
		"cost":   "high",
	})
}

func handleDataExport(ctx *lift.Context) error {
	exportType := ctx.Query("type")
	if exportType == "" {
		return lift.NewLiftError("BAD_REQUEST", "Export type is required", 400)
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
		return lift.NewLiftError("BAD_REQUEST", "Invalid request body", 400)
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
		return lift.NewLiftError("BAD_REQUEST", "Invalid request body", 400)
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
