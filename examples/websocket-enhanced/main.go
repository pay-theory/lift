package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/middleware"
	"github.com/pay-theory/lift/pkg/security"
)

// Message represents a WebSocket message
type Message struct {
	Type    string                 `json:"type"`
	Payload map[string]any `json:"payload"`
}

// Connection represents a WebSocket connection in DynamoDB
type Connection struct {
	ID        string                 `json:"id"`
	UserID    string                 `json:"user_id"`
	TenantID  string                 `json:"tenant_id"`
	CreatedAt string                 `json:"created_at"`
	Metadata  map[string]any `json:"metadata"`
}

// DynamoDBConnectionStore implements the ConnectionStore interface
type DynamoDBConnectionStore struct {
	// In a real implementation, this would have DynamoDB client
}

func (s *DynamoDBConnectionStore) Save(ctx context.Context, conn *lift.Connection) error {
	// Save to DynamoDB
	log.Printf("Saving connection: %+v", conn)
	return nil
}

func (s *DynamoDBConnectionStore) Get(ctx context.Context, connectionID string) (*lift.Connection, error) {
	// Get from DynamoDB
	return &lift.Connection{ID: connectionID}, nil
}

func (s *DynamoDBConnectionStore) Delete(ctx context.Context, connectionID string) error {
	// Delete from DynamoDB
	log.Printf("Deleting connection: %s", connectionID)
	return nil
}

func (s *DynamoDBConnectionStore) ListByUser(ctx context.Context, userID string) ([]*lift.Connection, error) {
	// Query DynamoDB by user
	return []*lift.Connection{}, nil
}

func (s *DynamoDBConnectionStore) ListByTenant(ctx context.Context, tenantID string) ([]*lift.Connection, error) {
	// Query DynamoDB by tenant
	return []*lift.Connection{}, nil
}

func (s *DynamoDBConnectionStore) CountActive(ctx context.Context) (int64, error) {
	// Count active connections in DynamoDB
	// In a real implementation, this would use a DynamoDB counter or query
	log.Printf("Counting active connections")
	return 0, nil // Return 0 for demo
}

func main() {
	// Create app with WebSocket support
	app := lift.New(lift.WithWebSocketSupport(lift.WebSocketOptions{
		EnableAutoConnectionManagement: true,
		ConnectionStore:                &DynamoDBConnectionStore{},
	}))

	// Configure observability (using NoOp for demo)
	logger := &lift.NoOpLogger{}
	metrics := &lift.NoOpMetrics{}

	app.WithLogger(logger).WithMetrics(metrics)

	// Configure middleware
	app.Use(middleware.WebSocketAuth(middleware.WebSocketAuthConfig{
		JWTConfig: security.JWTConfig{
			SigningMethod: "RS256",
			PublicKeyPath: os.Getenv("JWT_PUBLIC_KEY_PATH"),
			Issuer:        os.Getenv("JWT_ISSUER"),
		},
	}))
	app.Use(middleware.WebSocketMetrics(metrics))

	// Register WebSocket handlers - clean and simple!
	app.WebSocket("$connect", handleConnect)
	app.WebSocket("$disconnect", handleDisconnect)
	app.WebSocket("$default", handleDefault)
	app.WebSocket("ping", handlePing)
	app.WebSocket("broadcast", handleBroadcast)

	// Start Lambda handler
	lambda.Start(app.WebSocketHandler())
}

// handleConnect processes WebSocket connections
func handleConnect(ctx *lift.Context) error {
	// Convert to WebSocket context
	ws, err := ctx.AsWebSocket()
	if err != nil {
		return err
	}

	log.Printf("New connection: %s from user: %s", ws.ConnectionID(), ctx.UserID())

	// Connection is automatically saved by the framework
	// Just send a welcome message
	welcome := map[string]any{
		"type":    "welcome",
		"message": "Connected successfully",
		"user_id": ctx.UserID(),
	}

	// Note: Can't send messages in $connect response, would need to send after connection
	ctx.Set("welcome_message", welcome)

	return ctx.Status(200).JSON(map[string]string{
		"message": "Connected",
	})
}

// handleDisconnect processes WebSocket disconnections
func handleDisconnect(ctx *lift.Context) error {
	ws, err := ctx.AsWebSocket()
	if err != nil {
		return err
	}

	log.Printf("Connection closed: %s", ws.ConnectionID())

	// Connection is automatically removed by the framework
	// Just log the event
	return ctx.Status(200).JSON(map[string]string{
		"message": "Disconnected",
	})
}

// handleDefault handles unrouted messages
func handleDefault(ctx *lift.Context) error {
	ws, err := ctx.AsWebSocket()
	if err != nil {
		return err
	}

	// Parse message
	var msg Message
	if err := json.Unmarshal(ctx.Request.Body, &msg); err != nil {
		return ws.SendMessage([]byte(`{"error":"Invalid message format"}`))
	}

	// Echo back
	response := map[string]any{
		"type":    "echo",
		"message": msg,
	}

	responseData, _ := json.Marshal(response)
	return ws.SendMessage(responseData)
}

// handlePing responds to ping messages
func handlePing(ctx *lift.Context) error {
	ws, err := ctx.AsWebSocket()
	if err != nil {
		return err
	}

	return ws.SendMessage([]byte(`{"type":"pong"}`))
}

// handleBroadcast sends a message to all connections in the same tenant
func handleBroadcast(ctx *lift.Context) error {
	ws, err := ctx.AsWebSocket()
	if err != nil {
		return err
	}

	// In a real implementation, you would get the store from dependency injection
	// For now, we'll just demonstrate the pattern

	// Parse broadcast request
	var request struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(ctx.Request.Body, &request); err != nil {
		return ws.SendMessage([]byte(`{"error":"Invalid broadcast format"}`))
	}

	// Get tenant ID from context
	tenantID, _ := ctx.Get("tenant_id").(string)

	// Prepare broadcast message
	broadcast := map[string]any{
		"type":      "broadcast",
		"from":      ctx.UserID(),
		"tenant_id": tenantID,
		"message":   request.Message,
	}
	broadcastData, _ := json.Marshal(broadcast)

	// In a real implementation, you would:
	// 1. Get all connections for this tenant from the store
	// 2. Send to all connections using ws.BroadcastMessage(connectionIDs, broadcastData)

	// For demo, just echo back to sender
	return ws.SendMessage(broadcastData)
}
