package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/pay-theory/lift/pkg/lift"
)

func main() {
	app := lift.New()

	// API Gateway routes
	app.GET("/hello", func(ctx *lift.Context) error {
		return ctx.JSON(map[string]interface{}{
			"message":     "Hello from API Gateway!",
			"triggerType": ctx.Request.TriggerType,
			"method":      ctx.Request.Method,
			"path":        ctx.Request.Path,
		})
	})

	app.POST("/users", func(ctx *lift.Context) error {
		var user map[string]interface{}
		if err := json.Unmarshal(ctx.Request.Body, &user); err != nil {
			return ctx.Status(400).JSON(map[string]string{"error": "Invalid JSON"})
		}

		return ctx.JSON(map[string]interface{}{
			"message":     "User created",
			"triggerType": ctx.Request.TriggerType,
			"user":        user,
		})
	})

	// SQS handler
	app.Handle("POST", "/sqs", func(ctx *lift.Context) error {
		if ctx.Request.TriggerType != lift.TriggerSQS {
			return ctx.Status(400).JSON(map[string]string{"error": "Expected SQS trigger"})
		}

		return ctx.JSON(map[string]interface{}{
			"message":     "SQS messages processed",
			"triggerType": ctx.Request.TriggerType,
			"recordCount": len(ctx.Request.Records),
			"source":      ctx.Request.Source,
		})
	})

	// S3 handler
	app.Handle("POST", "/s3", func(ctx *lift.Context) error {
		if ctx.Request.TriggerType != lift.TriggerS3 {
			return ctx.Status(400).JSON(map[string]string{"error": "Expected S3 trigger"})
		}

		return ctx.JSON(map[string]interface{}{
			"message":     "S3 events processed",
			"triggerType": ctx.Request.TriggerType,
			"recordCount": len(ctx.Request.Records),
			"source":      ctx.Request.Source,
			"detailType":  ctx.Request.DetailType,
		})
	})

	// EventBridge handler
	app.Handle("POST", "/eventbridge", func(ctx *lift.Context) error {
		if ctx.Request.TriggerType != lift.TriggerEventBridge {
			return ctx.Status(400).JSON(map[string]string{"error": "Expected EventBridge trigger"})
		}

		return ctx.JSON(map[string]interface{}{
			"message":     "EventBridge event processed",
			"triggerType": ctx.Request.TriggerType,
			"source":      ctx.Request.Source,
			"detailType":  ctx.Request.DetailType,
			"detail":      ctx.Request.Detail,
		})
	})

	// Scheduled event handler
	app.Handle("POST", "/scheduled", func(ctx *lift.Context) error {
		if ctx.Request.TriggerType != lift.TriggerScheduled {
			return ctx.Status(400).JSON(map[string]string{"error": "Expected Scheduled trigger"})
		}

		return ctx.JSON(map[string]interface{}{
			"message":     "Scheduled event processed",
			"triggerType": ctx.Request.TriggerType,
			"source":      ctx.Request.Source,
			"detailType":  ctx.Request.DetailType,
		})
	})

	if err := app.Start(); err != nil {
		log.Fatalf("Failed to start app: %v", err)
	}

	// Demonstrate different event types
	fmt.Println("=== Event Adapter Demo ===")

	// Test API Gateway V2 event
	fmt.Println("\n1. Testing API Gateway V2 Event:")
	apiGatewayV2Event := map[string]interface{}{
		"version":  "2.0",
		"routeKey": "GET /hello",
		"requestContext": map[string]interface{}{
			"requestId": "test-request-id",
			"http": map[string]interface{}{
				"method": "GET",
				"path":   "/hello",
			},
		},
		"headers": map[string]interface{}{
			"content-type": "application/json",
		},
	}

	resp, err := app.HandleRequest(context.Background(), apiGatewayV2Event)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Response: %+v\n", resp)
	}

	// Test SQS event
	fmt.Println("\n2. Testing SQS Event:")
	sqsEvent := map[string]interface{}{
		"Records": []interface{}{
			map[string]interface{}{
				"eventSource":   "aws:sqs",
				"body":          `{"orderId": "12345"}`,
				"receiptHandle": "test-receipt-handle",
				"messageId":     "test-message-id",
			},
			map[string]interface{}{
				"eventSource":   "aws:sqs",
				"body":          `{"orderId": "67890"}`,
				"receiptHandle": "test-receipt-handle-2",
				"messageId":     "test-message-id-2",
			},
		},
	}

	resp, err = app.HandleRequest(context.Background(), sqsEvent)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Response: %+v\n", resp)
	}

	// Test EventBridge event
	fmt.Println("\n3. Testing EventBridge Event:")
	eventBridgeEvent := map[string]interface{}{
		"source":      "myapp.orders",
		"detail-type": "Order Placed",
		"detail": map[string]interface{}{
			"orderId":    "12345",
			"customerId": "67890",
			"amount":     99.99,
		},
		"time": "2023-01-01T00:00:00Z",
		"id":   "test-event-id",
	}

	resp, err = app.HandleRequest(context.Background(), eventBridgeEvent)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Response: %+v\n", resp)
	}

	// Test S3 event
	fmt.Println("\n4. Testing S3 Event:")
	s3Event := map[string]interface{}{
		"Records": []interface{}{
			map[string]interface{}{
				"eventSource": "aws:s3",
				"eventName":   "ObjectCreated:Put",
				"s3": map[string]interface{}{
					"bucket": map[string]interface{}{
						"name": "test-bucket",
					},
					"object": map[string]interface{}{
						"key": "test-key.jpg",
					},
				},
			},
		},
	}

	resp, err = app.HandleRequest(context.Background(), s3Event)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Response: %+v\n", resp)
	}

	// Test Scheduled event
	fmt.Println("\n5. Testing Scheduled Event:")
	scheduledEvent := map[string]interface{}{
		"source":      "aws.events",
		"detail-type": "Scheduled Event",
		"time":        "2023-01-01T00:00:00Z",
		"id":          "test-event-id",
		"resources":   []interface{}{"arn:aws:events:us-east-1:123456789012:rule/my-rule"},
	}

	resp, err = app.HandleRequest(context.Background(), scheduledEvent)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Response: %+v\n", resp)
	}

	fmt.Println("\n=== Demo Complete ===")
}
