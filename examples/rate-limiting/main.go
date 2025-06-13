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
		return lift.BadRequest("Invalid request body")
	}

	// Process signup...
	return ctx.Status(201).JSON(map[string]interface{}{
		"message": "Account created successfully",
		"user_id": "new-user-id",
	})
}

func handleLogin(ctx *lift.Context) error {
	var req LoginRequest
	if err := ctx.ParseRequest(&req); err != nil {
		return lift.BadRequest("Invalid request body")
	}

	// Process login...
	return ctx.JSON(map[string]interface{}{
		"token": "jwt-token-here",
		"user": map[string]interface{}{
			"id":    "user-id",
			"email": req.Email,
		},
	})
}

func handleForgotPassword(ctx *lift.Context) error {
	var req ForgotPasswordRequest
	if err := ctx.ParseRequest(&req); err != nil {
		return lift.BadRequest("Invalid request body")
	}

	// Process password reset...
	return ctx.JSON(map[string]interface{}{
		"message": "Password reset email sent",
	})
}

func handleExpensiveOperation(ctx *lift.Context) error {
	// This operation is expensive, so it would be rate limited in production
	if ctx.Logger != nil {
		ctx.Logger.Info("Starting expensive operation", map[string]interface{}{
			"tenant_id": ctx.TenantID(),
			"user_id":   ctx.UserID(),
		})
	}

	// Simulate expensive operation
	time.Sleep(100 * time.Millisecond) // Reduced for demo

	return ctx.JSON(map[string]interface{}{
		"result": "Operation completed",
		"cost":   "high",
	})
}

func handleDataExport(ctx *lift.Context) error {
	exportType := ctx.Query("type")
	if exportType == "" {
		return lift.BadRequest("Export type is required")
	}

	return ctx.JSON(map[string]interface{}{
		"export_id": "export-123",
		"type":      exportType,
		"status":    "processing",
	})
}

func handleListUsers(ctx *lift.Context) error {
	return ctx.JSON(map[string]interface{}{
		"users": []map[string]interface{}{
			{"id": "1", "name": "User 1"},
			{"id": "2", "name": "User 2"},
		},
		"total": 2,
	})
}

func handleCreateUser(ctx *lift.Context) error {
	var req CreateUserRequest
	if err := ctx.ParseRequest(&req); err != nil {
		return lift.BadRequest("Invalid request body")
	}

	return ctx.Status(201).JSON(map[string]interface{}{
		"id":   "new-user-id",
		"name": req.Name,
	})
}

func handleGetUser(ctx *lift.Context) error {
	userID := ctx.Param("id")
	return ctx.JSON(map[string]interface{}{
		"id":   userID,
		"name": "User Name",
	})
}

func handleUpdateUser(ctx *lift.Context) error {
	userID := ctx.Param("id")
	var req UpdateUserRequest
	if err := ctx.ParseRequest(&req); err != nil {
		return lift.BadRequest("Invalid request body")
	}

	return ctx.JSON(map[string]interface{}{
		"id":   userID,
		"name": req.Name,
	})
}

func handleDeleteUser(ctx *lift.Context) error {
	userID := ctx.Param("id")
	return ctx.JSON(map[string]interface{}{
		"message": fmt.Sprintf("User %s deleted", userID),
	})
}

func handleHealth(ctx *lift.Context) error {
	return ctx.JSON(map[string]interface{}{
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
