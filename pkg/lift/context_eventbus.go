package lift

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
)

// EventBusRecords returns the typed DynamoDB stream records for an EventBus-triggered invocation.
func (c *Context) EventBusRecords() ([]events.DynamoDBEventRecord, error) {
	if c == nil || c.Request == nil {
		return nil, fmt.Errorf("context request is required")
	}
	if c.Request.TriggerType != TriggerEventBus {
		return nil, fmt.Errorf("not an EventBus event")
	}

	records := make([]events.DynamoDBEventRecord, 0, len(c.Request.Records))
	for _, record := range c.Request.Records {
		switch v := record.(type) {
		case events.DynamoDBEventRecord:
			records = append(records, v)
		case *events.DynamoDBEventRecord:
			if v != nil {
				records = append(records, *v)
			}
		case map[string]any:
			encoded, err := json.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("failed to encode stream record: %w", err)
			}
			var parsed events.DynamoDBEventRecord
			if err := json.Unmarshal(encoded, &parsed); err != nil {
				return nil, fmt.Errorf("failed to decode stream record: %w", err)
			}
			records = append(records, parsed)
		default:
			return nil, fmt.Errorf("unexpected stream record type %T", record)
		}
	}

	return records, nil
}
