package adapters

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
)

// EventBusAdapter adapts DynamoDB Stream events into Lift's normalized Request structure.
//
// This adapter is intended for Lift's EventBus (services.DynamoDBEventBus) stream processors.
// It decodes the Lambda event into strongly-typed events.DynamoDBEventRecord values so runtime
// helpers can safely decode New/Old images using DynamORM.
type EventBusAdapter struct {
	BaseAdapter
}

// NewEventBusAdapter creates a new EventBus adapter.
func NewEventBusAdapter() *EventBusAdapter {
	return &EventBusAdapter{
		BaseAdapter: BaseAdapter{triggerType: TriggerEventBus},
	}
}

// CanHandle reports whether the adapter recognizes the raw event as a DynamoDB Stream event.
func (a *EventBusAdapter) CanHandle(event any) bool {
	eventMap, ok := event.(map[string]any)
	if !ok {
		return false
	}

	records, ok := eventMap["Records"].([]any)
	if !ok || len(records) == 0 {
		return false
	}

	first, ok := records[0].(map[string]any)
	if !ok {
		return false
	}

	// DynamoDB stream records have eventSource "aws:dynamodb".
	return extractStringField(first, "eventSource") == "aws:dynamodb"
}

// Validate checks that the raw event has the required DynamoDB stream record structure.
func (a *EventBusAdapter) Validate(event any) error {
	requiredFields := []string{"eventSource", "eventName", "dynamodb"}
	return validateRecordsEvent(event, "aws:dynamodb", requiredFields)
}

// Adapt converts a DynamoDB stream event into a normalized Request.
func (a *EventBusAdapter) Adapt(rawEvent any) (*Request, error) {
	if err := a.Validate(rawEvent); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	rawBytes, err := json.Marshal(rawEvent)
	if err != nil {
		return nil, fmt.Errorf("failed to encode DynamoDB stream event: %w", err)
	}

	var stream events.DynamoDBEvent
	if err := json.Unmarshal(rawBytes, &stream); err != nil {
		return nil, fmt.Errorf("failed to decode DynamoDB stream event: %w", err)
	}

	records := make([]any, 0, len(stream.Records))
	for _, record := range stream.Records {
		records = append(records, record)
	}

	var eventID string
	if len(stream.Records) > 0 {
		eventID = stream.Records[0].EventID
	}

	body, err := json.Marshal(stream.Records)
	if err != nil {
		return nil, fmt.Errorf("failed to encode DynamoDB stream records: %w", err)
	}

	return &Request{
		TriggerType: TriggerEventBus,
		RawEvent:    rawEvent,
		EventID:     eventID,
		Records:     records,
		Body:        body,
		Source:      "aws:dynamodb",
	}, nil
}
