package main

import (
	"time"

	"github.com/pay-theory/lift/pkg/dynamorm"
	"github.com/pay-theory/lift/pkg/lift"
)

// User represents a user entity
type User struct {
	ID        string    `json:"id" dynamodb:"id,hash"`
	TenantID  string    `json:"tenant_id" dynamodb:"tenant_id"`
	Email     string    `json:"email" validate:"required,email"`
	Name      string    `json:"name" validate:"required,min=1,max=100"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateUserRequest represents the request to create a user
type CreateUserRequest struct {
	Email string `json:"email" validate:"required,email"`
	Name  string `json:"name" validate:"required,min=1,max=100"`
}

// UpdateUserRequest represents the request to update a user
type UpdateUserRequest struct {
	Email  *string `json:"email,omitempty" validate:"omitempty,email"`
	Name   *string `json:"name,omitempty" validate:"omitempty,min=1,max=100"`
	Active *bool   `json:"active,omitempty"`
}

// UserResponse represents the response for user operations
type UserResponse struct {
	User    *User  `json:"user,omitempty"`
	Message string `json:"message,omitempty"`
}

// UsersResponse represents the response for listing users
type UsersResponse struct {
	Users   []User `json:"users"`
	Count   int    `json:"count"`
	NextKey string `json:"next_key,omitempty"`
}

func main() {
	// Create Lift application
	app := lift.New()

	// Add DynamORM middleware with tenant isolation
	dynamormConfig := &dynamorm.DynamORMConfig{
		TableName:       "lift_users",
		Region:          "us-east-1",
		AutoTransaction: true,
		TenantIsolation: true,
		TenantKey:       "tenant_id",
	}
	app.Use(dynamorm.WithDynamORM(dynamormConfig))

	// Add authentication middleware (simplified for example)
	app.Use(AuthMiddleware())

	// Add logging middleware
	app.Use(LoggingMiddleware())

	// User routes
	app.POST("/users", CreateUser)
	app.GET("/users/:id", GetUser)
	app.GET("/users", ListUsers)
	app.PUT("/users/:id", UpdateUser)
	app.DELETE("/users/:id", DeleteUser)

	// Health check
	app.GET("/health", HealthCheck)

	// Start the application
	app.Start()
}

// CreateUser creates a new user
func CreateUser(ctx *lift.Context) error {
	// Parse request
	var req CreateUserRequest
	if err := ctx.ParseRequest(&req); err != nil {
		return err
	}

	// Get tenant-scoped database
	db, err := dynamorm.TenantDB(ctx)
	if err != nil {
		return err
	}

	// Create user entity
	user := &User{
		ID:        generateID(),
		TenantID:  ctx.TenantID(),
		Email:     req.Email,
		Name:      req.Name,
		Active:    true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Save to database
	if err := db.Put(ctx, user); err != nil {
		return lift.InternalError("Failed to create user").WithCause(err)
	}

	// Log the creation
	ctx.Logger.Info("User created", map[string]interface{}{
		"user_id":   user.ID,
		"tenant_id": user.TenantID,
		"email":     user.Email,
	})

	// Return response
	return ctx.Status(201).JSON(&UserResponse{
		User:    user,
		Message: "User created successfully",
	})
}

// GetUser retrieves a user by ID
func GetUser(ctx *lift.Context) error {
	userID := ctx.Param("id")
	if userID == "" {
		return lift.BadRequest("User ID is required")
	}

	// Get tenant-scoped database
	db, err := dynamorm.TenantDB(ctx)
	if err != nil {
		return err
	}

	// Retrieve user
	var user User
	if err := db.Get(ctx, userID, &user); err != nil {
		return lift.NotFound("User not found")
	}

	// Verify tenant access
	if user.TenantID != ctx.TenantID() {
		return lift.NotFound("User not found")
	}

	return ctx.JSON(&UserResponse{User: &user})
}

// ListUsers lists all users for the tenant
func ListUsers(ctx *lift.Context) error {
	// Get tenant-scoped database
	db, err := dynamorm.TenantDB(ctx)
	if err != nil {
		return err
	}

	// Build query
	query := &dynamorm.Query{
		PartitionKey: ctx.TenantID(),
		Limit:        50, // Default limit
	}

	// Execute query
	result, err := db.Query(ctx, query)
	if err != nil {
		return lift.InternalError("Failed to list users").WithCause(err)
	}

	// Convert results to users
	users := make([]User, len(result.Items))
	for i, item := range result.Items {
		if user, ok := item.(User); ok {
			users[i] = user
		}
	}

	return ctx.JSON(&UsersResponse{
		Users: users,
		Count: len(users),
	})
}

// UpdateUser updates an existing user
func UpdateUser(ctx *lift.Context) error {
	userID := ctx.Param("id")
	if userID == "" {
		return lift.BadRequest("User ID is required")
	}

	// Parse request
	var req UpdateUserRequest
	if err := ctx.ParseRequest(&req); err != nil {
		return err
	}

	// Get tenant-scoped database
	db, err := dynamorm.TenantDB(ctx)
	if err != nil {
		return err
	}

	// Retrieve existing user
	var user User
	if err := db.Get(ctx, userID, &user); err != nil {
		return lift.NotFound("User not found")
	}

	// Verify tenant access
	if user.TenantID != ctx.TenantID() {
		return lift.NotFound("User not found")
	}

	// Apply updates
	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.Name != nil {
		user.Name = *req.Name
	}
	if req.Active != nil {
		user.Active = *req.Active
	}
	user.UpdatedAt = time.Now()

	// Save updated user
	if err := db.Put(ctx, user); err != nil {
		return lift.InternalError("Failed to update user").WithCause(err)
	}

	// Log the update
	ctx.Logger.Info("User updated", map[string]interface{}{
		"user_id":   user.ID,
		"tenant_id": user.TenantID,
		"email":     user.Email,
	})

	return ctx.JSON(&UserResponse{
		User:    &user,
		Message: "User updated successfully",
	})
}

// DeleteUser deletes a user
func DeleteUser(ctx *lift.Context) error {
	userID := ctx.Param("id")
	if userID == "" {
		return lift.BadRequest("User ID is required")
	}

	// Get tenant-scoped database
	db, err := dynamorm.TenantDB(ctx)
	if err != nil {
		return err
	}

	// Retrieve user to verify existence and tenant access
	var user User
	if err := db.Get(ctx, userID, &user); err != nil {
		return lift.NotFound("User not found")
	}

	// Verify tenant access
	if user.TenantID != ctx.TenantID() {
		return lift.NotFound("User not found")
	}

	// Delete user
	if err := db.Delete(ctx, userID); err != nil {
		return lift.InternalError("Failed to delete user").WithCause(err)
	}

	// Log the deletion
	ctx.Logger.Info("User deleted", map[string]interface{}{
		"user_id":   user.ID,
		"tenant_id": user.TenantID,
		"email":     user.Email,
	})

	return ctx.JSON(&UserResponse{
		Message: "User deleted successfully",
	})
}

// HealthCheck provides a health check endpoint
func HealthCheck(ctx *lift.Context) error {
	return ctx.JSON(map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"service":   "lift-crud-api",
	})
}

// AuthMiddleware provides simplified authentication for the example
func AuthMiddleware() lift.Middleware {
	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			// Skip auth for health check
			if ctx.Request.Path == "/health" {
				return next.Handle(ctx)
			}

			// Extract tenant ID from header (simplified)
			tenantID := ctx.Header("X-Tenant-ID")
			if tenantID == "" {
				return lift.Unauthorized("Tenant ID is required")
			}

			// Extract user ID from header (simplified)
			userID := ctx.Header("X-User-ID")
			if userID == "" {
				return lift.Unauthorized("User ID is required")
			}

			// Set in context
			ctx.Set("tenant_id", tenantID)
			ctx.Set("user_id", userID)

			return next.Handle(ctx)
		})
	}
}

// LoggingMiddleware provides request logging
func LoggingMiddleware() lift.Middleware {
	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			start := time.Now()

			// Execute request
			err := next.Handle(ctx)

			// Log completion
			if ctx.Logger != nil {
				fields := map[string]interface{}{
					"method":    ctx.Request.Method,
					"path":      ctx.Request.Path,
					"status":    ctx.Response.StatusCode,
					"duration":  time.Since(start).Milliseconds(),
					"tenant_id": ctx.TenantID(),
					"user_id":   ctx.UserID(),
				}

				if err != nil {
					fields["error"] = err.Error()
					ctx.Logger.Error("Request failed", fields)
				} else {
					ctx.Logger.Info("Request completed", fields)
				}
			}

			return err
		})
	}
}

// generateID generates a unique ID for entities
func generateID() string {
	// In a real implementation, use UUID or similar
	return "user_" + time.Now().Format("20060102150405")
}
