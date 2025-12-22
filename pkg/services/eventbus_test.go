package services

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestPayload struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func TestNewEvent(t *testing.T) {
	payload := TestPayload{
		Name:  "John Doe",
		Email: "john@example.com",
	}

	event, err := NewEvent("user.created", "tenant-1", "user-123", payload)
	require.NoError(t, err)
	require.NotNil(t, event)

	assert.NotEmpty(t, event.ID)
	assert.Equal(t, "user.created", event.EventType)
	assert.Equal(t, "tenant-1", event.TenantID)
	assert.Equal(t, "user-123", event.SourceID)
	assert.NotZero(t, event.PublishedAt)
	assert.NotZero(t, event.CreatedAt)
	assert.Equal(t, "tenant-1#user.created", event.PartitionKey)
	assert.NotEmpty(t, event.SortKey)
	assert.Equal(t, 1, event.Version)

	// Unmarshal payload
	var decoded TestPayload
	err = event.UnmarshalPayload(&decoded)
	require.NoError(t, err)
	assert.Equal(t, payload, decoded)
}

func TestEventWithTTL(t *testing.T) {
	event, err := NewEvent("test.event", "tenant-1", "source-1", nil)
	require.NoError(t, err)

	ttl := 24 * time.Hour
	event.WithTTL(ttl)

	expectedExpiry := event.CreatedAt.Add(ttl)
	assert.WithinDuration(t, expectedExpiry, event.ExpiresAt, time.Second)
}

func TestEventWithMetadata(t *testing.T) {
	event, err := NewEvent("test.event", "tenant-1", "source-1", nil)
	require.NoError(t, err)

	event.WithMetadata("key1", "value1").
		WithMetadata("key2", "value2")

	assert.Equal(t, "value1", event.Metadata["key1"])
	assert.Equal(t, "value2", event.Metadata["key2"])
}

func TestEventWithTags(t *testing.T) {
	event, err := NewEvent("test.event", "tenant-1", "source-1", nil)
	require.NoError(t, err)

	event.WithTags("tag1", "tag2", "tag3")

	assert.Len(t, event.Tags, 3)
	assert.Contains(t, event.Tags, "tag1")
	assert.Contains(t, event.Tags, "tag2")
	assert.Contains(t, event.Tags, "tag3")
}

func TestEventWithCorrelationID(t *testing.T) {
	event, err := NewEvent("test.event", "tenant-1", "source-1", nil)
	require.NoError(t, err)

	event.WithCorrelationID("correlation-123")

	assert.Equal(t, "correlation-123", event.CorrelationID)
}

func TestMemoryEventBus_Publish(t *testing.T) {
	bus := NewMemoryEventBus()
	ctx := context.Background()

	payload := TestPayload{Name: "Test", Email: "test@example.com"}
	event, err := NewEvent("user.created", "tenant-1", "user-1", payload)
	require.NoError(t, err)

	eventID, err := bus.Publish(ctx, event)
	require.NoError(t, err)
	assert.NotEmpty(t, eventID)
	assert.Equal(t, event.ID, eventID)

	// Verify event was stored
	assert.Equal(t, 1, bus.Count())
}

func TestMemoryEventBus_PublishNilEvent(t *testing.T) {
	bus := NewMemoryEventBus()
	ctx := context.Background()

	_, err := bus.Publish(ctx, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "event cannot be nil")
}

func TestMemoryEventBus_Query(t *testing.T) {
	bus := NewMemoryEventBus()
	ctx := context.Background()

	// Publish multiple events
	for i := 0; i < 5; i++ {
		event, err := NewEvent("user.created", "tenant-1", "user-1", nil)
		require.NoError(t, err)
		_, err = bus.Publish(ctx, event)
		require.NoError(t, err)
	}

	// Query all events
	query := &EventQuery{
		TenantID:  "tenant-1",
		EventType: "user.created",
		Limit:     10,
	}

	events, err := bus.Query(ctx, query)
	require.NoError(t, err)
	assert.Len(t, events, 5)
}

func TestMemoryEventBus_QueryWithTimeRange(t *testing.T) {
	bus := NewMemoryEventBus()
	ctx := context.Background()

	now := time.Now()
	past := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)

	// Publish event
	event, err := NewEvent("test.event", "tenant-1", "source-1", nil)
	require.NoError(t, err)
	_, err = bus.Publish(ctx, event)
	require.NoError(t, err)

	// Query with time range
	query := &EventQuery{
		TenantID:  "tenant-1",
		EventType: "test.event",
		StartTime: &past,
		EndTime:   &future,
	}

	events, err := bus.Query(ctx, query)
	require.NoError(t, err)
	assert.Len(t, events, 1)

	// Query with future start time should return no events
	futureStart := now.Add(2 * time.Hour)
	query.StartTime = &futureStart
	events, err = bus.Query(ctx, query)
	require.NoError(t, err)
	assert.Len(t, events, 0)
}

func TestMemoryEventBus_QueryWithTags(t *testing.T) {
	bus := NewMemoryEventBus()
	ctx := context.Background()

	// Publish events with different tags
	event1, _ := NewEvent("test.event", "tenant-1", "source-1", nil)
	event1.WithTags("tag1", "tag2")
	_, err := bus.Publish(ctx, event1)
	require.NoError(t, err)

	event2, _ := NewEvent("test.event", "tenant-1", "source-2", nil)
	event2.WithTags("tag2", "tag3")
	_, err = bus.Publish(ctx, event2)
	require.NoError(t, err)

	// Query with single tag
	query := &EventQuery{
		TenantID:  "tenant-1",
		EventType: "test.event",
		Tags:      []string{"tag1"},
	}

	events, err := bus.Query(ctx, query)
	require.NoError(t, err)
	assert.Len(t, events, 1)

	// Query with multiple tags (AND logic)
	query.Tags = []string{"tag2", "tag3"}
	events, err = bus.Query(ctx, query)
	require.NoError(t, err)
	assert.Len(t, events, 1)
}

func TestMemoryEventBus_QueryLimit(t *testing.T) {
	bus := NewMemoryEventBus()
	ctx := context.Background()

	// Publish 10 events
	for i := 0; i < 10; i++ {
		event, _ := NewEvent("test.event", "tenant-1", "source-1", nil)
		_, err := bus.Publish(ctx, event)
		require.NoError(t, err)
	}

	// Query with limit
	query := &EventQuery{
		TenantID:  "tenant-1",
		EventType: "test.event",
		Limit:     5,
	}

	events, err := bus.Query(ctx, query)
	require.NoError(t, err)
	assert.Len(t, events, 5)
}

func TestMemoryEventBus_Subscribe(t *testing.T) {
	bus := NewMemoryEventBus()
	ctx := context.Background()

	var handledEvents []*Event
	handler := func(ctx context.Context, event *Event) error {
		handledEvents = append(handledEvents, event)
		return nil
	}

	// Subscribe to events
	err := bus.Subscribe(ctx, "test.event", handler)
	require.NoError(t, err)

	// Publish event
	event, _ := NewEvent("test.event", "tenant-1", "source-1", nil)
	_, err = bus.Publish(ctx, event)
	require.NoError(t, err)

	// Give handler time to process (async)
	time.Sleep(50 * time.Millisecond)

	// Verify handler was called
	assert.Len(t, handledEvents, 1)
	assert.Equal(t, event.ID, handledEvents[0].ID)
}

func TestMemoryEventBus_SubscribeInvalidInputs(t *testing.T) {
	bus := NewMemoryEventBus()
	ctx := context.Background()

	// Empty event type
	err := bus.Subscribe(ctx, "", func(ctx context.Context, event *Event) error {
		return nil
	})
	assert.Error(t, err)

	// Nil handler
	err = bus.Subscribe(ctx, "test.event", nil)
	assert.Error(t, err)
}

func TestMemoryEventBus_GetEvent(t *testing.T) {
	bus := NewMemoryEventBus()
	ctx := context.Background()

	// Publish event
	event, _ := NewEvent("test.event", "tenant-1", "source-1", nil)
	eventID, err := bus.Publish(ctx, event)
	require.NoError(t, err)

	// Get event
	retrieved, err := bus.GetEvent(ctx, eventID)
	require.NoError(t, err)
	assert.Equal(t, event.ID, retrieved.ID)
	assert.Equal(t, event.EventType, retrieved.EventType)
}

func TestMemoryEventBus_GetEventNotFound(t *testing.T) {
	bus := NewMemoryEventBus()
	ctx := context.Background()

	_, err := bus.GetEvent(ctx, "non-existent-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "event not found")
}

func TestMemoryEventBus_DeleteEvent(t *testing.T) {
	bus := NewMemoryEventBus()
	ctx := context.Background()

	// Publish event
	event, _ := NewEvent("test.event", "tenant-1", "source-1", nil)
	eventID, err := bus.Publish(ctx, event)
	require.NoError(t, err)

	// Verify event exists
	assert.Equal(t, 1, bus.Count())

	// Delete event
	err = bus.DeleteEvent(ctx, eventID)
	require.NoError(t, err)

	// Verify event was deleted
	assert.Equal(t, 0, bus.Count())

	// Try to get deleted event
	_, err = bus.GetEvent(ctx, eventID)
	assert.Error(t, err)
}

func TestMemoryEventBus_DeleteEventNotFound(t *testing.T) {
	bus := NewMemoryEventBus()
	ctx := context.Background()

	err := bus.DeleteEvent(ctx, "non-existent-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "event not found")
}

func TestMemoryEventBus_Clear(t *testing.T) {
	bus := NewMemoryEventBus()
	ctx := context.Background()

	// Publish multiple events
	for i := 0; i < 5; i++ {
		event, _ := NewEvent("test.event", "tenant-1", "source-1", nil)
		_, err := bus.Publish(ctx, event)
		require.NoError(t, err)
	}

	assert.Equal(t, 5, bus.Count())

	// Clear all events
	bus.Clear()

	assert.Equal(t, 0, bus.Count())
}

func TestMemoryEventBus_MultipleEventTypes(t *testing.T) {
	bus := NewMemoryEventBus()
	ctx := context.Background()

	// Publish different event types
	event1, _ := NewEvent("user.created", "tenant-1", "user-1", nil)
	_, err := bus.Publish(ctx, event1)
	require.NoError(t, err)

	event2, _ := NewEvent("user.updated", "tenant-1", "user-1", nil)
	_, err = bus.Publish(ctx, event2)
	require.NoError(t, err)

	event3, _ := NewEvent("user.deleted", "tenant-1", "user-1", nil)
	_, err = bus.Publish(ctx, event3)
	require.NoError(t, err)

	// Query specific event type
	query := &EventQuery{
		TenantID:  "tenant-1",
		EventType: "user.created",
	}

	events, err := bus.Query(ctx, query)
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, "user.created", events[0].EventType)
}

func TestEventUnmarshalPayload(t *testing.T) {
	payload := TestPayload{
		Name:  "Jane Doe",
		Email: "jane@example.com",
	}

	event, err := NewEvent("test.event", "tenant-1", "source-1", payload)
	require.NoError(t, err)

	var decoded TestPayload
	err = event.UnmarshalPayload(&decoded)
	require.NoError(t, err)

	assert.Equal(t, payload.Name, decoded.Name)
	assert.Equal(t, payload.Email, decoded.Email)
}

func TestEventUnmarshalPayloadInvalid(t *testing.T) {
	event, err := NewEvent("test.event", "tenant-1", "source-1", nil)
	require.NoError(t, err)

	// Set invalid JSON
	event.Payload = json.RawMessage(`{invalid json}`)

	var decoded TestPayload
	err = event.UnmarshalPayload(&decoded)
	assert.Error(t, err)
}

func TestDefaultEventBusConfig(t *testing.T) {
	config := DefaultEventBusConfig()

	assert.Empty(t, config.TableName)
	assert.Equal(t, 30*24*time.Hour, config.TTL)
	assert.True(t, config.EnableMetrics)
	assert.Equal(t, "Lift/EventBus", config.MetricsNamespace)
	assert.Equal(t, 3, config.RetryAttempts)
	assert.Equal(t, 100*time.Millisecond, config.RetryBaseDelay)
	assert.Equal(t, 25, config.MaxBatchSize)
}

func TestMemoryEventBus_ConcurrentPublish(t *testing.T) {
	bus := NewMemoryEventBus()
	ctx := context.Background()

	// Publish events concurrently
	const numEvents = 100
	errs := make(chan error, numEvents)

	for i := 0; i < numEvents; i++ {
		go func() {
			event, _ := NewEvent("test.event", "tenant-1", "source-1", nil)
			_, err := bus.Publish(ctx, event)
			errs <- err
		}()
	}

	// Collect errors
	for i := 0; i < numEvents; i++ {
		err := <-errs
		assert.NoError(t, err)
	}

	// Verify all events were published
	assert.Equal(t, numEvents, bus.Count())
}

func TestMemoryEventBus_ConcurrentQuery(t *testing.T) {
	bus := NewMemoryEventBus()
	ctx := context.Background()

	// Publish some events
	for i := 0; i < 10; i++ {
		event, _ := NewEvent("test.event", "tenant-1", "source-1", nil)
		_, err := bus.Publish(ctx, event)
		require.NoError(t, err)
	}

	// Query concurrently
	const numQueries = 50
	results := make(chan []*Event, numQueries)

	query := &EventQuery{
		TenantID:  "tenant-1",
		EventType: "test.event",
	}

	for i := 0; i < numQueries; i++ {
		go func() {
			events, _ := bus.Query(ctx, query)
			results <- events
		}()
	}

	// Collect results
	for i := 0; i < numQueries; i++ {
		events := <-results
		assert.Len(t, events, 10)
	}
}
