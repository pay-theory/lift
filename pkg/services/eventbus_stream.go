package services

import (
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/pay-theory/dynamorm"
)

// EventFromStreamRecord decodes a DynamoDB stream record from the EventBus table into an Event.
//
// It supports INSERT/MODIFY/REMOVE by selecting the appropriate stream image:
//   - REMOVE: OldImage (required)
//   - INSERT/MODIFY: NewImage (preferred) falling back to OldImage
func EventFromStreamRecord(record events.DynamoDBEventRecord) (*Event, error) {
	image, err := eventStreamImage(record)
	if err != nil {
		return nil, err
	}

	event, err := EventFromStreamImage(image)
	if err != nil {
		return nil, fmt.Errorf("failed to decode EventBus stream record: %w", err)
	}

	return event, nil
}

// EventFromStreamImage decodes a DynamoDB stream image into an Event.
func EventFromStreamImage(image map[string]events.DynamoDBAttributeValue) (*Event, error) {
	if len(image) == 0 {
		return nil, fmt.Errorf("stream image is empty")
	}

	var event Event
	if err := dynamorm.UnmarshalStreamImage(image, &event); err != nil {
		return nil, fmt.Errorf("failed to unmarshal stream image: %w", err)
	}

	if event.ID == "" {
		return nil, fmt.Errorf("missing event id")
	}
	if event.EventType == "" {
		return nil, fmt.Errorf("missing event type")
	}

	return &event, nil
}

func eventStreamImage(record events.DynamoDBEventRecord) (map[string]events.DynamoDBAttributeValue, error) {
	switch record.EventName {
	case string(events.DynamoDBOperationTypeRemove):
		if len(record.Change.OldImage) == 0 {
			return nil, fmt.Errorf("REMOVE record missing OldImage")
		}
		return record.Change.OldImage, nil
	default:
		if len(record.Change.NewImage) > 0 {
			return record.Change.NewImage, nil
		}
		if len(record.Change.OldImage) > 0 {
			return record.Change.OldImage, nil
		}
		return nil, fmt.Errorf("record missing NewImage and OldImage")
	}
}
