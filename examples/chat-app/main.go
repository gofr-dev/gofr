package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"gofr.dev/pkg/gofr"
)

// Message represents a chat message
type Message struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"` // "message", "join", "leave"
}

// ChatRoom manages active connections and message broadcasting
type ChatRoom struct {
	clients    map[string]*gofr.Context
	broadcast  chan Message
	register   chan string
	unregister chan string
	mutex      sync.RWMutex
}

// NewChatRoom creates a new chat room
func NewChatRoom() *ChatRoom {
	return &ChatRoom{
		clients:    make(map[string]*gofr.Context),
		broadcast:  make(chan Message),
		register:   make(chan string),
		unregister: make(chan string),
	}
}

// Run starts the chat room message handling
func (cr *ChatRoom) Run() {
	for {
		select {
		case clientID := <-cr.register:
			cr.mutex.Lock()
			// We'll store the context when the client connects
			_ = clientID // Avoid unused variable warning
			cr.mutex.Unlock()

			// Send welcome message
			welcomeMsg := Message{
				ID:        fmt.Sprintf("msg_%d", time.Now().UnixNano()),
				Username:  "System",
				Content:   "Welcome to GoFr Chat!",
				Timestamp: time.Now(),
				Type:      "join",
			}
			cr.broadcast <- welcomeMsg

		case clientID := <-cr.unregister:
			cr.mutex.Lock()
			delete(cr.clients, clientID)
			cr.mutex.Unlock()

			// Send leave message
			leaveMsg := Message{
				ID:        fmt.Sprintf("msg_%d", time.Now().UnixNano()),
				Username:  "System",
				Content:   "A user left the chat",
				Timestamp: time.Now(),
				Type:      "leave",
			}
			cr.broadcast <- leaveMsg

		case message := <-cr.broadcast:
			cr.mutex.RLock()
			for clientID, client := range cr.clients {
				// Send message to client via WebSocket
				messageBytes, err := json.Marshal(message)
				if err != nil {
					client.Logger.Errorf("Error marshaling message: %v", err)
					continue
				}

				// Use the context's WriteMessageToSocket method
				err = client.WriteMessageToSocket(string(messageBytes))
				if err != nil {
					client.Logger.Errorf("Error sending message to client: %v", err)
					// Remove client if we can't send to them
					cr.mutex.RUnlock()
					cr.mutex.Lock()
					delete(cr.clients, clientID)
					cr.mutex.Unlock()
					cr.mutex.RLock()
				}
			}
			cr.mutex.RUnlock()
		}
	}
}

// BroadcastMessage sends a message to all connected clients
func (cr *ChatRoom) BroadcastMessage(msg Message) {
	cr.broadcast <- msg
}

// GetClientCount returns the number of connected clients
func (cr *ChatRoom) GetClientCount() int {
	cr.mutex.RLock()
	defer cr.mutex.RUnlock()
	return len(cr.clients)
}

var chatRoom = NewChatRoom()

func main() {
	app := gofr.New()

	// Start the chat room message handler
	go chatRoom.Run()

	// WebSocket handler for chat
	app.WebSocket("/ws", chatHandler)

	// HTTP endpoints for chat info
	app.GET("/api/status", statusHandler)
	app.GET("/api/clients", clientsHandler)

	// Serve static files for the frontend
	app.AddStaticFiles("/", "./static")

	// Start the server
	app.Run()
}

// chatHandler handles WebSocket connections for chat
func chatHandler(ctx *gofr.Context) (any, error) {
	// Generate a unique client ID
	clientID := fmt.Sprintf("client_%d", time.Now().UnixNano())

	// Register the client
	chatRoom.mutex.Lock()
	chatRoom.clients[clientID] = ctx
	chatRoom.mutex.Unlock()

	// Send registration event
	chatRoom.register <- clientID

	// Ensure client is unregistered when connection closes
	defer func() {
		chatRoom.unregister <- clientID
	}()

	// Handle incoming messages
	for {
		var message Message
		err := ctx.Bind(&message)
		if err != nil {
			ctx.Logger.Errorf("Error binding message: %v", err)
			return nil, err
		}

		// Set message metadata
		message.ID = fmt.Sprintf("msg_%d", time.Now().UnixNano())
		message.Timestamp = time.Now()
		message.Type = "message"

		// Log the message
		ctx.Logger.Infof("Received message from %s: %s", message.Username, message.Content)

		// Broadcast to all clients
		chatRoom.BroadcastMessage(message)

		// Echo back to sender (optional)
		return message, nil
	}
}

// statusHandler returns chat room status
func statusHandler(ctx *gofr.Context) (any, error) {
	return map[string]interface{}{
		"status":        "online",
		"clients":       chatRoom.GetClientCount(),
		"uptime":        time.Since(time.Now()).String(),
		"message_types": []string{"message", "join", "leave"},
	}, nil
}

// clientsHandler returns connected clients info
func clientsHandler(ctx *gofr.Context) (any, error) {
	return map[string]interface{}{
		"total_clients": chatRoom.GetClientCount(),
		"timestamp":     time.Now(),
	}, nil
}
