package adapters

import (
	"fmt"
)

// S3Adapter handles S3 events
type S3Adapter struct {
	BaseAdapter
}

// NewS3Adapter creates a new S3 adapter
func NewS3Adapter() *S3Adapter {
	return &S3Adapter{
		BaseAdapter: BaseAdapter{triggerType: TriggerS3},
	}
}

// CanHandle checks if this adapter can handle the given event
func (a *S3Adapter) CanHandle(event interface{}) bool {
	eventMap, ok := event.(map[string]interface{})
	if !ok {
		return false
	}

	// Check for S3 specific fields
	records, hasRecords := eventMap["Records"]
	if !hasRecords {
		return false
	}

	// Check if records is a slice
	recordsSlice, ok := records.([]interface{})
	if !ok || len(recordsSlice) == 0 {
		return false
	}

	// Check first record for S3 specific fields
	firstRecord, ok := recordsSlice[0].(map[string]interface{})
	if !ok {
		return false
	}

	// S3 records have eventSource "aws:s3"
	eventSource := extractStringField(firstRecord, "eventSource")
	return eventSource == "aws:s3"
}

// Validate checks if the event has the required S3 structure
func (a *S3Adapter) Validate(event interface{}) error {
	eventMap, ok := event.(map[string]interface{})
	if !ok {
		return fmt.Errorf("event must be a map[string]interface{}")
	}

	// Check for Records field
	records, exists := eventMap["Records"]
	if !exists {
		return fmt.Errorf("missing required field: Records")
	}

	// Validate records structure
	recordsSlice, ok := records.([]interface{})
	if !ok {
		return fmt.Errorf("Records must be a slice")
	}

	if len(recordsSlice) == 0 {
		return fmt.Errorf("Records slice cannot be empty")
	}

	// Validate first record
	firstRecord, ok := recordsSlice[0].(map[string]interface{})
	if !ok {
		return fmt.Errorf("Records must contain map objects")
	}

	// Check required fields in record
	requiredFields := []string{"eventSource", "eventName", "s3"}
	for _, field := range requiredFields {
		if _, exists := firstRecord[field]; !exists {
			return fmt.Errorf("missing required field in record: %s", field)
		}
	}

	return nil
}

// Adapt converts an S3 event to a normalized Request
func (a *S3Adapter) Adapt(rawEvent interface{}) (*Request, error) {
	if err := a.Validate(rawEvent); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	eventMap := rawEvent.(map[string]interface{})
	records := extractSliceField(eventMap, "Records")

	// Extract metadata from first record for event-level info
	var eventID, timestamp, eventName string
	if len(records) > 0 {
		if firstRecord, ok := records[0].(map[string]interface{}); ok {
			eventID = extractStringField(firstRecord, "responseElements.x-amz-request-id")
			timestamp = extractStringField(firstRecord, "eventTime")
			eventName = extractStringField(firstRecord, "eventName")
		}
	}

	return &Request{
		TriggerType: TriggerS3,
		RawEvent:    rawEvent,
		EventID:     eventID,
		Timestamp:   timestamp,
		Records:     records,
		Source:      "aws:s3",
		DetailType:  eventName,
	}, nil
}
