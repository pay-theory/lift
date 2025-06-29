package main

import (
	"encoding/json"
	"log"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/pay-theory/lift/pkg/lift"
)

func main() {
	app := lift.New()

	// Add basic logging middleware
	app.Use(func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			log.Printf("Event type: %s", ctx.Request.TriggerType)
			return next.Handle(ctx)
		})
	})

	// Handle HTTP requests from API Gateway
	app.GET("/status", func(ctx *lift.Context) error {
		return ctx.JSON(map[string]string{
			"status": "healthy",
			"handler": "multi-event",
		})
	})

	// Handle SQS messages
	app.SQS("process-order", func(ctx *lift.Context) error {
		log.Println("Processing SQS message")
		
		// Parse message body
		var order map[string]interface{}
		if err := json.Unmarshal(ctx.Request.Body, &order); err != nil {
			return lift.NewLiftError("INVALID_MESSAGE", "Failed to parse order", 400).WithCause(err)
		}

		log.Printf("Processing order: %+v", order)
		return nil
	})

	// Handle S3 events
	app.S3("file-uploaded", func(ctx *lift.Context) error {
		log.Println("Processing S3 event")
		
		// Get S3 event details from context
		event := ctx.Request.RawEvent
		log.Printf("S3 event: %+v", event)
		
		return nil
	})

	// Handle EventBridge events
	app.EventBridge("user-signup", func(ctx *lift.Context) error {
		log.Println("Processing EventBridge user signup event")
		
		// Process user signup
		var userData map[string]interface{}
		if err := json.Unmarshal(ctx.Request.Body, &userData); err != nil {
			return lift.NewLiftError("INVALID_EVENT", "Failed to parse user data", 400).WithCause(err)
		}

		log.Printf("New user signup: %+v", userData)
		return nil
	})

	// Handle DynamoDB Streams
	app.Handle("DynamoDBStreams", "user-table-stream", func(ctx *lift.Context) error {
		log.Println("Processing DynamoDB stream event")
		
		// Process stream records
		event := ctx.Request.RawEvent
		log.Printf("DynamoDB stream event: %+v", event)
		
		return nil
	})

	// Start Lambda handler - it will automatically route to the correct handler
	// based on the incoming event type
	lambda.Start(app.HandleRequest)
}