package adapters

import (
	"fmt"
)

// ScheduledAdapter handles CloudWatch Events/EventBridge scheduled events
type ScheduledAdapter struct {
	BaseAdapter
}

// NewScheduledAdapter creates a new Scheduled adapter
func NewScheduledAdapter() *ScheduledAdapter {
	return &ScheduledAdapter{
		BaseAdapter: BaseAdapter{triggerType: TriggerScheduled},
	}
}

// CanHandle checks if this adapter can handle the given event
func (a *ScheduledAdapter) CanHandle(event interface{}) bool {
	eventMap, ok := event.(map[string]interface{})
	if !ok {
		return false
	}

	// Check for scheduled event specific fields
	source := extractStringField(eventMap, "source")
	detailType := extractStringField(eventMap, "detail-type")

	// Scheduled events have source "aws.events" and detail-type "Scheduled Event"
	return source == "aws.events" && detailType == "Scheduled Event"
}

// Validate checks if the event has the required scheduled event structure
func (a *ScheduledAdapter) Validate(event interface{}) error {
	eventMap, ok := event.(map[string]interface{})
	if !ok {
		return fmt.Errorf("event must be a map[string]interface{}")
	}

	// Check required fields
	requiredFields := []string{"source", "detail-type", "time"}
	for _, field := range requiredFields {
		if _, exists := eventMap[field]; !exists {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	// Validate source and detail-type
	source := extractStringField(eventMap, "source")
	if source != "aws.events" {
		return fmt.Errorf("invalid source for scheduled event: %s", source)
	}

	detailType := extractStringField(eventMap, "detail-type")
	if detailType != "Scheduled Event" {
		return fmt.Errorf("invalid detail-type for scheduled event: %s", detailType)
	}

	return nil
}

// Adapt converts a scheduled event to a normalized Request
func (a *ScheduledAdapter) Adapt(rawEvent interface{}) (*Request, error) {
	if err := a.Validate(rawEvent); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	eventMap := rawEvent.(map[string]interface{})

	// Extract scheduled event specific fields
	source := extractStringField(eventMap, "source")
	detailType := extractStringField(eventMap, "detail-type")
	timestamp := extractStringField(eventMap, "time")
	eventID := extractStringField(eventMap, "id")

	// Extract resources (contains the rule ARN)
	resources := extractSliceField(eventMap, "resources")

	return &Request{
		TriggerType: TriggerScheduled,
		RawEvent:    rawEvent,
		EventID:     eventID,
		Timestamp:   timestamp,
		Source:      source,
		DetailType:  detailType,
		Records:     resources,
	}, nil
}
