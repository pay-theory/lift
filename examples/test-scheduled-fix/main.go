package main

import (
	"encoding/json"
	"log"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/pay-theory/lift/pkg/lift"
)

// This example demonstrates the fix for handling scheduled events
// that come from EventBridge (formerly CloudWatch Events)
func main() {
	app := lift.New()

	// Add simple logging and error recovery
	app.Use(func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			log.Printf("Processing request: %s", ctx.Request.Path)
			err := next.Handle(ctx)
			if err != nil {
				log.Printf("Request error: %v", err)
			}
			return err
		})
	})

	// Test endpoint for HTTP requests
	app.GET("/test", func(ctx *lift.Context) error {
		return ctx.JSON(map[string]string{
			"status": "ok",
			"message": "Test scheduled fix example",
		})
	})

	// Handle scheduled events from EventBridge
	// The pattern matches any scheduled event
	app.EventBridge("scheduled-test", func(ctx *lift.Context) error {
		log.Println("Scheduled event received")

		// Log the raw event for debugging
		eventJSON, _ := json.MarshalIndent(ctx.Request.RawEvent, "", "  ")
		log.Printf("Raw event:\n%s", eventJSON)

		// Parse EventBridge event structure
		eventMap, ok := ctx.Request.RawEvent.(map[string]interface{})
		if !ok {
			return lift.NewLiftError("PARSE_ERROR", "Failed to parse EventBridge event", 400)
		}

		// Extract event details
		detailType, _ := eventMap["detail-type"].(string)
		source, _ := eventMap["source"].(string)
		eventTime, _ := eventMap["time"].(string)
		
		log.Printf("Event details - Type: %s, Source: %s, Time: %s", detailType, source, eventTime)

		// Handle scheduled event
		if source == "aws.events" && detailType == "Scheduled Event" {
			return handleScheduledTask(ctx, eventMap)
		}

		return ctx.JSON(map[string]interface{}{
			"status": "processed",
			"eventType": detailType,
			"source": source,
		})
	})

	// Start Lambda handler
	lambda.Start(app.HandleRequest)
}

func handleScheduledTask(ctx *lift.Context, event map[string]interface{}) error {
	log.Println("Processing scheduled task")

	// Extract custom detail data if present
	detail, ok := event["detail"].(map[string]interface{})
	if ok && len(detail) > 0 {
		log.Printf("Custom event detail: %+v", detail)
		
		// Process based on custom detail
		taskType, _ := detail["taskType"].(string)
		switch taskType {
		case "cleanup":
			return performCleanup(ctx)
		case "report":
			return generateReport(ctx)
		default:
			log.Printf("Unknown task type: %s", taskType)
		}
	}

	// Default scheduled task processing
	return ctx.JSON(map[string]interface{}{
		"status": "completed",
		"message": "Scheduled task processed successfully",
		"timestamp": event["time"],
	})
}

func performCleanup(ctx *lift.Context) error {
	log.Println("Performing cleanup tasks")
	// Add cleanup logic here
	return ctx.JSON(map[string]string{
		"task": "cleanup",
		"status": "completed",
	})
}

func generateReport(ctx *lift.Context) error {
	log.Println("Generating report")
	// Add report generation logic here
	return ctx.JSON(map[string]string{
		"task": "report",
		"status": "completed",
	})
}