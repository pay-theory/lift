package lift

import (
	"context"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/require"
)

func TestContext_EventBusRecords_AndDecodeHelpers(t *testing.T) {
	_, err := (*Context)(nil).EventBusRecords()
	require.Error(t, err)

	_, err = (*Context)(nil).DynamoDBRecords()
	require.Error(t, err)

	ctx := NewContext(context.Background(), NewRequest(nil))
	_, err = ctx.EventBusRecords()
	require.Error(t, err)

	ctx.Request.TriggerType = TriggerAPIGateway
	_, err = ctx.EventBusRecords()
	require.Error(t, err)

	record := events.DynamoDBEventRecord{EventID: "rec-1"}
	recordPtr := &events.DynamoDBEventRecord{EventID: "rec-2"}
	ctx.Request.TriggerType = TriggerEventBus
	ctx.Request.Records = []any{
		record,
		recordPtr,
		map[string]any{"eventID": "rec-3"},
	}

	records, err := ctx.EventBusRecords()
	require.NoError(t, err)
	require.Len(t, records, 3)
	require.Equal(t, "rec-1", records[0].EventID)
	require.Equal(t, "rec-2", records[1].EventID)
	require.Equal(t, "rec-3", records[2].EventID)

	// DynamoDBRecords uses the same underlying trigger check as EventBusRecords.
	ddbRecords, err := ctx.DynamoDBRecords()
	require.NoError(t, err)
	require.Len(t, ddbRecords, 3)

	_, err = decodeDynamoDBStreamRecords([]any{"bad"})
	require.Error(t, err)

	_, err = decodeDynamoDBStreamRecords([]any{map[string]any{"bad": make(chan int)}})
	require.Error(t, err)

	_, err = decodeDynamoDBStreamRecords([]any{map[string]any{"eventID": map[string]any{"nested": true}}})
	require.Error(t, err)
}

func TestContext_DynamoDBRecords_TriggerTypeCheck(t *testing.T) {
	ctx := NewContext(context.Background(), NewRequest(&adapters.Request{TriggerType: TriggerSQS}))
	_, err := ctx.DynamoDBRecords()
	require.Error(t, err)
}
