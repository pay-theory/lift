package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pay-theory/lift/pkg/lift"
)

// Example of a JWT claims structure
type JWTClaims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// ConnectionStore interface for WebSocket connection management
type ConnectionStore interface {
	StoreConnection(ctx context.Context, conn *WebSocketConnection) error
	RemoveConnection(ctx context.Context, connectionID string) error
	GetActiveConnections(ctx context.Context) ([]*WebSocketConnection, error)
	GetConnectionByID(ctx context.Context, connectionID string) (*WebSocketConnection, error)
}

// WebSocketConnection represents a stored WebSocket connection
type WebSocketConnection struct {
	ConnectionID string    `json:"connection_id"`
	UserID       string    `json:"user_id"`
	Username     string    `json:"username"`
	Role         string    `json:"role"`
	ConnectedAt  time.Time `json:"connected_at"`
	LastSeen     time.Time `json:"last_seen"`
}

// Global connection store (in production, this would be injected via dependency injection)
var connectionStore ConnectionStore

// WebSocketJWTMiddleware validates JWT tokens from query parameters for WebSocket connections
func WebSocketJWTMiddleware(jwtSecret string) lift.Middleware {
	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			// Only validate on $connect events
			wsCtx, err := ctx.AsWebSocket()
			if err == nil && wsCtx.IsConnectEvent() {
				// Extract JWT from query parameters
				token := ctx.Query("Authorization")
				if token == "" {
					return ctx.Status(401).JSON(map[string]string{
						"error": "Missing authorization token",
					})
				}

				// Remove "Bearer " prefix if present
				token = strings.TrimPrefix(token, "Bearer ")

				// Validate JWT token
				claims, err := validateJWTToken(token, jwtSecret)
				if err != nil {
					return ctx.Status(401).JSON(map[string]string{
						"error":   "Invalid or expired token",
						"details": err.Error(),
					})
				}

				// Store claims in context
				ctx.Set("user_claims", claims)
				ctx.SetUserID(claims.UserID)
			}

			return next.Handle(ctx)
		})
	}
}

// validateJWTToken validates and parses a JWT token
func validateJWTToken(tokenString, secret string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (any, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		// Additional validation can be added here (expiration, issuer, etc.)
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token claims")
}

// storeConnection stores connection information in DynamoDB
func storeConnection(ctx context.Context, connectionID, userID, username, role string) error {
	if connectionStore == nil {
		// In production, initialize with actual DynamoDB connection store
		return fmt.Errorf("connection store not configured")
	}

	conn := &WebSocketConnection{
		ConnectionID: connectionID,
		UserID:       userID,
		Username:     username,
		Role:         role,
		ConnectedAt:  time.Now(),
		LastSeen:     time.Now(),
	}

	return connectionStore.StoreConnection(ctx, conn)
}

// removeConnection removes connection information from DynamoDB
func removeConnection(ctx context.Context, connectionID string) error {
	if connectionStore == nil {
		return fmt.Errorf("connection store not configured")
	}

	return connectionStore.RemoveConnection(ctx, connectionID)
}

// getActiveConnections retrieves all active connections from DynamoDB
func getActiveConnections(ctx context.Context) ([]*WebSocketConnection, error) {
	if connectionStore == nil {
		return nil, fmt.Errorf("connection store not configured")
	}

	return connectionStore.GetActiveConnections(ctx)
}

func main() {
	app := lift.New()

	// Initialize connection store (in production, use actual DynamoDB implementation)
	// connectionStore = lift.NewDynamoDBConnectionStore(context.Background(), lift.DynamoDBConnectionStoreConfig{
	//     TableName: "websocket-connections",
	//     Region:    "us-east-1",
	// })

	// Add logging middleware
	app.Use(func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			if ctx.Logger != nil {
				ctx.Logger.Info("WebSocket request", map[string]any{
					"method": ctx.Request.Method,
					"path":   ctx.Request.Path,
					"type":   ctx.Request.TriggerType,
				})
			}
			return next.Handle(ctx)
		})
	})

	// Add WebSocket JWT middleware with secret (in production, load from environment)
	app.Use(WebSocketJWTMiddleware("your-jwt-secret-key"))

	// Handle WebSocket $connect events
	app.Handle("CONNECT", "/connect", handleConnect)

	// Handle WebSocket $disconnect events
	app.Handle("DISCONNECT", "/disconnect", handleDisconnect)

	// Handle WebSocket message events
	app.Handle("MESSAGE", "/message", handleMessage)
	app.Handle("MESSAGE", "/broadcast", handleBroadcast)
	app.Handle("MESSAGE", "/ping", handlePing)

	// Start the Lambda handler
	if err := app.Start(); err != nil {
		log.Fatalf("Failed to start: %v", err)
	}
}

// handleConnect processes WebSocket connection requests
func handleConnect(ctx *lift.Context) error {
	wsCtx, err := ctx.AsWebSocket()
	if err != nil {
		return err
	}

	// Get user claims from context (set by middleware)
	claims, ok := ctx.Get("user_claims").(*JWTClaims)
	if !ok {
		return ctx.Status(401).JSON(map[string]string{
			"error": "Unauthorized",
		})
	}

	// Log connection
	if ctx.Logger != nil {
		ctx.Logger.Info("WebSocket connection established", map[string]any{
			"connectionId": wsCtx.ConnectionID(),
			"userId":       claims.UserID,
			"username":     claims.Username,
		})
	}

	// Store connection info in DynamoDB for tracking active connections
	err = storeConnection(ctx.Context, wsCtx.ConnectionID(), claims.UserID, claims.Username, claims.Role)
	if err != nil {
		ctx.Logger.Error("Failed to store connection", map[string]any{
			"error": err.Error(),
		})
		// Continue anyway - don't fail connection for storage issues
	}

	// Return success response
	return ctx.Status(200).JSON(map[string]string{
		"message": "Connected successfully",
	})
}

// handleDisconnect processes WebSocket disconnection events
func handleDisconnect(ctx *lift.Context) error {
	wsCtx, err := ctx.AsWebSocket()
	if err != nil {
		return err
	}

	// Log disconnection
	if ctx.Logger != nil {
		ctx.Logger.Info("WebSocket connection closed", map[string]any{
			"connectionId": wsCtx.ConnectionID(),
		})
	}

	// Remove connection from DynamoDB
	err = removeConnection(ctx.Context, wsCtx.ConnectionID())
	if err != nil {
		ctx.Logger.Error("Failed to remove connection", map[string]any{
			"error": err.Error(),
		})
		// Continue anyway - connection cleanup is best effort
	}

	// No response needed for disconnect
	return ctx.Status(200).JSON(map[string]string{
		"message": "Disconnected",
	})
}

// handleMessage processes incoming WebSocket messages
func handleMessage(ctx *lift.Context) error {
	wsCtx, err := ctx.AsWebSocket()
	if err != nil {
		return err
	}

	// Parse incoming message
	var message map[string]any
	if err := json.Unmarshal(ctx.Request.Body, &message); err != nil {
		return wsCtx.SendJSONMessage(map[string]string{
			"error": "Invalid message format",
		})
	}

	// Echo the message back with additional info
	response := map[string]any{
		"type":         "echo",
		"originalMsg":  message,
		"connectionId": wsCtx.ConnectionID(),
		"timestamp":    ctx.Request.Timestamp,
	}

	return wsCtx.SendJSONMessage(response)
}

// handleBroadcast broadcasts a message to all connected clients
func handleBroadcast(ctx *lift.Context) error {
	wsCtx, err := ctx.AsWebSocket()
	if err != nil {
		return err
	}

	// Parse broadcast request
	var request struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(ctx.Request.Body, &request); err != nil {
		return wsCtx.SendJSONMessage(map[string]string{
			"error": "Invalid broadcast format",
		})
	}

	// Get all active connections from DynamoDB
	connections, err := getActiveConnections(ctx.Context)
	if err != nil {
		return wsCtx.SendJSONMessage(map[string]string{
			"error":   "Failed to get active connections",
			"details": err.Error(),
		})
	}

	// Extract connection IDs for broadcasting
	var connectionIDs []string
	for _, conn := range connections {
		// Don't send back to the sender
		if conn.ConnectionID != wsCtx.ConnectionID() {
			connectionIDs = append(connectionIDs, conn.ConnectionID)
		}
	}

	if len(connectionIDs) == 0 {
		return wsCtx.SendJSONMessage(map[string]string{
			"message": "No other active connections to broadcast to",
		})
	}

	// Create broadcast message
	broadcastMsg := map[string]any{
		"type":    "broadcast",
		"from":    wsCtx.ConnectionID(),
		"message": request.Message,
		"time":    time.Now().Format(time.RFC3339),
	}

	broadcastData, err := json.Marshal(broadcastMsg)
	if err != nil {
		return wsCtx.SendJSONMessage(map[string]string{
			"error": "Failed to encode broadcast message",
		})
	}

	// Broadcast to all active connections
	err = wsCtx.BroadcastMessage(connectionIDs, broadcastData)
	if err != nil {
		ctx.Logger.Error("Broadcast failed", map[string]any{
			"error": err.Error(),
		})
		return wsCtx.SendJSONMessage(map[string]string{
			"error":   "Failed to broadcast message",
			"details": err.Error(),
		})
	}

	// Send confirmation back to sender
	return wsCtx.SendJSONMessage(map[string]any{
		"type":       "broadcast_sent",
		"message":    fmt.Sprintf("Message broadcasted to %d connections", len(connectionIDs)),
		"recipients": len(connectionIDs),
	})
}

// handlePing responds to ping messages
func handlePing(ctx *lift.Context) error {
	wsCtx, err := ctx.AsWebSocket()
	if err != nil {
		return err
	}

	return wsCtx.SendJSONMessage(map[string]string{
		"type":    "pong",
		"message": "pong",
	})
}

// Example of how to adapt existing WebSocket handlers to Lift
func adaptLegacyHandler(handler func(context.Context, map[string]any) (map[string]any, error)) lift.Handler {
	return lift.HandlerFunc(func(ctx *lift.Context) error {
		// Call the legacy handler with the raw event
		response, err := handler(ctx.Context, ctx.Request.RawEvent.(map[string]any))
		if err != nil {
			return err
		}

		// Convert response to Lift format
		if statusCode, ok := response["statusCode"].(int); ok {
			ctx.Status(statusCode)
		}

		if body, ok := response["body"].(string); ok {
			return ctx.Text(body)
		}

		return ctx.JSON(response)
	})
}
