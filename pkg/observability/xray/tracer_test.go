package xray

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/assert"
)

func TestNewXRayTracer(t *testing.T) {
	tests := []struct {
		name     string
		config   XRayConfig
		expected XRayConfig
	}{
		{
			name:   "default values",
			config: XRayConfig{},
			expected: XRayConfig{
				ServiceName:  "lift-service",
				SamplingRate: 0.1,
				Annotations:  map[string]string{},
				Metadata:     map[string]string{},
			},
		},
		{
			name: "custom values",
			config: XRayConfig{
				ServiceName:  "payment-api",
				SamplingRate: 0.5,
				Annotations:  map[string]string{"version": "1.0"},
				Metadata:     map[string]string{"team": "payments"},
			},
			expected: XRayConfig{
				ServiceName:  "payment-api",
				SamplingRate: 0.5,
				Annotations:  map[string]string{"version": "1.0"},
				Metadata:     map[string]string{"team": "payments"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracer := NewXRayTracer(tt.config)
			assert.Equal(t, tt.expected.ServiceName, tracer.config.ServiceName)
			assert.Equal(t, tt.expected.SamplingRate, tracer.config.SamplingRate)
			assert.Equal(t, tt.expected.Annotations, tracer.config.Annotations)
			assert.Equal(t, tt.expected.Metadata, tracer.config.Metadata)
		})
	}
}

func TestXRayMiddleware(t *testing.T) {
	// Configure X-Ray for testing
	xray.Configure(xray.Config{
		DaemonAddr:     "127.0.0.1:2000", // Non-existent daemon for testing
		LogLevel:       "silent",
		ServiceVersion: "test",
	})

	config := XRayConfig{
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Environment:    "test",
		Annotations: map[string]string{
			"custom": "annotation",
		},
		Metadata: map[string]string{
			"custom": "metadata",
		},
	}

	middleware := XRayMiddleware(config)

	// Create test handler
	handler := lift.HandlerFunc(func(ctx *lift.Context) error {
		// Verify trace context is available
		traceID := GetTraceID(ctx.Context)
		segmentID := GetSegmentID(ctx.Context)

		assert.NotEmpty(t, traceID)
		assert.NotEmpty(t, segmentID)

		// Test adding custom annotations and metadata
		AddAnnotation(ctx.Context, "test.annotation", "value")
		AddMetadata(ctx.Context, "test", "metadata", "value")

		return nil
	})

	wrappedHandler := middleware(handler)

	// Create test context
	ctx := createTestContext()

	// Execute handler
	err := wrappedHandler.Handle(ctx)
	assert.NoError(t, err)

	// Verify trace headers were added
	assert.NotEmpty(t, ctx.Request.Headers["X-Trace-Id"])
	assert.NotEmpty(t, ctx.Request.Headers["X-Span-Id"])
}

func TestXRayMiddleware_WithError(t *testing.T) {
	// Configure X-Ray for testing
	xray.Configure(xray.Config{
		DaemonAddr: "127.0.0.1:2000",
		LogLevel:   "silent",
	})

	config := XRayConfig{
		ServiceName: "test-service",
	}

	middleware := XRayMiddleware(config)

	// Create test handler that returns an error
	testError := errors.New("test error")
	handler := lift.HandlerFunc(func(ctx *lift.Context) error {
		return testError
	})

	wrappedHandler := middleware(handler)

	// Create test context
	ctx := createTestContext()

	// Execute handler
	err := wrappedHandler.Handle(ctx)
	assert.Equal(t, testError, err)
}

func TestTraceDynamoDBOperation(t *testing.T) {
	// Configure X-Ray for testing
	xray.Configure(xray.Config{
		DaemonAddr: "127.0.0.1:2000",
		LogLevel:   "silent",
	})

	// Create a context with a segment
	ctx, segment := xray.BeginSegment(context.Background(), "test")
	defer segment.Close(nil)

	// Test DynamoDB operation tracing
	tracedCtx, finish := TraceDynamoDBOperation(ctx, "GetItem", "users")
	defer finish()

	// Verify context is updated
	assert.NotEqual(t, ctx, tracedCtx)

	// Test with no segment (should not panic)
	noSegmentCtx := context.Background()
	tracedCtx2, finish2 := TraceDynamoDBOperation(noSegmentCtx, "PutItem", "orders")
	defer finish2()

	assert.Equal(t, noSegmentCtx, tracedCtx2)
}

func TestTraceHTTPCall(t *testing.T) {
	// Configure X-Ray for testing
	xray.Configure(xray.Config{
		DaemonAddr: "127.0.0.1:2000",
		LogLevel:   "silent",
	})

	// Create a context with a segment
	ctx, segment := xray.BeginSegment(context.Background(), "test")
	defer segment.Close(nil)

	// Test HTTP call tracing
	tracedCtx, finish := TraceHTTPCall(ctx, "GET", "https://api.example.com/users")

	// Simulate HTTP call completion
	finish(200, nil)

	// Verify context is updated
	assert.NotEqual(t, ctx, tracedCtx)

	// Test with error
	tracedCtx2, finish2 := TraceHTTPCall(ctx, "POST", "https://api.example.com/orders")
	finish2(500, errors.New("server error"))

	assert.NotEqual(t, ctx, tracedCtx2)
}

func TestTraceCustomOperation(t *testing.T) {
	// Configure X-Ray for testing
	xray.Configure(xray.Config{
		DaemonAddr: "127.0.0.1:2000",
		LogLevel:   "silent",
	})

	// Create a context with a segment
	ctx, segment := xray.BeginSegment(context.Background(), "test")
	defer segment.Close(nil)

	// Test custom operation tracing
	metadata := map[string]interface{}{
		"user_id":   "123",
		"operation": "payment_processing",
		"amount":    100.50,
	}

	tracedCtx, finish := TraceCustomOperation(ctx, "ProcessPayment", metadata)

	// Simulate operation completion
	finish(nil)

	// Verify context is updated
	assert.NotEqual(t, ctx, tracedCtx)

	// Test with error
	tracedCtx2, finish2 := TraceCustomOperation(ctx, "ValidateCard", metadata)
	finish2(errors.New("invalid card"))

	assert.NotEqual(t, ctx, tracedCtx2)
}

func TestGetTraceID(t *testing.T) {
	// Configure X-Ray for testing
	xray.Configure(xray.Config{
		DaemonAddr: "127.0.0.1:2000",
		LogLevel:   "silent",
	})

	// Test with segment
	ctx, segment := xray.BeginSegment(context.Background(), "test")
	defer segment.Close(nil)

	traceID := GetTraceID(ctx)
	assert.NotEmpty(t, traceID)

	// Test without segment
	noSegmentCtx := context.Background()
	traceID2 := GetTraceID(noSegmentCtx)
	assert.Empty(t, traceID2)
}

func TestGetSegmentID(t *testing.T) {
	// Configure X-Ray for testing
	xray.Configure(xray.Config{
		DaemonAddr: "127.0.0.1:2000",
		LogLevel:   "silent",
	})

	// Test with segment
	ctx, segment := xray.BeginSegment(context.Background(), "test")
	defer segment.Close(nil)

	segmentID := GetSegmentID(ctx)
	assert.NotEmpty(t, segmentID)

	// Test without segment
	noSegmentCtx := context.Background()
	segmentID2 := GetSegmentID(noSegmentCtx)
	assert.Empty(t, segmentID2)
}

func TestAddAnnotation(t *testing.T) {
	// Configure X-Ray for testing
	xray.Configure(xray.Config{
		DaemonAddr: "127.0.0.1:2000",
		LogLevel:   "silent",
	})

	// Test with segment
	ctx, segment := xray.BeginSegment(context.Background(), "test")
	defer segment.Close(nil)

	// Should not panic
	AddAnnotation(ctx, "test.key", "test.value")

	// Test without segment (should not panic)
	noSegmentCtx := context.Background()
	AddAnnotation(noSegmentCtx, "test.key", "test.value")
}

func TestAddMetadata(t *testing.T) {
	// Configure X-Ray for testing
	xray.Configure(xray.Config{
		DaemonAddr: "127.0.0.1:2000",
		LogLevel:   "silent",
	})

	// Test with segment
	ctx, segment := xray.BeginSegment(context.Background(), "test")
	defer segment.Close(nil)

	// Should not panic
	AddMetadata(ctx, "test", "key", "value")

	// Test without segment (should not panic)
	noSegmentCtx := context.Background()
	AddMetadata(noSegmentCtx, "test", "key", "value")
}

func TestSetError(t *testing.T) {
	// Configure X-Ray for testing
	xray.Configure(xray.Config{
		DaemonAddr: "127.0.0.1:2000",
		LogLevel:   "silent",
	})

	// Test with segment
	ctx, segment := xray.BeginSegment(context.Background(), "test")
	defer segment.Close(nil)

	// Should not panic
	SetError(ctx, errors.New("test error"))

	// Test without segment (should not panic)
	noSegmentCtx := context.Background()
	SetError(noSegmentCtx, errors.New("test error"))
}

func TestFilterSensitiveHeaders(t *testing.T) {
	headers := map[string]string{
		"authorization": "Bearer token123",
		"cookie":        "session=abc123",
		"x-api-key":     "key123",
		"x-auth-token":  "token456",
		"content-type":  "application/json",
		"user-agent":    "test-agent",
	}

	filtered := filterSensitiveHeaders(headers)

	// Sensitive headers should be redacted
	assert.Equal(t, "[REDACTED]", filtered["authorization"])
	assert.Equal(t, "[REDACTED]", filtered["cookie"])
	assert.Equal(t, "[REDACTED]", filtered["x-api-key"])
	assert.Equal(t, "[REDACTED]", filtered["x-auth-token"])

	// Non-sensitive headers should be preserved
	assert.Equal(t, "application/json", filtered["content-type"])
	assert.Equal(t, "test-agent", filtered["user-agent"])
}

func TestXRayMiddleware_Performance(t *testing.T) {
	// Configure X-Ray for testing
	xray.Configure(xray.Config{
		DaemonAddr: "127.0.0.1:2000",
		LogLevel:   "silent",
	})

	config := XRayConfig{
		ServiceName: "perf-test",
	}

	middleware := XRayMiddleware(config)

	// Create simple handler
	handler := lift.HandlerFunc(func(ctx *lift.Context) error {
		return nil
	})

	wrappedHandler := middleware(handler)

	// Measure performance
	start := time.Now()
	iterations := 1000

	for i := 0; i < iterations; i++ {
		ctx := createTestContext()
		_ = wrappedHandler.Handle(ctx)
	}

	duration := time.Since(start)
	perOperation := duration / time.Duration(iterations)

	t.Logf("X-Ray middleware overhead: %v per operation", perOperation)

	// Should be under 1ms per operation
	assert.Less(t, perOperation, 1*time.Millisecond)
}

// Helper function to create test context
func createTestContext() *lift.Context {
	request := &lift.Request{
		Request: &adapters.Request{
			Method:      "GET",
			Path:        "/test",
			Headers:     make(map[string]string),
			QueryParams: make(map[string]string),
			PathParams:  make(map[string]string),
			TriggerType: adapters.TriggerAPIGateway,
		},
	}

	response := lift.NewResponse()

	ctx := &lift.Context{
		Context:   context.Background(),
		Request:   request,
		Response:  response,
		RequestID: "test-request-123",
	}

	return ctx
}
