package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

// EventBus defines the interface for publishing and consuming events
// This interface is designed to work in serverless environments with DynamoDB backing
type EventBus interface {
	// Publish publishes an event to the bus and returns the event ID
	Publish(ctx context.Context, event *Event) (string, error)

	// Query retrieves events based on filters
	Query(ctx context.Context, query *EventQuery) ([]*Event, error)

	// Subscribe registers a handler for specific event types (for stream processing)
	Subscribe(ctx context.Context, eventType string, handler EventHandler) error

	// GetEvent retrieves a specific event by ID
	GetEvent(ctx context.Context, eventID string) (*Event, error)

	// DeleteEvent removes an event (for cleanup/GDPR)
	DeleteEvent(ctx context.Context, eventID string) error
}

// Event represents a single event in the system
type Event struct {
	// Primary identifiers and timestamps (8-byte aligned)
	PublishedAt time.Time `json:"published_at" dynamodbav:"published_at"`
	CreatedAt   time.Time `json:"created_at" dynamodbav:"created_at"`
	ExpiresAt   time.Time `json:"expires_at,omitempty" dynamodbav:"expires_at,omitempty"`

	// String fields (16 bytes each)
	ID            string `json:"id" dynamodbav:"id"`                 // ULID for ordering
	EventType     string `json:"event_type" dynamodbav:"event_type"` // e.g., "partner.created"
	TenantID      string `json:"tenant_id" dynamodbav:"tenant_id"`   // For multi-tenancy
	SourceID      string `json:"source_id" dynamodbav:"source_id"`   // Source entity ID
	PartitionKey  string `json:"partition_key" dynamodbav:"pk"`      // DynamoDB partition key
	SortKey       string `json:"sort_key" dynamodbav:"sk"`           // DynamoDB sort key
	CorrelationID string `json:"correlation_id,omitempty" dynamodbav:"correlation_id,omitempty"`

	// Complex types
	Payload  json.RawMessage   `json:"payload" dynamodbav:"payload"`                       // Event data
	Metadata map[string]string `json:"metadata,omitempty" dynamodbav:"metadata,omitempty"` // Additional context
	Tags     []string          `json:"tags,omitempty" dynamodbav:"tags,omitempty"`         // For filtering

	// Smaller numeric types
	Version    int `json:"version" dynamodbav:"version"`         // Schema version
	RetryCount int `json:"retry_count" dynamodbav:"retry_count"` // For failed processing
}

// EventQuery defines parameters for querying events
type EventQuery struct {
	LastEvaluatedKey map[string]interface{}
	NextKey          map[string]interface{} // Returned pagination token for next query
	StartTime        *time.Time
	EndTime          *time.Time
	TenantID         string
	EventType        string
	Tags             []string
	Limit            int
}

// EventHandler is a function that processes events
type EventHandler func(ctx context.Context, event *Event) error

// EventBusConfig configures the event bus behavior
type EventBusConfig struct {
	TableName        string
	MetricsNamespace string
	TTL              time.Duration
	RetryBaseDelay   time.Duration
	RetryAttempts    int
	MaxBatchSize     int
	EnableMetrics    bool
}

// DefaultEventBusConfig returns sensible defaults
func DefaultEventBusConfig() EventBusConfig {
	return EventBusConfig{
		TableName:        "lift-events",
		TTL:              30 * 24 * time.Hour, // 30 days
		EnableMetrics:    true,
		MetricsNamespace: "Lift/EventBus",
		RetryAttempts:    3,
		RetryBaseDelay:   100 * time.Millisecond,
		MaxBatchSize:     25, // DynamoDB limit
	}
}

// NewEvent creates a new event with generated ID and timestamps
func NewEvent(eventType, tenantID, sourceID string, payload interface{}) (*Event, error) {
	// Generate ULID for time-ordered IDs
	id := ulid.Make().String()

	// Marshal payload
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	now := time.Now()

	// Construct partition and sort keys
	// Pattern: tenant_id#event_type for partition, timestamp#id for sort
	partitionKey := fmt.Sprintf("%s#%s", tenantID, eventType)
	sortKey := fmt.Sprintf("%d#%s", now.UnixNano(), id)

	return &Event{
		ID:           id,
		EventType:    eventType,
		TenantID:     tenantID,
		SourceID:     sourceID,
		Payload:      payloadBytes,
		PublishedAt:  now,
		CreatedAt:    now,
		PartitionKey: partitionKey,
		SortKey:      sortKey,
		Version:      1,
		Metadata:     make(map[string]string),
		Tags:         make([]string, 0),
	}, nil
}

// WithTTL sets an expiration time for the event
func (e *Event) WithTTL(ttl time.Duration) *Event {
	e.ExpiresAt = e.CreatedAt.Add(ttl)
	return e
}

// WithMetadata adds metadata to the event
func (e *Event) WithMetadata(key, value string) *Event {
	if e.Metadata == nil {
		e.Metadata = make(map[string]string)
	}
	e.Metadata[key] = value
	return e
}

// WithTags adds tags to the event
func (e *Event) WithTags(tags ...string) *Event {
	e.Tags = append(e.Tags, tags...)
	return e
}

// WithCorrelationID sets a correlation ID for tracing related events
func (e *Event) WithCorrelationID(correlationID string) *Event {
	e.CorrelationID = correlationID
	return e
}

// UnmarshalPayload unmarshals the event payload into the provided struct
func (e *Event) UnmarshalPayload(v interface{}) error {
	return json.Unmarshal(e.Payload, v)
}

// MemoryEventBus provides an in-memory implementation for testing and development
// WARNING: This is NOT suitable for production Lambda environments as events
// are lost when the Lambda container scales down or is recycled
type MemoryEventBus struct {
	events   map[string]*Event         // events by ID
	byType   map[string][]*Event       // events indexed by type
	handlers map[string][]EventHandler // registered handlers
	mu       sync.RWMutex
}

// NewMemoryEventBus creates a new in-memory event bus
// This should only be used for testing or local development
func NewMemoryEventBus() *MemoryEventBus {
	return &MemoryEventBus{
		events:   make(map[string]*Event),
		byType:   make(map[string][]*Event),
		handlers: make(map[string][]EventHandler),
	}
}

// Publish stores an event in memory and triggers handlers
func (m *MemoryEventBus) Publish(ctx context.Context, event *Event) (string, error) {
	if event == nil {
		return "", fmt.Errorf("event cannot be nil")
	}

	// Generate ID if not set
	if event.ID == "" {
		event.ID = ulid.Make().String()
	}

	// Set timestamps if not set
	if event.PublishedAt.IsZero() {
		event.PublishedAt = time.Now()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Store event
	m.events[event.ID] = event

	// Index by type
	m.byType[event.EventType] = append(m.byType[event.EventType], event)

	// Trigger handlers asynchronously (don't block publishing)
	if handlers, ok := m.handlers[event.EventType]; ok {
		for _, handler := range handlers {
			go func(h EventHandler) {
				// Intentionally ignore errors in async handlers - they don't affect publishing
				// nolint:errcheck
				_ = h(ctx, event)
			}(handler)
		}
	}

	return event.ID, nil
}

// Query retrieves events based on filters
func (m *MemoryEventBus) Query(_ context.Context, query *EventQuery) ([]*Event, error) {
	if query == nil {
		return nil, fmt.Errorf("query cannot be nil")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Pre-allocate results slice with estimated capacity
	results := make([]*Event, 0, 100)

	// Get events by type or all events
	var candidates []*Event
	if query.EventType != "" {
		candidates = m.byType[query.EventType]
	} else {
		for _, event := range m.events {
			candidates = append(candidates, event)
		}
	}

	// Apply filters
	for _, event := range candidates {
		if !m.matchesQuery(event, query) {
			continue
		}
		results = append(results, event)
	}

	// Apply limit
	limit := query.Limit
	if limit == 0 {
		limit = 100
	}
	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// matchesQuery checks if an event matches the query filters
func (m *MemoryEventBus) matchesQuery(event *Event, query *EventQuery) bool {
	// Filter by tenant
	if query.TenantID != "" && event.TenantID != query.TenantID {
		return false
	}

	// Filter by time range
	if query.StartTime != nil && event.PublishedAt.Before(*query.StartTime) {
		return false
	}
	if query.EndTime != nil && event.PublishedAt.After(*query.EndTime) {
		return false
	}

	// Filter by tags
	if len(query.Tags) > 0 {
		tagMap := make(map[string]bool)
		for _, tag := range event.Tags {
			tagMap[tag] = true
		}
		for _, queryTag := range query.Tags {
			if !tagMap[queryTag] {
				return false
			}
		}
	}

	return true
}

// Subscribe registers a handler for specific event types
func (m *MemoryEventBus) Subscribe(_ context.Context, eventType string, handler EventHandler) error {
	if eventType == "" {
		return fmt.Errorf("event type cannot be empty")
	}
	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.handlers[eventType] = append(m.handlers[eventType], handler)
	return nil
}

// GetEvent retrieves a specific event by ID
func (m *MemoryEventBus) GetEvent(_ context.Context, eventID string) (*Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	event, exists := m.events[eventID]
	if !exists {
		return nil, fmt.Errorf("event not found: %s", eventID)
	}

	return event, nil
}

// DeleteEvent removes an event from the bus
func (m *MemoryEventBus) DeleteEvent(_ context.Context, eventID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	event, exists := m.events[eventID]
	if !exists {
		return fmt.Errorf("event not found: %s", eventID)
	}

	// Remove from main index
	delete(m.events, eventID)

	// Remove from type index
	if typeEvents, ok := m.byType[event.EventType]; ok {
		for i, e := range typeEvents {
			if e.ID == eventID {
				m.byType[event.EventType] = append(typeEvents[:i], typeEvents[i+1:]...)
				break
			}
		}
	}

	return nil
}

// Clear removes all events (useful for testing)
func (m *MemoryEventBus) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.events = make(map[string]*Event)
	m.byType = make(map[string][]*Event)
}

// Count returns the total number of events
func (m *MemoryEventBus) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.events)
}
