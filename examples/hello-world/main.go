package main

import (
	"fmt"

	"github.com/pay-theory/lift/pkg/lift"
)

// UserRequest represents a simple request structure
type UserRequest struct {
	Name string `json:"name" validate:"required"`
	Age  int    `json:"age" validate:"min=0,max=120"`
}

// UserResponse represents a simple response structure
type UserResponse struct {
	Message  string `json:"message"`
	UserID   string `json:"user_id,omitempty"`
	TenantID string `json:"tenant_id,omitempty"`
}

func main() {
	// Create a new Lift application
	app := lift.New()

	// Basic health check endpoint
	app.GET("/health", func(ctx *lift.Context) error {
		return ctx.JSON(map[string]string{
			"status":  "healthy",
			"service": "lift-demo",
		})
	})

	// Simple hello world endpoint
	app.GET("/hello", func(ctx *lift.Context) error {
		name := ctx.Query("name")
		if name == "" {
			name = "World"
		}

		return ctx.JSON(map[string]string{
			"message": fmt.Sprintf("Hello, %s!", name),
			"tenant":  ctx.TenantID(),
		})
	})

	// Type-safe handler example
	app.POST("/users", lift.SimpleHandler(func(ctx *lift.Context, req UserRequest) (UserResponse, error) {
		// Simulate user creation logic
		userID := "user_123"

		return UserResponse{
			Message:  fmt.Sprintf("User %s created successfully", req.Name),
			UserID:   userID,
			TenantID: ctx.TenantID(),
		}, nil
	}))

	// Path parameter example
	app.GET("/users/:id", func(ctx *lift.Context) error {
		userID := ctx.Param("id")

		return ctx.JSON(map[string]any{
			"user_id": userID,
			"name":    fmt.Sprintf("User %s", userID),
			"tenant":  ctx.TenantID(),
		})
	})

	// Error handling example
	app.POST("/error", func(ctx *lift.Context) error {
		return fmt.Errorf("this is a demo error")
	})

	// Start the application
	if err := app.Start(); err != nil {
		panic(fmt.Sprintf("Failed to start app: %v", err))
	}

	// For demonstration - in real Lambda, this would be called by the runtime
	// Here we show how you would use HandleRequest
	fmt.Println("Lift application ready!")
	fmt.Println("To use in Lambda, call: app.HandleRequest(ctx, event)")

	// Example usage (would normally be done by Lambda runtime)
	// resp, err := app.HandleRequest(context.Background(), mockEvent)
}
