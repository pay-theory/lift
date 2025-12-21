package lift

import (
	"context"
	"io"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/require"
)

func TestSSEResponse_ReturnsStreamingResponse(t *testing.T) {
	app := New()

	require.NoError(t, app.GET("/stream", func(ctx *Context) error {
		eventChan := make(chan SSEEvent, 2)
		eventChan <- SSEEvent{Data: "hello"}
		eventChan <- SSEEvent{ID: "1", Event: "done", Data: "goodbye", Retry: 500}
		close(eventChan)

		return SSEResponse(ctx, eventChan)
	}))

	respAny, err := app.HandleRequest(context.Background(), newAPIGatewayV1Event("GET", "/stream"))
	require.NoError(t, err)

	streamingResp, ok := respAny.(*events.LambdaFunctionURLStreamingResponse)
	require.True(t, ok, "expected streaming response, got %T", respAny)

	body, err := io.ReadAll(streamingResp)
	require.NoError(t, err)

	bodyStr := string(body)
	require.Contains(t, bodyStr, "text/event-stream")
	require.Contains(t, bodyStr, "data: hello\n\n")
	require.Contains(t, bodyStr, "id: 1\n")
	require.Contains(t, bodyStr, "event: done\n")
	require.Contains(t, bodyStr, "retry: 500\n")
	require.Contains(t, bodyStr, "data: goodbye\n\n")
}

func newAPIGatewayV1Event(method, path string) map[string]any {
	return map[string]any{
		"resource":   path,
		"path":       path,
		"httpMethod": method,
		"headers":    map[string]any{},
		"requestContext": map[string]any{
			"requestId":        "req-1",
			"stage":            "prod",
			"requestTimeEpoch": "0",
		},
		"isBase64Encoded": false,
	}
}
