package lift

import (
	"context"
	"testing"

	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/require"
)

func TestEventRouter_MatchingEdgeCases(t *testing.T) {
	router := NewEventRouter()

	ctx := NewContext(context.Background(), NewRequest(&adapters.Request{TriggerType: TriggerSQS, Records: nil}))
	require.False(t, router.matchSQSPattern(ctx, "queue"))

	ctx.Request.Records = []any{"not-a-map"}
	require.False(t, router.matchSQSPattern(ctx, "queue"))

	ctx.Request.Records = []any{map[string]any{}}
	require.False(t, router.matchSQSPattern(ctx, "queue"))

	require.True(t, router.matchObjectKeyPattern("a/b/c.txt", "**"))
	require.True(t, router.matchObjectKeyPattern("a/b/c.txt", "*"))

	require.True(t, newWildcardMatcher("abc", "*abc*").match())
}

func TestEventRouter_EventBridgePatternVariants(t *testing.T) {
	router := NewEventRouter()

	ctx := NewContext(context.Background(), NewRequest(&adapters.Request{
		TriggerType: TriggerEventBridge,
		Source:      "my.source",
		DetailType:  "Test",
	}))

	require.True(t, router.matchEventBridgePattern(ctx, "*"))
	require.True(t, router.matchEventBridgePattern(ctx, "my*"))
	require.True(t, router.matchEventBridgePattern(ctx, "my.source"))
	require.False(t, router.matchEventBridgePattern(ctx, "other"))
}

func TestEventRouter_ScheduledEventPatternVariants(t *testing.T) {
	router := NewEventRouter()

	ctx := NewContext(context.Background(), NewRequest(&adapters.Request{
		TriggerType: TriggerEventBridge,
		Source:      "aws.events",
		DetailType:  "Scheduled Event",
		Records:     []any{}, // no rule name
	}))
	require.True(t, router.matchScheduledEventPattern(ctx, "*"))
	require.False(t, router.matchScheduledEventPattern(ctx, "my-rule"))

	ctx.Request.Records = []any{"arn:aws:events:us-east-1:123:rule/bin-foo-daily"}
	require.True(t, router.matchScheduledEventPattern(ctx, "*-daily"))
	require.True(t, router.matchScheduledEventPattern(ctx, "bin-*-daily"))
	require.True(t, router.matchScheduledEventPattern(ctx, "bin-foo-daily"))
}

func TestContext_ParseS3Event_Errors(t *testing.T) {
	ctx := NewContext(context.Background(), NewRequest(&adapters.Request{TriggerType: TriggerAPIGateway}))
	_, err := ctx.ParseS3Event()
	require.Error(t, err)

	ctx = NewContext(context.Background(), NewRequest(&adapters.Request{TriggerType: TriggerS3, Records: nil}))
	_, err = ctx.ParseS3Event()
	require.Error(t, err)

	ctx = NewContext(context.Background(), NewRequest(&adapters.Request{TriggerType: TriggerS3, Records: []any{"bad"}}))
	_, err = ctx.ParseS3Event()
	require.Error(t, err)
}

func TestEventRouter_getMapField_Defaults(t *testing.T) {
	require.Empty(t, getMapField(map[string]any{}, "missing"))
	require.Empty(t, getMapField(map[string]any{"k": "not-map"}, "k"))
}

func TestEventRouter_S3Matching_NoBucketReturnsFalse(t *testing.T) {
	router := NewEventRouter()
	ctx := NewContext(context.Background(), NewRequest(&adapters.Request{
		TriggerType: TriggerS3,
		Records: []any{
			map[string]any{},
		},
	}))
	require.False(t, router.matchS3Pattern(ctx, "my-bucket"))
}

func TestContext_GetScheduledRuleName_EmptyWhenMissing(t *testing.T) {
	ctx := NewContext(context.Background(), NewRequest(&adapters.Request{
		TriggerType: TriggerEventBridge,
		Source:      "aws.events",
		DetailType:  "Not Scheduled",
	}))
	require.Empty(t, ctx.GetScheduledRuleName())

	ctx = NewContext(context.Background(), NewRequest(&adapters.Request{
		TriggerType: TriggerEventBridge,
		Source:      "aws.events",
		DetailType:  "Scheduled Event",
		Records:     []any{"arn:aws:events:us-east-1:123:thing/not-a-rule"},
	}))
	require.Empty(t, ctx.GetScheduledRuleName())
}
