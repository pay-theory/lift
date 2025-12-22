package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/oklog/ulid/v2"
	"github.com/pay-theory/dynamorm/pkg/core"
	dynamormerrors "github.com/pay-theory/dynamorm/pkg/errors"
)

// minInt returns the smaller of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// DynamoDBEventBus implements EventBus using DynamORM-backed DynamoDB storage.
// This implementation is serverless-safe and survives Lambda container recycling.
type DynamoDBEventBus struct {
	handlers map[string][]EventHandler // For local processing if needed (24 bytes)
	db       core.ExtendedDB           // DynamORM DB (8 bytes)
	cwClient *cloudwatch.Client        // CloudWatch client (8 bytes)
	config   EventBusConfig            // Configuration (variable size)
}

// NewDynamoDBEventBus creates a new DynamoDB-backed event bus using DynamORM.
func NewDynamoDBEventBus(db core.ExtendedDB, config EventBusConfig) *DynamoDBEventBus {
	// Apply defaults
	if config.TTL == 0 {
		config.TTL = 30 * 24 * time.Hour
	}
	if config.RetryAttempts == 0 {
		config.RetryAttempts = 3
	}
	if config.RetryBaseDelay == 0 {
		config.RetryBaseDelay = 100 * time.Millisecond
	}
	if config.MaxBatchSize == 0 {
		config.MaxBatchSize = 25
	}
	if config.MetricsNamespace == "" {
		config.MetricsNamespace = "Lift/EventBus"
	}
	if config.TableName != "" {
		// Override Event.TableName() for the process lifetime.
		// DynamORM caches table metadata per model type, so table names must be stable.
		_ = setEventBusTableNameOverride(config.TableName)
	} else {
		config.TableName = (&Event{}).TableName()
	}

	return &DynamoDBEventBus{
		db:       db,
		config:   config,
		handlers: make(map[string][]EventHandler),
	}
}

// WithCloudWatch adds CloudWatch client for metrics
func (d *DynamoDBEventBus) WithCloudWatch(client *cloudwatch.Client) *DynamoDBEventBus {
	d.cwClient = client
	d.config.EnableMetrics = true
	return d
}

// Publish publishes an event to DynamoDB with retry logic
func (d *DynamoDBEventBus) Publish(ctx context.Context, event *Event) (string, error) {
	startTime := time.Now()

	if event == nil {
		return "", fmt.Errorf("event cannot be nil")
	}

	// Generate ID if not set
	if event.ID == "" {
		event.ID = ulid.Make().String()
	}

	// Set timestamps
	now := time.Now()
	if event.PublishedAt.IsZero() {
		event.PublishedAt = now
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = now
	}

	// Set partition and sort keys if not set
	if event.PartitionKey == "" {
		event.PartitionKey = fmt.Sprintf("%s#%s", event.TenantID, event.EventType)
	}
	if event.SortKey == "" {
		event.SortKey = fmt.Sprintf("%d#%s", now.UnixNano(), event.ID)
	}

	// Set TTL if configured
	if d.config.TTL > 0 && event.ExpiresAt.IsZero() {
		event.ExpiresAt = now.Add(d.config.TTL)
	}
	if !event.ExpiresAt.IsZero() {
		event.TTL = event.ExpiresAt.Unix()
	}

	// Put item with retry
	var lastErr error
	for attempt := 0; attempt <= d.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			// Exponential backoff (capped at reasonable maximum)
			backoffMultiplier := 1 << minInt(attempt-1, 10) // Cap at 2^10 to prevent overflow
			delay := d.config.RetryBaseDelay * time.Duration(backoffMultiplier)
			time.Sleep(delay)
		}

		err := d.db.WithContext(ctx).Model(event).IfNotExists().Create()
		if err == nil {
			// Success
			duration := time.Since(startTime)
			d.emitMetric(ctx, "PublishSuccess", 1, map[string]string{
				"event_type": event.EventType,
				"tenant_id":  event.TenantID,
			})
			d.emitMetric(ctx, "PublishLatency", float64(duration.Milliseconds()), map[string]string{
				"event_type": event.EventType,
			})
			return event.ID, nil
		}

		if errors.Is(err, dynamormerrors.ErrConditionFailed) {
			// Idempotent publish: the event already exists under the provided PK/SK.
			// Treat this as success to avoid overwriting and re-triggering downstream processing.
			d.emitMetric(ctx, "PublishDeduped", 1, map[string]string{
				"event_type": event.EventType,
				"tenant_id":  event.TenantID,
			})
			return event.ID, nil
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryableError(err) {
			break
		}
	}

	d.emitMetric(ctx, "PublishError", 1, map[string]string{
		"error_type": "put_item_failed",
		"event_type": event.EventType,
	})
	return "", fmt.Errorf("failed to publish event after %d attempts: %w", d.config.RetryAttempts+1, lastErr)
}

// Query retrieves events based on filters using DynamoDB query
func (d *DynamoDBEventBus) Query(ctx context.Context, query *EventQuery) ([]*Event, error) {
	if query == nil {
		return nil, fmt.Errorf("query cannot be nil")
	}
	if query.TenantID == "" {
		return nil, fmt.Errorf("tenant_id is required for queries")
	}

	// For tenant-wide queries without event type, use the GSI.
	// For event-type specific queries, use the main table with composite key.
	useGSI := query.EventType == ""

	q := d.db.WithContext(ctx).Model(&Event{})

	if useGSI {
		q = q.Index("tenant-timestamp-index").
			Where("TenantID", "=", query.TenantID)

		applyPublishedAtTimeRange(q, query)
		q = q.OrderBy("PublishedAt", "DESC")
	} else {
		partitionKey := fmt.Sprintf("%s#%s", query.TenantID, query.EventType)
		q = q.Where("PartitionKey", "=", partitionKey)

		applySortKeyTimeRange(q, query)
		q = q.OrderBy("SortKey", "DESC")
	}

	// Add tag filters (AND semantics)
	for _, tag := range query.Tags {
		if tag == "" {
			continue
		}
		q = q.Filter("Tags", "CONTAINS", tag)
	}

	limit := 100
	if query.Limit > 0 && query.Limit <= 1000 {
		limit = query.Limit
	}
	q = q.Limit(limit)

	if query.LastEvaluatedKey != nil {
		if cursor, ok := query.LastEvaluatedKey["cursor"].(string); ok && cursor != "" {
			q = q.Cursor(cursor)
		}
	}

	var events []*Event
	page, err := q.AllPaginated(&events)
	if err != nil {
		d.emitMetric(ctx, "QueryError", 1, map[string]string{
			"error_type": "query_failed",
		})
		return nil, fmt.Errorf("failed to query events: %w", err)
	}

	if page != nil && page.HasMore && page.NextCursor != "" {
		query.NextKey = map[string]interface{}{"cursor": page.NextCursor}
	} else {
		query.NextKey = nil
	}

	d.emitMetric(ctx, "QuerySuccess", 1, map[string]string{
		"event_type": query.EventType,
	})

	return events, nil
}

func applySortKeyTimeRange(q core.Query, query *EventQuery) {
	switch {
	case query.StartTime != nil && query.EndTime != nil:
		startKey := fmt.Sprintf("%d#", query.StartTime.UnixNano())
		endKey := fmt.Sprintf("%d#", query.EndTime.UnixNano()+1) // exclusive upper bound
		q.Where("SortKey", "BETWEEN", []any{startKey, endKey})
	case query.StartTime != nil:
		startKey := fmt.Sprintf("%d#", query.StartTime.UnixNano())
		q.Where("SortKey", ">=", startKey)
	case query.EndTime != nil:
		endKey := fmt.Sprintf("%d#", query.EndTime.UnixNano())
		q.Where("SortKey", "<", endKey)
	}
}

func applyPublishedAtTimeRange(q core.Query, query *EventQuery) {
	switch {
	case query.StartTime != nil && query.EndTime != nil:
		q.Where("PublishedAt", "BETWEEN", []any{*query.StartTime, *query.EndTime})
	case query.StartTime != nil:
		q.Where("PublishedAt", ">=", *query.StartTime)
	case query.EndTime != nil:
		q.Where("PublishedAt", "<=", *query.EndTime)
	}
}

// Subscribe registers a handler (for DynamoDB Streams integration)
// Note: This stores handlers in memory and won't persist across Lambda restarts
// For production, use DynamoDB Streams + Lambda triggers instead
func (d *DynamoDBEventBus) Subscribe(_ context.Context, eventType string, handler EventHandler) error {
	if eventType == "" {
		return fmt.Errorf("event type cannot be empty")
	}
	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}

	d.handlers[eventType] = append(d.handlers[eventType], handler)
	return nil
}

// GetEvent retrieves a specific event by ID
func (d *DynamoDBEventBus) GetEvent(ctx context.Context, eventID string) (*Event, error) {
	if eventID == "" {
		return nil, fmt.Errorf("event ID cannot be empty")
	}

	var event Event
	err := d.db.WithContext(ctx).Model(&Event{}).
		Index("event-id-index").
		Where("ID", "=", eventID).
		First(&event)
	if err != nil {
		if errors.Is(err, dynamormerrors.ErrItemNotFound) {
			return nil, fmt.Errorf("event not found: %s", eventID)
		}
		return nil, fmt.Errorf("failed to get event: %w", err)
	}

	return &event, nil
}

// DeleteEvent removes an event from DynamoDB
func (d *DynamoDBEventBus) DeleteEvent(ctx context.Context, eventID string) error {
	if eventID == "" {
		return fmt.Errorf("event ID cannot be empty")
	}

	// First, get the event to find its partition and sort keys
	event, err := d.GetEvent(ctx, eventID)
	if err != nil {
		return err
	}

	err = d.db.WithContext(ctx).Model(&Event{}).
		Where("PartitionKey", "=", event.PartitionKey).
		Where("SortKey", "=", event.SortKey).
		Delete()
	if err != nil {
		d.emitMetric(ctx, "DeleteError", 1, map[string]string{
			"error_type": "delete_failed",
		})
		return fmt.Errorf("failed to delete event: %w", err)
	}

	d.emitMetric(ctx, "DeleteSuccess", 1, nil)
	return nil
}

// emitMetric sends a metric to CloudWatch if enabled
func (d *DynamoDBEventBus) emitMetric(ctx context.Context, metricName string, value float64, dimensions map[string]string) {
	if !d.config.EnableMetrics || d.cwClient == nil {
		return
	}

	// Build dimensions with pre-allocation
	metricDims := make([]types.Dimension, 0, len(dimensions)+1)
	for k, v := range dimensions {
		metricDims = append(metricDims, types.Dimension{
			Name:  aws.String(k),
			Value: aws.String(v),
		})
	}

	// Add standard dimensions
	metricDims = append(metricDims, types.Dimension{
		Name:  aws.String("TableName"),
		Value: aws.String(d.config.TableName),
	})

	// Send metric (fire and forget, don't block)
	go func() {
		// Intentionally ignore errors - metrics are best-effort
		// nolint:errcheck
		_, _ = d.cwClient.PutMetricData(ctx, &cloudwatch.PutMetricDataInput{
			Namespace: aws.String(d.config.MetricsNamespace),
			MetricData: []types.MetricDatum{
				{
					MetricName: aws.String(metricName),
					Value:      aws.Float64(value),
					Timestamp:  aws.Time(time.Now()),
					Dimensions: metricDims,
					Unit:       types.StandardUnitCount,
				},
			},
		})
	}()
}

// isRetryableError checks if an error is retryable using AWS SDK error types
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check error message for common retryable error patterns
	errMsg := err.Error()
	retryableErrors := []string{
		"ProvisionedThroughputExceededException",
		"ThrottlingException",
		"RequestLimitExceeded",
		"ServiceUnavailable",
		"InternalServerError",
		"RequestThrottled",
	}

	for _, retryable := range retryableErrors {
		if stringContains(errMsg, retryable) {
			return true
		}
	}

	return false
}

// stringContains performs a simple substring search
func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// BatchPublish publishes multiple events in a batch operation.
// This is more efficient than publishing events one at a time.
func (d *DynamoDBEventBus) BatchPublish(ctx context.Context, events []*Event) ([]string, error) {
	if len(events) == 0 {
		return nil, fmt.Errorf("no events to publish")
	}

	eventIDs := make([]string, 0, len(events))
	now := time.Now()

	for _, event := range events {
		if event == nil {
			return eventIDs, fmt.Errorf("event cannot be nil")
		}

		if event.ID == "" {
			event.ID = ulid.Make().String()
		}

		if event.PublishedAt.IsZero() {
			event.PublishedAt = now
		}
		if event.CreatedAt.IsZero() {
			event.CreatedAt = now
		}

		if event.PartitionKey == "" {
			event.PartitionKey = fmt.Sprintf("%s#%s", event.TenantID, event.EventType)
		}
		if event.SortKey == "" {
			event.SortKey = fmt.Sprintf("%d#%s", now.UnixNano(), event.ID)
		}

		if d.config.TTL > 0 && event.ExpiresAt.IsZero() {
			event.ExpiresAt = now.Add(d.config.TTL)
		}
		if !event.ExpiresAt.IsZero() {
			event.TTL = event.ExpiresAt.Unix()
		}

		eventIDs = append(eventIDs, event.ID)
	}

	if err := d.db.WithContext(ctx).Model(&Event{}).BatchCreate(events); err != nil {
		d.emitMetric(ctx, "BatchPublishError", 1, map[string]string{
			"error_type": "batch_write_failed",
		})
		return eventIDs, fmt.Errorf("failed to batch publish events: %w", err)
	}

	d.emitMetric(ctx, "BatchPublishSuccess", float64(len(events)), nil)
	return eventIDs, nil
}
