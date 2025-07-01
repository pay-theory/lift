package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/middleware"
)

// PaymentIntentRequest represents a payment creation request
type PaymentIntentRequest struct {
	Amount      int64  `json:"amount" validate:"required,min=1"`
	Currency    string `json:"currency" validate:"required,len=3"`
	Description string `json:"description,omitempty"`
}

// PaymentIntentResponse represents a payment intent
type PaymentIntentResponse struct {
	ID          string `json:"id"`
	Amount      int64  `json:"amount"`
	Currency    string `json:"currency"`
	Status      string `json:"status"`
	Description string `json:"description,omitempty"`
	CreatedAt   int64  `json:"created_at"`
}

func main() {
	// Create Lift app
	app := lift.New()

	// Setup idempotency store
	idempotencyStore, err := setupIdempotencyStore()
	if err != nil {
		log.Fatal("Failed to setup idempotency store:", err)
	}

	// Add middleware - convert from middleware.Middleware to lift.Middleware
	app.Use(func(next lift.Handler) lift.Handler {
		return middleware.ErrorHandler()(next)
	})
	app.Use(func(next lift.Handler) lift.Handler {
		return middleware.Logger()(next)
	})
	app.Use(func(next lift.Handler) lift.Handler {
		return middleware.Idempotency(middleware.IdempotencyOptions{
		Store:              idempotencyStore,
		HeaderName:         "Idempotency-Key",
		TTL:                24 * 60 * 60, // 24 hours
		ProcessingTimeout:  30,           // 30 seconds
		IncludeRequestHash: true,         // Validate request body hasn't changed
		OnDuplicate: func(ctx *lift.Context, record *middleware.IdempotencyRecord) {
			// Log duplicate request
			if ctx.Logger != nil {
				ctx.Logger.Info("Duplicate request detected", map[string]any{
					"idempotency_key": record.Key,
					"original_created": record.CreatedAt,
					"status": record.Status,
				})
			}
		},
	})(next)
	})

	// Payment intent creation endpoint
	app.POST("/v1/payment_intents", lift.SimpleHandler(createPaymentIntent))

	// Health check endpoint (no idempotency needed)
	app.GET("/health", func(ctx *lift.Context) error {
		return ctx.JSON(map[string]string{
			"status": "healthy",
			"service": "payment-api",
		})
	})

	// Start the application
	if err := app.Start(); err != nil {
		log.Fatal("Failed to start app:", err)
	}
}

func createPaymentIntent(ctx *lift.Context, req PaymentIntentRequest) (PaymentIntentResponse, error) {
	// Simulate payment processing
	// In a real application, this would call your payment processor
	
	// Generate payment intent ID
	paymentID := "pi_" + generateID()
	
	// Log the creation
	if ctx.Logger != nil {
		ctx.Logger.Info("Creating payment intent", map[string]any{
			"payment_id": paymentID,
			"amount": req.Amount,
			"currency": req.Currency,
		})
	}

	// Return response
	return PaymentIntentResponse{
		ID:          paymentID,
		Amount:      req.Amount,
		Currency:    req.Currency,
		Status:      "requires_payment_method",
		Description: req.Description,
		CreatedAt:   time.Now().Unix(),
	}, nil
}

func setupIdempotencyStore() (middleware.IdempotencyStore, error) {
	// Check if we're running locally or in Lambda
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") == "" {
		// Local development - use in-memory store
		log.Println("Using in-memory idempotency store for local development")
		return middleware.NewMemoryIdempotencyStore(), nil
	}

	// Production - use DynamoDB
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, err
	}

	client := dynamodb.NewFromConfig(cfg)
	tableName := os.Getenv("IDEMPOTENCY_TABLE_NAME")
	if tableName == "" {
		tableName = "idempotency-keys"
	}

	log.Printf("Using DynamoDB idempotency store with table: %s", tableName)
	return middleware.NewDynamoDBIdempotencyStore(client, tableName), nil
}

func generateID() string {
	// Simple ID generation - in production use a proper UUID library
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// Example usage:
//
// First request:
// curl -X POST http://localhost:8080/v1/payment_intents \
//   -H "Content-Type: application/json" \
//   -H "Idempotency-Key: unique-request-123" \
//   -d '{"amount": 1000, "currency": "USD", "description": "Test payment"}'
//
// Response:
// {
//   "id": "pi_1234567890",
//   "amount": 1000,
//   "currency": "USD",
//   "status": "requires_payment_method",
//   "description": "Test payment",
//   "created_at": 1234567890
// }
//
// Second request with same idempotency key returns cached response:
// curl -X POST http://localhost:8080/v1/payment_intents \
//   -H "Content-Type: application/json" \
//   -H "Idempotency-Key: unique-request-123" \
//   -H "X-Debug: true" \
//   -d '{"amount": 1000, "currency": "USD", "description": "Test payment"}'
//
// Response includes header: X-Idempotent-Replay: true