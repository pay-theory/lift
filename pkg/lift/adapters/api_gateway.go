package adapters

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// APIGatewayAdapter handles API Gateway V1 (REST API) events
type APIGatewayAdapter struct {
	BaseAdapter
}

// NewAPIGatewayAdapter creates a new API Gateway V1 adapter
func NewAPIGatewayAdapter() *APIGatewayAdapter {
	return &APIGatewayAdapter{
		BaseAdapter: BaseAdapter{triggerType: TriggerAPIGateway},
	}
}

// CanHandle checks if this adapter can handle the given event
func (a *APIGatewayAdapter) CanHandle(event interface{}) bool {
	eventMap, ok := event.(map[string]interface{})
	if !ok {
		return false
	}

	// Check for API Gateway V1 specific fields
	_, hasResource := eventMap["resource"]
	_, hasHttpMethod := eventMap["httpMethod"]
	_, hasRequestContext := eventMap["requestContext"]

	// API Gateway V1 events have resource, httpMethod, and requestContext
	// but no version field (or version "1.0")
	if hasResource && hasHttpMethod && hasRequestContext {
		// If version exists, it should be "1.0" or not "2.0"
		if version, exists := eventMap["version"]; exists {
			if versionStr, ok := version.(string); ok {
				return versionStr == "1.0" || versionStr != "2.0"
			}
		}
		return true
	}

	return false
}

// Validate checks if the event has the required API Gateway V1 structure
func (a *APIGatewayAdapter) Validate(event interface{}) error {
	eventMap, ok := event.(map[string]interface{})
	if !ok {
		return fmt.Errorf("event must be a map[string]interface{}")
	}

	// Check required fields
	requiredFields := []string{"resource", "httpMethod", "requestContext"}
	for _, field := range requiredFields {
		if _, exists := eventMap[field]; !exists {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	return nil
}

// Adapt converts an API Gateway V1 event to a normalized Request
func (a *APIGatewayAdapter) Adapt(rawEvent interface{}) (*Request, error) {
	if err := a.Validate(rawEvent); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	eventMap := rawEvent.(map[string]interface{})

	// Extract request context
	requestContext := extractMapField(eventMap, "requestContext")

	// Extract basic HTTP information
	method := extractStringField(eventMap, "httpMethod")
	path := extractStringField(eventMap, "path")
	if path == "" {
		// Fallback to resource if path is not available
		path = extractStringField(eventMap, "resource")
	}

	// Extract headers (case-insensitive)
	headers := make(map[string]string)
	if headersMap := extractMapField(eventMap, "headers"); len(headersMap) > 0 {
		for k, v := range headersMap {
			if str, ok := v.(string); ok {
				headers[strings.ToLower(k)] = str
			}
		}
	}

	// Handle multi-value headers
	if multiHeaders := extractMapField(eventMap, "multiValueHeaders"); len(multiHeaders) > 0 {
		for k, v := range multiHeaders {
			if slice, ok := v.([]interface{}); ok && len(slice) > 0 {
				// Take the first value for simplicity
				if str, ok := slice[0].(string); ok {
					headers[strings.ToLower(k)] = str
				}
			}
		}
	}

	// Extract query parameters
	queryParams := extractStringMapField(eventMap, "queryStringParameters")
	if queryParams == nil {
		queryParams = make(map[string]string)
	}

	// Handle multi-value query parameters
	if multiQuery := extractMapField(eventMap, "multiValueQueryStringParameters"); len(multiQuery) > 0 {
		for k, v := range multiQuery {
			if slice, ok := v.([]interface{}); ok && len(slice) > 0 {
				// Take the first value for simplicity
				if str, ok := slice[0].(string); ok {
					queryParams[k] = str
				}
			}
		}
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
	timestamp := extractStringField(requestContext, "requestTimeEpoch")

	return &Request{
		TriggerType: TriggerAPIGateway,
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
