package lift

import (
	"context"
	"testing"
)

func TestNonHTTPEventRouting(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		path        string
		event       interface{}
		wantStatus  int
		wantCalled  bool
	}{
		{
			name:   "EventBridge event routes correctly",
			method: "EventBridge",
			path:   "myapp.users",
			event: map[string]interface{}{
				"version":     "0",
				"id":          "eb-123",
				"detail-type": "User Created",
				"source":      "myapp.users",
				"time":        "2023-10-04T12:00:00Z",
				"detail":      map[string]interface{}{"userId": "123"},
			},
			wantStatus: 200,
			wantCalled: true,
		},
		{
			name:   "S3 event routes correctly",
			method: "S3",
			path:   "my-bucket",
			event: map[string]interface{}{
				"Records": []interface{}{
					map[string]interface{}{
						"eventSource": "aws:s3",
						"eventName":   "ObjectCreated:Put",
						"s3": map[string]interface{}{
							"bucket": map[string]interface{}{
								"name": "my-bucket",
							},
							"object": map[string]interface{}{
								"key": "test.jpg",
							},
						},
					},
				},
			},
			wantStatus: 200,
			wantCalled: true,
		},
		{
			name:   "SQS event routes correctly",
			method: "SQS",
			path:   "my-queue",
			event: map[string]interface{}{
				"Records": []interface{}{
					map[string]interface{}{
						"eventSource":    "aws:sqs",
						"eventSourceARN": "arn:aws:sqs:us-east-1:123456789012:my-queue",
						"body":           "test message",
						"messageId":      "123",
						"receiptHandle":  "handle",
					},
				},
			},
			wantStatus: 200,
			wantCalled: true,
		},
		{
			name:   "Scheduled event routes as EventBridge",
			method: "EventBridge",
			path:   "my-rule",
			event: map[string]interface{}{
				"version":     "0",
				"id":          "scheduled-123",
				"detail-type": "Scheduled Event",
				"source":      "aws.events",
				"time":        "2023-10-04T12:00:00Z",
				"resources": []interface{}{
					"arn:aws:events:us-east-1:123456789012:rule/my-rule",
				},
				"detail": map[string]interface{}{},
			},
			wantStatus: 200,
			wantCalled: true,
		},
		{
			name:   "S3 event through EventBridge routes to S3 handler",
			method: "S3",
			path:   "*",
			event: map[string]interface{}{
				"version":     "0",
				"id":          "s3-eb-123",
				"detail-type": "Object Created:Put",
				"source":      "aws.s3",
				"time":        "2023-10-04T12:00:00Z",
				"resources": []interface{}{
					"arn:aws:s3:::my-bucket",
				},
				"detail": map[string]interface{}{
					"bucket": map[string]interface{}{
						"name": "my-bucket",
					},
					"object": map[string]interface{}{
						"key": "test.jpg",
						"size": 12345,
					},
				},
			},
			wantStatus: 200,
			wantCalled: true,
		},
		{
			name:   "S3 object key pattern matching",
			method: "S3",
			path:   "/uploads/*",
			event: map[string]interface{}{
				"version":     "0",
				"id":          "s3-eb-456",
				"detail-type": "Object Created:Put",
				"source":      "aws.s3",
				"time":        "2023-10-04T12:00:00Z",
				"resources": []interface{}{
					"arn:aws:s3:::my-bucket",
				},
				"detail": map[string]interface{}{
					"bucket": map[string]interface{}{
						"name": "my-bucket",
					},
					"object": map[string]interface{}{
						"key":  "uploads/file.txt",
						"size": 12345,
					},
				},
			},
			wantStatus: 200,
			wantCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := New()
			called := false

			// Register handler
			app.Handle(tt.method, tt.path, func(ctx *Context) error {
				called = true
				return ctx.JSON(map[string]string{"status": "ok"})
			})

			// Process event
			resp, err := app.HandleRequest(context.Background(), tt.event)
			if err != nil {
				t.Fatalf("HandleRequest() error = %v", err)
			}

			// Check if handler was called
			if called != tt.wantCalled {
				t.Errorf("Handler called = %v, want %v", called, tt.wantCalled)
			}

			// Check response status
			if respMap, ok := resp.(map[string]interface{}); ok {
				if status, ok := respMap["statusCode"].(int); ok && status != tt.wantStatus {
					t.Errorf("Response status = %v, want %v", status, tt.wantStatus)
				}
			}
		})
	}
}

func TestEventRouterIntegration(t *testing.T) {
	t.Run("EventRouter is initialized", func(t *testing.T) {
		app := New()
		if app.eventRouter == nil {
			t.Error("EventRouter should be initialized")
		}
	})

	t.Run("Non-HTTP events use EventRouter", func(t *testing.T) {
		app := New()
		handlerCalled := false

		// Register a non-HTTP handler
		app.Handle("EventBridge", "test.source", func(ctx *Context) error {
			handlerCalled = true
			if ctx.Request.TriggerType != TriggerEventBridge {
				t.Errorf("Expected TriggerType = %v, got %v", TriggerEventBridge, ctx.Request.TriggerType)
			}
			return ctx.JSON(map[string]string{"status": "ok"})
		})

		// Check that the route was added to eventRouter
		routes := app.eventRouter.GetRoutes()
		if len(routes[TriggerEventBridge]) != 1 {
			t.Errorf("Expected 1 EventBridge route, got %d", len(routes[TriggerEventBridge]))
		}

		// Test handling the event
		event := map[string]interface{}{
			"version":     "0",
			"id":          "test-123",
			"detail-type": "Test Event",
			"source":      "test.source",
			"time":        "2023-10-04T12:00:00Z",
			"detail":      map[string]interface{}{},
		}

		_, err := app.HandleRequest(context.Background(), event)
		if err != nil {
			t.Fatalf("HandleRequest() error = %v", err)
		}

		if !handlerCalled {
			t.Error("Handler should have been called")
		}
	})

	t.Run("HTTP events use regular Router", func(t *testing.T) {
		app := New()

		// Register an HTTP handler
		app.GET("/test", func(ctx *Context) error {
			return ctx.JSON(map[string]string{"status": "ok"})
		})

		// Check that eventRouter doesn't have HTTP routes
		routes := app.eventRouter.GetRoutes()
		if len(routes) != 0 {
			t.Errorf("EventRouter should not have HTTP routes, got %d trigger types", len(routes))
		}
	})
}