package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/pay-theory/lift/pkg/lift"
)

// Example of a JWT claims structure
type JWTClaims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// WebSocketJWTMiddleware validates JWT tokens from query parameters for WebSocket connections
func WebSocketJWTMiddleware() lift.Middleware {
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

				// TODO: Validate JWT token here
				// For demo purposes, we'll just decode a simple claim
				// In production, use a proper JWT library
				claims := &JWTClaims{
					UserID:   "user123",
					Username: "demo_user",
					Role:     "user",
				}

				// Store claims in context
				ctx.Set("user_claims", claims)
				ctx.SetUserID(claims.UserID)
			}

			return next.Handle(ctx)
		})
	}
}

func main() {
	app := lift.New()

	// Add logging middleware
	app.Use(func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			if ctx.Logger != nil {
				ctx.Logger.Info("WebSocket request", map[string]interface{}{
					"method": ctx.Request.Method,
					"path":   ctx.Request.Path,
					"type":   ctx.Request.TriggerType,
				})
			}
			return next.Handle(ctx)
		})
	})

	// Add WebSocket JWT middleware
	app.Use(WebSocketJWTMiddleware())

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
		ctx.Logger.Info("WebSocket connection established", map[string]interface{}{
			"connectionId": wsCtx.ConnectionID(),
			"userId":       claims.UserID,
			"username":     claims.Username,
		})
	}

	// TODO: Store connection info in DynamoDB for tracking active connections
	// Example:
	// err = storeConnection(wsCtx.ConnectionID(), claims.UserID, claims.Username)
	// if err != nil {
	//     return ctx.Status(500).JSON(map[string]string{"error": "Failed to store connection"})
	// }

	// Send welcome message
	welcomeMsg := map[string]interface{}{
		"type":    "welcome",
		"message": fmt.Sprintf("Welcome %s! You are now connected.", claims.Username),
		"userId":  claims.UserID,
	}

	if err := wsCtx.SendJSONMessage(welcomeMsg); err != nil {
		// Note: $connect can't send messages back through the response
		// This would need to be sent after connection is established
		ctx.Logger.Error("Failed to send welcome message", map[string]interface{}{
			"error": err.Error(),
		})
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
		ctx.Logger.Info("WebSocket connection closed", map[string]interface{}{
			"connectionId": wsCtx.ConnectionID(),
		})
	}

	// TODO: Remove connection from DynamoDB
	// Example:
	// err = removeConnection(wsCtx.ConnectionID())
	// if err != nil {
	//     ctx.Logger.Error("Failed to remove connection", map[string]interface{}{
	//         "error": err.Error(),
	//     })
	// }

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
	var message map[string]interface{}
	if err := json.Unmarshal(ctx.Request.Body, &message); err != nil {
		return wsCtx.SendJSONMessage(map[string]string{
			"error": "Invalid message format",
		})
	}

	// Echo the message back with additional info
	response := map[string]interface{}{
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

	// TODO: Get all active connections from DynamoDB
	// Example:
	// connections, err := getActiveConnections()
	// if err != nil {
	//     return wsCtx.SendJSONMessage(map[string]string{
	//         "error": "Failed to get connections",
	//     })
	// }

	// For demo, we'll just send back to the sender
	broadcastMsg := map[string]interface{}{
		"type":    "broadcast",
		"from":    wsCtx.ConnectionID(),
		"message": request.Message,
	}

	// In a real implementation, you would:
	// connectionIDs := extractConnectionIDs(connections)
	// err = wsCtx.BroadcastMessage(connectionIDs, broadcastData)

	return wsCtx.SendJSONMessage(broadcastMsg)
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
func adaptLegacyHandler(handler func(context.Context, map[string]interface{}) (map[string]interface{}, error)) lift.Handler {
	return lift.HandlerFunc(func(ctx *lift.Context) error {
		// Call the legacy handler with the raw event
		response, err := handler(ctx.Context, ctx.Request.RawEvent.(map[string]interface{}))
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
