package adapters

import (
	"encoding/json"
	"testing"
)

func TestAppSyncAdapter_CanHandle(t *testing.T) {
	adapter := NewAppSyncAdapter()

	tests := []struct {
		name     string
		event    any
		expected bool
	}{
		{
			name: "valid AppSync mutation event",
			event: map[string]any{
				"arguments": map[string]any{
					"input": map[string]any{
						"merchantUid": "test-123",
						"scope":       "PARTNER",
					},
				},
				"identity": map[string]any{
					"sub":      "user-123",
					"username": "test-user",
				},
				"source": nil,
				"request": map[string]any{
					"headers": map[string]any{
						"x-api-key": "test-key",
					},
				},
				"info": map[string]any{
					"fieldName":      "createPazeOnboarding",
					"parentTypeName": "Mutation",
					"variables":      map[string]any{},
				},
			},
			expected: true,
		},
		{
			name: "valid AppSync query event",
			event: map[string]any{
				"arguments": map[string]any{
					"uid": "merchant-123",
				},
				"info": map[string]any{
					"fieldName":      "getPazeMerchantIdentity",
					"parentTypeName": "Query",
				},
				"request": map[string]any{},
			},
			expected: true,
		},
		{
			name: "missing info field",
			event: map[string]any{
				"arguments": map[string]any{},
				"request":   map[string]any{},
			},
			expected: false,
		},
		{
			name: "missing fieldName in info",
			event: map[string]any{
				"arguments": map[string]any{},
				"info": map[string]any{
					"parentTypeName": "Query",
				},
			},
			expected: false,
		},
		{
			name: "API Gateway v2 event (should not match)",
			event: map[string]any{
				"version":        "2.0",
				"routeKey":       "POST /paze/onboard",
				"rawPath":        "/paze/onboard",
				"requestContext": map[string]any{},
			},
			expected: false,
		},
		{
			name:     "nil event",
			event:    nil,
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

func TestAppSyncAdapter_Validate(t *testing.T) {
	adapter := NewAppSyncAdapter()

	tests := []struct {
		name    string
		event   any
		wantErr bool
	}{
		{
			name: "valid event",
			event: map[string]any{
				"arguments": map[string]any{},
				"info": map[string]any{
					"fieldName":      "testField",
					"parentTypeName": "Mutation",
				},
			},
			wantErr: false,
		},
		{
			name: "missing info",
			event: map[string]any{
				"arguments": map[string]any{},
			},
			wantErr: true,
		},
		{
			name: "missing fieldName",
			event: map[string]any{
				"arguments": map[string]any{},
				"info": map[string]any{
					"parentTypeName": "Query",
				},
			},
			wantErr: true,
		},
		{
			name: "missing parentTypeName",
			event: map[string]any{
				"arguments": map[string]any{},
				"info": map[string]any{
					"fieldName": "testField",
				},
			},
			wantErr: true,
		},
		{
			name: "missing arguments",
			event: map[string]any{
				"info": map[string]any{
					"fieldName":      "testField",
					"parentTypeName": "Query",
				},
			},
			wantErr: true,
		},
		{
			name:    "not a map",
			event:   "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := adapter.Validate(tt.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAppSyncAdapter_Adapt(t *testing.T) {
	adapter := NewAppSyncAdapter()

	tests := []struct {
		name           string
		event          map[string]any
		expectedMethod string
		expectedPath   string
		checkBody      bool
		expectedBody   map[string]any
	}{
		{
			name: "mutation with input",
			event: map[string]any{
				"arguments": map[string]any{
					"input": map[string]any{
						"merchantUid": "merchant-123",
						"scope":       "PARTNER",
					},
				},
				"identity": map[string]any{
					"sub":      "user-123",
					"username": "test-user",
				},
				"source": nil,
				"request": map[string]any{
					"headers": map[string]any{
						"x-api-key": "test-key",
					},
				},
				"info": map[string]any{
					"fieldName":      "createPazeOnboarding",
					"parentTypeName": "Mutation",
					"variables":      map[string]any{},
				},
			},
			expectedMethod: "POST",
			expectedPath:   "/createPazeOnboarding",
			checkBody:      true,
			expectedBody: map[string]any{
				"input": map[string]any{
					"merchantUid": "merchant-123",
					"scope":       "PARTNER",
				},
			},
		},
		{
			name: "query with arguments",
			event: map[string]any{
				"arguments": map[string]any{
					"uid": "merchant-456",
				},
				"info": map[string]any{
					"fieldName":      "getPazeMerchantIdentity",
					"parentTypeName": "Query",
				},
				"request": map[string]any{},
			},
			expectedMethod: "GET",
			expectedPath:   "/getPazeMerchantIdentity",
			checkBody:      true,
			expectedBody: map[string]any{
				"uid": "merchant-456",
			},
		},
		{
			name: "subscription",
			event: map[string]any{
				"arguments": map[string]any{},
				"info": map[string]any{
					"fieldName":      "onPazeUpdate",
					"parentTypeName": "Subscription",
				},
				"request": map[string]any{},
			},
			expectedMethod: "GET",
			expectedPath:   "/onPazeUpdate",
			checkBody:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := adapter.Adapt(tt.event)
			if err != nil {
				t.Fatalf("Adapt() error = %v", err)
			}

			if req.TriggerType != TriggerAppSync {
				t.Errorf("TriggerType = %v, want %v", req.TriggerType, TriggerAppSync)
			}

			if req.Method != tt.expectedMethod {
				t.Errorf("Method = %v, want %v", req.Method, tt.expectedMethod)
			}

			if req.Path != tt.expectedPath {
				t.Errorf("Path = %v, want %v", req.Path, tt.expectedPath)
			}

			// Check metadata
			if req.Metadata == nil {
				t.Fatal("Metadata is nil")
			}

			if fieldName, ok := req.Metadata["fieldName"].(string); !ok || fieldName == "" {
				t.Error("Metadata missing fieldName")
			}

			if parentTypeName, ok := req.Metadata["parentTypeName"].(string); !ok || parentTypeName == "" {
				t.Error("Metadata missing parentTypeName")
			}

			if isAppSync, ok := req.Metadata["isAppSync"].(bool); !ok || !isAppSync {
				t.Error("Metadata missing or false isAppSync flag")
			}

			// Check body if expected
			if tt.checkBody {
				if len(req.Body) == 0 {
					t.Fatal("Body is empty")
				}

				var bodyMap map[string]any
				if err := json.Unmarshal(req.Body, &bodyMap); err != nil {
					t.Fatalf("Failed to unmarshal body: %v", err)
				}

				if tt.expectedBody != nil {
					// Deep comparison would be better, but for now check keys exist
					for key := range tt.expectedBody {
						if _, exists := bodyMap[key]; !exists {
							t.Errorf("Body missing expected key: %s", key)
						}
					}
				}
			}
		})
	}
}

func TestAppSyncAdapter_AdaptWithIdentity(t *testing.T) {
	adapter := NewAppSyncAdapter()

	event := map[string]any{
		"arguments": map[string]any{},
		"identity": map[string]any{
			"sub":      "user-sub-123",
			"username": "john.doe",
			"claims": map[string]any{
				"email": "john@example.com",
			},
		},
		"info": map[string]any{
			"fieldName":      "testField",
			"parentTypeName": "Query",
		},
		"request": map[string]any{},
	}

	req, err := adapter.Adapt(event)
	if err != nil {
		t.Fatalf("Adapt() error = %v", err)
	}

	// Check identity in metadata
	identity, ok := req.Metadata["identity"].(map[string]any)
	if !ok {
		t.Fatal("Metadata missing identity")
	}

	if sub, ok := identity["sub"].(string); !ok || sub != "user-sub-123" {
		t.Errorf("Identity sub = %v, want user-sub-123", sub)
	}

	// Check convenience fields
	if userSub, ok := req.Metadata["userSub"].(string); !ok || userSub != "user-sub-123" {
		t.Errorf("Metadata userSub = %v, want user-sub-123", userSub)
	}

	if username, ok := req.Metadata["username"].(string); !ok || username != "john.doe" {
		t.Errorf("Metadata username = %v, want john.doe", username)
	}
}
