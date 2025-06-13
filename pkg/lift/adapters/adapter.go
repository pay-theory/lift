package adapters

import (
	"fmt"
	"reflect"
)

// TriggerType represents the type of Lambda trigger
type TriggerType string

const (
	TriggerAPIGateway   TriggerType = "api_gateway"
	TriggerAPIGatewayV2 TriggerType = "api_gateway_v2"
	TriggerSQS          TriggerType = "sqs"
	TriggerS3           TriggerType = "s3"
	TriggerEventBridge  TriggerType = "eventbridge"
	TriggerScheduled    TriggerType = "scheduled"
	TriggerWebSocket    TriggerType = "websocket"
	TriggerUnknown      TriggerType = "unknown"
)

// Request represents a normalized request from any event source
type Request struct {
	// Event metadata
	TriggerType TriggerType `json:"trigger_type"`
	RawEvent    interface{} `json:"raw_event,omitempty"`
	EventID     string      `json:"event_id,omitempty"`
	Timestamp   string      `json:"timestamp,omitempty"`

	// HTTP-like fields (for API Gateway compatibility)
	Method      string            `json:"method,omitempty"`
	Path        string            `json:"path,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	QueryParams map[string]string `json:"query_params,omitempty"`
	PathParams  map[string]string `json:"path_params,omitempty"`
	Body        []byte            `json:"body,omitempty"`

	// Event-specific data
	Records    []interface{}          `json:"records,omitempty"`
	Detail     map[string]interface{} `json:"detail,omitempty"`
	Source     string                 `json:"source,omitempty"`
	DetailType string                 `json:"detail_type,omitempty"`

	// Additional metadata for specific event types (e.g., WebSocket)
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// EventAdapter defines the interface for converting Lambda events to normalized requests
type EventAdapter interface {
	// Adapt converts a raw Lambda event to a normalized Request
	Adapt(rawEvent interface{}) (*Request, error)

	// GetTriggerType returns the trigger type this adapter handles
	GetTriggerType() TriggerType

	// Validate checks if the raw event matches this adapter's expected format
	Validate(event interface{}) error

	// CanHandle returns true if this adapter can handle the given event
	CanHandle(event interface{}) bool
}

// AdapterRegistry manages event adapters and provides automatic event type detection
type AdapterRegistry struct {
	adapters map[TriggerType]EventAdapter
}

// NewAdapterRegistry creates a new adapter registry with default adapters
func NewAdapterRegistry() *AdapterRegistry {
	registry := &AdapterRegistry{
		adapters: make(map[TriggerType]EventAdapter),
	}

	// Register default adapters
	registry.Register(NewAPIGatewayAdapter())
	registry.Register(NewAPIGatewayV2Adapter())
	registry.Register(NewSQSAdapter())
	registry.Register(NewS3Adapter())
	registry.Register(NewEventBridgeAdapter())
	registry.Register(NewScheduledAdapter())
	registry.Register(NewWebSocketAdapter())

	return registry
}

// Register adds an adapter to the registry
func (r *AdapterRegistry) Register(adapter EventAdapter) {
	r.adapters[adapter.GetTriggerType()] = adapter
}

// GetAdapter returns the adapter for a specific trigger type
func (r *AdapterRegistry) GetAdapter(triggerType TriggerType) (EventAdapter, bool) {
	adapter, exists := r.adapters[triggerType]
	return adapter, exists
}

// DetectAndAdapt automatically detects the event type and adapts it
func (r *AdapterRegistry) DetectAndAdapt(rawEvent interface{}) (*Request, error) {
	// Try each adapter to see which one can handle the event
	for _, adapter := range r.adapters {
		if adapter.CanHandle(rawEvent) {
			return adapter.Adapt(rawEvent)
		}
	}

	// If no adapter can handle it, return an error with event details
	eventType := reflect.TypeOf(rawEvent)
	return nil, fmt.Errorf("no adapter found for event type: %v", eventType)
}

// AdaptWithType adapts an event using a specific adapter type
func (r *AdapterRegistry) AdaptWithType(rawEvent interface{}, triggerType TriggerType) (*Request, error) {
	adapter, exists := r.adapters[triggerType]
	if !exists {
		return nil, fmt.Errorf("no adapter registered for trigger type: %s", triggerType)
	}

	return adapter.Adapt(rawEvent)
}

// ListSupportedTriggers returns all supported trigger types
func (r *AdapterRegistry) ListSupportedTriggers() []TriggerType {
	triggers := make([]TriggerType, 0, len(r.adapters))
	for triggerType := range r.adapters {
		triggers = append(triggers, triggerType)
	}
	return triggers
}

// BaseAdapter provides common functionality for all adapters
type BaseAdapter struct {
	triggerType TriggerType
}

// GetTriggerType returns the trigger type for this adapter
func (b *BaseAdapter) GetTriggerType() TriggerType {
	return b.triggerType
}

// extractStringField safely extracts a string field from a map
func extractStringField(data map[string]interface{}, key string) string {
	if value, exists := data[key]; exists {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}

// extractMapField safely extracts a map field from a map
func extractMapField(data map[string]interface{}, key string) map[string]interface{} {
	if value, exists := data[key]; exists {
		if mapValue, ok := value.(map[string]interface{}); ok {
			return mapValue
		}
	}
	return make(map[string]interface{})
}

// extractStringMapField safely extracts a string map field from a map
func extractStringMapField(data map[string]interface{}, key string) map[string]string {
	result := make(map[string]string)
	if value, exists := data[key]; exists {
		if mapValue, ok := value.(map[string]interface{}); ok {
			for k, v := range mapValue {
				if str, ok := v.(string); ok {
					result[k] = str
				}
			}
		}
	}
	return result
}

// extractSliceField safely extracts a slice field from a map
func extractSliceField(data map[string]interface{}, key string) []interface{} {
	if value, exists := data[key]; exists {
		if slice, ok := value.([]interface{}); ok {
			return slice
		}
	}
	return make([]interface{}, 0)
}
