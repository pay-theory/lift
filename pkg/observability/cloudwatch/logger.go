package cloudwatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// Debug logs a debug message
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
			entry.Fields[k] = v
		}
	}

	// Merge all field maps
	for _, fieldMap := range fieldMaps {
		for k, v := range fieldMap {
			entry.Fields[k] = v
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
	// Create a temporary slice to collect buffered entries
	var entries []*observability.LogEntry

	// Drain the buffer with timeout
	timeout := time.After(1 * time.Second)

	for {
		select {
		case entry := <-l.buffer:
			entries = append(entries, entry)
			// Continue draining until buffer is empty
		case <-timeout:
			// Timeout reached, flush what we have
			if len(entries) > 0 {
				l.flushBatch(entries)
			}
			return nil
		default:
			// Buffer is empty, flush collected entries
			if len(entries) > 0 {
				l.flushBatch(entries)
			}
			return nil
		}
	}
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
