package main

import (
	"encoding/json"
	"log"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/pay-theory/lift/pkg/lift"
)

func main() {
	// Create Lift app with WebSocket support
	app := lift.New(lift.WithWebSocketSupport())

	// Handle new connections
	if err := app.WebSocket("$connect", func(ctx *lift.Context) error {
		connectionID, ok := ctx.Request.Metadata["connectionId"].(string)
		if !ok {
			return ctx.Status(400).JSON(map[string]string{
				"error": "Missing connection ID",
			})
		}
		log.Printf("New connection: %s", connectionID)

		// You can authenticate here using query parameters
		token := ctx.Query("token")
		if token == "" {
			return ctx.Status(401).JSON(map[string]string{
				"error": "Missing authentication token",
			})
		}

		// Store any connection metadata you need
		ctx.Set("authenticated", true)

		return ctx.Status(200).JSON(map[string]string{
			"message":      "Connected successfully",
			"connectionId": connectionID,
		})
	}); err != nil {
		log.Fatalf("Failed to register WebSocket $connect handler: %v", err)
	}

	// Handle disconnections
	if err := app.WebSocket("$disconnect", func(ctx *lift.Context) error {
		connectionID, ok := ctx.Request.Metadata["connectionId"].(string)
		if !ok {
			log.Printf("Warning: Missing connection ID in disconnect event")
			return ctx.Status(200).JSON(map[string]string{
				"message": "Disconnected",
			})
		}
		log.Printf("Disconnected: %s", connectionID)

		// Clean up any resources for this connection
		// The connection will be automatically removed if using ConnectionStore

		return nil // No response needed for disconnect
	}); err != nil {
		log.Fatalf("Failed to register WebSocket $disconnect handler: %v", err)
	}

	// Handle incoming messages
	if err := app.WebSocket("message", func(ctx *lift.Context) error {
		connectionID, ok := ctx.Request.Metadata["connectionId"].(string)
		if !ok {
			return ctx.Status(400).JSON(map[string]string{
				"error": "Missing connection ID",
			})
		}

		// Parse the incoming message
		var msg struct {
			Action string          `json:"action"`
			Data   json.RawMessage `json:"data"`
		}

		if err := ctx.ParseRequest(&msg); err != nil {
			return ctx.Status(400).JSON(map[string]string{
				"error": "Invalid message format",
			})
		}

		log.Printf("Message from %s: action=%s", connectionID, msg.Action)

		// Route based on action
		switch msg.Action {
		case "echo":
			// Simple echo response
			return ctx.Status(200).JSON(map[string]string{
				"action": "echo",
				"data":   string(msg.Data),
			})

		case "broadcast":
			// For broadcasting, you'll need to use the WebSocket context
			wsCtx, err := ctx.AsWebSocket()
			if err != nil {
				return ctx.Status(500).JSON(map[string]string{
					"error": "WebSocket context error",
				})
			}

			// In a real app, you'd get other connection IDs from a store
			// For now, just send back to the same connection
			response := map[string]any{
				"action": "broadcast",
				"from":   connectionID,
				"data":   msg.Data,
			}

			responseData, err := json.Marshal(response)
			if err != nil {
				log.Printf("Failed to marshal response: %v", err)
				return ctx.Status(500).JSON(map[string]string{
					"error": "Failed to process message",
				})
			}
			if err := wsCtx.SendMessage(responseData); err != nil {
				log.Printf("Failed to send message: %v", err)
			}

			return ctx.Status(200).JSON(map[string]string{
				"status": "broadcast sent",
			})

		default:
			return ctx.Status(400).JSON(map[string]string{
				"error": "Unknown action",
			})
		}
	}); err != nil {
		log.Fatalf("Failed to register WebSocket message handler: %v", err)
	}

	// Handle any other routes with a default handler
	if err := app.WebSocket("$default", func(ctx *lift.Context) error {
		routeKey, ok := ctx.Request.Metadata["routeKey"].(string)
		if !ok {
			routeKey = "unknown"
		}
		return ctx.Status(404).JSON(map[string]string{
			"error": "Unknown route",
			"route": routeKey,
		})
	}); err != nil {
		log.Fatalf("Failed to register WebSocket $default handler: %v", err)
	}

	// Start the Lambda handler
	lambda.Start(app.WebSocketHandler())
}
