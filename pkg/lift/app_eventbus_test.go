package lift_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/services"
	"github.com/stretchr/testify/require"
)

func TestApp_EventBus_RoutesAndDecodesEvents(t *testing.T) {
	app := lift.New()

	var gotType string
	var gotID string
	var gotRecords int

	require.NoError(t, app.EventBus("partner.created", func(ctx *lift.Context, e *services.Event) error {
		gotType = e.EventType
		gotID = e.ID
		records, err := ctx.EventBusRecords()
		require.NoError(t, err)
		gotRecords = len(records)
		return nil
	}))

	resp, err := app.HandleRequest(context.Background(), map[string]any{
		"Records": []any{
			map[string]any{
				"eventSource": "aws:dynamodb",
				"eventName":   "INSERT",
				"eventID":     "rec-1",
				"dynamodb": map[string]any{
					"NewImage": map[string]any{
						"id":         map[string]any{"S": "evt_123"},
						"event_type": map[string]any{"S": "partner.created"},
					},
				},
			},
		},
	})
	require.NoError(t, err)

	ddbResp, ok := resp.(events.DynamoDBEventResponse)
	require.True(t, ok, "expected events.DynamoDBEventResponse, got %T", resp)
	require.Empty(t, ddbResp.BatchItemFailures)

	require.Equal(t, "partner.created", gotType)
	require.Equal(t, "evt_123", gotID)
	require.Equal(t, 1, gotRecords)
}

func TestApp_EventBus_ReturnsBatchItemFailures(t *testing.T) {
	app := lift.New()

	require.NoError(t, app.EventBus("partner.created", func(_ *lift.Context, _ *services.Event) error {
		return errors.New("boom")
	}))

	resp, err := app.HandleRequest(context.Background(), map[string]any{
		"Records": []any{
			map[string]any{
				"eventSource": "aws:dynamodb",
				"eventName":   "INSERT",
				"eventID":     "rec-1",
				"dynamodb": map[string]any{
					"NewImage": map[string]any{
						"id":         map[string]any{"S": "evt_123"},
						"event_type": map[string]any{"S": "partner.created"},
					},
				},
			},
			map[string]any{
				"eventSource": "aws:dynamodb",
				"eventName":   "INSERT",
				"eventID":     "rec-2",
				"dynamodb": map[string]any{
					"NewImage": map[string]any{
						"id":         map[string]any{"S": "evt_456"},
						"event_type": map[string]any{"S": "partner.updated"},
					},
				},
			},
		},
	})
	require.NoError(t, err)

	ddbResp, ok := resp.(events.DynamoDBEventResponse)
	require.True(t, ok, "expected events.DynamoDBEventResponse, got %T", resp)
	require.Len(t, ddbResp.BatchItemFailures, 1)
	require.Equal(t, "rec-1", ddbResp.BatchItemFailures[0].ItemIdentifier)
}

func TestApp_EventBus_WildcardPattern(t *testing.T) {
	app := lift.New()

	called := 0
	require.NoError(t, app.EventBus("partner.*", func(_ *lift.Context, _ *services.Event) error {
		called++
		return nil
	}))

	_, err := app.HandleRequest(context.Background(), map[string]any{
		"Records": []any{
			map[string]any{
				"eventSource": "aws:dynamodb",
				"eventName":   "INSERT",
				"eventID":     "rec-1",
				"dynamodb": map[string]any{
					"NewImage": map[string]any{
						"id":         map[string]any{"S": "evt_123"},
						"event_type": map[string]any{"S": "partner.created"},
					},
				},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, 1, called)
}
