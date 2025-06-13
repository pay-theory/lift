package adapters

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// APIGatewayV2Adapter handles API Gateway V2 (HTTP API) events
type APIGatewayV2Adapter struct {
	BaseAdapter
}

// NewAPIGatewayV2Adapter creates a new API Gateway V2 adapter
func NewAPIGatewayV2Adapter() *APIGatewayV2Adapter {
	return &APIGatewayV2Adapter{
		BaseAdapter: BaseAdapter{triggerType: TriggerAPIGatewayV2},
	}
}

// CanHandle checks if this adapter can handle the given event
func (a *APIGatewayV2Adapter) CanHandle(event interface{}) bool {
	eventMap, ok := event.(map[string]interface{})
	if !ok {
		return false
	}

	// Check for API Gateway V2 specific fields
	_, hasVersion := eventMap["version"]
	_, hasRouteKey := eventMap["routeKey"]
	_, hasRequestContext := eventMap["requestContext"]

	// API Gateway V2 events have version "2.0" and routeKey
	if hasVersion && hasRouteKey && hasRequestContext {
		if version, ok := eventMap["version"].(string); ok {
			return version == "2.0"
		}
	}

	return false
}

// Validate checks if the event has the required API Gateway V2 structure
func (a *APIGatewayV2Adapter) Validate(event interface{}) error {
	eventMap, ok := event.(map[string]interface{})
	if !ok {
		return fmt.Errorf("event must be a map[string]interface{}")
	}

	// Check required fields
	requiredFields := []string{"version", "routeKey", "requestContext"}
	for _, field := range requiredFields {
		if _, exists := eventMap[field]; !exists {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	// Validate version
	if version := extractStringField(eventMap, "version"); version != "2.0" {
		return fmt.Errorf("unsupported API Gateway version: %s", version)
	}

	return nil
}

// Adapt converts an API Gateway V2 event to a normalized Request
func (a *APIGatewayV2Adapter) Adapt(rawEvent interface{}) (*Request, error) {
	if err := a.Validate(rawEvent); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	eventMap := rawEvent.(map[string]interface{})

	// Extract request context
	requestContext := extractMapField(eventMap, "requestContext")
	httpContext := extractMapField(requestContext, "http")

	// Extract basic HTTP information
	method := extractStringField(httpContext, "method")
	path := extractStringField(httpContext, "path")

	// Extract headers (case-insensitive)
	headers := make(map[string]string)
	if headersMap := extractMapField(eventMap, "headers"); len(headersMap) > 0 {
		for k, v := range headersMap {
			if str, ok := v.(string); ok {
				headers[strings.ToLower(k)] = str
			}
		}
	}

	// Extract query parameters
	queryParams := extractStringMapField(eventMap, "queryStringParameters")
	if queryParams == nil {
		queryParams = make(map[string]string)
	}

	// Extract path parameters
	pathParams := extractStringMapField(eventMap, "pathParameters")
	if pathParams == nil {
		pathParams = make(map[string]string)
	}

	// Extract and decode body
	var body []byte
	if bodyStr := extractStringField(eventMap, "body"); bodyStr != "" {
		// Check if body is base64 encoded
		if isBase64Encoded, ok := eventMap["isBase64Encoded"].(bool); ok && isBase64Encoded {
			decoded, err := base64.StdEncoding.DecodeString(bodyStr)
			if err != nil {
				return nil, fmt.Errorf("failed to decode base64 body: %w", err)
			}
			body = decoded
		} else {
			body = []byte(bodyStr)
		}
	}

	// Extract event metadata
	eventID := extractStringField(requestContext, "requestId")
	timestamp := extractStringField(requestContext, "timeEpoch")

	return &Request{
		TriggerType: TriggerAPIGatewayV2,
		RawEvent:    rawEvent,
		EventID:     eventID,
		Timestamp:   timestamp,
		Method:      method,
		Path:        path,
		Headers:     headers,
		QueryParams: queryParams,
		PathParams:  pathParams,
		Body:        body,
	}, nil
}
