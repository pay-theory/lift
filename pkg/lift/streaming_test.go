package lift

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/require"
)

func TestSSEResponse_ReturnsStreamingResponse(t *testing.T) {
	app := New()

	require.NoError(t, app.GET("/stream", func(ctx *Context) error {
		ctx.Response.Header("X-Custom", "1")
		ctx.AddMultiValueHeader("X-Multi", "a")
		ctx.AddMultiValueHeader("X-Multi", "b")

		eventChan := make(chan SSEEvent, 2)
		eventChan <- SSEEvent{Data: "hello"}
		eventChan <- SSEEvent{ID: "1", Event: "done", Data: "goodbye", Retry: 500}
		close(eventChan)

		return SSEResponse(ctx, eventChan)
	}))

	respAny, err := app.HandleRequest(context.Background(), newAPIGatewayV1Event("GET", "/stream"))
	require.NoError(t, err)

	streamingResp, ok := respAny.(*events.APIGatewayProxyStreamingResponse)
	require.True(t, ok, "expected streaming response, got %T", respAny)

	body, err := io.ReadAll(streamingResp)
	require.NoError(t, err)

	delimiter := make([]byte, 8)
	delimiterIndex := bytes.Index(body, delimiter)
	require.NotEqual(t, -1, delimiterIndex, "expected 8-byte delimiter in streaming response prelude")

	metadata := body[:delimiterIndex]
	payload := body[delimiterIndex+len(delimiter):]

	require.NotEmpty(t, metadata, "expected streaming response metadata")

	var meta map[string]any
	require.NoError(t, json.Unmarshal(metadata, &meta))
	require.Equal(t, float64(200), meta["statusCode"])

	for k := range meta {
		switch k {
		case "statusCode", "headers", "multiValueHeaders", "cookies":
		default:
			require.Fail(t, "unexpected metadata key", "key=%s", k)
		}
	}

	payloadStr := string(payload)
	require.Contains(t, payloadStr, "data: hello\n\n")
	require.Contains(t, payloadStr, "id: 1\n")
	require.Contains(t, payloadStr, "event: done\n")
	require.Contains(t, payloadStr, "retry: 500\n")
	require.Contains(t, payloadStr, "data: goodbye\n\n")

	headersAny := meta["headers"]
	headers, ok := headersAny.(map[string]any)
	require.True(t, ok, "expected headers to be an object")
	require.Equal(t, "text/event-stream", headers[HeaderContentType])
	require.Equal(t, "1", headers["X-Custom"])

	multiHeadersAny := meta["multiValueHeaders"]
	multiHeaders, ok := multiHeadersAny.(map[string]any)
	require.True(t, ok, "expected multiValueHeaders to be an object")

	valuesAny := multiHeaders["X-Multi"]
	values, ok := valuesAny.([]any)
	require.True(t, ok, "expected multiValueHeaders[X-Multi] to be an array")
	require.Len(t, values, 2)
	require.Contains(t, values, "a")
	require.Contains(t, values, "b")
}

func TestSSEResponse_ReturnsFunctionURLStreamingResponseForNonV1Triggers(t *testing.T) {
	app := New()

	require.NoError(t, app.GET("/stream", func(ctx *Context) error {
		eventChan := make(chan SSEEvent, 1)
		eventChan <- SSEEvent{Data: "hello"}
		close(eventChan)

		return SSEResponse(ctx, eventChan)
	}))

	respAny, err := app.HandleRequest(context.Background(), newAPIGatewayV2Event("GET", "/stream"))
	require.NoError(t, err)

	streamingResp, ok := respAny.(*events.LambdaFunctionURLStreamingResponse)
	require.True(t, ok, "expected function-url streaming response, got %T", respAny)

	body, err := io.ReadAll(streamingResp)
	require.NoError(t, err)

	bodyStr := string(body)
	require.Contains(t, bodyStr, "text/event-stream")
	require.Contains(t, bodyStr, "data: hello\n\n")
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

func newAPIGatewayV2Event(method, path string) map[string]any {
	return map[string]any{
		"version":  "2.0",
		"routeKey": method + " " + path,
		"rawPath":  path,
		"requestContext": map[string]any{
			"http": map[string]any{
				"method": method,
				"path":   path,
			},
		},
		"headers":         map[string]any{},
		"isBase64Encoded": false,
	}
}
