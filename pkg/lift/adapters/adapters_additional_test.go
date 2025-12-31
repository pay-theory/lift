package adapters

import (
	"encoding/base64"
	"testing"
)

func TestAdapterRegistry_GetAdapterAndAdaptWithType(t *testing.T) {
	registry := NewAdapterRegistry()

	if _, ok := registry.GetAdapter(TriggerUnknown); ok {
		t.Fatal("did not expect an adapter to be registered for TriggerUnknown")
	}
	if adapter, ok := registry.GetAdapter(TriggerAPIGatewayV2); !ok || adapter.GetTriggerType() != TriggerAPIGatewayV2 {
		t.Fatalf("expected adapter for %s to be registered", TriggerAPIGatewayV2)
	}

	if _, err := registry.AdaptWithType(map[string]any{}, TriggerUnknown); err == nil {
		t.Fatal("expected error adapting with unknown trigger type")
	}

	event := map[string]any{
		"version":  "2.0",
		"routeKey": "GET /hello",
		"requestContext": map[string]any{
			"requestId": "test-request-id",
			"http": map[string]any{
				"method": "GET",
				"path":   "/hello",
			},
		},
	}

	req, err := registry.AdaptWithType(event, TriggerAPIGatewayV2)
	if err != nil {
		t.Fatalf("unexpected error adapting with type: %v", err)
	}
	if req.TriggerType != TriggerAPIGatewayV2 {
		t.Fatalf("expected trigger type %s, got %s", TriggerAPIGatewayV2, req.TriggerType)
	}
}

func TestAPIGatewayAdapter_StageAndMultiValueAndBase64Body(t *testing.T) {
	adapter := NewAPIGatewayAdapter()
	body := base64.StdEncoding.EncodeToString([]byte("hello"))

	event := map[string]any{
		"resource":   "/users/{id}",
		"httpMethod": "GET",
		"path":       "/prod/users/123",
		"requestContext": map[string]any{
			"requestId": "rid",
			"stage":     "prod",
		},
		"headers": map[string]any{
			"X-Single": "one",
		},
		"multiValueHeaders": map[string]any{
			"X-Multi": []any{"two", "ignored"},
		},
		"queryStringParameters": map[string]any{
			"q": "1",
		},
		"multiValueQueryStringParameters": map[string]any{
			"mq": []any{"2", "ignored"},
		},
		"isBase64Encoded": true,
		"body":            body,
	}

	req, err := adapter.Adapt(event)
	if err != nil {
		t.Fatalf("unexpected error adapting event: %v", err)
	}
	if req.Path != "/users/123" {
		t.Fatalf("expected stage prefix to be stripped, got %q", req.Path)
	}
	if req.Headers["x-single"] != "one" || req.Headers["x-multi"] != "two" {
		t.Fatalf("unexpected headers: %#v", req.Headers)
	}
	if req.QueryParams["q"] != "1" || req.QueryParams["mq"] != "2" {
		t.Fatalf("unexpected query params: %#v", req.QueryParams)
	}
	if string(req.Body) != "hello" {
		t.Fatalf("expected decoded base64 body, got %q", string(req.Body))
	}

	// Stage-only path should normalize to root.
	event["path"] = "/prod"
	req, err = adapter.Adapt(event)
	if err != nil {
		t.Fatalf("unexpected error adapting stage-only path: %v", err)
	}
	if req.Path != "/" {
		t.Fatalf("expected stage-only path to normalize to '/', got %q", req.Path)
	}
}

func TestAPIGatewayAdapter_Adapt_MissingPathAndInvalidBase64Body(t *testing.T) {
	adapter := NewAPIGatewayAdapter()

	eventMissingPath := map[string]any{
		"resource":   "/hello",
		"httpMethod": "GET",
		"requestContext": map[string]any{
			"requestId": "rid",
		},
	}
	if _, err := adapter.Adapt(eventMissingPath); err == nil {
		t.Fatal("expected error when API Gateway v1 event is missing path")
	}

	eventBadBody := map[string]any{
		"resource":   "/hello",
		"httpMethod": "POST",
		"path":       "/hello",
		"requestContext": map[string]any{
			"requestId": "rid",
		},
		"isBase64Encoded": true,
		"body":            "not-base64",
	}
	if _, err := adapter.Adapt(eventBadBody); err == nil {
		t.Fatal("expected error decoding invalid base64 body")
	}
}

func TestAPIGatewayV2Adapter_Base64BodyDecode(t *testing.T) {
	adapter := NewAPIGatewayV2Adapter()

	t.Run("success", func(t *testing.T) {
		encoded := base64.StdEncoding.EncodeToString([]byte("hello"))
		event := map[string]any{
			"version":         "2.0",
			"routeKey":        "POST /hello",
			"body":            encoded,
			"isBase64Encoded": true,
			"requestContext": map[string]any{
				"requestId": "rid",
				"http": map[string]any{
					"method": "POST",
					"path":   "/hello",
				},
			},
		}

		req, err := adapter.Adapt(event)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(req.Body) != "hello" {
			t.Fatalf("expected decoded body, got %q", string(req.Body))
		}
	})

	t.Run("invalid base64", func(t *testing.T) {
		event := map[string]any{
			"version":         "2.0",
			"routeKey":        "POST /hello",
			"body":            "not-base64",
			"isBase64Encoded": true,
			"requestContext": map[string]any{
				"requestId": "rid",
				"http": map[string]any{
					"method": "POST",
					"path":   "/hello",
				},
			},
		}

		if _, err := adapter.Adapt(event); err == nil {
			t.Fatal("expected error decoding invalid base64 body")
		}
	})
}

func TestValidateRecordsEvent_ErrorPaths(t *testing.T) {
	if err := validateRecordsEvent("not-a-map", "aws:sqs", []string{"body"}); err == nil {
		t.Fatal("expected error when event is not a map")
	}
	if err := validateRecordsEvent(map[string]any{}, "aws:sqs", []string{"body"}); err == nil {
		t.Fatal("expected error when Records field is missing")
	}
	if err := validateRecordsEvent(map[string]any{"Records": "not-a-slice"}, "aws:sqs", []string{"body"}); err == nil {
		t.Fatal("expected error when Records is not a slice")
	}
	if err := validateRecordsEvent(map[string]any{"Records": []any{}}, "aws:sqs", []string{"body"}); err == nil {
		t.Fatal("expected error when Records is empty")
	}
	if err := validateRecordsEvent(map[string]any{"Records": []any{"not-a-map"}}, "aws:sqs", []string{"body"}); err == nil {
		t.Fatal("expected error when record is not a map")
	}
	if err := validateRecordsEvent(map[string]any{"Records": []any{map[string]any{"eventSource": "aws:s3"}}}, "aws:sqs", []string{"body"}); err == nil {
		t.Fatal("expected error when eventSource does not match")
	}
	if err := validateRecordsEvent(map[string]any{"Records": []any{map[string]any{"eventSource": "aws:sqs"}}}, "aws:sqs", []string{"body"}); err == nil {
		t.Fatal("expected error when required field is missing")
	}
}

