package lift

import (
	"context"
	"testing"

	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/require"
)

func TestEventRouter_FindEventHandler_Errors(t *testing.T) {
	router := NewEventRouter()

	ctx := NewContext(context.Background(), NewRequest(&adapters.Request{TriggerType: TriggerSQS}))
	_, err := router.FindEventHandler(ctx)
	require.Error(t, err)
}

func TestEventRouter_SQSAndS3PatternMatching(t *testing.T) {
	router := NewEventRouter()

	sqsCalled := 0
	router.AddEventRoute(TriggerSQS, "my-queue", EventHandlerFunc(func(*Context) error {
		sqsCalled++
		return nil
	}))

	s3Called := 0
	router.AddEventRoute(TriggerS3, "my-bucket/uploads/*", EventHandlerFunc(func(*Context) error {
		s3Called++
		return nil
	}))

	sqsCtx := NewContext(context.Background(), NewRequest(&adapters.Request{
		TriggerType: TriggerSQS,
		Records: []any{
			map[string]any{"eventSourceARN": "arn:aws:sqs:us-east-1:123:my-queue"},
		},
	}))
	handler, err := router.FindEventHandler(sqsCtx)
	require.NoError(t, err)
	require.NoError(t, handler.HandleEvent(sqsCtx))
	require.Equal(t, 1, sqsCalled)

	s3Ctx := NewContext(context.Background(), NewRequest(&adapters.Request{
		TriggerType: TriggerS3,
		Source:      "aws.s3",
		Detail: map[string]any{
			"bucket": map[string]any{"name": "my-bucket"},
			"object": map[string]any{"key": "uploads/file.txt"},
		},
	}))
	handler, err = router.FindEventHandler(s3Ctx)
	require.NoError(t, err)
	require.NoError(t, handler.HandleEvent(s3Ctx))
	require.Equal(t, 1, s3Called)
}

func TestEventRouter_EventBridgeScheduledRuleMatching(t *testing.T) {
	router := NewEventRouter()
	called := 0
	router.AddEventRoute(TriggerEventBridge, "my-*", EventHandlerFunc(func(*Context) error {
		called++
		return nil
	}))

	ctx := NewContext(context.Background(), NewRequest(&adapters.Request{
		TriggerType: TriggerEventBridge,
		Source:      "aws.events",
		DetailType:  "Scheduled Event",
		Records: []any{
			"arn:aws:events:us-east-1:123:rule/my-rule",
		},
	}))
	handler, err := router.FindEventHandler(ctx)
	require.NoError(t, err)
	require.NoError(t, handler.HandleEvent(ctx))
	require.Equal(t, 1, called)
}

func TestWildcardMatcher_MatchVariants(t *testing.T) {
	require.True(t, newWildcardMatcher("abc", "abc").match())
	require.True(t, newWildcardMatcher("abc", "*").match())
	require.True(t, newWildcardMatcher("prefix-abc", "prefix-*").match())
	require.True(t, newWildcardMatcher("abc-suffix", "*suffix").match())
	require.True(t, newWildcardMatcher("start-middle-end", "start*-end").match())
	require.True(t, newWildcardMatcher("a-b-c", "a*b*c").match())
	require.False(t, newWildcardMatcher("abc", "a*d").match())
}

func TestContext_EventParsingHelpers(t *testing.T) {
	t.Run("ParseSQSMessages rejects non-SQS", func(t *testing.T) {
		ctx := NewContext(context.Background(), NewRequest(&adapters.Request{TriggerType: TriggerAPIGateway}))
		_, err := ctx.ParseSQSMessages()
		require.Error(t, err)
	})

	t.Run("ParseSQSMessages extracts fields", func(t *testing.T) {
		ctx := NewContext(context.Background(), NewRequest(&adapters.Request{
			TriggerType: TriggerSQS,
			Records: []any{
				map[string]any{
					"messageId":     "m1",
					"body":          "hi",
					"receiptHandle": "rh",
					"eventSource":   "aws:sqs",
					"attributes":    map[string]any{"ApproximateReceiveCount": "1"},
				},
			},
		}))

		msgs, err := ctx.ParseSQSMessages()
		require.NoError(t, err)
		require.Len(t, msgs, 1)
		require.Equal(t, "m1", msgs[0].MessageID)
		require.Equal(t, "hi", msgs[0].Body)
		require.Equal(t, "rh", msgs[0].ReceiptHandle)
	})

	t.Run("ParseS3Event extracts bucket and key", func(t *testing.T) {
		ctx := NewContext(context.Background(), NewRequest(&adapters.Request{
			TriggerType: TriggerS3,
			Records: []any{
				map[string]any{
					"eventSource": "aws:s3",
					"s3": map[string]any{
						"bucket": map[string]any{"name": "my-bucket"},
						"object": map[string]any{"key": "file.txt"},
					},
				},
			},
		}))

		event, err := ctx.ParseS3Event()
		require.NoError(t, err)
		require.Equal(t, "my-bucket", event.Bucket)
		require.Equal(t, "file.txt", event.ObjectKey)
	})

	t.Run("ParseEventBridgeEvent returns resources", func(t *testing.T) {
		ctx := NewContext(context.Background(), NewRequest(&adapters.Request{
			TriggerType: TriggerEventBridge,
			Source:      "aws.events",
			DetailType:  "Scheduled Event",
			EventID:     "evt-1",
			Timestamp:   "2024-01-01T00:00:00Z",
			Records: []any{
				"arn:aws:events:us-east-1:123:rule/my-rule",
			},
		}))

		event, err := ctx.ParseEventBridgeEvent()
		require.NoError(t, err)
		require.Equal(t, "aws.events", event.Source)
		require.Equal(t, "Scheduled Event", event.DetailType)
		require.Equal(t, "evt-1", event.ID)
		require.Len(t, event.Resources, 1)
		require.Equal(t, "arn:aws:events:us-east-1:123:rule/my-rule", event.Resources[0])
		require.True(t, ctx.IsScheduledEvent())
		require.Equal(t, "my-rule", ctx.GetScheduledRuleName())
	})
}

func TestEventRouter_HandleEvent_AppliesMiddleware(t *testing.T) {
	router := NewEventRouter()

	order := make([]string, 0, 2)
	router.AddEventRoute(TriggerSQS, "*", EventHandlerFunc(func(*Context) error {
		order = append(order, "handler")
		return nil
	}))

	mw1 := Middleware(func(next Handler) Handler {
		return HandlerFunc(func(ctx *Context) error {
			order = append(order, "mw1")
			return next.Handle(ctx)
		})
	})
	mw2 := Middleware(func(next Handler) Handler {
		return HandlerFunc(func(ctx *Context) error {
			order = append(order, "mw2")
			return next.Handle(ctx)
		})
	})

	ctx := NewContext(context.Background(), NewRequest(&adapters.Request{
		TriggerType: TriggerSQS,
		Records: []any{
			map[string]any{"eventSourceARN": "arn:aws:sqs:us-east-1:123:queue"},
		},
	}))

	require.NoError(t, router.HandleEvent(ctx, []Middleware{mw1, mw2}))
	require.Equal(t, []string{"mw1", "mw2", "handler"}, order)
}
