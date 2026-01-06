package lift_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/stretchr/testify/require"
)

func TestApp_SQS_ReturnsBatchResponseRaw(t *testing.T) {
	app := lift.New()

	require.NoError(t, app.SQS("*", func(_ *lift.Context) (any, error) {
		return events.SQSEventResponse{
			BatchItemFailures: []events.SQSBatchItemFailure{
				{ItemIdentifier: "msg-1"},
			},
		}, nil
	}))

	respAny, err := app.HandleRequest(context.Background(), map[string]any{
		"Records": []any{
			map[string]any{
				"messageId":      "msg-1",
				"receiptHandle":  "rh-1",
				"body":           "hello",
				"eventSource":    "aws:sqs",
				"eventSourceARN": "arn:aws:sqs:us-east-1:123456789012:test-queue",
				"attributes": map[string]any{
					"SentTimestamp": "1700000000",
				},
			},
		},
	})
	require.NoError(t, err)

	batchResp, ok := respAny.(events.SQSEventResponse)
	require.True(t, ok, "expected events.SQSEventResponse, got %T", respAny)
	require.Len(t, batchResp.BatchItemFailures, 1)
	require.Equal(t, "msg-1", batchResp.BatchItemFailures[0].ItemIdentifier)
}

func TestApp_SQS_PropagatesHandlerErrors(t *testing.T) {
	app := lift.New()

	require.NoError(t, app.SQS("*", func(_ *lift.Context) error {
		return errors.New("boom")
	}))

	respAny, err := app.HandleRequest(context.Background(), map[string]any{
		"Records": []any{
			map[string]any{
				"messageId":     "msg-1",
				"receiptHandle": "rh-1",
				"body":          "hello",
				"eventSource":   "aws:sqs",
			},
		},
	})
	require.Error(t, err)
	require.Nil(t, respAny)
}

func TestApp_HTTP_StillReturnsProxyResponse(t *testing.T) {
	app := lift.New()

	require.NoError(t, app.GET("/ok", func(ctx *lift.Context) error {
		return ctx.JSON(map[string]string{"status": "ok"})
	}))

	respAny, err := app.HandleRequest(context.Background(), map[string]any{
		"version":  "2.0",
		"routeKey": "GET /ok",
		"rawPath":  "/ok",
		"requestContext": map[string]any{
			"http": map[string]any{
				"method": "GET",
				"path":   "/ok",
			},
		},
		"headers":         map[string]any{},
		"isBase64Encoded": false,
	})
	require.NoError(t, err)

	_, ok := respAny.(*lift.Response)
	require.True(t, ok, "expected *lift.Response, got %T", respAny)
}
