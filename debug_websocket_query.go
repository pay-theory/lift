package main

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/adapters"
)

func main() {
	// Create the exact event structure as reported in the bug
	event := events.APIGatewayWebsocketProxyRequest{
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			ConnectionID: "test-connection-123",
			RouteKey:     "$connect",
			Stage:        "test",
			RequestID:    "test-req-123",
			DomainName:   "api.example.com",
			APIID:        "test-api",
		},
		QueryStringParameters: map[string]string{
			"Authorization": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...",
		},
	}

	fmt.Println("=== Debug WebSocket Query Parameter Issue ===")
	fmt.Printf("Original event QueryStringParameters: %+v\n", event.QueryStringParameters)

	// Convert to generic format (as done in app_websocket.go)
	genericEvent := convertWebSocketEventToGeneric(event)
	fmt.Printf("Generic event queryStringParameters: %+v\n", genericEvent["queryStringParameters"])
	fmt.Printf("Type of queryStringParameters: %T\n", genericEvent["queryStringParameters"])

	// Test the adapter
	adapter := adapters.NewWebSocketAdapter()
	req, err := adapter.Adapt(genericEvent)
	if err != nil {
		log.Fatalf("Failed to adapt event: %v", err)
	}

	fmt.Printf("Adapter result QueryParams: %+v\n", req.QueryParams)

	// Create Lift context
	liftReq := lift.NewRequest(req)
	ctx := lift.NewContext(context.Background(), liftReq)

	// Test ctx.Query() method
	authToken := ctx.Query("Authorization")
	fmt.Printf("ctx.Query('Authorization'): '%s'\n", authToken)

	if authToken == "" {
		fmt.Println("❌ BUG CONFIRMED: Authorization query parameter is empty!")
	} else {
		fmt.Println("✅ Query parameter working correctly")
	}

	// Additional debugging
	fmt.Printf("ctx.Request.QueryParams: %+v\n", ctx.Request.QueryParams)
	fmt.Printf("ctx.Request.Request.QueryParams: %+v\n", ctx.Request.Request.QueryParams)
}

// Helper function to convert WebSocket event to generic format
func convertWebSocketEventToGeneric(event events.APIGatewayWebsocketProxyRequest) map[string]interface{} {
	return map[string]interface{}{
		"requestContext": map[string]interface{}{
			"routeKey":          event.RequestContext.RouteKey,
			"messageId":         event.RequestContext.MessageID,
			"eventType":         event.RequestContext.EventType,
			"extendedRequestId": event.RequestContext.ExtendedRequestID,
			"requestTime":       event.RequestContext.RequestTime,
			"messageDirection":  event.RequestContext.MessageDirection,
			"stage":             event.RequestContext.Stage,
			"connectedAt":       event.RequestContext.ConnectedAt,
			"requestTimeEpoch":  event.RequestContext.RequestTimeEpoch,
			"requestId":         event.RequestContext.RequestID,
			"domainName":        event.RequestContext.DomainName,
			"connectionId":      event.RequestContext.ConnectionID,
			"apiId":             event.RequestContext.APIID,
		},
		"body":                            event.Body,
		"isBase64Encoded":                 event.IsBase64Encoded,
		"stageVariables":                  event.StageVariables,
		"headers":                         event.Headers,
		"multiValueHeaders":               event.MultiValueHeaders,
		"queryStringParameters":           event.QueryStringParameters,
		"multiValueQueryStringParameters": event.MultiValueQueryStringParameters,
	}
}
