package lift

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/require"
)

func TestStreamingHelpers_Branches(t *testing.T) {
	require.Error(t, validateSSEResponseInputs(nil, make(chan SSEEvent)))
	require.Error(t, validateSSEResponseInputs(&Context{}, nil))

	pipeReader, pipeWriter := io.Pipe()
	abort := newSSEAbortFn(pipeWriter)
	abort(nil)
	abort(errors.New("boom"))
	_ = pipeReader.Close()

	require.Error(t, writeSSEEvent(nil, SSEEvent{Data: "x"}))

	ctx := NewContext(context.Background(), NewRequest(&adapters.Request{TriggerType: TriggerAPIGateway}))
	ctx.Response.Headers["X-Test"] = "1"
	headers := buildSSEHeaders(ctx)
	require.Equal(t, "1", headers["X-Test"])
	require.Equal(t, "text/event-stream", headers[HeaderContentType])

	AddMultiValueHeader := func(c *Context) {
		c.AddMultiValueHeader("X-Multi", "a")
		c.AddMultiValueHeader("X-Multi", "b")
	}

	(*Context)(nil).AddMultiValueHeader("X", "y")
	ctx.AddMultiValueHeader("", "y")
	AddMultiValueHeader(ctx)
	require.Equal(t, map[string][]string{"X-Multi": []string{"a", "b"}}, ctx.multiValueResponseHeaders())

	syncStreamingHeadersToLiftResponse((*Context)(nil), headers)
	syncStreamingHeadersToLiftResponse(&Context{}, headers)

	ctx2 := NewContext(context.Background(), NewRequest(&adapters.Request{TriggerType: TriggerAPIGatewayV2}))
	ctx2.Response.Headers = nil
	syncStreamingHeadersToLiftResponse(ctx2, headers)
	require.Equal(t, 200, ctx2.Response.StatusCode)
	require.Equal(t, "text/event-stream", ctx2.Response.Headers[HeaderContentType])

	respAny := buildStreamingResponse(ctx, pipeReader, headers, ctx.multiValueResponseHeaders())
	require.IsType(t, &events.APIGatewayProxyStreamingResponse{}, respAny)

	respAny = buildStreamingResponse(ctx2, pipeReader, headers, nil)
	require.IsType(t, &events.LambdaFunctionURLStreamingResponse{}, respAny)
}

func TestRequestCancelFunc(t *testing.T) {
	ctx := NewContext(context.Background(), NewRequest(nil))
	require.Nil(t, requestCancelFunc(ctx))

	ctx.Set(requestCancelKey, "not-a-cancel")
	require.Nil(t, requestCancelFunc(ctx))

	canceled := false
	cancel := func() { canceled = true }
	ctx.Set(requestCancelKey, context.CancelFunc(cancel))

	got := requestCancelFunc(ctx)
	require.NotNil(t, got)
	got()
	require.True(t, canceled)
}

func TestStreamingStateHelpers(t *testing.T) {
	(*Context)(nil).setStreamingState(&streamingState{})
	(*Context)(nil).clearStreamingState()
	require.Nil(t, (*Context)(nil).streamingState())

	ctx := &Context{}
	ctx.clearStreamingState()
	require.Nil(t, ctx.streamingState())

	ctx.setStreamingState(&streamingState{})
	require.NotNil(t, ctx.streamingState())
	ctx.clearStreamingState()
	require.Nil(t, ctx.streamingState())

	ctx.Set(streamingStateKey, "wrong-type")
	require.Nil(t, ctx.streamingState())
}

func TestStreamSSE_StopsOnContextDoneAndClosedChannel(t *testing.T) {
	streamSSE(context.Background(), nil, make(chan SSEEvent))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, pipeWriter := io.Pipe()
	streamSSE(ctx, pipeWriter, make(chan SSEEvent))

	ch := make(chan SSEEvent)
	close(ch)
	_, pipeWriter2 := io.Pipe()
	streamSSE(context.Background(), pipeWriter2, ch)

	_, pipeWriter3 := io.Pipe()
	_ = pipeWriter3.Close()
	ch2 := make(chan SSEEvent, 1)
	ch2 <- SSEEvent{Data: "x"}
	close(ch2)
	streamSSE(context.Background(), pipeWriter3, ch2)
}
