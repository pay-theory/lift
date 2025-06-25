package lift

import (
	"fmt"
	"strings"
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

	er.routes[triggerType] = append(er.routes[triggerType], route)
}

// FindEventHandler finds the appropriate handler for an event
func (er *EventRouter) FindEventHandler(ctx *Context) (EventHandler, error) {
	triggerType := ctx.Request.TriggerType

	// Get routes for this trigger type
	routes, exists := er.routes[triggerType]
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

	// If no specific pattern matches, use the first route as default
	// This allows for catch-all handlers
	if len(routes) > 0 {
		return routes[0].Handler, nil
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

// matchS3Pattern matches S3 bucket names
func (er *EventRouter) matchS3Pattern(ctx *Context, pattern string) bool {
	// Extract bucket information from the first record
	if len(ctx.Request.Records) == 0 {
		return false
	}

	if record, ok := ctx.Request.Records[0].(map[string]interface{}); ok {
		if s3Data, ok := record["s3"].(map[string]interface{}); ok {
			if bucket, ok := s3Data["bucket"].(map[string]interface{}); ok {
				if bucketName, ok := bucket["name"].(string); ok {
					return bucketName == pattern || pattern == "*"
				}
			}
		}
	}

	return false
}

// matchEventBridgePattern matches EventBridge source patterns
func (er *EventRouter) matchEventBridgePattern(ctx *Context, pattern string) bool {
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
	return er.routes
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
