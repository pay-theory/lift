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

// TestWebSocketQueryParameterBugFix verifies that the bug where query parameters
// from map[string]string were not being extracted is fixed
func TestWebSocketQueryParameterBugFix(t *testing.T) {
	adapter := NewWebSocketAdapter()

	// Test with map[string]string query parameters (the bug case)
	event := map[string]interface{}{
		"requestContext": map[string]interface{}{
			"connectionId": "test-connection-123",
			"routeKey":     "$connect",
			"stage":        "test",
			"requestId":    "test-req-123",
			"domainName":   "api.example.com",
			"apiId":        "test-api",
		},
		// This is the key: queryStringParameters as map[string]string
		// (as it comes from AWS Lambda events)
		"queryStringParameters": map[string]string{
			"Authorization": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...",
			"userId":        "12345",
		},
	}

	req, err := adapter.Adapt(event)
	if err != nil {
		t.Fatalf("Failed to adapt event: %v", err)
	}

	// Verify query parameters are correctly extracted
	if req.QueryParams == nil {
		t.Fatal("QueryParams is nil")
	}

	if len(req.QueryParams) != 2 {
		t.Errorf("Expected 2 query parameters, got %d", len(req.QueryParams))
	}

	if req.QueryParams["Authorization"] != "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9..." {
		t.Errorf("Authorization = %s, want eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...", req.QueryParams["Authorization"])
	}

	if req.QueryParams["userId"] != "12345" {
		t.Errorf("userId = %s, want 12345", req.QueryParams["userId"])
	}
}

// TestExtractStringMapFieldBothTypes tests that extractStringMapField handles both
// map[string]string and map[string]interface{} input types
func TestExtractStringMapFieldBothTypes(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]interface{}
		key      string
		expected map[string]string
	}{
		{
			name: "map[string]string input",
			data: map[string]interface{}{
				"params": map[string]string{
					"Authorization": "Bearer token123",
					"userId":        "12345",
				},
			},
			key: "params",
			expected: map[string]string{
				"Authorization": "Bearer token123",
				"userId":        "12345",
			},
		},
		{
			name: "map[string]interface{} input",
			data: map[string]interface{}{
				"params": map[string]interface{}{
					"Authorization": "Bearer token123",
					"userId":        "12345",
				},
			},
			key: "params",
			expected: map[string]string{
				"Authorization": "Bearer token123",
				"userId":        "12345",
			},
		},
		{
			name: "mixed types in map[string]interface{}",
			data: map[string]interface{}{
				"params": map[string]interface{}{
					"Authorization": "Bearer token123",
					"userId":        12345, // non-string value should be ignored
					"active":        true,  // non-string value should be ignored
				},
			},
			key: "params",
			expected: map[string]string{
				"Authorization": "Bearer token123",
			},
		},
		{
			name: "empty map",
			data: map[string]interface{}{
				"params": map[string]string{},
			},
			key:      "params",
			expected: map[string]string{},
		},
		{
			name:     "missing key",
			data:     map[string]interface{}{},
			key:      "params",
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractStringMapField(tt.data, tt.key)

			if len(result) != len(tt.expected) {
				t.Errorf("extractStringMapField() returned %d items, expected %d", len(result), len(tt.expected))
			}

			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("extractStringMapField()[%s] = %s, want %s", k, result[k], v)
				}
			}
		})
	}
}
