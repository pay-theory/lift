package adapters

import (
	"fmt"
)

// SQSAdapter handles SQS events
type SQSAdapter struct {
	BaseAdapter
}

// NewSQSAdapter creates a new SQS adapter
func NewSQSAdapter() *SQSAdapter {
	return &SQSAdapter{
		BaseAdapter: BaseAdapter{triggerType: TriggerSQS},
	}
}

// CanHandle checks if this adapter can handle the given event
func (a *SQSAdapter) CanHandle(event interface{}) bool {
	eventMap, ok := event.(map[string]interface{})
	if !ok {
		return false
	}

	// Check for SQS specific fields
	records, hasRecords := eventMap["Records"]
	if !hasRecords {
		return false
	}

	// Check if records is a slice
	recordsSlice, ok := records.([]interface{})
	if !ok || len(recordsSlice) == 0 {
		return false
	}

	// Check first record for SQS specific fields
	firstRecord, ok := recordsSlice[0].(map[string]interface{})
	if !ok {
		return false
	}

	// SQS records have eventSource "aws:sqs"
	eventSource := extractStringField(firstRecord, "eventSource")
	return eventSource == "aws:sqs"
}

// Validate checks if the event has the required SQS structure
func (a *SQSAdapter) Validate(event interface{}) error {
	eventMap, ok := event.(map[string]interface{})
	if !ok {
		return fmt.Errorf("event must be a map[string]interface{}")
	}

	// Check for Records field
	records, exists := eventMap["Records"]
	if !exists {
		return fmt.Errorf("missing required field: records")
	}

	// Validate records structure
	recordsSlice, ok := records.([]interface{})
	if !ok {
		return fmt.Errorf("records must be a slice")
	}

	if len(recordsSlice) == 0 {
		return fmt.Errorf("records slice cannot be empty")
	}

	// Validate first record
	firstRecord, ok := recordsSlice[0].(map[string]interface{})
	if !ok {
		return fmt.Errorf("records must contain map objects")
	}

	// Check required fields in record
	requiredFields := []string{"eventSource", "body", "receiptHandle"}
	for _, field := range requiredFields {
		if _, exists := firstRecord[field]; !exists {
			return fmt.Errorf("missing required field in record: %s", field)
		}
	}

	return nil
}

// Adapt converts an SQS event to a normalized Request
func (a *SQSAdapter) Adapt(rawEvent interface{}) (*Request, error) {
	if err := a.Validate(rawEvent); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	eventMap := rawEvent.(map[string]interface{})
	records := extractSliceField(eventMap, "Records")

	// Extract metadata from first record for event-level info
	var eventID, timestamp string
	if len(records) > 0 {
		if firstRecord, ok := records[0].(map[string]interface{}); ok {
			eventID = extractStringField(firstRecord, "messageId")
			timestamp = extractStringField(firstRecord, "attributes.SentTimestamp")
		}
	}

	return &Request{
		TriggerType: TriggerSQS,
		RawEvent:    rawEvent,
		EventID:     eventID,
		Timestamp:   timestamp,
		Records:     records,
		Source:      "aws:sqs",
	}, nil
}
