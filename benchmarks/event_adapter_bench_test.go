package benchmarks

import (
	"encoding/json"
	"testing"

	"github.com/pay-theory/lift/pkg/lift/adapters"
)

// Sample event payloads for benchmarking
var (
	apiGatewayV1Event = map[string]interface{}{
		"resource":                        "/users/{id}",
		"path":                            "/users/123",
		"httpMethod":                      "GET",
		"headers":                         map[string]interface{}{"Content-Type": "application/json"},
		"multiValueHeaders":               map[string]interface{}{},
		"queryStringParameters":           map[string]interface{}{"filter": "active"},
		"multiValueQueryStringParameters": map[string]interface{}{},
		"pathParameters":                  map[string]interface{}{"id": "123"},
		"stageVariables":                  map[string]interface{}{},
		"requestContext": map[string]interface{}{
			"requestId":  "test-request-id",
			"stage":      "prod",
			"httpMethod": "GET",
		},
		"body":            `{"name": "test"}`,
		"isBase64Encoded": false,
	}

	apiGatewayV2Event = map[string]interface{}{
		"version":               "2.0",
		"routeKey":              "GET /users/{id}",
		"rawPath":               "/users/123",
		"rawQueryString":        "filter=active",
		"headers":               map[string]interface{}{"content-type": "application/json"},
		"queryStringParameters": map[string]interface{}{"filter": "active"},
		"pathParameters":        map[string]interface{}{"id": "123"},
		"requestContext": map[string]interface{}{
			"requestId": "test-request-id",
			"http": map[string]interface{}{
				"method": "GET",
				"path":   "/users/123",
			},
		},
		"body":            `{"name": "test"}`,
		"isBase64Encoded": false,
	}

	sqsEvent = map[string]interface{}{
		"Records": []interface{}{
			map[string]interface{}{
				"messageId":     "test-message-1",
				"receiptHandle": "test-receipt-1",
				"body":          `{"type": "user_created", "data": {"id": "123"}}`,
				"attributes": map[string]interface{}{
					"ApproximateReceiveCount": "1",
				},
				"messageAttributes": map[string]interface{}{},
				"md5OfBody":         "test-md5",
				"eventSource":       "aws:sqs",
				"eventSourceARN":    "arn:aws:sqs:us-east-1:123456789012:test-queue",
				"awsRegion":         "us-east-1",
			},
			map[string]interface{}{
				"messageId":     "test-message-2",
				"receiptHandle": "test-receipt-2",
				"body":          `{"type": "user_updated", "data": {"id": "456"}}`,
				"attributes": map[string]interface{}{
					"ApproximateReceiveCount": "1",
				},
				"messageAttributes": map[string]interface{}{},
				"md5OfBody":         "test-md5-2",
				"eventSource":       "aws:sqs",
				"eventSourceARN":    "arn:aws:sqs:us-east-1:123456789012:test-queue",
				"awsRegion":         "us-east-1",
			},
		},
	}

	s3Event = map[string]interface{}{
		"Records": []interface{}{
			map[string]interface{}{
				"eventVersion": "2.1",
				"eventSource":  "aws:s3",
				"awsRegion":    "us-east-1",
				"eventTime":    "2023-01-01T00:00:00.000Z",
				"eventName":    "ObjectCreated:Put",
				"s3": map[string]interface{}{
					"s3SchemaVersion": "1.0",
					"configurationId": "test-config",
					"bucket": map[string]interface{}{
						"name": "test-bucket",
						"arn":  "arn:aws:s3:::test-bucket",
					},
					"object": map[string]interface{}{
						"key":  "test-file.json",
						"size": 1024,
						"eTag": "test-etag",
					},
				},
			},
		},
	}

	eventBridgeEvent = map[string]interface{}{
		"version":     "0",
		"id":          "test-event-id",
		"detail-type": "User Action",
		"source":      "myapp.users",
		"account":     "123456789012",
		"time":        "2023-01-01T00:00:00Z",
		"region":      "us-east-1",
		"detail": map[string]interface{}{
			"action": "created",
			"userId": "123",
		},
	}

	scheduledEvent = map[string]interface{}{
		"id":          "test-scheduled-event",
		"detail-type": "Scheduled Event",
		"source":      "aws.events",
		"account":     "123456789012",
		"time":        "2023-01-01T00:00:00Z",
		"region":      "us-east-1",
		"detail":      map[string]interface{}{},
		"resources":   []interface{}{"arn:aws:events:us-east-1:123456789012:rule/test-rule"},
	}
)

// BenchmarkAPIGatewayV1Adapter tests API Gateway V1 event parsing performance
func BenchmarkAPIGatewayV1Adapter(b *testing.B) {
	adapter := adapters.NewAPIGatewayAdapter()
	benchmarkEventAdapter(b, adapter, apiGatewayV1Event)
}

// BenchmarkAPIGatewayV2Adapter tests API Gateway V2 event parsing performance
func BenchmarkAPIGatewayV2Adapter(b *testing.B) {
	adapter := adapters.NewAPIGatewayV2Adapter()
	benchmarkEventAdapter(b, adapter, apiGatewayV2Event)
}

// BenchmarkSQSAdapter tests SQS event parsing performance
func BenchmarkSQSAdapter(b *testing.B) {
	adapter := adapters.NewSQSAdapter()
	benchmarkEventAdapter(b, adapter, sqsEvent)
}

// BenchmarkS3Adapter tests S3 event parsing performance
func BenchmarkS3Adapter(b *testing.B) {
	adapter := adapters.NewS3Adapter()
	benchmarkEventAdapter(b, adapter, s3Event)
}

// BenchmarkEventBridgeAdapter tests EventBridge event parsing performance
func BenchmarkEventBridgeAdapter(b *testing.B) {
	adapter := adapters.NewEventBridgeAdapter()
	benchmarkEventAdapter(b, adapter, eventBridgeEvent)
}

// BenchmarkScheduledAdapter tests Scheduled event parsing performance
func BenchmarkScheduledAdapter(b *testing.B) {
	adapter := adapters.NewScheduledAdapter()
	benchmarkEventAdapter(b, adapter, scheduledEvent)
}

// BenchmarkEventDetection tests automatic event type detection performance
func BenchmarkEventDetection(b *testing.B) {
	registry := adapters.NewAdapterRegistry()

	events := []interface{}{
		apiGatewayV1Event,
		apiGatewayV2Event,
		sqsEvent,
		s3Event,
		eventBridgeEvent,
		scheduledEvent,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		event := events[i%len(events)]
		_, err := registry.DetectAndAdapt(event)
		if err != nil {
			b.Fatalf("Failed to detect and adapt event: %v", err)
		}
	}
}

// BenchmarkEventDetectionWorstCase tests detection when event is checked last
func BenchmarkEventDetectionWorstCase(b *testing.B) {
	registry := adapters.NewAdapterRegistry()

	// Use scheduled event which might be checked last
	event := scheduledEvent

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := registry.DetectAndAdapt(event)
		if err != nil {
			b.Fatalf("Failed to detect and adapt event: %v", err)
		}
	}
}

// BenchmarkLargeEventParsing tests parsing performance with large events
func BenchmarkLargeEventParsing(b *testing.B) {
	// Create a large SQS event with many records
	largeEvent := map[string]interface{}{
		"Records": make([]interface{}, 100),
	}

	for i := 0; i < 100; i++ {
		largeEvent["Records"].([]interface{})[i] = map[string]interface{}{
			"messageId":     "test-message-" + string(rune(i)),
			"receiptHandle": "test-receipt-" + string(rune(i)),
			"body":          `{"type": "large_event", "data": {"id": "` + string(rune(i)) + `", "payload": "` + string(make([]byte, 1024)) + `"}}`,
			"attributes": map[string]interface{}{
				"ApproximateReceiveCount": "1",
			},
			"messageAttributes": map[string]interface{}{},
			"md5OfBody":         "test-md5-" + string(rune(i)),
			"eventSource":       "aws:sqs",
			"eventSourceARN":    "arn:aws:sqs:us-east-1:123456789012:test-queue",
			"awsRegion":         "us-east-1",
		}
	}

	adapter := adapters.NewSQSAdapter()
	benchmarkEventAdapter(b, adapter, largeEvent)
}

// BenchmarkEventValidation tests event validation performance
func BenchmarkEventValidation(b *testing.B) {
	adapter := adapters.NewAPIGatewayAdapter()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		err := adapter.Validate(apiGatewayV1Event)
		if err != nil {
			b.Fatalf("Validation failed: %v", err)
		}
	}
}

// BenchmarkEventCanHandle tests CanHandle method performance
func BenchmarkEventCanHandle(b *testing.B) {
	adapters := []adapters.EventAdapter{
		adapters.NewAPIGatewayAdapter(),
		adapters.NewAPIGatewayV2Adapter(),
		adapters.NewSQSAdapter(),
		adapters.NewS3Adapter(),
		adapters.NewEventBridgeAdapter(),
		adapters.NewScheduledAdapter(),
	}

	events := []interface{}{
		apiGatewayV1Event,
		apiGatewayV2Event,
		sqsEvent,
		s3Event,
		eventBridgeEvent,
		scheduledEvent,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		event := events[i%len(events)]
		for _, adapter := range adapters {
			_ = adapter.CanHandle(event)
		}
	}
}

// BenchmarkJSONMarshaling tests JSON marshaling overhead in event processing
func BenchmarkJSONMarshaling(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Simulate JSON marshaling that might happen in event processing
		data, err := json.Marshal(apiGatewayV1Event)
		if err != nil {
			b.Fatalf("JSON marshaling failed: %v", err)
		}

		var unmarshaled map[string]interface{}
		err = json.Unmarshal(data, &unmarshaled)
		if err != nil {
			b.Fatalf("JSON unmarshaling failed: %v", err)
		}
	}
}

// BenchmarkConcurrentEventParsing tests event parsing under concurrent load
func BenchmarkConcurrentEventParsing(b *testing.B) {
	registry := adapters.NewAdapterRegistry()

	events := []interface{}{
		apiGatewayV1Event,
		apiGatewayV2Event,
		sqsEvent,
		s3Event,
		eventBridgeEvent,
		scheduledEvent,
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			event := events[0] // Use first event for consistency
			_, err := registry.DetectAndAdapt(event)
			if err != nil {
				b.Fatalf("Failed to detect and adapt event: %v", err)
			}
		}
	})
}

// Helper function
func benchmarkEventAdapter(b *testing.B, adapter adapters.EventAdapter, event interface{}) {
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := adapter.Adapt(event)
		if err != nil {
			b.Fatalf("Failed to adapt event: %v", err)
		}
	}
}
