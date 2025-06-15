package cloudwatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/observability"
)

// CloudWatchLogger implements the StructuredLogger interface with CloudWatch Logs backend
type CloudWatchLogger struct {
	client        observability.CloudWatchLogsClient
	logGroup      string
	logStream     string
	batchSize     int
	flushInterval time.Duration
	buffer        chan *observability.LogEntry
	flushSignal   chan struct{} // Signal to force immediate flush
	done          chan struct{}
	wg            sync.WaitGroup
	mu            sync.RWMutex
	sequenceToken *string
	stats         *loggerStats
	contextFields map[string]interface{}
}

// loggerStats tracks performance metrics
type loggerStats struct {
	entriesLogged    int64
	entriesDropped   int64
	flushCount       int64
	lastFlush        int64 // Unix timestamp
	errorCount       int64
	lastError        string
	averageFlushTime int64 // Nanoseconds
}

// NewCloudWatchLogger creates a new CloudWatch logger instance
func NewCloudWatchLogger(config observability.LoggerConfig, client observability.CloudWatchLogsClient) (*CloudWatchLogger, error) {
	if config.BatchSize <= 0 {
		config.BatchSize = 25 // CloudWatch Logs max batch size
	}
	if config.FlushInterval <= 0 {
		config.FlushInterval = 5 * time.Second
	}
	if config.BufferSize <= 0 {
		config.BufferSize = config.BatchSize * 2
	}

	logger := &CloudWatchLogger{
		client:        client,
		logGroup:      config.LogGroup,
		logStream:     config.LogStream,
		batchSize:     config.BatchSize,
		flushInterval: config.FlushInterval,
		buffer:        make(chan *observability.LogEntry, config.BufferSize),
		flushSignal:   make(chan struct{}),
		done:          make(chan struct{}),
		stats:         &loggerStats{},
		contextFields: make(map[string]interface{}),
	}

	// Ensure log group and stream exist
	if err := logger.ensureLogGroupExists(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ensure log group exists: %w", err)
	}

	if err := logger.ensureLogStreamExists(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ensure log stream exists: %w", err)
	}

	// Start background flusher
	logger.wg.Add(1)
	go logger.flushLoop()

	return logger, nil
}

// Debug logs a debug message (with enhanced sanitization for security)
func (l *CloudWatchLogger) Debug(message string, fields ...map[string]interface{}) {
	l.log("DEBUG", message, fields...)
}

// Info logs an info message
func (l *CloudWatchLogger) Info(message string, fields ...map[string]interface{}) {
	l.log("INFO", message, fields...)
}

// Warn logs a warning message
func (l *CloudWatchLogger) Warn(message string, fields ...map[string]interface{}) {
	l.log("WARN", message, fields...)
}

// Error logs an error message
func (l *CloudWatchLogger) Error(message string, fields ...map[string]interface{}) {
	l.log("ERROR", message, fields...)
}

// WithField returns a new logger with an additional field
func (l *CloudWatchLogger) WithField(key string, value interface{}) lift.Logger {
	return l.WithFields(map[string]interface{}{key: value})
}

// WithFields returns a new logger with additional fields
func (l *CloudWatchLogger) WithFields(fields map[string]interface{}) lift.Logger {
	newFields := make(map[string]interface{})
	for k, v := range l.contextFields {
		newFields[k] = v
	}
	for k, v := range fields {
		newFields[k] = v
	}

	return &CloudWatchLogger{
		client:        l.client,
		logGroup:      l.logGroup,
		logStream:     l.logStream,
		batchSize:     l.batchSize,
		flushInterval: l.flushInterval,
		buffer:        l.buffer,        // Share the same buffer
		flushSignal:   l.flushSignal,   // Share the same flush signal
		done:          l.done,          // Share the same done channel
		stats:         l.stats,         // Share the same stats
		contextFields: newFields,       // Only context fields are different
		sequenceToken: l.sequenceToken, // Share sequence token
		mu:            l.mu,            // Share mutex
		wg:            l.wg,            // Share wait group
	}
}

// WithRequestID adds request ID to logger context
func (l *CloudWatchLogger) WithRequestID(requestID string) observability.StructuredLogger {
	return l.WithField("request_id", requestID).(observability.StructuredLogger)
}

// WithTenantID adds tenant ID to logger context
func (l *CloudWatchLogger) WithTenantID(tenantID string) observability.StructuredLogger {
	return l.WithField("tenant_id", tenantID).(observability.StructuredLogger)
}

// WithUserID adds user ID to logger context
func (l *CloudWatchLogger) WithUserID(userID string) observability.StructuredLogger {
	return l.WithField("user_id", userID).(observability.StructuredLogger)
}

// WithTraceID adds trace ID to logger context
func (l *CloudWatchLogger) WithTraceID(traceID string) observability.StructuredLogger {
	return l.WithField("trace_id", traceID).(observability.StructuredLogger)
}

// WithSpanID adds span ID to logger context
func (l *CloudWatchLogger) WithSpanID(spanID string) observability.StructuredLogger {
	return l.WithField("span_id", spanID).(observability.StructuredLogger)
}

// log is the internal logging method
func (l *CloudWatchLogger) log(level, message string, fieldMaps ...map[string]interface{}) {
	entry := &observability.LogEntry{
		Timestamp: time.Now().UTC(),
		Level:     level,
		Message:   message,
		Fields:    make(map[string]interface{}),
	}

	// Add context fields
	for k, v := range l.contextFields {
		switch k {
		case "request_id":
			if s, ok := v.(string); ok {
				entry.RequestID = s
			}
		case "tenant_id":
			if s, ok := v.(string); ok {
				entry.TenantID = s
			}
		case "user_id":
			if s, ok := v.(string); ok {
				entry.UserID = s
			}
		case "trace_id":
			if s, ok := v.(string); ok {
				entry.TraceID = s
			}
		case "span_id":
			if s, ok := v.(string); ok {
				entry.SpanID = s
			}
		default:
			entry.Fields[k] = l.sanitizeFieldValue(k, v)
		}
	}

	// Merge all field maps with sanitization
	for _, fieldMap := range fieldMaps {
		for k, v := range fieldMap {
			entry.Fields[k] = l.sanitizeFieldValue(k, v)
		}
	}

	// Non-blocking send to buffer
	select {
	case l.buffer <- entry:
		atomic.AddInt64(&l.stats.entriesLogged, 1)
	default:
		// Buffer full, drop log entry
		atomic.AddInt64(&l.stats.entriesDropped, 1)
	}
}

// sanitizeFieldValue sanitizes field values to prevent sensitive data exposure
func (l *CloudWatchLogger) sanitizeFieldValue(key string, value interface{}) interface{} {
	keyLower := strings.ToLower(key)

	// Always sanitize highly sensitive field names
	highSensitiveFields := []string{
		"password", "token", "secret", "key", "auth", "credential",
		"email", "phone", "ssn", "card", "account", "routing",
		"pin", "cvv", "security", "private", "confidential",
	}

	for _, sensitive := range highSensitiveFields {
		if strings.Contains(keyLower, sensitive) {
			return "[REDACTED]"
		}
	}

	// Sanitize user-generated content fields
	userContentFields := []string{
		"body", "request_body", "response_body", "user_input",
		"query", "search", "message", "comment", "description",
	}

	for _, userField := range userContentFields {
		if strings.Contains(keyLower, userField) {
			if str, ok := value.(string); ok && len(str) > 0 {
				// For user content, only show length and type
				return fmt.Sprintf("[USER_CONTENT_%d_CHARS]", len(str))
			}
			return "[USER_CONTENT]"
		}
	}

	// Sanitize error messages that might contain user data
	if keyLower == "error" || strings.Contains(keyLower, "error") {
		if str, ok := value.(string); ok {
			// Only show system error types, not detailed messages
			if len(str) > 50 ||
				strings.Contains(strings.ToLower(str), "input") ||
				strings.Contains(strings.ToLower(str), "invalid") {
				return "[SANITIZED_ERROR]"
			}
		}
	}

	// Check for very long strings that likely contain user data
	if str, ok := value.(string); ok && len(str) > 200 {
		return fmt.Sprintf("[LARGE_STRING_%d_CHARS]", len(str))
	}

	return value
}

// flushLoop runs in background to batch and send logs
func (l *CloudWatchLogger) flushLoop() {
	defer l.wg.Done()

	ticker := time.NewTicker(l.flushInterval)
	defer ticker.Stop()

	var batch []*observability.LogEntry

	for {
		select {
		case entry := <-l.buffer:
			batch = append(batch, entry)
			if len(batch) >= l.batchSize {
				l.flushBatch(batch)
				batch = batch[:0] // Reset slice
			}

		case <-l.flushSignal:
			// Force immediate flush of current batch
			if len(batch) > 0 {
				l.flushBatch(batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			if len(batch) > 0 {
				l.flushBatch(batch)
				batch = batch[:0]
			}

		case <-l.done:
			// Flush remaining entries
			if len(batch) > 0 {
				l.flushBatch(batch)
			}
			// Drain buffer
			for {
				select {
				case entry := <-l.buffer:
					batch = append(batch, entry)
					if len(batch) >= l.batchSize {
						l.flushBatch(batch)
						batch = batch[:0]
					}
				default:
					if len(batch) > 0 {
						l.flushBatch(batch)
					}
					return
				}
			}
		}
	}
}

// flushBatch sends a batch of log entries to CloudWatch
func (l *CloudWatchLogger) flushBatch(batch []*observability.LogEntry) {
	if len(batch) == 0 {
		return
	}

	start := time.Now()
	defer func() {
		duration := time.Since(start)
		atomic.AddInt64(&l.stats.flushCount, 1)
		atomic.StoreInt64(&l.stats.lastFlush, time.Now().Unix())

		// Update average flush time
		currentAvg := atomic.LoadInt64(&l.stats.averageFlushTime)
		flushCount := atomic.LoadInt64(&l.stats.flushCount)
		newAvg := (currentAvg*(flushCount-1) + duration.Nanoseconds()) / flushCount
		atomic.StoreInt64(&l.stats.averageFlushTime, newAvg)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Convert entries to CloudWatch format
	events := make([]types.InputLogEvent, len(batch))
	for i, entry := range batch {
		message, _ := json.Marshal(entry)
		events[i] = types.InputLogEvent{
			Message:   aws.String(string(message)),
			Timestamp: aws.Int64(entry.Timestamp.UnixMilli()),
		}
	}

	// Send to CloudWatch
	l.mu.Lock()
	input := &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  aws.String(l.logGroup),
		LogStreamName: aws.String(l.logStream),
		LogEvents:     events,
		SequenceToken: l.sequenceToken,
	}
	l.mu.Unlock()

	output, err := l.client.PutLogEvents(ctx, input)
	if err != nil {
		atomic.AddInt64(&l.stats.errorCount, 1)
		l.stats.lastError = err.Error()
		return
	}

	// Update sequence token
	l.mu.Lock()
	l.sequenceToken = output.NextSequenceToken
	l.mu.Unlock()
}

// ensureLogGroupExists creates the log group if it doesn't exist
func (l *CloudWatchLogger) ensureLogGroupExists(ctx context.Context) error {
	_, err := l.client.CreateLogGroup(ctx, &cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: aws.String(l.logGroup),
	})

	// Ignore if already exists
	if err != nil {
		var alreadyExists *types.ResourceAlreadyExistsException
		if !errors.As(err, &alreadyExists) {
			return err
		}
	}

	return nil
}

// ensureLogStreamExists creates the log stream if it doesn't exist
func (l *CloudWatchLogger) ensureLogStreamExists(ctx context.Context) error {
	_, err := l.client.CreateLogStream(ctx, &cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  aws.String(l.logGroup),
		LogStreamName: aws.String(l.logStream),
	})

	// Ignore if already exists
	if err != nil {
		var alreadyExists *types.ResourceAlreadyExistsException
		if !errors.As(err, &alreadyExists) {
			return err
		}
	}

	return nil
}

// Flush forces a flush of buffered log entries
func (l *CloudWatchLogger) Flush(ctx context.Context) error {
	// Give a small delay to ensure any recent log entries have been buffered
	time.Sleep(25 * time.Millisecond)

	// Send flush signal to background loop
	select {
	case l.flushSignal <- struct{}{}:
		// Signal sent successfully
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(1 * time.Second):
		return fmt.Errorf("flush signal timeout")
	}

	// Give a little time for the background loop to process the flush
	time.Sleep(50 * time.Millisecond)

	return nil
}

// Close gracefully shuts down the logger
func (l *CloudWatchLogger) Close() error {
	close(l.done)
	l.wg.Wait()
	return nil
}

// IsHealthy returns true if the logger is functioning properly
func (l *CloudWatchLogger) IsHealthy() bool {
	errorCount := atomic.LoadInt64(&l.stats.errorCount)
	entriesLogged := atomic.LoadInt64(&l.stats.entriesLogged)

	// Consider healthy if:
	// 1. No errors at all, OR
	// 2. We haven't logged anything yet, OR
	// 3. Error rate is less than 10% of total entries
	if errorCount == 0 {
		return true
	}
	if entriesLogged == 0 {
		return true
	}

	errorRate := float64(errorCount) / float64(entriesLogged)
	return errorRate < 0.1 // Less than 10% error rate
}

// GetStats returns logger performance statistics
func (l *CloudWatchLogger) GetStats() observability.LoggerStats {
	return observability.LoggerStats{
		EntriesLogged:    atomic.LoadInt64(&l.stats.entriesLogged),
		EntriesDropped:   atomic.LoadInt64(&l.stats.entriesDropped),
		FlushCount:       atomic.LoadInt64(&l.stats.flushCount),
		LastFlush:        time.Unix(atomic.LoadInt64(&l.stats.lastFlush), 0),
		BufferSize:       len(l.buffer),
		BufferCapacity:   cap(l.buffer),
		AverageFlushTime: time.Duration(atomic.LoadInt64(&l.stats.averageFlushTime)),
		ErrorCount:       atomic.LoadInt64(&l.stats.errorCount),
		LastError:        l.stats.lastError,
	}
}
