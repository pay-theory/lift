package lift

import (
	"context"
	"testing"

	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/require"
)

func TestContext_getRegionFromContext_PriorityOrder(t *testing.T) {
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")

	req := NewRequest(&adapters.Request{TriggerType: TriggerWebSocket, Metadata: map[string]any{}})
	ctx := NewContext(context.Background(), req)

	ctx.Set("aws_region", "us-west-2")
	require.Equal(t, "us-west-2", ctx.getRegionFromContext())

	ctx = NewContext(context.Background(), NewRequest(&adapters.Request{
		TriggerType: TriggerWebSocket,
		Metadata:    map[string]any{"region": "eu-west-1"},
	}))
	require.Equal(t, "eu-west-1", ctx.getRegionFromContext())

	ctx = NewContext(context.Background(), NewRequest(&adapters.Request{
		TriggerType: TriggerWebSocket,
		Metadata:    map[string]any{},
		RawEvent: map[string]any{
			"requestContext": map[string]any{
				"region": "ap-south-1",
			},
		},
	}))
	require.Equal(t, "ap-south-1", ctx.getRegionFromContext())

	t.Setenv("AWS_REGION", "ca-central-1")
	ctx = NewContext(context.Background(), NewRequest(&adapters.Request{TriggerType: TriggerWebSocket}))
	require.Equal(t, "ca-central-1", ctx.getRegionFromContext())

	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "sa-east-1")
	ctx = NewContext(context.Background(), NewRequest(&adapters.Request{TriggerType: TriggerWebSocket}))
	require.Equal(t, "sa-east-1", ctx.getRegionFromContext())

	t.Setenv("AWS_DEFAULT_REGION", "")
	ctx = NewContext(context.Background(), NewRequest(&adapters.Request{TriggerType: TriggerWebSocket}))
	require.Equal(t, defaultRegion, ctx.getRegionFromContext())
}

func TestWebSocketContext_GetManagementAPI_SuccessAndCaching(t *testing.T) {
	req := NewRequest(&adapters.Request{
		TriggerType: TriggerWebSocket,
		Metadata: map[string]any{
			"managementEndpoint": "https://example.com/prod",
		},
	})
	ctx := NewContext(context.Background(), req)
	wsCtx, err := ctx.AsWebSocket()
	require.NoError(t, err)

	client1, err := wsCtx.GetManagementAPI()
	require.NoError(t, err)
	require.NotNil(t, client1)

	client2, err := wsCtx.GetManagementAPI()
	require.NoError(t, err)
	require.Same(t, client1, client2)
}

func TestWebSocketContext_BroadcastJSONMessage_CoversMarshalAndEmptyList(t *testing.T) {
	req := NewRequest(&adapters.Request{
		TriggerType: TriggerWebSocket,
		Metadata: map[string]any{
			"connectionId": "c",
		},
	})
	ctx := NewContext(context.Background(), req)
	wsCtx, err := ctx.AsWebSocket()
	require.NoError(t, err)

	require.NoError(t, wsCtx.BroadcastJSONMessage([]string{}, map[string]string{"ok": "true"}))

	err = wsCtx.BroadcastJSONMessage([]string{"c1"}, make(chan int))
	require.Error(t, err)
}

func TestWebSocketContext_GetConnectionInfo_And_Metadata_ErrorPaths(t *testing.T) {
	req := NewRequest(&adapters.Request{
		TriggerType: TriggerWebSocket,
		Metadata: map[string]any{
			"connectionId": "c",
		},
	})
	ctx := NewContext(context.Background(), req)
	wsCtx, err := ctx.AsWebSocket()
	require.NoError(t, err)

	info, err := wsCtx.GetConnectionInfo("c")
	require.Error(t, err)
	require.Nil(t, info)

	meta, err := wsCtx.GetConnectionMetadata("c")
	require.Error(t, err)
	require.Nil(t, meta)
}
