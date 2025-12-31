package lift_test

import (
	"context"
	"errors"
	"testing"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/stretchr/testify/require"
)

func TestApp_DynamoDB_RoutesRecords(t *testing.T) {
	app := lift.New()

	called := 0
	require.NoError(t, app.DynamoDB("*", func(ctx *lift.Context) error {
		records, err := ctx.DynamoDBRecords()
		require.NoError(t, err)
		require.Len(t, records, 1)
		called++
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
						"id": map[string]any{"S": "row_1"},
					},
				},
			},
		},
	})
	require.NoError(t, err)
	require.Nil(t, resp)
	require.Equal(t, 1, called)
}

func TestApp_DynamoDB_PropagatesHandlerErrors(t *testing.T) {
	app := lift.New()

	require.NoError(t, app.DynamoDB("*", func(_ *lift.Context) error {
		return errors.New("boom")
	}))

	_, err := app.HandleRequest(context.Background(), map[string]any{
		"Records": []any{
			map[string]any{
				"eventSource": "aws:dynamodb",
				"eventName":   "INSERT",
				"eventID":     "rec-1",
				"dynamodb": map[string]any{
					"NewImage": map[string]any{
						"id": map[string]any{"S": "row_1"},
					},
				},
			},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "boom")
}
