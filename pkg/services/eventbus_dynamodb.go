package services

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/oklog/ulid/v2"
)

// minInt returns the smaller of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// DynamoDBEventBus implements EventBus using DynamoDB for durable storage
// This implementation is serverless-safe and survives Lambda container recycling
type DynamoDBEventBus struct {
	handlers map[string][]EventHandler // For local processing if needed (24 bytes)
	client   *dynamodb.Client          // DynamoDB client (8 bytes)
	cwClient *cloudwatch.Client        // CloudWatch client (8 bytes)
	config   EventBusConfig            // Configuration (variable size)
}

// NewDynamoDBEventBus creates a new DynamoDB-backed event bus
func NewDynamoDBEventBus(client *dynamodb.Client, config EventBusConfig) *DynamoDBEventBus {
	// Apply defaults
	if config.TableName == "" {
		config.TableName = "lift-events"
	}
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

	return &DynamoDBEventBus{
		client:   client,
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

	// Convert to DynamoDB attribute values
	item, err := attributevalue.MarshalMap(event)
	if err != nil {
		d.emitMetric(ctx, "PublishError", 1, map[string]string{
			"error_type": "marshal_error",
		})
		return "", fmt.Errorf("failed to marshal event: %w", err)
	}

	// Add TTL attribute (Unix timestamp)
	if !event.ExpiresAt.IsZero() {
		item["ttl"] = &dynamodbtypes.AttributeValueMemberN{
			Value: fmt.Sprintf("%d", event.ExpiresAt.Unix()),
		}
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

		_, err = d.client.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String(d.config.TableName),
			Item:      item,
		})

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

	// For tenant-wide queries without event type, use the GSI
	// For event-type specific queries, use the main table with composite key
	var keyCondition expression.KeyConditionBuilder
	var indexName *string
	useGSI := query.EventType == ""

	if query.EventType != "" {
		// Query specific event type using main table
		partitionKey := fmt.Sprintf("%s#%s", query.TenantID, query.EventType)
		keyCondition = expression.Key("pk").Equal(expression.Value(partitionKey))
	} else {
		// Query all event types for tenant using GSI
		keyCondition = expression.Key("tenant_id").Equal(expression.Value(query.TenantID))
		indexName = aws.String("tenant-timestamp-index")
	}

	// Add time range to sort key condition if specified
	keyCondition = applyTimeRangeToKeyCondition(keyCondition, query, useGSI)

	// Build filter expression for tags if specified
	filterExpr, hasFilter := buildTagFilterExpression(query.Tags)

	// Build expression
	expr, err := buildQueryExpression(keyCondition, filterExpr, hasFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to build expression: %w", err)
	}

	// Set limit with safe conversion
	limit := int32(100)
	if query.Limit > 0 && query.Limit <= 1000 {
		limit = int32(query.Limit) // Safe: already validated range
	}

	// Execute query
	input := &dynamodb.QueryInput{
		TableName:                 aws.String(d.config.TableName),
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		Limit:                     aws.Int32(limit),
		ScanIndexForward:          aws.Bool(false), // Most recent first
		IndexName:                 indexName,
	}

	if expr.Filter() != nil {
		input.FilterExpression = expr.Filter()
	}

	if query.LastEvaluatedKey != nil {
		lastKeyMarshaled, marshalErr := attributevalue.MarshalMap(query.LastEvaluatedKey)
		if marshalErr == nil {
			input.ExclusiveStartKey = lastKeyMarshaled
		}
	}

	result, err := d.client.Query(ctx, input)
	if err != nil {
		d.emitMetric(ctx, "QueryError", 1, map[string]string{
			"error_type": "query_failed",
		})
		return nil, fmt.Errorf("failed to query events: %w", err)
	}

	// Unmarshal results
	var events []*Event
	err = attributevalue.UnmarshalListOfMaps(result.Items, &events)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal events: %w", err)
	}

	// Set pagination token if there are more results
	if result.LastEvaluatedKey != nil {
		nextKey := make(map[string]interface{})
		if unmarshalErr := attributevalue.UnmarshalMap(result.LastEvaluatedKey, &nextKey); unmarshalErr == nil {
			query.NextKey = nextKey
		}
	}

	d.emitMetric(ctx, "QuerySuccess", 1, map[string]string{
		"event_type": query.EventType,
	})

	return events, nil
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
// This uses a scan which can be slow - consider adding a GSI on ID if needed
func (d *DynamoDBEventBus) GetEvent(ctx context.Context, eventID string) (*Event, error) {
	if eventID == "" {
		return nil, fmt.Errorf("event ID cannot be empty")
	}

	// Build filter expression
	filter := expression.Name("id").Equal(expression.Value(eventID))
	expr, err := expression.NewBuilder().WithFilter(filter).Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build expression: %w", err)
	}

	// Scan for the event (consider adding GSI for better performance)
	result, err := d.client.Scan(ctx, &dynamodb.ScanInput{
		TableName:                 aws.String(d.config.TableName),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		Limit:                     aws.Int32(1),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get event: %w", err)
	}

	if len(result.Items) == 0 {
		return nil, fmt.Errorf("event not found: %s", eventID)
	}

	var event Event
	err = attributevalue.UnmarshalMap(result.Items[0], &event)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal event: %w", err)
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

	// Delete the item
	_, err = d.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(d.config.TableName),
		Key: map[string]dynamodbtypes.AttributeValue{
			"pk": &dynamodbtypes.AttributeValueMemberS{Value: event.PartitionKey},
			"sk": &dynamodbtypes.AttributeValueMemberS{Value: event.SortKey},
		},
	})
	if err != nil {
		d.emitMetric(ctx, "DeleteError", 1, map[string]string{
			"error_type": "delete_failed",
		})
		return fmt.Errorf("failed to delete event: %w", err)
	}

	d.emitMetric(ctx, "DeleteSuccess", 1, nil)
	return nil
}

// ProcessStreamRecord processes a DynamoDB Stream record
// This is typically called from a Lambda function triggered by DynamoDB Streams
func (d *DynamoDBEventBus) ProcessStreamRecord(ctx context.Context, record map[string]dynamodbtypes.AttributeValue) error {
	// Unmarshal the event
	var event Event
	err := attributevalue.UnmarshalMap(record, &event)
	if err != nil {
		return fmt.Errorf("failed to unmarshal stream record: %w", err)
	}

	// Call registered handlers
	if handlers, ok := d.handlers[event.EventType]; ok {
		for _, handler := range handlers {
			if err := handler(ctx, &event); err != nil {
				// Log error but continue processing other handlers
				d.emitMetric(ctx, "HandlerError", 1, map[string]string{
					"event_type": event.EventType,
				})
			}
		}
	}

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

// BatchPublish publishes multiple events in a batch operation
// This is more efficient than publishing events one at a time
func (d *DynamoDBEventBus) BatchPublish(ctx context.Context, events []*Event) ([]string, error) {
	if len(events) == 0 {
		return nil, fmt.Errorf("no events to publish")
	}

	eventIDs := make([]string, 0, len(events))
	batches := d.batchEvents(events)

	for _, batch := range batches {
		ids, err := d.publishBatch(ctx, batch)
		if err != nil {
			return eventIDs, err
		}
		eventIDs = append(eventIDs, ids...)
	}

	d.emitMetric(ctx, "BatchPublishSuccess", float64(len(events)), nil)
	return eventIDs, nil
}

// publishBatch processes a single batch of events
func (d *DynamoDBEventBus) publishBatch(ctx context.Context, batch []*Event) ([]string, error) {
	writeRequests := make([]dynamodbtypes.WriteRequest, 0, len(batch))
	eventIDs := make([]string, 0, len(batch))

	for _, event := range batch {
		if err := d.prepareEvent(event); err != nil {
			return eventIDs, err
		}

		item, err := d.marshalEventWithTTL(event)
		if err != nil {
			return eventIDs, err
		}

		writeRequests = append(writeRequests, dynamodbtypes.WriteRequest{
			PutRequest: &dynamodbtypes.PutRequest{Item: item},
		})
		eventIDs = append(eventIDs, event.ID)
	}

	return eventIDs, d.executeBatchWrite(ctx, writeRequests)
}

// prepareEvent sets IDs, timestamps, and keys for an event
func (d *DynamoDBEventBus) prepareEvent(event *Event) error {
	if event.ID == "" {
		event.ID = ulid.Make().String()
	}

	now := time.Now()
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

	return nil
}

// marshalEventWithTTL marshals an event and adds TTL attribute
func (d *DynamoDBEventBus) marshalEventWithTTL(event *Event) (map[string]dynamodbtypes.AttributeValue, error) {
	item, err := attributevalue.MarshalMap(event)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event: %w", err)
	}

	if !event.ExpiresAt.IsZero() {
		item["ttl"] = &dynamodbtypes.AttributeValueMemberN{
			Value: fmt.Sprintf("%d", event.ExpiresAt.Unix()),
		}
	}

	return item, nil
}

// executeBatchWrite executes a DynamoDB batch write operation with retry for unprocessed items
func (d *DynamoDBEventBus) executeBatchWrite(ctx context.Context, writeRequests []dynamodbtypes.WriteRequest) error {
	requestItems := map[string][]dynamodbtypes.WriteRequest{
		d.config.TableName: writeRequests,
	}

	attempt := 0
	for attempt <= d.config.RetryAttempts {
		if attempt > 0 {
			// Exponential backoff for retries
			backoffMultiplier := 1 << minInt(attempt-1, 10)
			delay := d.config.RetryBaseDelay * time.Duration(backoffMultiplier)
			time.Sleep(delay)
		}

		result, err := d.client.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
			RequestItems: requestItems,
		})

		if err != nil {
			if isRetryableError(err) && attempt < d.config.RetryAttempts {
				attempt++
				continue
			}
			d.emitMetric(ctx, "BatchPublishError", 1, map[string]string{
				"error_type": "batch_write_failed",
			})
			return fmt.Errorf("failed to batch publish events: %w", err)
		}

		// Check for unprocessed items
		if result.UnprocessedItems == nil || len(result.UnprocessedItems[d.config.TableName]) == 0 {
			// All items processed successfully
			return nil
		}

		// Retry unprocessed items
		requestItems = result.UnprocessedItems
		d.emitMetric(ctx, "BatchPublishRetry", float64(len(requestItems[d.config.TableName])), map[string]string{
			"attempt": fmt.Sprintf("%d", attempt+1),
		})
		attempt++
	}

	// Still have unprocessed items after all retries
	unprocessedCount := len(requestItems[d.config.TableName])
	d.emitMetric(ctx, "BatchPublishError", 1, map[string]string{
		"error_type":        "unprocessed_items",
		"unprocessed_count": fmt.Sprintf("%d", unprocessedCount),
	})
	return fmt.Errorf("failed to process %d items after %d attempts", unprocessedCount, d.config.RetryAttempts+1)
}

// batchEvents splits events into batches of max size
func (d *DynamoDBEventBus) batchEvents(events []*Event) [][]*Event {
	var batches [][]*Event
	for i := 0; i < len(events); i += d.config.MaxBatchSize {
		end := i + d.config.MaxBatchSize
		if end > len(events) {
			end = len(events)
		}
		batches = append(batches, events[i:end])
	}
	return batches
}
