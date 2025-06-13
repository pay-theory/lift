package middleware

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/observability"
	"github.com/pay-theory/lift/pkg/observability/xray"
)

// EnhancedObservabilityConfig holds configuration for the complete observability stack
type EnhancedObservabilityConfig struct {
	// Core components
	Logger  observability.StructuredLogger
	Metrics observability.MetricsCollector
	Tracer  *xray.XRayTracer

	// Feature flags
	EnableLogging bool `json:"enable_logging"`
	EnableMetrics bool `json:"enable_metrics"`
	EnableTracing bool `json:"enable_tracing"`

	// Custom extractors
	OperationNameFunc func(*lift.Context) string
	TenantIDFunc      func(*lift.Context) string
	UserIDFunc        func(*lift.Context) string

	// Performance settings
	LogRequestBody  bool    `json:"log_request_body"`
	LogResponseBody bool    `json:"log_response_body"`
	MaxBodyLogSize  int     `json:"max_body_log_size"`
	SampleRate      float64 `json:"sample_rate"` // 0.0 to 1.0

	// Custom dimensions/tags
	DefaultTags map[string]string `json:"default_tags"`
}

// EnhancedObservabilityMiddleware provides comprehensive observability with logging, metrics, and tracing
func EnhancedObservabilityMiddleware(config EnhancedObservabilityConfig) lift.Middleware {
	// Set defaults
	if config.OperationNameFunc == nil {
		config.OperationNameFunc = func(ctx *lift.Context) string {
			return fmt.Sprintf("%s_%s", ctx.Request.Method, ctx.Request.Path)
		}
	}
	if config.TenantIDFunc == nil {
		config.TenantIDFunc = func(ctx *lift.Context) string {
			return ctx.TenantID()
		}
	}
	if config.UserIDFunc == nil {
		config.UserIDFunc = func(ctx *lift.Context) string {
			return ctx.UserID()
		}
	}
	if config.MaxBodyLogSize == 0 {
		config.MaxBodyLogSize = 1024 // 1KB default
	}
	if config.SampleRate == 0 {
		config.SampleRate = 1.0 // 100% by default
	}
	if config.DefaultTags == nil {
		config.DefaultTags = make(map[string]string)
	}

	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			// Start timing
			start := time.Now()

			// Extract context information
			operation := config.OperationNameFunc(ctx)
			tenantID := config.TenantIDFunc(ctx)
			userID := config.UserIDFunc(ctx)

			// Build base tags for metrics
			baseTags := make(map[string]string)
			for k, v := range config.DefaultTags {
				baseTags[k] = v
			}
			baseTags["method"] = ctx.Request.Method
			baseTags["path"] = ctx.Request.Path
			baseTags["tenant_id"] = tenantID
			baseTags["operation"] = operation

			// Initialize observability components
			var contextLogger observability.StructuredLogger
			var tenantMetrics observability.MetricsCollector

			// Setup logging
			if config.EnableLogging && config.Logger != nil {
				contextLogger = config.Logger.
					WithRequestID(ctx.RequestID).
					WithTenantID(tenantID).
					WithUserID(userID)

				// Add trace context if available
				if traceID := xray.GetTraceID(ctx.Context); traceID != "" {
					contextLogger = contextLogger.WithTraceID(traceID)
				}
				if spanID := xray.GetSegmentID(ctx.Context); spanID != "" {
					contextLogger = contextLogger.WithSpanID(spanID)
				}

				ctx.Logger = contextLogger

				// Log request start
				logFields := map[string]interface{}{
					"operation":    operation,
					"method":       ctx.Request.Method,
					"path":         ctx.Request.Path,
					"query_params": ctx.Request.QueryParams,
					"source_ip":    ctx.Request.Headers["X-Forwarded-For"],
					"user_agent":   ctx.Request.Headers["User-Agent"],
					"tenant_id":    tenantID,
					"user_id":      userID,
				}

				// Optionally log request body
				if config.LogRequestBody && len(ctx.Request.Body) > 0 {
					bodyStr := string(ctx.Request.Body)
					if len(bodyStr) > config.MaxBodyLogSize {
						bodyStr = bodyStr[:config.MaxBodyLogSize] + "...[truncated]"
					}
					logFields["request_body"] = bodyStr
				}

				contextLogger.Info("Request started", logFields)
			}

			// Setup metrics
			if config.EnableMetrics && config.Metrics != nil {
				tenantMetrics = config.Metrics.WithTags(baseTags)

				// Record request count
				counter := tenantMetrics.Counter("requests.total")
				counter.Inc()

				// Record concurrent requests
				gauge := tenantMetrics.Gauge("requests.active")
				gauge.Inc()
				defer gauge.Dec()
			}

			// Setup tracing
			if config.EnableTracing {
				// Add custom annotations to current trace
				xray.AddAnnotation(ctx.Context, "operation", operation)
				xray.AddAnnotation(ctx.Context, "tenant_id", tenantID)
				xray.AddAnnotation(ctx.Context, "user_id", userID)

				// Add metadata
				xray.AddMetadata(ctx.Context, "request", "operation", operation)
				xray.AddMetadata(ctx.Context, "request", "tenant_id", tenantID)
				xray.AddMetadata(ctx.Context, "request", "user_id", userID)
			}

			// Execute handler
			err := next.Handle(ctx)

			// Calculate duration
			duration := time.Since(start)

			// Determine status code
			statusCode := ctx.Response.StatusCode
			if statusCode == 0 {
				if err != nil {
					statusCode = 500
				} else {
					statusCode = 200
				}
			}

			// Enhanced logging based on result
			if config.EnableLogging && contextLogger != nil {
				logFields := map[string]interface{}{
					"operation": operation,
					"duration":  duration.String(),
					"status":    statusCode,
				}

				// Optionally log response body
				if config.LogResponseBody && ctx.Response.Body != nil {
					var bodyStr string
					switch v := ctx.Response.Body.(type) {
					case string:
						bodyStr = v
					case []byte:
						bodyStr = string(v)
					default:
						if data, jsonErr := json.Marshal(v); jsonErr == nil {
							bodyStr = string(data)
						}
					}
					if len(bodyStr) > config.MaxBodyLogSize {
						bodyStr = bodyStr[:config.MaxBodyLogSize] + "...[truncated]"
					}
					logFields["response_body"] = bodyStr
				}

				if err != nil {
					logFields["error"] = err.Error()
					contextLogger.Error("Request failed", logFields)
				} else {
					contextLogger.Info("Request completed", logFields)
				}
			}

			// Enhanced metrics collection
			if config.EnableMetrics && tenantMetrics != nil {
				// Add status code to tags
				statusTags := make(map[string]string)
				for k, v := range baseTags {
					statusTags[k] = v
				}
				statusTags["status"] = fmt.Sprintf("%d", statusCode)
				statusTags["status_class"] = fmt.Sprintf("%dxx", statusCode/100)

				statusMetrics := config.Metrics.WithTags(statusTags)

				// Record latency
				histogram := statusMetrics.Histogram("requests.duration")
				histogram.Observe(float64(duration.Milliseconds()))

				// Record response size if available
				if ctx.Response.Body != nil {
					var size int
					switch v := ctx.Response.Body.(type) {
					case string:
						size = len(v)
					case []byte:
						size = len(v)
					default:
						if data, jsonErr := json.Marshal(v); jsonErr == nil {
							size = len(data)
						}
					}
					if size > 0 {
						gauge := statusMetrics.Gauge("response.size")
						gauge.Set(float64(size))
					}
				}

				// Record errors
				if err != nil {
					errorTags := make(map[string]string)
					for k, v := range baseTags {
						errorTags[k] = v
					}
					errorTags["error_type"] = fmt.Sprintf("%T", err)

					errorMetrics := config.Metrics.WithTags(errorTags)
					errorCounter := errorMetrics.Counter("requests.errors")
					errorCounter.Inc()
				}

				// Record operation-specific metrics
				operationMetrics := config.Metrics.WithTags(map[string]string{
					"operation": operation,
					"tenant_id": tenantID,
				})

				opCounter := operationMetrics.Counter(fmt.Sprintf("operation.%s.total", operation))
				opCounter.Inc()

				opHistogram := operationMetrics.Histogram(fmt.Sprintf("operation.%s.duration", operation))
				opHistogram.Observe(float64(duration.Milliseconds()))

				if err != nil {
					opErrorCounter := operationMetrics.Counter(fmt.Sprintf("operation.%s.errors", operation))
					opErrorCounter.Inc()
				}
			}

			// Enhanced tracing
			if config.EnableTracing {
				// Add timing information
				xray.AddMetadata(ctx.Context, "timing", "duration_ms", duration.Milliseconds())
				xray.AddMetadata(ctx.Context, "timing", "start_time", start.Format(time.RFC3339Nano))
				xray.AddMetadata(ctx.Context, "timing", "end_time", time.Now().Format(time.RFC3339Nano))

				// Add response information
				xray.AddAnnotation(ctx.Context, "http.status_code", statusCode)
				xray.AddMetadata(ctx.Context, "response", "status_code", statusCode)

				// Add error information
				if err != nil {
					xray.SetError(ctx.Context, err)
					xray.AddAnnotation(ctx.Context, "error", "true")
					xray.AddMetadata(ctx.Context, "error", "type", fmt.Sprintf("%T", err))
				} else {
					xray.AddAnnotation(ctx.Context, "error", "false")
				}
			}

			return err
		})
	}
}

// ObservabilityStats provides comprehensive statistics about observability performance
type ObservabilityStats struct {
	Logger  *observability.LoggerStats  `json:"logger,omitempty"`
	Metrics *observability.MetricsStats `json:"metrics,omitempty"`
	Tracing *TracingStats               `json:"tracing,omitempty"`
}

// TracingStats provides statistics about tracing performance
type TracingStats struct {
	TracesGenerated int64     `json:"traces_generated"`
	LastTrace       time.Time `json:"last_trace"`
	ErrorCount      int64     `json:"error_count"`
}

// GetObservabilityStats returns comprehensive observability statistics
func GetObservabilityStats(config EnhancedObservabilityConfig) ObservabilityStats {
	stats := ObservabilityStats{}

	if config.Logger != nil {
		loggerStats := config.Logger.GetStats()
		stats.Logger = &loggerStats
	}

	if config.Metrics != nil {
		metricsStats := config.Metrics.GetStats()
		stats.Metrics = &metricsStats
	}

	// Note: X-Ray doesn't provide built-in stats, so we'd need to track these separately
	// This is a placeholder for future implementation
	stats.Tracing = &TracingStats{
		TracesGenerated: 0,
		LastTrace:       time.Now(),
		ErrorCount:      0,
	}

	return stats
}

// HealthCheckObservability creates a health check for the observability stack
func HealthCheckObservability(config EnhancedObservabilityConfig) func() error {
	return func() error {
		// Check logger health
		if config.EnableLogging && config.Logger != nil {
			if !config.Logger.IsHealthy() {
				return fmt.Errorf("logger is unhealthy")
			}
		}

		// Check metrics health (if metrics collector provides health check)
		if config.EnableMetrics && config.Metrics != nil {
			stats := config.Metrics.GetStats()
			if stats.ErrorCount > 0 && stats.LastError != "" {
				// Allow some errors, but not too many
				if float64(stats.ErrorCount)/float64(stats.MetricsRecorded) > 0.1 {
					return fmt.Errorf("metrics error rate too high: %s", stats.LastError)
				}
			}
		}

		// Tracing health is harder to check since X-Ray is fire-and-forget
		// We could implement custom tracking if needed

		return nil
	}
}
