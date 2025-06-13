package main

import (
	"fmt"
	"time"

	"github.com/pay-theory/lift/pkg/dynamorm"
	"github.com/pay-theory/lift/pkg/lift"
)

// User represents a user model for DynamORM
type User struct {
	ID        string    `dynamorm:"pk" json:"id"`
	TenantID  string    `dynamorm:"gsi1pk" json:"tenant_id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateUserRequest represents the request to create a user
type CreateUserRequest struct {
	Email string `json:"email" validate:"required,email"`
	Name  string `json:"name" validate:"required"`
}

// UserResponse represents a user response
type UserResponse struct {
	User    *User  `json:"user"`
	Message string `json:"message"`
}

func main() {
	// Create a new Lift application
	app := lift.New()

	// Configure DynamORM middleware
	app.Use(dynamorm.WithDynamORM(&dynamorm.DynamORMConfig{
		TableName:       "lift_users",
		Region:          "us-east-1",
		Endpoint:        "http://localhost:8000", // For local DynamoDB testing
		TenantIsolation: true,                    // Enable tenant isolation
		AutoTransaction: true,                    // Enable automatic transactions
		ConsistentRead:  false,                   // Use eventually consistent reads
	}))

	// Health check endpoint
	app.GET("/health", func(ctx *lift.Context) error {
		return ctx.JSON(map[string]string{
			"status":  "healthy",
			"service": "dynamorm-integration-demo",
		})
	})

	// Create user endpoint
	app.POST("/users", func(ctx *lift.Context) error {
		var req CreateUserRequest
		if err := ctx.ParseRequest(&req); err != nil {
			return lift.BadRequest("Invalid request body").WithCause(err)
		}

		// Get DynamORM instance from context
		db, err := dynamorm.TenantDB(ctx)
		if err != nil {
			return err
		}

		// Create new user
		user := &User{
			ID:        fmt.Sprintf("user_%d", time.Now().UnixNano()),
			TenantID:  ctx.TenantID(),
			Email:     req.Email,
			Name:      req.Name,
			Active:    true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// Save user to DynamoDB
		if err := db.Put(ctx.Context, user); err != nil {
			return lift.InternalError("Failed to create user").WithCause(err)
		}

		return ctx.JSON(UserResponse{
			User:    user,
			Message: "User created successfully",
		})
	})

	// Get user endpoint
	app.GET("/users/:id", func(ctx *lift.Context) error {
		userID := ctx.Param("id")

		// Get DynamORM instance from context
		db, err := dynamorm.TenantDB(ctx)
		if err != nil {
			return err
		}

		// Retrieve user from DynamoDB
		var user User
		if err := db.Get(ctx.Context, userID, &user); err != nil {
			return lift.NotFound("User not found").WithCause(err)
		}

		// Verify tenant isolation
		if user.TenantID != ctx.TenantID() {
			return lift.NotFound("User not found")
		}

		return ctx.JSON(UserResponse{
			User:    &user,
			Message: "User retrieved successfully",
		})
	})

	// Update user endpoint
	app.PUT("/users/:id", func(ctx *lift.Context) error {
		userID := ctx.Param("id")

		var req CreateUserRequest
		if err := ctx.ParseRequest(&req); err != nil {
			return lift.BadRequest("Invalid request body").WithCause(err)
		}

		// Get DynamORM instance from context
		db, err := dynamorm.TenantDB(ctx)
		if err != nil {
			return err
		}

		// Retrieve existing user
		var user User
		if err := db.Get(ctx.Context, userID, &user); err != nil {
			return lift.NotFound("User not found").WithCause(err)
		}

		// Verify tenant isolation
		if user.TenantID != ctx.TenantID() {
			return lift.NotFound("User not found")
		}

		// Update user fields
		user.Email = req.Email
		user.Name = req.Name
		user.UpdatedAt = time.Now()

		// Save updated user
		if err := db.Put(ctx.Context, &user); err != nil {
			return lift.InternalError("Failed to update user").WithCause(err)
		}

		return ctx.JSON(UserResponse{
			User:    &user,
			Message: "User updated successfully",
		})
	})

	// Delete user endpoint
	app.DELETE("/users/:id", func(ctx *lift.Context) error {
		userID := ctx.Param("id")

		// Get DynamORM instance from context
		db, err := dynamorm.TenantDB(ctx)
		if err != nil {
			return err
		}

		// Verify user exists and belongs to tenant
		var user User
		if err := db.Get(ctx.Context, userID, &user); err != nil {
			return lift.NotFound("User not found").WithCause(err)
		}

		if user.TenantID != ctx.TenantID() {
			return lift.NotFound("User not found")
		}

		// Delete user
		if err := db.Delete(ctx.Context, userID); err != nil {
			return lift.InternalError("Failed to delete user").WithCause(err)
		}

		return ctx.JSON(map[string]string{
			"message": "User deleted successfully",
		})
	})

	// List users for tenant
	app.GET("/users", func(ctx *lift.Context) error {
		// Get DynamORM instance from context
		db, err := dynamorm.TenantDB(ctx)
		if err != nil {
			return err
		}

		// Query users for this tenant
		query := &dynamorm.Query{
			PartitionKey: ctx.TenantID(),
			IndexName:    "GSI1", // Assuming GSI1 is set up for tenant queries
			Limit:        50,
		}

		result, err := db.Query(ctx.Context, query)
		if err != nil {
			return lift.InternalError("Failed to list users").WithCause(err)
		}

		return ctx.JSON(map[string]interface{}{
			"users": result.Items,
			"count": result.Count,
		})
	})

	// Start the application
	if err := app.Start(); err != nil {
		panic(fmt.Sprintf("Failed to start app: %v", err))
	}

	fmt.Println("DynamORM Integration Demo started successfully!")
	fmt.Println("Endpoints:")
	fmt.Println("  GET    /health")
	fmt.Println("  POST   /users")
	fmt.Println("  GET    /users/:id")
	fmt.Println("  PUT    /users/:id")
	fmt.Println("  DELETE /users/:id")
	fmt.Println("  GET    /users")
	fmt.Println("")
	fmt.Println("Features demonstrated:")
	fmt.Println("  ✅ DynamORM integration")
	fmt.Println("  ✅ Tenant isolation")
	fmt.Println("  ✅ Automatic transactions")
	fmt.Println("  ✅ CRUD operations")
	fmt.Println("  ✅ Error handling")
}
