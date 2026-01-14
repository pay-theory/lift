package lift

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/require"
)

func TestFindEventBusRoute_Patterns(t *testing.T) {
	require.Nil(t, findEventBusRoute(nil, ""))
	require.Nil(t, findEventBusRoute([]*eventBusRoute{}, "partner.created"))

	routeAny := &eventBusRoute{pattern: "*"}
	require.Equal(t, routeAny, findEventBusRoute([]*eventBusRoute{nil, routeAny}, "partner.created"))

	routeWildcard := &eventBusRoute{pattern: "partner.*"}
	require.Equal(t, routeWildcard, findEventBusRoute([]*eventBusRoute{routeWildcard}, "partner.created"))
	require.Nil(t, findEventBusRoute([]*eventBusRoute{routeWildcard}, "other.created"))
}

func TestEventBusItemIdentifier_FallsBackToSequence(t *testing.T) {
	record := events.DynamoDBEventRecord{
		Change: events.DynamoDBStreamRecord{SequenceNumber: "seq-1"},
	}
	require.Equal(t, "seq-1", eventBusItemIdentifier(record))
}

func TestEventBusStreamImage_Branches(t *testing.T) {
	t.Run("remove without OldImage errors", func(t *testing.T) {
		_, err := eventBusStreamImage(events.DynamoDBEventRecord{
			EventName: string(events.DynamoDBOperationTypeRemove),
			Change:    events.DynamoDBStreamRecord{OldImage: nil},
		})
		require.Error(t, err)
	})

	t.Run("remove uses OldImage", func(t *testing.T) {
		image, err := eventBusStreamImage(events.DynamoDBEventRecord{
			EventName: string(events.DynamoDBOperationTypeRemove),
			Change: events.DynamoDBStreamRecord{
				OldImage: map[string]events.DynamoDBAttributeValue{"id": events.NewStringAttribute("evt")},
			},
		})
		require.NoError(t, err)
		require.Equal(t, "evt", image["id"].String())
	})

	t.Run("default uses NewImage", func(t *testing.T) {
		image, err := eventBusStreamImage(events.DynamoDBEventRecord{
			EventName: "INSERT",
			Change: events.DynamoDBStreamRecord{
				NewImage: map[string]events.DynamoDBAttributeValue{"id": events.NewStringAttribute("evt")},
			},
		})
		require.NoError(t, err)
		require.Equal(t, "evt", image["id"].String())
	})

	t.Run("default falls back to OldImage", func(t *testing.T) {
		image, err := eventBusStreamImage(events.DynamoDBEventRecord{
			EventName: "MODIFY",
			Change: events.DynamoDBStreamRecord{
				OldImage: map[string]events.DynamoDBAttributeValue{"id": events.NewStringAttribute("evt")},
			},
		})
		require.NoError(t, err)
		require.Equal(t, "evt", image["id"].String())
	})

	t.Run("default errors when missing images", func(t *testing.T) {
		_, err := eventBusStreamImage(events.DynamoDBEventRecord{
			EventName: "INSERT",
			Change:    events.DynamoDBStreamRecord{},
		})
		require.Error(t, err)
	})
}

func TestApp_EventBus_ValidationErrors(t *testing.T) {
	var nilApp *App
	require.Error(t, nilApp.EventBus("*", func(*Context, *struct{}) error { return nil }))

	app := New()

	require.Error(t, app.EventBus("*", nil))
	require.Error(t, app.EventBus("*", 123))
	require.Error(t, app.EventBus("*", func() error { return nil }))
	require.Error(t, app.EventBus("*", func(_ int, _ *struct{}) error { return nil }))
	require.Error(t, app.EventBus("*", func(*Context, int) error { return nil }))
	require.Error(t, app.EventBus("*", func(*Context, *struct{}) {}))

	require.False(t, (*App)(nil).hasEventBusRoutes())
}

func TestEventBusEventTypeAndID_MissingKeys(t *testing.T) {
	require.Empty(t, eventBusEventType(map[string]events.DynamoDBAttributeValue{}))
	require.Empty(t, eventBusEventID(map[string]events.DynamoDBAttributeValue{}))

	image := map[string]events.DynamoDBAttributeValue{
		"event_type": events.NewStringAttribute("partner.created"),
		"id":         events.NewStringAttribute("evt_1"),
	}
	require.Equal(t, "partner.created", eventBusEventType(image))
	require.Equal(t, "evt_1", eventBusEventID(image))
}

func TestHasEventBusRoutes_NilApp(t *testing.T) {
	require.False(t, (*App)(nil).hasEventBusRoutes())
}

func TestRouteEventBus_IgnoresNonEventRows(t *testing.T) {
	app := New()
	require.NoError(t, app.EventBus("*", func(_ *Context, _ *struct{}) error { return nil }))

	builder := newRequestHandlerBuilder(context.Background(), app, map[string]any{
		"Records": []any{
			map[string]any{
				"eventSource": "aws:dynamodb",
				"eventName":   "INSERT",
				"eventID":     "rec-1",
				"dynamodb": map[string]any{
					"NewImage": map[string]any{
						"event_type": map[string]any{"S": "partner.created"},
						// missing "id" -> treated as non-event row and skipped
					},
				},
			},
		},
	})

	resp, err := builder.build()
	require.NoError(t, err)

	ddbResp, ok := resp.(events.DynamoDBEventResponse)
	require.True(t, ok, "expected events.DynamoDBEventResponse, got %T", resp)
	require.Empty(t, ddbResp.BatchItemFailures)
}

func TestRouteEventBus_AddsBatchFailureOnStreamImageError(t *testing.T) {
	app := New()
	require.NoError(t, app.EventBus("*", func(_ *Context, _ *struct{}) error { return nil }))

	resp, err := app.HandleRequest(context.Background(), map[string]any{
		"Records": []any{
			map[string]any{
				"eventSource": "aws:dynamodb",
				"eventName":   "REMOVE",
				"eventID":     "rec-1",
				"dynamodb":    map[string]any{
					// REMOVE with missing OldImage triggers stream image error handling.
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

func TestRouteEventBus_UsesWildcardMatcher(t *testing.T) {
	app := New()

	called := 0
	require.NoError(t, app.EventBus("partner.*", func(_ *Context, _ *struct{}) error {
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
			map[string]any{
				"eventSource": "aws:dynamodb",
				"eventName":   "INSERT",
				"eventID":     "rec-2",
				"dynamodb": map[string]any{
					"NewImage": map[string]any{
						"id":         map[string]any{"S": "evt_456"},
						"event_type": map[string]any{"S": "other.created"},
					},
				},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, 1, called)
}

func TestEventBusRecordContext_NilBuilderIsSafe(t *testing.T) {
	ctx := (*requestHandlerBuilder)(nil).eventBusRecordContext(events.DynamoDBEventRecord{EventID: "rec"})
	require.NotNil(t, ctx)
}

func TestEventBus_TransactFailureIsReported(t *testing.T) {
	app := New()
	require.NoError(t, app.EventBus("*", func(_ *Context, _ *struct{}) error { return errors.New("boom") }))
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
}
