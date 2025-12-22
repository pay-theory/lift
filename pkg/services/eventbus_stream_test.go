package services

import (
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/require"
)

func TestEventFromStreamRecord_Insert(t *testing.T) {
	record := events.DynamoDBEventRecord{
		EventName: string(events.DynamoDBOperationTypeInsert),
		Change: events.DynamoDBStreamRecord{
			NewImage: map[string]events.DynamoDBAttributeValue{
				"id":         events.NewStringAttribute("evt_123"),
				"event_type": events.NewStringAttribute("partner.created"),
			},
		},
	}

	event, err := EventFromStreamRecord(record)
	require.NoError(t, err)
	require.Equal(t, "evt_123", event.ID)
	require.Equal(t, "partner.created", event.EventType)
}

func TestEventFromStreamRecord_Remove_UsesOldImage(t *testing.T) {
	record := events.DynamoDBEventRecord{
		EventName: string(events.DynamoDBOperationTypeRemove),
		Change: events.DynamoDBStreamRecord{
			OldImage: map[string]events.DynamoDBAttributeValue{
				"id":         events.NewStringAttribute("evt_456"),
				"event_type": events.NewStringAttribute("partner.deleted"),
			},
		},
	}

	event, err := EventFromStreamRecord(record)
	require.NoError(t, err)
	require.Equal(t, "evt_456", event.ID)
	require.Equal(t, "partner.deleted", event.EventType)
}

func TestEventFromStreamRecord_Modify_FallsBackToOldImage(t *testing.T) {
	record := events.DynamoDBEventRecord{
		EventName: string(events.DynamoDBOperationTypeModify),
		Change: events.DynamoDBStreamRecord{
			OldImage: map[string]events.DynamoDBAttributeValue{
				"id":         events.NewStringAttribute("evt_789"),
				"event_type": events.NewStringAttribute("partner.updated"),
			},
		},
	}

	event, err := EventFromStreamRecord(record)
	require.NoError(t, err)
	require.Equal(t, "evt_789", event.ID)
	require.Equal(t, "partner.updated", event.EventType)
}

func TestEventFromStreamRecord_ErrorsOnMissingImages(t *testing.T) {
	record := events.DynamoDBEventRecord{
		EventName: string(events.DynamoDBOperationTypeInsert),
		Change:    events.DynamoDBStreamRecord{},
	}

	_, err := EventFromStreamRecord(record)
	require.Error(t, err)
}
