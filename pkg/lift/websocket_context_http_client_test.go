package lift

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi/types"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/require"
)

type stubHTTPClient struct {
	do func(*http.Request) (*http.Response, error)
}

func (c stubHTTPClient) Do(r *http.Request) (*http.Response, error) {
	return c.do(r)
}

func TestWebSocketContext_HTTPClient_DrivenOperations(t *testing.T) {
	httpClient := stubHTTPClient{
		do: func(r *http.Request) (*http.Response, error) {
			connID := ""
			if idx := strings.Index(r.URL.Path, "/@connections/"); idx != -1 {
				connID = strings.TrimPrefix(r.URL.Path[idx:], "/@connections/")
				connID = strings.TrimPrefix(connID, "/")
			}

			status := 200
			body := ""
			switch r.Method {
			case http.MethodPost:
				if connID == "gone" {
					status = 410
					body = `{"message":"Gone"}`
				}
			case http.MethodDelete:
				if connID == "gone" {
					status = 410
					body = `{"message":"Gone"}`
				}
			case http.MethodGet:
				if connID == "gone" {
					status = 410
					body = `{"message":"Gone"}`
				} else {
					body = `{"identity":{"sourceIp":"203.0.113.10"},"connectedAt":"2024-01-01T00:00:00Z"}`
				}
			}

			headers := http.Header{"Content-Type": []string{"application/json"}}
			if status == 410 {
				headers.Set("x-amzn-errortype", "GoneException")
			}

			return &http.Response{
				StatusCode: status,
				Header:     headers,
				Body:       io.NopCloser(strings.NewReader(body)),
				Request:    r,
			}, nil
		},
	}

	cfg := aws.Config{
		Region:      "us-east-1",
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider("AKID", "SECRET", "")),
		HTTPClient:  httpClient,
	}
	client := apigatewaymanagementapi.NewFromConfig(cfg, func(o *apigatewaymanagementapi.Options) {
		o.BaseEndpoint = aws.String("https://example.com")
	})

	req := NewRequest(&adapters.Request{
		TriggerType: TriggerWebSocket,
		Metadata: map[string]any{
			"connectionId": "ok",
		},
	})
	ctx := NewContext(context.Background(), req)
	wsCtx, err := ctx.AsWebSocket()
	require.NoError(t, err)
	wsCtx.managementAPI = client
	wsCtx.Logger = &NoOpLogger{}

	require.NoError(t, wsCtx.SendMessage([]byte("hi")))

	require.NoError(t, wsCtx.BroadcastMessage([]string{"ok", "gone"}, []byte("hi")))

	err = wsCtx.SendMessage([]byte("hi"))
	require.NoError(t, err)

	wsCtx.Request.Metadata["connectionId"] = "gone"
	err = wsCtx.SendMessage([]byte("hi"))
	require.Error(t, err)

	var goneErr *types.GoneException
	require.ErrorAs(t, err, &goneErr)

	require.NoError(t, wsCtx.Disconnect("gone"))

	info, err := wsCtx.GetConnectionInfo("ok")
	require.NoError(t, err)
	require.NotNil(t, info)

	meta, err := wsCtx.GetConnectionMetadata("ok")
	require.NoError(t, err)
	require.NotNil(t, meta)
	require.Equal(t, "ok", meta.ConnectionID)
	require.NotNil(t, meta.ConnectedAt)
	require.WithinDuration(t, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), *meta.ConnectedAt, time.Second)
	require.Equal(t, map[string]string{"sourceIp": "203.0.113.10"}, meta.Identity)

	_, err = wsCtx.GetConnectionInfo("gone")
	require.Error(t, err)
}

func TestWebSocketContext_baseContext_NilCases(t *testing.T) {
	require.Equal(t, context.Background(), (*WebSocketContext)(nil).baseContext())
	require.Equal(t, context.Background(), (&WebSocketContext{}).baseContext())
	wc := &WebSocketContext{Context: &Context{}}
	require.Same(t, wc.Context, wc.baseContext())

	wc = &WebSocketContext{Context: &Context{Context: context.Background()}}
	require.Equal(t, context.Background(), wc.baseContext())
}

func TestWebSocketContext_getRegionFromContext_Delegate(t *testing.T) {
	require.Equal(t, defaultRegion, (&WebSocketContext{}).getRegionFromContext())
}
