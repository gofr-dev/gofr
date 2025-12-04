package main

import (
	"fmt"
	"time"

	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	// Register WebSocket endpoints
	app.WebSocket("/ws", handleBasicConnection)
	app.WebSocket("/chat", handleChatRoom)

	app.Run()
}

// handleBasicConnection demonstrates basic WebSocket usage and identifies gaps
func handleBasicConnection(ctx *gofr.Context) (any, error) {
	var message string

	err := ctx.Bind(&message)
	if err != nil {
		ctx.Logger.Errorf("Error binding message: %v", err)
		return nil, err
	}

	ctx.Logger.Infof("Received: %s", message)

	// Gap #1: No heartbeat/ping-pong mechanism
	// Current: Cannot set up automatic ping/pong to detect dead connections
	// Workaround: Would need manual implementation

	// Gap #3: Limited message type support
	// Current: Only TextMessage easily accessible
	// Can't easily send binary data, ping, pong, or close messages

	response := fmt.Sprintf("Echo: %s (sent at %s)", message, time.Now().Format(time.RFC3339))
	return response, nil
}

// handleChatRoom demonstrates gaps in broadcast and room functionality
func handleChatRoom(ctx *gofr.Context) (any, error) {
	var chatMessage struct {
		User    string `json:"user"`
		Message string `json:"message"`
		Room    string `json:"room"`
	}

	err := ctx.Bind(&chatMessage)
	if err != nil {
		ctx.Logger.Errorf("Error binding chat message: %v", err)
		return nil, err
	}

	// Gap #2: No broadcast functionality
	// Current: Cannot broadcast message to all/specific connections
	// Would need to manually iterate through connections
	// Example of what should be possible:
	// ctx.BroadcastToRoom(chatMessage.Room, chatMessage)
	// ctx.BroadcastToAll(chatMessage)

	// Gap #4: No connection state management
	// Current: Cannot check connection state, metadata
	// Would be useful to have:
	// - ctx.ConnectionState()
	// - ctx.ConnectionInfo() (connected time, last activity, etc.)

	// Gap #8: No middleware support
	// Current: Cannot add WebSocket-specific middleware
	// Would be useful for:
	// - Message validation
	// - Rate limiting
	// - Authentication per message

	ctx.Logger.Infof("Chat message from %s in room %s: %s", 
		chatMessage.User, chatMessage.Room, chatMessage.Message)

	return map[string]interface{}{
		"status":    "received",
		"user":      chatMessage.User,
		"message":   chatMessage.Message,
		"room":      chatMessage.Room,
		"timestamp": time.Now().Unix(),
	}, nil
}

// The gaps identified through this POC:
//
// 1. No ping/pong heartbeat mechanism for detecting dead connections
// 2. No broadcast functionality for sending to multiple clients
// 3. Limited message type support (only TextMessage constant exposed)
// 4. No connection state management or metadata tracking
// 5. Limited error handling with no WebSocket-specific error types
// 6. No WebSocket-specific middleware support
// 7. No compression support configuration
// 8. No subprotocol negotiation
// 9. No connection limits or rate limiting
// 10. Limited reconnection strategy (no exponential backoff)
// 11. No metrics/monitoring for WebSocket connections
// 12. No built-in testing utilities for WebSocket
// 13. No graceful shutdown for all connections
// 14. Limited documentation for advanced patterns
