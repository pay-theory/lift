package xray

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/pay-theory/lift/pkg/lift"
)

// XRayConfig holds configuration for X-Ray tracing
type XRayConfig struct {
	ServiceName       string            `json:"service_name"`
	ServiceVersion    string            `json:"service_version"`
	Environment       string            `json:"environment"`
	SamplingRate      float64           `json:"sampling_rate"`      // 0.0 to 1.0
	Annotations       map[string]string `json:"annotations"`        // Default annotations
	Metadata          map[string]string `json:"metadata"`           // Default metadata
	EnableSubsegments bool              `json:"enable_subsegments"` // Enable automatic subsegments
}

// XRayTracer provides X-Ray distributed tracing capabilities
type XRayTracer struct {
	config XRayConfig
}

// NewXRayTracer creates a new X-Ray tracer with the given configuration
func NewXRayTracer(config XRayConfig) *XRayTracer {
	// Set defaults
	if config.ServiceName == "" {
		config.ServiceName = "lift-service"
	}
	if config.SamplingRate == 0 {
		config.SamplingRate = 0.1 // 10% sampling by default
	}
	if config.Annotations == nil {
		config.Annotations = make(map[string]string)
	}
	if config.Metadata == nil {
		config.Metadata = make(map[string]string)
	}

	return &XRayTracer{
		config: config,
	}
}

// XRayMiddleware creates middleware for automatic X-Ray tracing
func XRayMiddleware(config XRayConfig) lift.Middleware {
	tracer := NewXRayTracer(config)

	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			// Start segment for the request
			var segment *xray.Segment
			ctx.Context, segment = xray.BeginSegment(ctx.Context, config.ServiceName)

			// Add panic recovery to prevent crashes
			defer func() {
				if r := recover(); r != nil {
					if segment != nil {
						if err := segment.AddError(fmt.Errorf("panic in xray middleware: %v", r)); err != nil {
							// Log error but don't fail the request
							fmt.Printf("Failed to add panic error to XRay segment: %v\n", err)
						}
						segment.Close(fmt.Errorf("panic: %v", r))
					}
					panic(r) // Re-panic after logging
				}
			}()

			// Ensure segment is closed
			defer func() {
				if segment != nil {
					segment.Close(nil)
				}
			}()

			// Add standard annotations
			tracer.addStandardAnnotations(segment, ctx)

			// Add custom annotations from config
			if config.Annotations != nil {
				for key, value := range config.Annotations {
					segment.AddAnnotation(key, value)
				}
			}

			// Add metadata
			tracer.addStandardMetadata(segment, ctx)
			if config.Metadata != nil {
				for key, value := range config.Metadata {
					segment.AddMetadata("custom", map[string]interface{}{key: value})
				}
			}

			// Add trace information to context for logging
			if ctx.Request != nil {
				// Ensure Headers map is initialized
				if ctx.Request.Headers == nil {
					ctx.Request.Headers = make(map[string]string)
				}

				if traceID := segment.TraceID; traceID != "" {
					ctx.Request.Headers["X-Trace-Id"] = traceID
				}
				if segmentID := segment.ID; segmentID != "" {
					ctx.Request.Headers["X-Span-Id"] = segmentID
				}
			}

			// Execute handler
			start := time.Now()
			err := next.Handle(ctx)
			duration := time.Since(start)

			// Record timing
			segment.AddMetadata("timing", map[string]interface{}{
				"duration_ms": duration.Milliseconds(),
			})

			// Handle errors
			if err != nil {
				if addErr := segment.AddError(err); addErr != nil {
					// Log error but don't fail the request
					fmt.Printf("Failed to add error to XRay segment: %v\n", addErr)
				}
				segment.AddAnnotation("error", "true")
				segment.AddMetadata("error", map[string]interface{}{
					"message": err.Error(),
				})
			} else {
				segment.AddAnnotation("error", "false")
			}

			// Add response information
			segment.AddAnnotation("http.status_code", ctx.Response.StatusCode)
			segment.AddMetadata("response", map[string]interface{}{
				"status_code": ctx.Response.StatusCode,
			})

			return err
		})
	}
}

// addStandardAnnotations adds standard annotations to the segment
func (t *XRayTracer) addStandardAnnotations(segment *xray.Segment, ctx *lift.Context) {
	// HTTP information (only if request is not nil)
	if ctx.Request != nil {
		segment.AddAnnotation("http.method", ctx.Request.Method)
		segment.AddAnnotation("http.url", ctx.Request.Path)
	}

	// Multi-tenant information
	if tenantID := ctx.TenantID(); tenantID != "" {
		segment.AddAnnotation("tenant_id", tenantID)
	}
	if userID := ctx.UserID(); userID != "" {
		segment.AddAnnotation("user_id", userID)
	}

	// Request information
	if ctx.RequestID != "" {
		segment.AddAnnotation("request_id", ctx.RequestID)
	}

	// Service information
	segment.AddAnnotation("service.name", t.config.ServiceName)
	if t.config.ServiceVersion != "" {
		segment.AddAnnotation("service.version", t.config.ServiceVersion)
	}
	if t.config.Environment != "" {
		segment.AddAnnotation("environment", t.config.Environment)
	}
}

// addStandardMetadata adds standard metadata to the segment
func (t *XRayTracer) addStandardMetadata(segment *xray.Segment, ctx *lift.Context) {
	if ctx.Request == nil {
		return // Skip if request is nil
	}

	// HTTP metadata
	httpMetadata := map[string]interface{}{
		"method": ctx.Request.Method,
		"path":   ctx.Request.Path,
	}

	// Add query params if not nil
	if ctx.Request.QueryParams != nil {
		httpMetadata["query_params"] = ctx.Request.QueryParams
	}

	// Add filtered headers if headers exist
	if ctx.Request.Headers != nil {
		httpMetadata["headers"] = filterSensitiveHeaders(ctx.Request.Headers)
	}

	segment.AddMetadata("http", httpMetadata)

	// Request metadata
	requestMetadata := map[string]interface{}{
		"request_id":   ctx.RequestID,
		"tenant_id":    ctx.TenantID(),
		"user_id":      ctx.UserID(),
		"trigger_type": ctx.Request.TriggerType,
	}
	segment.AddMetadata("lift", requestMetadata)

	// Service metadata
	serviceMetadata := map[string]interface{}{
		"name":        t.config.ServiceName,
		"version":     t.config.ServiceVersion,
		"environment": t.config.Environment,
	}
	segment.AddMetadata("service", serviceMetadata)
}

// filterSensitiveHeaders removes sensitive headers from tracing
func filterSensitiveHeaders(headers map[string]string) map[string]string {
	filtered := make(map[string]string)
	sensitiveHeaders := map[string]bool{
		"authorization": true,
		"cookie":        true,
		"x-api-key":     true,
		"x-auth-token":  true,
	}

	for key, value := range headers {
		if sensitiveHeaders[key] {
			filtered[key] = "[REDACTED]"
		} else {
			filtered[key] = value
		}
	}

	return filtered
}

// TraceDynamoDBOperation creates a subsegment for DynamoDB operations
func TraceDynamoDBOperation(ctx context.Context, operation, tableName string) (context.Context, func()) {
	newCtx, subsegment := xray.BeginSubsegment(ctx, fmt.Sprintf("DynamoDB.%s", operation))
	if subsegment == nil {
		// X-Ray not available, return no-op
		return ctx, func() {}
	}

	// Add DynamoDB-specific annotations
	subsegment.AddAnnotation("aws.operation", operation)
	subsegment.AddAnnotation("aws.table_name", tableName)
	subsegment.AddAnnotation("aws.service", "DynamoDB")

	// Add metadata
	subsegment.AddMetadata("aws", map[string]interface{}{
		"operation":  operation,
		"table_name": tableName,
		"service":    "DynamoDB",
	})

	return newCtx, func() {
		subsegment.Close(nil)
	}
}

// TraceHTTPCall creates a subsegment for HTTP calls to other services
func TraceHTTPCall(ctx context.Context, method, url string) (context.Context, func(statusCode int, err error)) {
	newCtx, subsegment := xray.BeginSubsegment(ctx, fmt.Sprintf("HTTP.%s", method))
	if subsegment == nil {
		// X-Ray not available, return no-op
		return ctx, func(int, error) {}
	}

	// Add HTTP-specific annotations
	subsegment.AddAnnotation("http.method", method)
	subsegment.AddAnnotation("http.url", url)

	// Add metadata
	subsegment.AddMetadata("http", map[string]interface{}{
		"method": method,
		"url":    url,
	})

	return newCtx, func(statusCode int, err error) {
		if statusCode > 0 {
			subsegment.AddAnnotation("http.status_code", statusCode)
			subsegment.AddMetadata("http", map[string]interface{}{
				"status_code": statusCode,
			})
		}

		if err != nil {
			if addErr := subsegment.AddError(err); addErr != nil {
				// Log error but don't fail the operation
				fmt.Printf("Failed to add HTTP error to XRay subsegment: %v\n", addErr)
			}
		}

		subsegment.Close(err)
	}
}

// TraceCustomOperation creates a subsegment for custom operations
func TraceCustomOperation(ctx context.Context, operationName string, metadata map[string]interface{}) (context.Context, func(error)) {
	newCtx, subsegment := xray.BeginSubsegment(ctx, operationName)
	if subsegment == nil {
		// X-Ray not available, return no-op
		return ctx, func(error) {}
	}

	// Add operation name
	subsegment.AddAnnotation("operation", operationName)

	// Add custom metadata
	if len(metadata) > 0 {
		subsegment.AddMetadata("custom", metadata)
	}

	return newCtx, func(err error) {
		if err != nil {
			if addErr := subsegment.AddError(err); addErr != nil {
				// Log error but don't fail the operation
				fmt.Printf("Failed to add custom operation error to XRay subsegment: %v\n", addErr)
			}
			subsegment.AddAnnotation("error", "true")
		} else {
			subsegment.AddAnnotation("error", "false")
		}

		subsegment.Close(err)
	}
}

// GetTraceID extracts the trace ID from the context
func GetTraceID(ctx context.Context) string {
	if segment := xray.GetSegment(ctx); segment != nil {
		return segment.TraceID
	}
	return ""
}

// GetSegmentID extracts the segment ID from the context
func GetSegmentID(ctx context.Context) string {
	if segment := xray.GetSegment(ctx); segment != nil {
		return segment.ID
	}
	return ""
}

// AddAnnotation adds an annotation to the current segment
func AddAnnotation(ctx context.Context, key string, value interface{}) {
	if segment := xray.GetSegment(ctx); segment != nil {
		segment.AddAnnotation(key, value)
	}
}

// AddMetadata adds metadata to the current segment
func AddMetadata(ctx context.Context, namespace, key string, value interface{}) {
	if segment := xray.GetSegment(ctx); segment != nil {
		segment.AddMetadata(namespace, map[string]interface{}{key: value})
	}
}

// SetError marks the current segment as having an error
func SetError(ctx context.Context, err error) {
	if segment := xray.GetSegment(ctx); segment != nil {
		if addErr := segment.AddError(err); addErr != nil {
			// Log error but don't fail the operation
			fmt.Printf("Failed to add error to XRay segment: %v\n", addErr)
		}
	}
}
