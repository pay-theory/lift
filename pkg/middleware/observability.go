package middleware

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/observability"
)

// ObservabilityConfig holds configuration for observability middleware
type ObservabilityConfig struct {
	Logger  observability.StructuredLogger
	Metrics observability.MetricsCollector
	// Optional: custom operation name extractor
	OperationNameFunc func(*lift.Context) string
}

// ObservabilityMiddleware provides comprehensive logging and metrics collection
func ObservabilityMiddleware(config ObservabilityConfig) lift.Middleware {
	// Default operation name function
	if config.OperationNameFunc == nil {
		config.OperationNameFunc = func(ctx *lift.Context) string {
			return fmt.Sprintf("%s_%s", ctx.Request.Method, ctx.Request.Path)
		}
	}

	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			// Start timing
			start := time.Now()

			// Add logger with context to the request
			if config.Logger != nil {
				contextLogger := config.Logger.
					WithRequestID(ctx.RequestID).
					WithTenantID(ctx.TenantID()).
					WithUserID(ctx.UserID())

				// Add trace context if available
				if traceID := ctx.Request.Headers["X-Trace-Id"]; traceID != "" {
					contextLogger = contextLogger.WithTraceID(traceID)
				}
				if spanID := ctx.Request.Headers["X-Span-Id"]; spanID != "" {
					contextLogger = contextLogger.WithSpanID(spanID)
				}

				ctx.Logger = contextLogger

				// Log request start
				ctx.Logger.Info("Request started", map[string]interface{}{
					"method":     ctx.Request.Method,
					"path":       ctx.Request.Path,
					"query":      "[SANITIZED_QUERY_PARAMS]", // Sanitized for security
					"source_ip":  ctx.Request.Headers["X-Forwarded-For"],
					"user_agent": ctx.Request.Headers["User-Agent"],
				})
			}

			// Get operation name for metrics
			operation := config.OperationNameFunc(ctx)

			// Add metrics collector with tenant context
			if config.Metrics != nil {
				tenantMetrics := config.Metrics.WithTags(map[string]string{
					"tenant_id": ctx.TenantID(),
					"method":    ctx.Request.Method,
					"path":      ctx.Request.Path,
				})

				// Record request count
				counter := tenantMetrics.Counter("requests.total")
				counter.Inc()
			}

			// Execute handler
			err := next.Handle(ctx)

			// Calculate duration
			duration := time.Since(start)

			// Log and record metrics based on result
			if err != nil {
				// Log error
				if ctx.Logger != nil {
					ctx.Logger.Error("Request failed", map[string]interface{}{
						"error":    "[SANITIZED_ERROR]", // Sanitized for security
						"duration": duration.String(),
						"status":   ctx.Response.StatusCode,
					})
				}

				// Record error metrics
				if config.Metrics != nil {
					errorMetrics := config.Metrics.WithTags(map[string]string{
						"tenant_id": ctx.TenantID(),
						"method":    ctx.Request.Method,
						"path":      ctx.Request.Path,
						"error":     "true",
					})

					errorCounter := errorMetrics.Counter("requests.errors")
					errorCounter.Inc()

					// Record latency even for errors
					histogram := errorMetrics.Histogram("requests.duration")
					histogram.Observe(float64(duration.Milliseconds()))
				}
			} else {
				// Log success
				if ctx.Logger != nil {
					ctx.Logger.Info("Request completed", map[string]interface{}{
						"duration": duration.String(),
						"status":   ctx.Response.StatusCode,
					})
				}

				// Record success metrics
				if config.Metrics != nil {
					successMetrics := config.Metrics.WithTags(map[string]string{
						"tenant_id": ctx.TenantID(),
						"method":    ctx.Request.Method,
						"path":      ctx.Request.Path,
						"status":    fmt.Sprintf("%d", ctx.Response.StatusCode),
					})

					// Record latency
					histogram := successMetrics.Histogram("requests.duration")
					histogram.Observe(float64(duration.Milliseconds()))

					// Record response size if available
					if ctx.Response.Body != nil {
						// Try to estimate response size
						var size int
						switch v := ctx.Response.Body.(type) {
						case string:
							size = len(v)
						case []byte:
							size = len(v)
						default:
							// For other types, marshal to JSON to get size estimate
							if data, err := json.Marshal(v); err == nil {
								size = len(data)
							}
						}
						if size > 0 {
							gauge := successMetrics.Gauge("response.size")
							gauge.Set(float64(size))
						}
					}
				}
			}

			// Record operation-specific metrics
			if config.Metrics != nil {
				// Record operation-specific counters
				operationCounter := config.Metrics.Counter(fmt.Sprintf("operation.%s", operation))
				operationCounter.Inc()

				// Record operation duration
				operationHistogram := config.Metrics.Histogram(fmt.Sprintf("operation.%s.duration", operation))
				operationHistogram.Observe(float64(duration.Milliseconds()))
			}

			return err
		})
	}
}

// MetricsOnlyMiddleware provides lightweight metrics collection without logging
func MetricsOnlyMiddleware(metrics lift.MetricsCollector) lift.Middleware {
	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			start := time.Now()

			// Record request
			counter := metrics.Counter("http.requests", map[string]string{
				"method": ctx.Request.Method,
				"path":   ctx.Request.Path,
			})
			counter.Inc()

			// Execute handler
			err := next.Handle(ctx)

			// Record duration
			duration := time.Since(start)
			histogram := metrics.Histogram("http.duration", map[string]string{
				"method": ctx.Request.Method,
				"path":   ctx.Request.Path,
				"status": fmt.Sprintf("%d", ctx.Response.StatusCode),
			})
			histogram.Observe(float64(duration.Milliseconds()))

			// Record errors
			if err != nil {
				errorCounter := metrics.Counter("http.errors", map[string]string{
					"method": ctx.Request.Method,
					"path":   ctx.Request.Path,
				})
				errorCounter.Inc()
			}

			return err
		})
	}
}
