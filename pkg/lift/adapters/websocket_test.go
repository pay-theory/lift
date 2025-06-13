package adapters

import (
	"encoding/base64"
	"testing"
)

func TestWebSocketAdapter_CanHandle(t *testing.T) {
	adapter := NewWebSocketAdapter()

	tests := []struct {
		name     string
		event    interface{}
		expected bool
	}{
		{
			name: "Valid WebSocket connect event",
			event: map[string]interface{}{
				"requestContext": map[string]interface{}{
					"connectionId": "abc123",
					"routeKey":     "$connect",
					"eventType":    "CONNECT",
				},
			},
			expected: true,
		},
		{
			name: "Valid WebSocket disconnect event",
			event: map[string]interface{}{
				"requestContext": map[string]interface{}{
					"connectionId": "abc123",
					"routeKey":     "$disconnect",
					"eventType":    "DISCONNECT",
				},
			},
			expected: true,
		},
		{
			name: "Valid WebSocket message event",
			event: map[string]interface{}{
				"requestContext": map[string]interface{}{
					"connectionId": "abc123",
					"routeKey":     "sendMessage",
				},
			},
			expected: true,
		},
		{
			name: "Missing connectionId",
			event: map[string]interface{}{
				"requestContext": map[string]interface{}{
					"routeKey": "$connect",
				},
			},
			expected: false,
		},
		{
			name: "Not a WebSocket event",
			event: map[string]interface{}{
				"httpMethod": "GET",
				"path":       "/test",
			},
			expected: false,
		},
		{
			name:     "Invalid event type",
			event:    "not a map",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.CanHandle(tt.event)
			if result != tt.expected {
				t.Errorf("CanHandle() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestWebSocketAdapter_Adapt(t *testing.T) {
	adapter := NewWebSocketAdapter()

	tests := []struct {
		name        string
		event       interface{}
		wantErr     bool
		checkResult func(*testing.T, *Request)
	}{
		{
			name: "Connect event with query parameters",
			event: map[string]interface{}{
				"requestContext": map[string]interface{}{
					"connectionId": "abc123",
					"routeKey":     "$connect",
					"stage":        "prod",
					"requestId":    "req123",
					"domainName":   "api.example.com",
					"apiId":        "api123",
				},
				"queryStringParameters": map[string]interface{}{
					"Authorization": "Bearer token123",
				},
				"headers": map[string]interface{}{
					"User-Agent": "WebSocket Client",
				},
			},
			wantErr: false,
			checkResult: func(t *testing.T, req *Request) {
				if req.Method != "CONNECT" {
					t.Errorf("Method = %s, want CONNECT", req.Method)
				}
				if req.Path != "/connect" {
					t.Errorf("Path = %s, want /connect", req.Path)
				}
				if req.QueryParams["Authorization"] != "Bearer token123" {
					t.Errorf("QueryParams[Authorization] = %s, want Bearer token123", req.QueryParams["Authorization"])
				}
				if metadata, ok := req.Metadata["connectionId"].(string); !ok || metadata != "abc123" {
					t.Errorf("Metadata[connectionId] = %v, want abc123", req.Metadata["connectionId"])
				}
			},
		},
		{
			name: "Message event with body",
			event: map[string]interface{}{
				"requestContext": map[string]interface{}{
					"connectionId": "xyz789",
					"routeKey":     "sendMessage",
					"stage":        "dev",
					"requestId":    "req456",
					"domainName":   "api.example.com",
					"apiId":        "api456",
				},
				"body": `{"action": "sendMessage", "data": "Hello"}`,
			},
			wantErr: false,
			checkResult: func(t *testing.T, req *Request) {
				if req.Method != "MESSAGE" {
					t.Errorf("Method = %s, want MESSAGE", req.Method)
				}
				if req.Path != "/sendMessage" {
					t.Errorf("Path = %s, want /sendMessage", req.Path)
				}
				expectedBody := `{"action": "sendMessage", "data": "Hello"}`
				if string(req.Body) != expectedBody {
					t.Errorf("Body = %s, want %s", string(req.Body), expectedBody)
				}
			},
		},
		{
			name: "Disconnect event",
			event: map[string]interface{}{
				"requestContext": map[string]interface{}{
					"connectionId": "conn123",
					"routeKey":     "$disconnect",
					"stage":        "prod",
					"requestId":    "req789",
					"domainName":   "api.example.com",
					"apiId":        "api789",
				},
			},
			wantErr: false,
			checkResult: func(t *testing.T, req *Request) {
				if req.Method != "DISCONNECT" {
					t.Errorf("Method = %s, want DISCONNECT", req.Method)
				}
				if req.Path != "/disconnect" {
					t.Errorf("Path = %s, want /disconnect", req.Path)
				}
			},
		},
		{
			name: "Base64 encoded body",
			event: map[string]interface{}{
				"requestContext": map[string]interface{}{
					"connectionId": "conn456",
					"routeKey":     "message",
					"stage":        "prod",
					"requestId":    "req999",
					"domainName":   "api.example.com",
					"apiId":        "api999",
				},
				"body":            base64.StdEncoding.EncodeToString([]byte("binary data")),
				"isBase64Encoded": true,
			},
			wantErr: false,
			checkResult: func(t *testing.T, req *Request) {
				expectedBody := "binary data"
				if string(req.Body) != expectedBody {
					t.Errorf("Body = %s, want %s", string(req.Body), expectedBody)
				}
			},
		},
		{
			name: "Invalid event - missing requestContext",
			event: map[string]interface{}{
				"body": "test",
			},
			wantErr: true,
		},
		{
			name: "Invalid event - missing connectionId",
			event: map[string]interface{}{
				"requestContext": map[string]interface{}{
					"routeKey": "$connect",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := adapter.Adapt(tt.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("Adapt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.checkResult != nil {
				tt.checkResult(t, result)
			}
		})
	}
}

func TestMapWebSocketRoute(t *testing.T) {
	tests := []struct {
		routeKey   string
		wantMethod string
		wantPath   string
	}{
		{"$connect", "CONNECT", "/connect"},
		{"$disconnect", "DISCONNECT", "/disconnect"},
		{"$default", "MESSAGE", "/message"},
		{"sendMessage", "MESSAGE", "/sendMessage"},
		{"customRoute", "MESSAGE", "/customRoute"},
	}

	for _, tt := range tests {
		t.Run(tt.routeKey, func(t *testing.T) {
			method, path := mapWebSocketRoute(tt.routeKey)
			if method != tt.wantMethod {
				t.Errorf("mapWebSocketRoute() method = %s, want %s", method, tt.wantMethod)
			}
			if path != tt.wantPath {
				t.Errorf("mapWebSocketRoute() path = %s, want %s", path, tt.wantPath)
			}
		})
	}
}
