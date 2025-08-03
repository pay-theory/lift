package middleware

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/observability"
	"github.com/pay-theory/lift/pkg/observability/xray"
)

// Global tracing statistics
var tracingStats struct {
	tracesGenerated int64
	lastTrace       int64
	errorCount      int64
}

// EnhancedObservabilityConfig holds configuration for the complete observability stack
type EnhancedObservabilityConfig struct {
	Metrics           observability.MetricsCollector
	Logger            observability.StructuredLogger
	Tracer            *xray.XRayTracer
	DefaultTags       map[string]string `json:"default_tags"`
	OperationNameFunc func(*lift.Context) string
	TenantIDFunc      func(*lift.Context) string
	UserIDFunc        func(*lift.Context) string
	MaxBodyLogSize    int     `json:"max_body_log_size"`
	SampleRate        float64 `json:"sample_rate"`
	EnableLogging     bool    `json:"enable_logging"`
	LogResponseBody   bool    `json:"log_response_body"`
	LogRequestBody    bool    `json:"log_request_body"`
	EnableTracing     bool    `json:"enable_tracing"`
	EnableMetrics     bool    `json:"enable_metrics"`
}

// EnhancedObservabilityMiddleware provides comprehensive observability with logging, metrics, and tracing
func EnhancedObservabilityMiddleware(config EnhancedObservabilityConfig) lift.Middleware {
	// Set defaults
	config = setObservabilityDefaults(config)
	
	// Create the coordinated observability handler
	handler := newObservabilityHandler(config)
	
	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			return handler.handle(ctx, next)
		})
	}
}

// setObservabilityDefaults applies default values to the configuration
func setObservabilityDefaults(config EnhancedObservabilityConfig) EnhancedObservabilityConfig {
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
	return config
}

// ObservabilityStats provides comprehensive statistics about observability performance
type ObservabilityStats struct {
	Logger  *observability.LoggerStats  `json:"logger,omitempty"`
	Metrics *observability.MetricsStats `json:"metrics,omitempty"`
	Tracing *TracingStats               `json:"tracing,omitempty"`
}

// TracingStats provides statistics about tracing performance
type TracingStats struct {
	LastTrace       time.Time `json:"last_trace"`
	TracesGenerated int64     `json:"traces_generated"`
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

	// Get actual tracing statistics
	stats.Tracing = &TracingStats{
		TracesGenerated: atomic.LoadInt64(&tracingStats.tracesGenerated),
		LastTrace:       time.Unix(atomic.LoadInt64(&tracingStats.lastTrace), 0),
		ErrorCount:      atomic.LoadInt64(&tracingStats.errorCount),
	}

	return stats
}

// recordTraceGenerated increments the trace counter
func recordTraceGenerated() {
	atomic.AddInt64(&tracingStats.tracesGenerated, 1)
	atomic.StoreInt64(&tracingStats.lastTrace, time.Now().Unix())
}

// recordTraceError increments the error counter
func recordTraceError() {
	atomic.AddInt64(&tracingStats.errorCount, 1)
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

// observabilityHandler coordinates logging, metrics, and tracing components
type observabilityHandler struct {
	config  EnhancedObservabilityConfig
	logger  *loggingHandler
	metrics *metricsHandler
	tracer  *tracingHandler
}

// newObservabilityHandler creates a new handler with the given configuration
func newObservabilityHandler(config EnhancedObservabilityConfig) *observabilityHandler {
	return &observabilityHandler{
		config:  config,
		logger:  newLoggingHandler(config),
		metrics: newMetricsHandler(config),
		tracer:  newTracingHandler(config),
	}
}

// handle processes the request with coordinated observability
func (h *observabilityHandler) handle(ctx *lift.Context, next lift.Handler) error {
	start := time.Now()
	
	// Extract common context
	operation := h.extractOperation(ctx)
	tenantID := h.extractTenantID(ctx)
	userID := h.extractUserID(ctx)
	
	// Start observability components
	h.logger.before(ctx, operation, tenantID, userID)
	h.metrics.before(ctx, operation, tenantID, userID)
	h.tracer.before(ctx, operation, tenantID, userID)
	
	// Execute handler
	err := next.Handle(ctx)
	
	// Calculate duration and status
	duration := time.Since(start)
	statusCode := h.determineStatusCode(ctx, err)
	
	// Finish observability components
	h.logger.after(ctx, operation, duration, statusCode, err)
	h.metrics.after(ctx, operation, tenantID, duration, statusCode, err)
	h.tracer.after(ctx, operation, duration, statusCode, err)
	
	return err
}

// extractOperation gets the operation name for the request
func (h *observabilityHandler) extractOperation(ctx *lift.Context) string {
	if h.config.OperationNameFunc != nil {
		return h.config.OperationNameFunc(ctx)
	}
	return fmt.Sprintf("%s_%s", ctx.Request.Method, ctx.Request.Path)
}


// extractTenantID gets the tenant ID for the request
func (h *observabilityHandler) extractTenantID(ctx *lift.Context) string {
	if h.config.TenantIDFunc != nil {
		return h.config.TenantIDFunc(ctx)
	}
	return ctx.TenantID()
}

// extractUserID gets the user ID for the request
func (h *observabilityHandler) extractUserID(ctx *lift.Context) string {
	if h.config.UserIDFunc != nil {
		return h.config.UserIDFunc(ctx)
	}
	return ctx.UserID()
}

// determineStatusCode calculates the HTTP status code from context and error
func (h *observabilityHandler) determineStatusCode(ctx *lift.Context, err error) int {
	if ctx.Response.StatusCode != 0 {
		return ctx.Response.StatusCode
	}
	if err != nil {
		return 500
	}
	return 200
}

// loggingHandler handles the logging aspect of observability
type loggingHandler struct {
	config EnhancedObservabilityConfig
	logger observability.StructuredLogger
}

// newLoggingHandler creates a new logging handler
func newLoggingHandler(config EnhancedObservabilityConfig) *loggingHandler {
	return &loggingHandler{
		config: config,
		logger: config.Logger,
	}
}

// before handles logging setup before request processing
func (l *loggingHandler) before(ctx *lift.Context, operation, tenantID, userID string) {
	if !l.config.EnableLogging || l.logger == nil {
		return
	}
	
	contextLogger := l.logger.
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
	logFields := map[string]any{
		"operation":    operation,
		"method":       ctx.Request.Method,
		"path":         ctx.Request.Path,
		"query_params": "[SANITIZED_QUERY_PARAMS]",
		"source_ip":    ctx.Request.Headers["X-Forwarded-For"],
		"user_agent":   ctx.Request.Headers["User-Agent"],
		"tenant_id":    tenantID,
		"user_id":      userID,
	}
	
	if l.config.LogRequestBody && len(ctx.Request.Body) > 0 {
		logFields["request_body_size"] = len(ctx.Request.Body)
		logFields["request_body"] = "[USER_CONTENT_REDACTED]"
	}
	
	contextLogger.Info("Request started", logFields)
}

// after handles logging after request processing
func (l *loggingHandler) after(ctx *lift.Context, operation string, duration time.Duration, statusCode int, err error) {
	if !l.config.EnableLogging || ctx.Logger == nil {
		return
	}
	
	logFields := map[string]any{
		"operation": operation,
		"duration":  duration.String(),
		"status":    statusCode,
	}
	
	if l.config.LogResponseBody && ctx.Response.Body != nil {
		var bodySize int
		switch v := ctx.Response.Body.(type) {
		case string:
			bodySize = len(v)
		case []byte:
			bodySize = len(v)
		default:
			if data, jsonErr := json.Marshal(v); jsonErr == nil {
				bodySize = len(data)
			}
		}
		logFields["response_body_size"] = bodySize
		logFields["response_body"] = "[RESPONSE_CONTENT_REDACTED]"
	}
	
	if err != nil {
		logFields["error"] = "[SANITIZED_ERROR]"
		ctx.Logger.Error("Request failed", logFields)
	} else {
		ctx.Logger.Info("Request completed", logFields)
	}
}

// metricsHandler handles the metrics aspect of observability
type metricsHandler struct {
	config    EnhancedObservabilityConfig
	collector observability.MetricsCollector
	baseTags  map[string]string
}

// newMetricsHandler creates a new metrics handler
func newMetricsHandler(config EnhancedObservabilityConfig) *metricsHandler {
	baseTags := make(map[string]string)
	for k, v := range config.DefaultTags {
		baseTags[k] = v
	}
	
	return &metricsHandler{
		config:    config,
		collector: config.Metrics,
		baseTags:  baseTags,
	}
}

// before handles metrics setup before request processing
func (m *metricsHandler) before(ctx *lift.Context, operation, tenantID, userID string) {
	if !m.config.EnableMetrics || m.collector == nil {
		return
	}
	
	// Record request count
	tags := m.buildTags(ctx.Request.Method, ctx.Request.Path, tenantID, operation)
	counter := m.collector.WithTags(tags).Counter("requests.total")
	counter.Inc()
	
	// Record concurrent requests
	gauge := m.collector.WithTags(tags).Gauge("requests.active")
	gauge.Inc()
	
	// Store gauge in context for cleanup
	ctx.Set("metrics_active_gauge", gauge)
}

// after handles metrics after request processing
func (m *metricsHandler) after(ctx *lift.Context, operation, tenantID string, duration time.Duration, statusCode int, err error) {
	if !m.config.EnableMetrics || m.collector == nil {
		return
	}
	
	// Cleanup active requests gauge
	if gauge := ctx.Get("metrics_active_gauge"); gauge != nil {
		if g, ok := gauge.(interface{ Dec() }); ok {
			g.Dec()
		}
	}
	
	// Record response metrics
	statusTags := m.buildStatusTags(ctx.Request.Method, ctx.Request.Path, tenantID, operation, statusCode)
	statusMetrics := m.collector.WithTags(statusTags)
	
	// Record latency
	histogram := statusMetrics.Histogram("requests.duration")
	histogram.Observe(float64(duration.Milliseconds()))
	
	// Record response size
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
	
	// Record errors and operation-specific metrics
	if err != nil {
		m.recordError(operation, tenantID, err)
	}
	m.recordOperationMetrics(operation, tenantID, duration, err)
}

// buildTags creates base tags for metrics
func (m *metricsHandler) buildTags(method, path, tenantID, operation string) map[string]string {
	tags := make(map[string]string)
	for k, v := range m.baseTags {
		tags[k] = v
	}
	tags["method"] = method
	tags["path"] = path
	tags["tenant_id"] = tenantID
	tags["operation"] = operation
	return tags
}

// buildStatusTags creates tags including status information
func (m *metricsHandler) buildStatusTags(method, path, tenantID, operation string, statusCode int) map[string]string {
	tags := m.buildTags(method, path, tenantID, operation)
	tags["status"] = fmt.Sprintf("%d", statusCode)
	tags["status_class"] = fmt.Sprintf("%dxx", statusCode/100)
	return tags
}

// recordError records error-specific metrics
func (m *metricsHandler) recordError(operation, tenantID string, err error) {
	errorTags := make(map[string]string)
	for k, v := range m.baseTags {
		errorTags[k] = v
	}
	errorTags["error_type"] = fmt.Sprintf("%T", err)
	errorTags["operation"] = operation
	errorTags["tenant_id"] = tenantID
	
	errorMetrics := m.collector.WithTags(errorTags)
	errorCounter := errorMetrics.Counter("requests.errors")
	errorCounter.Inc()
}

// recordOperationMetrics records operation-specific metrics
func (m *metricsHandler) recordOperationMetrics(operation, tenantID string, duration time.Duration, err error) {
	operationTags := map[string]string{
		"operation": operation,
		"tenant_id": tenantID,
	}
	operationMetrics := m.collector.WithTags(operationTags)
	
	opCounter := operationMetrics.Counter(fmt.Sprintf("operation.%s.total", operation))
	opCounter.Inc()
	
	opHistogram := operationMetrics.Histogram(fmt.Sprintf("operation.%s.duration", operation))
	opHistogram.Observe(float64(duration.Milliseconds()))
	
	if err != nil {
		opErrorCounter := operationMetrics.Counter(fmt.Sprintf("operation.%s.errors", operation))
		opErrorCounter.Inc()
	}
}

// tracingHandler handles the tracing aspect of observability
type tracingHandler struct {
	config EnhancedObservabilityConfig
}

// newTracingHandler creates a new tracing handler
func newTracingHandler(config EnhancedObservabilityConfig) *tracingHandler {
	return &tracingHandler{
		config: config,
	}
}

// before handles tracing setup before request processing
func (t *tracingHandler) before(ctx *lift.Context, operation, tenantID, userID string) {
	if !t.config.EnableTracing {
		return
	}
	
	// Add custom annotations to current trace
	xray.AddAnnotation(ctx.Context, "operation", operation)
	xray.AddAnnotation(ctx.Context, "tenant_id", tenantID)
	xray.AddAnnotation(ctx.Context, "user_id", userID)
	
	// Add metadata
	xray.AddMetadata(ctx.Context, "request", "operation", operation)
	xray.AddMetadata(ctx.Context, "request", "tenant_id", tenantID)
	xray.AddMetadata(ctx.Context, "request", "user_id", userID)
}

// after handles tracing after request processing
func (t *tracingHandler) after(ctx *lift.Context, operation string, duration time.Duration, statusCode int, err error) {
	if !t.config.EnableTracing {
		return
	}
	
	// Record that we generated a trace
	recordTraceGenerated()
	
	// Add timing information
	xray.AddMetadata(ctx.Context, "timing", "duration_ms", duration.Milliseconds())
	xray.AddMetadata(ctx.Context, "timing", "start_time", time.Now().Add(-duration).Format(time.RFC3339Nano))
	xray.AddMetadata(ctx.Context, "timing", "end_time", time.Now().Format(time.RFC3339Nano))
	
	// Add response information
	xray.AddAnnotation(ctx.Context, "http.status_code", statusCode)
	xray.AddMetadata(ctx.Context, "response", "status_code", statusCode)
	
	// Add error information
	if err != nil {
		recordTraceError()
		xray.SetError(ctx.Context, err)
		xray.AddAnnotation(ctx.Context, "error", "true")
		xray.AddMetadata(ctx.Context, "error", "type", fmt.Sprintf("%T", err))
	} else {
		xray.AddAnnotation(ctx.Context, "error", "false")
	}
}
