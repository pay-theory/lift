package adapters

import (
	"fmt"
)

// EventBridgeAdapter handles EventBridge events
type EventBridgeAdapter struct {
	BaseAdapter
}

// NewEventBridgeAdapter creates a new EventBridge adapter
func NewEventBridgeAdapter() *EventBridgeAdapter {
	return &EventBridgeAdapter{
		BaseAdapter: BaseAdapter{triggerType: TriggerEventBridge},
	}
}

// CanHandle checks if this adapter can handle the given event
func (a *EventBridgeAdapter) CanHandle(event interface{}) bool {
	eventMap, ok := event.(map[string]interface{})
	if !ok {
		return false
	}

	// Check for EventBridge specific fields
	_, hasSource := eventMap["source"]
	_, hasDetailType := eventMap["detail-type"]
	_, hasDetail := eventMap["detail"]
	_, hasTime := eventMap["time"]

	// EventBridge events have source, detail-type, detail, and time fields
	return hasSource && hasDetailType && hasDetail && hasTime
}

// Validate checks if the event has the required EventBridge structure
func (a *EventBridgeAdapter) Validate(event interface{}) error {
	eventMap, ok := event.(map[string]interface{})
	if !ok {
		return fmt.Errorf("event must be a map[string]interface{}")
	}

	// Check required fields
	requiredFields := []string{"source", "detail-type", "detail", "time"}
	for _, field := range requiredFields {
		if _, exists := eventMap[field]; !exists {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	return nil
}

// Adapt converts an EventBridge event to a normalized Request
func (a *EventBridgeAdapter) Adapt(rawEvent interface{}) (*Request, error) {
	if err := a.Validate(rawEvent); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	eventMap := rawEvent.(map[string]interface{})

	// Extract EventBridge specific fields
	source := extractStringField(eventMap, "source")
	detailType := extractStringField(eventMap, "detail-type")
	detail := extractMapField(eventMap, "detail")
	timestamp := extractStringField(eventMap, "time")
	eventID := extractStringField(eventMap, "id")

	return &Request{
		TriggerType: TriggerEventBridge,
		RawEvent:    rawEvent,
		EventID:     eventID,
		Timestamp:   timestamp,
		Source:      source,
		DetailType:  detailType,
		Detail:      detail,
	}, nil
}
