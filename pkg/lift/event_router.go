package lift

import (
	"fmt"
	"strings"
	"sync"
)

// EventHandler represents a handler for non-HTTP events
type EventHandler interface {
	HandleEvent(ctx *Context) error
}

// EventHandlerFunc is an adapter to allow ordinary functions to be used as EventHandlers
type EventHandlerFunc func(ctx *Context) error

// HandleEvent calls f(ctx)
func (f EventHandlerFunc) HandleEvent(ctx *Context) error {
	return f(ctx)
}

// EventRoute represents a route for a specific event type
type EventRoute struct {
	TriggerType TriggerType
	Pattern     string // For matching specific sources, queues, buckets, etc.
	Handler     EventHandler
}

// EventRouter handles routing for non-HTTP Lambda events
type EventRouter struct {
	mu     sync.RWMutex
	routes map[TriggerType][]*EventRoute
}

// NewEventRouter creates a new event router
func NewEventRouter() *EventRouter {
	return &EventRouter{
		routes: make(map[TriggerType][]*EventRoute),
	}
}

// AddEventRoute adds a route for a specific event type
func (er *EventRouter) AddEventRoute(triggerType TriggerType, pattern string, handler EventHandler) {
	route := &EventRoute{
		TriggerType: triggerType,
		Pattern:     pattern,
		Handler:     handler,
	}

	er.mu.Lock()
	defer er.mu.Unlock()
	er.routes[triggerType] = append(er.routes[triggerType], route)
}

// FindEventHandler finds the appropriate handler for an event
func (er *EventRouter) FindEventHandler(ctx *Context) (EventHandler, error) {
	triggerType := ctx.Request.TriggerType

	// Get routes for this trigger type
	er.mu.RLock()
	routes, exists := er.routes[triggerType]
	er.mu.RUnlock()
	
	if !exists || len(routes) == 0 {
		return nil, fmt.Errorf("no routes found for trigger type: %s", triggerType)
	}

	// For now, use simple pattern matching
	// In the future, this could be enhanced with more sophisticated matching
	for _, route := range routes {
		if er.matchesPattern(ctx, route) {
			return route.Handler, nil
		}
	}

	return nil, fmt.Errorf("no handler found for trigger type: %s", triggerType)
}

// matchesPattern checks if the event matches the route pattern
func (er *EventRouter) matchesPattern(ctx *Context, route *EventRoute) bool {
	// If pattern is empty or "*", it matches everything
	if route.Pattern == "" || route.Pattern == "*" {
		return true
	}

	switch route.TriggerType {
	case TriggerSQS:
		return er.matchSQSPattern(ctx, route.Pattern)
	case TriggerS3:
		return er.matchS3Pattern(ctx, route.Pattern)
	case TriggerEventBridge:
		return er.matchEventBridgePattern(ctx, route.Pattern)
	default:
		return true // Default to match for unknown types
	}
}

// matchSQSPattern matches SQS queue names or ARNs
func (er *EventRouter) matchSQSPattern(ctx *Context, pattern string) bool {
	// Extract queue information from the first record
	if len(ctx.Request.Records) == 0 {
		return false
	}

	if record, ok := ctx.Request.Records[0].(map[string]interface{}); ok {
		if eventSourceARN, ok := record["eventSourceARN"].(string); ok {
			// Match against queue name or ARN
			return strings.Contains(eventSourceARN, pattern) ||
				strings.HasSuffix(eventSourceARN, ":"+pattern)
		}
	}

	return false
}

// matchS3Pattern matches S3 bucket names and object keys
func (er *EventRouter) matchS3Pattern(ctx *Context, pattern string) bool {
	var bucketName, objectKey string
	
	// Check if this is an S3 event through EventBridge
	if ctx.Request.Source == "aws.s3" && ctx.Request.Detail != nil {
		// For EventBridge S3 events, bucket and object info is in the detail field
		if bucket, ok := ctx.Request.Detail["bucket"].(map[string]interface{}); ok {
			bucketName, _ = bucket["name"].(string)
		}
		if object, ok := ctx.Request.Detail["object"].(map[string]interface{}); ok {
			objectKey, _ = object["key"].(string)
		}
	} else if len(ctx.Request.Records) > 0 {
		// For direct S3 events, extract from records
		if record, ok := ctx.Request.Records[0].(map[string]interface{}); ok {
			if s3Data, ok := record["s3"].(map[string]interface{}); ok {
				if bucket, ok := s3Data["bucket"].(map[string]interface{}); ok {
					bucketName, _ = bucket["name"].(string)
				}
				if object, ok := s3Data["object"].(map[string]interface{}); ok {
					objectKey, _ = object["key"].(string)
				}
			}
		}
	}

	if bucketName == "" {
		return false
	}

	return er.matchS3PatternString(bucketName, objectKey, pattern)
}

// matchS3PatternString matches S3 patterns against bucket names and object keys
func (er *EventRouter) matchS3PatternString(bucketName, objectKey, pattern string) bool {
	// Wildcard matches everything
	if pattern == "*" {
		return true
	}
	
	// If pattern starts with /, treat it as an object key pattern
	if strings.HasPrefix(pattern, "/") {
		return er.matchObjectKeyPattern(objectKey, pattern[1:]) // Remove leading /
	}
	
	// If pattern contains /, treat it as bucket/key pattern
	if strings.Contains(pattern, "/") {
		parts := strings.SplitN(pattern, "/", 2)
		bucketPattern := parts[0]
		keyPattern := parts[1]
		
		// Match bucket part
		if !er.matchWildcardPattern(bucketName, bucketPattern) {
			return false
		}
		
		// Match key part
		return er.matchObjectKeyPattern(objectKey, keyPattern)
	}
	
	// Otherwise, just match bucket name
	return er.matchWildcardPattern(bucketName, pattern)
}

// matchObjectKeyPattern matches patterns against object keys
func (er *EventRouter) matchObjectKeyPattern(objectKey, pattern string) bool {
	if pattern == "*" || pattern == "**" {
		return true
	}
	
	// Support path-like patterns
	// e.g., "uploads/*", "*/file.zip", "data/*/reports"
	return er.matchWildcardPattern(objectKey, pattern)
}

// matchWildcardPattern matches a string against a pattern with wildcards
func (er *EventRouter) matchWildcardPattern(str, pattern string) bool {
	// Exact match
	if pattern == str {
		return true
	}
	
	// Single wildcard
	if pattern == "*" {
		return true
	}
	
	// Prefix match: "prefix*"
	if strings.HasSuffix(pattern, "*") && !strings.Contains(pattern[:len(pattern)-1], "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(str, prefix)
	}
	
	// Suffix match: "*suffix"
	if strings.HasPrefix(pattern, "*") && !strings.Contains(pattern[1:], "*") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(str, suffix)
	}
	
	// Middle wildcard: "prefix*suffix"
	if strings.Count(pattern, "*") == 1 {
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 {
			return strings.HasPrefix(str, parts[0]) && strings.HasSuffix(str, parts[1])
		}
	}
	
	// Multiple wildcards - convert to simple regex-like matching
	// This is a simplified implementation
	if strings.Contains(pattern, "*") {
		// For now, do a simple contains check for each non-wildcard part
		parts := strings.Split(pattern, "*")
		lastIndex := 0
		for i, part := range parts {
			if part == "" {
				continue
			}
			index := strings.Index(str[lastIndex:], part)
			if index == -1 {
				return false
			}
			// First part must match at the beginning
			if i == 0 && index != 0 {
				return false
			}
			lastIndex = lastIndex + index + len(part)
		}
		// Last part must match at the end if it's not empty
		if len(parts) > 0 && parts[len(parts)-1] != "" {
			return strings.HasSuffix(str, parts[len(parts)-1])
		}
		return true
	}
	
	return false
}

// matchEventBridgePattern matches EventBridge source patterns
func (er *EventRouter) matchEventBridgePattern(ctx *Context, pattern string) bool {
	// For scheduled events, match against the rule name in resources
	if ctx.Request.Source == "aws.events" && ctx.Request.DetailType == "Scheduled Event" {
		return er.matchScheduledEventPattern(ctx, pattern)
	}

	// For other EventBridge events, match against source
	source := ctx.Request.Source

	// Support simple wildcard matching
	if pattern == "*" {
		return true
	}

	// Support prefix matching with wildcards
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(source, prefix)
	}

	// Exact match
	return source == pattern
}

// matchScheduledEventPattern matches scheduled event rule patterns
func (er *EventRouter) matchScheduledEventPattern(ctx *Context, pattern string) bool {
	// Extract rule name from resources
	ruleName := ""
	for _, record := range ctx.Request.Records {
		if resourceStr, ok := record.(string); ok {
			// Rule ARN format: arn:aws:events:region:account:rule/rule-name
			parts := strings.Split(resourceStr, "/")
			if len(parts) > 1 && strings.Contains(resourceStr, ":rule/") {
				ruleName = parts[len(parts)-1]
				break
			}
		}
	}

	if ruleName == "" {
		return pattern == "*"
	}

	// Support wildcard matching
	if pattern == "*" {
		return true
	}

	// Support prefix matching with wildcards
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(ruleName, prefix)
	}

	// Support suffix matching with wildcards
	if strings.HasPrefix(pattern, "*") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(ruleName, suffix)
	}

	// Support middle wildcards (e.g., "bin-*-daily")
	if strings.Contains(pattern, "*") {
		// Convert pattern to regex-like matching
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 {
			return strings.HasPrefix(ruleName, parts[0]) && strings.HasSuffix(ruleName, parts[1])
		}
	}

	// Exact match
	return ruleName == pattern
}


// HandleEvent routes an event to the appropriate handler
func (er *EventRouter) HandleEvent(ctx *Context) error {
	handler, err := er.FindEventHandler(ctx)
	if err != nil {
		return err
	}

	return handler.HandleEvent(ctx)
}

// GetRoutes returns all routes for debugging/inspection
func (er *EventRouter) GetRoutes() map[TriggerType][]*EventRoute {
	er.mu.RLock()
	defer er.mu.RUnlock()
	
	// Create a copy to avoid external modifications
	routesCopy := make(map[TriggerType][]*EventRoute)
	for k, v := range er.routes {
		routesCopy[k] = append([]*EventRoute(nil), v...)
	}
	return routesCopy
}

// SQSMessage represents a parsed SQS message for type-safe handling
type SQSMessage struct {
	MessageID     string                 `json:"messageId"`
	Body          string                 `json:"body"`
	ReceiptHandle string                 `json:"receiptHandle"`
	Attributes    map[string]interface{} `json:"attributes"`
	EventSource   string                 `json:"eventSource"`
}

// S3Event represents a parsed S3 event for type-safe handling
type S3Event struct {
	EventSource string                 `json:"eventSource"`
	EventName   string                 `json:"eventName"`
	EventTime   string                 `json:"eventTime"`
	Bucket      string                 `json:"bucket"`
	ObjectKey   string                 `json:"objectKey"`
	ObjectSize  int64                  `json:"objectSize"`
	S3Data      map[string]interface{} `json:"s3"`
}

// EventBridgeEvent represents a parsed EventBridge event for type-safe handling
type EventBridgeEvent struct {
	Source     string                 `json:"source"`
	DetailType string                 `json:"detail-type"`
	Detail     map[string]interface{} `json:"detail"`
	Time       string                 `json:"time"`
	ID         string                 `json:"id"`
	Resources  []string               `json:"resources"`
}


// ParseSQSMessages extracts SQS messages from the request
func (ctx *Context) ParseSQSMessages() ([]SQSMessage, error) {
	if ctx.Request.TriggerType != TriggerSQS {
		return nil, fmt.Errorf("not an SQS event")
	}

	var messages []SQSMessage
	for _, record := range ctx.Request.Records {
		if recordMap, ok := record.(map[string]interface{}); ok {
			message := SQSMessage{
				MessageID:     getStringField(recordMap, "messageId"),
				Body:          getStringField(recordMap, "body"),
				ReceiptHandle: getStringField(recordMap, "receiptHandle"),
				EventSource:   getStringField(recordMap, "eventSource"),
				Attributes:    getMapField(recordMap, "attributes"),
			}
			messages = append(messages, message)
		}
	}

	return messages, nil
}

// ParseS3Event extracts S3 event information from the request
func (ctx *Context) ParseS3Event() (*S3Event, error) {
	if ctx.Request.TriggerType != TriggerS3 {
		return nil, fmt.Errorf("not an S3 event")
	}

	if len(ctx.Request.Records) == 0 {
		return nil, fmt.Errorf("no S3 records found")
	}

	// Use the first record
	if recordMap, ok := ctx.Request.Records[0].(map[string]interface{}); ok {
		s3Data := getMapField(recordMap, "s3")
		bucket := getMapField(s3Data, "bucket")
		object := getMapField(s3Data, "object")

		event := &S3Event{
			EventSource: getStringField(recordMap, "eventSource"),
			EventName:   getStringField(recordMap, "eventName"),
			EventTime:   getStringField(recordMap, "eventTime"),
			Bucket:      getStringField(bucket, "name"),
			ObjectKey:   getStringField(object, "key"),
			S3Data:      s3Data,
		}

		// Parse object size if available
		if sizeStr := getStringField(object, "size"); sizeStr != "" {
			// Convert string to int64 if needed
			event.ObjectSize = 0 // Default value
		}

		return event, nil
	}

	return nil, fmt.Errorf("invalid S3 record format")
}

// ParseEventBridgeEvent extracts EventBridge event information
func (ctx *Context) ParseEventBridgeEvent() (*EventBridgeEvent, error) {
	if ctx.Request.TriggerType != TriggerEventBridge {
		return nil, fmt.Errorf("not an EventBridge event")
	}

	event := &EventBridgeEvent{
		Source:     ctx.Request.Source,
		DetailType: ctx.Request.DetailType,
		Detail:     ctx.Request.Detail,
		Time:       ctx.Request.Timestamp,
		ID:         ctx.Request.EventID,
	}

	// Convert Records to string slice if available
	for _, record := range ctx.Request.Records {
		if str, ok := record.(string); ok {
			event.Resources = append(event.Resources, str)
		}
	}

	return event, nil
}

// IsScheduledEvent checks if this EventBridge event is a scheduled event
func (ctx *Context) IsScheduledEvent() bool {
	return ctx.Request.TriggerType == TriggerEventBridge && 
		ctx.Request.Source == "aws.events" && 
		ctx.Request.DetailType == "Scheduled Event"
}

// GetScheduledRuleName extracts the rule name from a scheduled event
func (ctx *Context) GetScheduledRuleName() string {
	if !ctx.IsScheduledEvent() {
		return ""
	}

	// Extract rule name from resources
	for _, record := range ctx.Request.Records {
		if resourceStr, ok := record.(string); ok {
			// Rule ARN format: arn:aws:events:region:account:rule/rule-name
			parts := strings.Split(resourceStr, "/")
			if len(parts) > 1 && strings.Contains(resourceStr, ":rule/") {
				return parts[len(parts)-1]
			}
		}
	}

	return ""
}


// Helper functions for safe field extraction
func getStringField(data map[string]interface{}, key string) string {
	if value, exists := data[key]; exists {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}

func getMapField(data map[string]interface{}, key string) map[string]interface{} {
	if value, exists := data[key]; exists {
		if mapValue, ok := value.(map[string]interface{}); ok {
			return mapValue
		}
	}
	return make(map[string]interface{})
}
