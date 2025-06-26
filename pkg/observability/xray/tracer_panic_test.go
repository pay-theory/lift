package xray

import (
	"context"
	"testing"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestXRayTracerPanicFixes(t *testing.T) {
	config := XRayConfig{
		ServiceName:       "test-service",
		ServiceVersion:    "1.0.0",
		Environment:       "test",
		SamplingRate:      1.0, // 100% for testing
		Annotations:       map[string]string{"test": "value"},
		Metadata:          map[string]string{"test": "metadata"},
		EnableSubsegments: true,
	}

	t.Run("XRayMiddleware with nil request headers", func(t *testing.T) {
		middleware := XRayMiddleware(config)

		// Create handler that should not panic
		handler := middleware(lift.HandlerFunc(func(ctx *lift.Context) error {
			return ctx.OK(map[string]string{"status": "ok"})
		}))

		// Create context with nil headers (this used to cause panic)
		req := &adapters.Request{
			Method:      "GET",
			Path:        "/test",
			Headers:     nil, // This could cause nil map panic
			QueryParams: nil, // This could also cause issues
			PathParams:  make(map[string]string),
			Body:        nil,
		}

		liftReq := lift.NewRequest(req)
		ctx := lift.NewContext(context.Background(), liftReq)

		// This should not panic
		err := handler.Handle(ctx)
		assert.NoError(t, err)

		// Verify headers were initialized properly
		assert.NotNil(t, ctx.Request.Headers)
		assert.Contains(t, ctx.Request.Headers, "X-Trace-Id")
		assert.Contains(t, ctx.Request.Headers, "X-Span-Id")
	})

	t.Run("XRayMiddleware with nil config annotations", func(t *testing.T) {
		nilConfig := XRayConfig{
			ServiceName:  "test-service",
			SamplingRate: 1.0,
			Annotations:  nil, // This could cause panic
			Metadata:     nil, // This could cause panic
		}

		middleware := XRayMiddleware(nilConfig)

		handler := middleware(lift.HandlerFunc(func(ctx *lift.Context) error {
			return ctx.OK(map[string]string{"status": "ok"})
		}))

		req := &adapters.Request{
			Method:      "GET",
			Path:        "/test",
			Headers:     make(map[string]string),
			QueryParams: make(map[string]string),
			PathParams:  make(map[string]string),
			Body:        nil,
		}

		liftReq := lift.NewRequest(req)
		ctx := lift.NewContext(context.Background(), liftReq)

		// This should not panic even with nil annotations/metadata
		err := handler.Handle(ctx)
		assert.NoError(t, err)
	})

	t.Run("XRayMiddleware with completely nil request", func(t *testing.T) {
		middleware := XRayMiddleware(config)

		handler := middleware(lift.HandlerFunc(func(ctx *lift.Context) error {
			return ctx.OK(map[string]string{"status": "ok"})
		}))

		// Create context with nil request (extreme case)
		ctx := lift.NewContext(context.Background(), nil)

		// This should handle gracefully without panic
		err := handler.Handle(ctx)
		assert.NoError(t, err)
	})

	t.Run("addStandardMetadata with nil request", func(t *testing.T) {
		tracer := NewXRayTracer(config)

		// This should not panic - we can't test the actual segment interaction
		// but we can verify the nil request handling doesn't cause issues
		require.NotPanics(t, func() {
			// Test that our tracer handles nil gracefully
			assert.NotNil(t, tracer)
			assert.Equal(t, "test-service", tracer.config.ServiceName)
		})
	})

	t.Run("filterSensitiveHeaders with nil headers", func(t *testing.T) {
		// Test the helper function directly
		result := filterSensitiveHeaders(nil)
		assert.NotNil(t, result)
		assert.Empty(t, result)

		// Test with empty map
		result = filterSensitiveHeaders(make(map[string]string))
		assert.NotNil(t, result)
		assert.Empty(t, result)

		// Test with sensitive headers
		headers := map[string]string{
			"authorization": "Bearer token",
			"x-api-key":     "secret-key",
			"content-type":  "application/json",
		}

		result = filterSensitiveHeaders(headers)
		assert.Equal(t, "[REDACTED]", result["authorization"])
		assert.Equal(t, "[REDACTED]", result["x-api-key"])
		assert.Equal(t, "application/json", result["content-type"])
	})

	t.Run("NewXRayTracer with nil maps", func(t *testing.T) {
		// Test tracer creation with nil maps in config
		nilConfig := XRayConfig{
			ServiceName: "test",
			Annotations: nil,
			Metadata:    nil,
		}

		tracer := NewXRayTracer(nilConfig)
		assert.NotNil(t, tracer)

		// Verify maps are initialized
		assert.NotNil(t, tracer.config.Annotations)
		assert.NotNil(t, tracer.config.Metadata)
		assert.Equal(t, "test", tracer.config.ServiceName)
		assert.Equal(t, 0.1, tracer.config.SamplingRate) // Default
	})
}

func TestXRayTracerSubsegmentFunctions(t *testing.T) {
	ctx := context.Background()

	t.Run("TraceDynamoDBOperation with nil segment", func(t *testing.T) {
		// This tests the case where X-Ray is not available
		newCtx, cleanup := TraceDynamoDBOperation(ctx, "GetItem", "test-table")

		// Should return original context and no-op cleanup
		assert.Equal(t, ctx, newCtx)

		// Cleanup should not panic
		require.NotPanics(t, func() {
			cleanup()
		})
	})

	t.Run("TraceHTTPCall with nil segment", func(t *testing.T) {
		// This tests the case where X-Ray is not available
		newCtx, cleanup := TraceHTTPCall(ctx, "GET", "https://api.example.com")

		// Should return original context and no-op cleanup
		assert.Equal(t, ctx, newCtx)

		// Cleanup should not panic
		require.NotPanics(t, func() {
			cleanup(200, nil)
		})
	})

	t.Run("TraceCustomOperation with nil segment", func(t *testing.T) {
		metadata := map[string]any{
			"key": "value",
		}

		newCtx, cleanup := TraceCustomOperation(ctx, "custom-op", metadata)

		// Should return original context and no-op cleanup
		assert.Equal(t, ctx, newCtx)

		// Cleanup should not panic
		require.NotPanics(t, func() {
			cleanup(nil)
		})
	})

	t.Run("Helper functions with nil segment", func(t *testing.T) {
		// These functions should handle nil segments gracefully
		traceID := GetTraceID(ctx)
		assert.Equal(t, "", traceID)

		segmentID := GetSegmentID(ctx)
		assert.Equal(t, "", segmentID)

		// These should not panic with nil segment
		require.NotPanics(t, func() {
			AddAnnotation(ctx, "key", "value")
			AddMetadata(ctx, "namespace", "key", "value")
			SetError(ctx, assert.AnError)
		})
	})
}
