package adapters

import (
	"encoding/json"
	"fmt"
)

// AppSyncAdapter adapts AWS AppSync Lambda resolver events into the normalized
// Request structure used by Lift.
//
// AppSync Lambda resolvers receive events in this format:
//
//	{
//	  "arguments": { /* GraphQL field arguments */ },
//	  "identity": { /* caller identity */ },
//	  "source": { /* parent object for nested resolvers */ },
//	  "request": { "headers": { /* HTTP headers */ } },
//	  "info": {
//	    "fieldName": "createPazeOnboarding",
//	    "parentTypeName": "Mutation",
//	    "variables": { /* GraphQL variables */ }
//	  }
//	}
type AppSyncAdapter struct {
	BaseAdapter
}

// NewAppSyncAdapter creates a new AppSync adapter.
func NewAppSyncAdapter() *AppSyncAdapter {
	return &AppSyncAdapter{
		BaseAdapter: BaseAdapter{triggerType: TriggerAppSync},
	}
}

// CanHandle reports whether the adapter recognizes the given raw event as an
// AppSync Lambda resolver event.
func (a *AppSyncAdapter) CanHandle(event any) bool {
	eventMap, ok := event.(map[string]any)
	if !ok {
		return false
	}

	// AppSync events have an "info" object with "fieldName" and "parentTypeName"
	info, hasInfo := eventMap["info"]
	if !hasInfo {
		return false
	}

	infoMap, ok := info.(map[string]any)
	if !ok {
		return false
	}

	_, hasFieldName := infoMap["fieldName"]
	_, hasParentTypeName := infoMap["parentTypeName"]

	// AppSync events also typically have "arguments" and "request"
	_, hasArguments := eventMap["arguments"]

	return hasInfo && hasFieldName && hasParentTypeName && hasArguments
}

// Validate checks that the raw event has the required AppSync fields before
// adapting it.
func (a *AppSyncAdapter) Validate(event any) error {
	eventMap, ok := event.(map[string]any)
	if !ok {
		return fmt.Errorf("event must be a map[string]any")
	}

	// Check for info object
	info, exists := eventMap["info"]
	if !exists {
		return fmt.Errorf("missing required field: info")
	}

	infoMap, ok := info.(map[string]any)
	if !ok {
		return fmt.Errorf("info must be a map[string]any")
	}

	// Check required fields in info
	requiredInfoFields := []string{"fieldName", "parentTypeName"}
	for _, field := range requiredInfoFields {
		if _, exists := infoMap[field]; !exists {
			return fmt.Errorf("missing required field in info: %s", field)
		}
	}

	// Arguments field is required (can be empty object)
	if _, exists := eventMap["arguments"]; !exists {
		return fmt.Errorf("missing required field: arguments")
	}

	return nil
}

// Adapt converts an AppSync Lambda resolver event into a normalized Request.
//
// The adapter maps GraphQL concepts to HTTP-like semantics:
//   - Mutation → POST method
//   - Query → GET method
//   - Subscription → GET method (with subscription metadata)
//   - fieldName → path (e.g., "createPazeOnboarding" → "/createPazeOnboarding")
//   - arguments → request body (JSON encoded)
func (a *AppSyncAdapter) Adapt(rawEvent any) (*Request, error) {
	if err := a.Validate(rawEvent); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	eventMap, ok := rawEvent.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("event must be a map[string]any, got %T", rawEvent)
	}

	// Extract info object
	infoMap := extractMapField(eventMap, "info")
	fieldName := extractStringField(infoMap, "fieldName")
	parentTypeName := extractStringField(infoMap, "parentTypeName")
	variables := extractMapField(infoMap, "variables")

	// Extract arguments (the input to the GraphQL field)
	arguments := extractMapField(eventMap, "arguments")

	// Extract identity (Cognito/IAM caller info)
	identity := extractMapField(eventMap, "identity")

	// Extract request headers if present
	requestObj := extractMapField(eventMap, "request")
	headers := extractStringMapField(requestObj, "headers")

	// Extract source (parent object for nested resolvers)
	source := extractMapField(eventMap, "source")

	// Determine HTTP-like method based on GraphQL operation type
	method := "POST" // Default to POST for mutations
	switch parentTypeName {
	case "Query":
		method = "GET"
	case "Mutation":
		method = "POST"
	case "Subscription":
		method = "GET" // Subscriptions are treated like queries
	}

	// Create path from field name (e.g., "createPazeOnboarding" → "/createPazeOnboarding")
	path := "/" + fieldName

	// Marshal arguments to JSON for body
	var body []byte
	if len(arguments) > 0 {
		var err error
		body, err = json.Marshal(arguments)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal arguments: %w", err)
		}
	}

	// Build metadata with AppSync-specific information
	metadata := map[string]any{
		"fieldName":       fieldName,
		"parentTypeName":  parentTypeName,
		"operationType":   parentTypeName, // Mutation, Query, or Subscription
		"isAppSync":       true,
		"hasSourceObject": len(source) > 0,
	}

	if len(variables) > 0 {
		metadata["variables"] = variables
	}

	if len(identity) > 0 {
		metadata["identity"] = identity
		// Extract common identity fields for convenience
		if sub, ok := identity["sub"].(string); ok {
			metadata["userSub"] = sub
		}
		if username, ok := identity["username"].(string); ok {
			metadata["username"] = username
		}
	}

	if len(source) > 0 {
		metadata["source"] = source
	}

	return &Request{
		TriggerType: TriggerAppSync,
		RawEvent:    rawEvent,
		Method:      method,
		Path:        path,
		Headers:     headers,
		Body:        body,
		Metadata:    metadata,
	}, nil
}
