package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/streamer"
)

// Message represents a WebSocket message
type Message struct {
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data"`
}

// BroadcastData represents broadcast message data
type BroadcastData struct {
	Text          string   `json:"text"`
	ConnectionIDs []string `json:"connectionIds,omitempty"`
}

func main() {
	// Create Lift app with WebSocket support
	app := lift.New(lift.WithWebSocketSupport())

	// Handle new connections
	if err := app.WebSocket("$connect", handleConnect); err != nil {
		log.Fatalf("Failed to register $connect handler: %v", err)
	}

	// Handle disconnections
	if err := app.WebSocket("$disconnect", handleDisconnect); err != nil {
		log.Fatalf("Failed to register $disconnect handler: %v", err)
	}

	// Handle incoming messages
	if err := app.WebSocket("message", handleMessage); err != nil {
		log.Fatalf("Failed to register message handler: %v", err)
	}

	// Handle default route
	if err := app.WebSocket("$default", handleDefault); err != nil {
		log.Fatalf("Failed to register $default handler: %v", err)
	}

	// Start the Lambda handler
	lambda.Start(app.WebSocketHandler())
}

// handleConnect handles new WebSocket connections
func handleConnect(ctx *lift.Context) error {
	connectionID, ok := ctx.Request.Metadata["connectionId"].(string)
	if !ok {
		return ctx.Status(400).JSON(map[string]string{
			"error": "Missing connection ID",
		})
	}

	log.Printf("New connection: %s", connectionID)

	// Optionally authenticate using query parameters
	token := ctx.Query("token")
	if token == "" {
		return ctx.Status(401).JSON(map[string]string{
			"error": "Missing authentication token",
		})
	}

	// You can store connection info in DynamoDB here
	// For this example, we'll just log it

	return ctx.Status(200).JSON(map[string]string{
		"message":      "Connected successfully",
		"connectionId": connectionID,
	})
}

// handleDisconnect handles WebSocket disconnections
func handleDisconnect(ctx *lift.Context) error {
	connectionID, ok := ctx.Request.Metadata["connectionId"].(string)
	if !ok {
		log.Printf("Warning: Missing connection ID in disconnect event")
		return ctx.Status(200).JSON(map[string]string{
			"message": "Disconnected",
		})
	}

	log.Printf("Disconnected: %s", connectionID)

	// Clean up connection resources here (e.g., remove from DynamoDB)

	return nil // No response needed for disconnect
}

// handleMessage handles incoming WebSocket messages using streamer
func handleMessage(ctx *lift.Context) error {
	connectionID, ok := ctx.Request.Metadata["connectionId"].(string)
	if !ok {
		return ctx.Status(400).JSON(map[string]string{
			"error": "Missing connection ID",
		})
	}

	// Parse the incoming message
	var msg Message
	if err := ctx.ParseRequest(&msg); err != nil {
		return ctx.Status(400).JSON(map[string]string{
			"error": "Invalid message format",
		})
	}

	log.Printf("Message from %s: action=%s", connectionID, msg.Action)

	// Create streamer client from WebSocket context
	wsCtx, err := ctx.AsWebSocket()
	if err != nil {
		return ctx.Status(500).JSON(map[string]string{
			"error": "Failed to get WebSocket context",
		})
	}

	client, err := createStreamerClient(wsCtx)
	if err != nil {
		return ctx.Status(500).JSON(map[string]string{
			"error": fmt.Sprintf("Failed to create streamer client: %v", err),
		})
	}

	// Route based on action
	switch msg.Action {
	case "echo":
		return handleEcho(ctx, client, connectionID, msg.Data)

	case "broadcast":
		return handleBroadcast(ctx, client, connectionID, msg.Data)

	case "getInfo":
		return handleGetInfo(ctx, client, connectionID)

	default:
		return ctx.Status(400).JSON(map[string]string{
			"error": "Unknown action",
		})
	}
}

// handleEcho echoes back the received data
func handleEcho(ctx *lift.Context, client streamer.Client, connectionID string, data json.RawMessage) error {
	response := map[string]any{
		"action": "echo",
		"data":   string(data),
	}

	responseData, err := json.Marshal(response)
	if err != nil {
		return ctx.Status(500).JSON(map[string]string{
			"error": "Failed to marshal response",
		})
	}

	// Use streamer client to send message
	if err := client.PostToConnection(ctx.Context, connectionID, responseData); err != nil {
		return handleStreamerError(ctx, err)
	}

	return ctx.Status(200).JSON(map[string]string{
		"status": "echo sent",
	})
}

// handleBroadcast broadcasts a message to multiple connections
func handleBroadcast(ctx *lift.Context, client streamer.Client, connectionID string, data json.RawMessage) error {
	var broadcastData BroadcastData
	if err := json.Unmarshal(data, &broadcastData); err != nil {
		return ctx.Status(400).JSON(map[string]string{
			"error": "Invalid broadcast data",
		})
	}

	// In a real application, you would fetch connection IDs from your database
	// For this example, we'll use the provided connection IDs or default to sender
	targetConnections := broadcastData.ConnectionIDs
	if len(targetConnections) == 0 {
		targetConnections = []string{connectionID}
	}

	response := map[string]any{
		"action": "broadcast",
		"from":   connectionID,
		"text":   broadcastData.Text,
	}

	responseData, err := json.Marshal(response)
	if err != nil {
		return ctx.Status(500).JSON(map[string]string{
			"error": "Failed to marshal response",
		})
	}

	// Send to all target connections
	successCount := 0
	failedConnections := []string{}

	for _, targetID := range targetConnections {
		if err := client.PostToConnection(ctx.Context, targetID, responseData); err != nil {
			// Handle gone connections gracefully
			if errors.Is(err, streamer.ErrConnectionGone) {
				log.Printf("Connection %s is gone, skipping", targetID)
				failedConnections = append(failedConnections, targetID)
			} else {
				log.Printf("Failed to send to %s: %v", targetID, err)
				failedConnections = append(failedConnections, targetID)
			}
		} else {
			successCount++
		}
	}

	return ctx.Status(200).JSON(map[string]any{
		"status":            "broadcast sent",
		"successCount":      successCount,
		"failedConnections": failedConnections,
	})
}

// handleGetInfo retrieves connection information
func handleGetInfo(ctx *lift.Context, client streamer.Client, connectionID string) error {
	// Get connection info using streamer
	connInfo, err := client.GetConnection(ctx.Context, connectionID)
	if err != nil {
		return handleStreamerError(ctx, err)
	}

	response := map[string]any{
		"action":       "connectionInfo",
		"connectionId": connInfo.ConnectionID,
		"connectedAt":  connInfo.ConnectedAt.Format("2006-01-02T15:04:05Z"),
		"lastActiveAt": connInfo.LastActiveAt.Format("2006-01-02T15:04:05Z"),
		"sourceIP":     connInfo.SourceIP,
		"userAgent":    connInfo.UserAgent,
		"isActive":     connInfo.IsActive(),
		"age":          connInfo.Age().String(),
		"idleTime":     connInfo.IdleDuration().String(),
	}

	responseData, err := json.Marshal(response)
	if err != nil {
		return ctx.Status(500).JSON(map[string]string{
			"error": "Failed to marshal response",
		})
	}

	if err := client.PostToConnection(ctx.Context, connectionID, responseData); err != nil {
		return handleStreamerError(ctx, err)
	}

	return ctx.Status(200).JSON(map[string]string{
		"status": "info sent",
	})
}

// handleDefault handles unknown routes
func handleDefault(ctx *lift.Context) error {
	routeKey, ok := ctx.Request.Metadata["routeKey"].(string)
	if !ok {
		routeKey = "unknown"
	}
	return ctx.Status(404).JSON(map[string]string{
		"error": "Unknown route",
		"route": routeKey,
	})
}

// createStreamerClient creates a streamer client from WebSocket context
func createStreamerClient(wsCtx *lift.WebSocketContext) (streamer.Client, error) {
	endpoint := wsCtx.ManagementEndpoint()
	if endpoint == "" {
		return nil, fmt.Errorf("management endpoint not found")
	}

	region := wsCtx.GetRegion()
	if region == "" {
		region = os.Getenv("AWS_REGION")
	}
	if region == "" {
		region = "us-east-1"
	}

	return streamer.NewClient(context.Background(), streamer.ClientConfig{
		Endpoint: endpoint,
		Region:   region,
	})
}

// handleStreamerError handles errors from streamer operations
func handleStreamerError(ctx *lift.Context, err error) error {
	// Check if it's a streamer API error
	if apiErr, ok := err.(streamer.APIError); ok {
		statusCode := apiErr.HTTPStatusCode()
		errorCode := apiErr.ErrorCode()
		isRetryable := apiErr.IsRetryable()

		log.Printf("Streamer error: %s (code: %s, status: %d, retryable: %v)",
			err.Error(), errorCode, statusCode, isRetryable)

		// Handle specific error types
		switch {
		case errors.Is(err, streamer.ErrConnectionGone):
			return ctx.Status(410).JSON(map[string]string{
				"error": "Connection no longer exists",
			})
		case errors.Is(err, streamer.ErrForbidden):
			return ctx.Status(403).JSON(map[string]string{
				"error": "Operation forbidden",
			})
		case errors.Is(err, streamer.ErrPayloadTooLarge):
			return ctx.Status(413).JSON(map[string]string{
				"error": "Message too large",
			})
		case errors.Is(err, streamer.ErrThrottled):
			return ctx.Status(429).JSON(map[string]string{
				"error": "Rate limited",
			})
		default:
			return ctx.Status(500).JSON(map[string]string{
				"error": fmt.Sprintf("Internal error: %v", err),
			})
		}
	}

	// Generic error
	return ctx.Status(500).JSON(map[string]string{
		"error": fmt.Sprintf("Failed to send message: %v", err),
	})
}
