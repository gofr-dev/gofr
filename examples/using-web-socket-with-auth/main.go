package main

import (
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/container"
)

// Message represents a chat message
type Message struct {
	Username string    `json:"username"`
	Content  string    `json:"content"`
	Time     time.Time `json:"time"`
}

// ActiveUsers keeps track of connected users
var (
	activeUsers = make(map[string]bool)
	usersMutex  sync.RWMutex
)

// validateCredentials is a custom validator function for basic auth
// In a real application, you would validate against a database
func validateCredentials(_ *container.Container, username, password string) bool {
	validUsers := map[string]string{
		"user1": "password1",
		"user2": "password2",
		"admin": "admin123",
	}

	storedPassword, exists := validUsers[username]
	return exists && storedPassword == password
}

// extractUsernameFromAuthHeader extracts the username from the Authorization header
func extractUsernameFromAuthHeader(authHeader string) string {
	if authHeader == "" || !strings.HasPrefix(authHeader, "Basic ") {
		return ""
	}

	// Remove "Basic " prefix
	encodedCreds := strings.TrimPrefix(authHeader, "Basic ")

	// Decode base64
	decodedCreds, err := base64.StdEncoding.DecodeString(encodedCreds)
	if err != nil {
		return ""
	}

	// Split username:password
	creds := strings.SplitN(string(decodedCreds), ":", 2)
	if len(creds) != 2 {
		return ""
	}

	return creds[0]
}

func main() {
	app := gofr.New()

	// Enable Basic Authentication with custom validator
	app.EnableBasicAuthWithValidator(validateCredentials)

	// Register WebSocket handler with authentication
	app.WebSocket("/ws", WSHandler)

	// Add a simple HTTP endpoint to list active users
	app.GET("/users", listActiveUsers)

	app.Run()
}

// listActiveUsers returns a list of currently connected users
func listActiveUsers(ctx *gofr.Context) (any, error) {
	usersMutex.RLock()
	defer usersMutex.RUnlock()

	users := make([]string, 0, len(activeUsers))
	for user := range activeUsers {
		users = append(users, user)
	}

	// Return a simple response with the active users
	return struct {
		ActiveUsers []string `json:"active_users"`
		Count       int      `json:"count"`
	}{
		ActiveUsers: users,
		Count:       len(users),
	}, nil
}

// WSHandler handles WebSocket connections
// Since authentication middleware is applied at the HTTP level before upgrading to WebSocket,
// only authenticated users will reach this handler
func WSHandler(ctx *gofr.Context) (any, error) {
	// Get username from the authentication info
	// The username is set by the basic auth middleware
	username := ctx.GetAuthInfo().GetUsername()
	if username == "" {
		username = "anonymous" // Fallback, though this shouldn't happen due to auth middleware
	}

	// Add user to active users
	usersMutex.Lock()
	activeUsers[username] = true
	usersMutex.Unlock()

	// Remove user when connection closes
	defer func() {
		usersMutex.Lock()
		delete(activeUsers, username)
		usersMutex.Unlock()

		ctx.Logger.Infof("User %s disconnected", username)
	}()

	ctx.Logger.Infof("User %s connected", username)

	// Send welcome message
	welcomeMsg := fmt.Sprintf("Welcome, %s! You are now connected to the chat.", username)
	err := ctx.WriteMessageToSocket(welcomeMsg)
	if err != nil {
		return nil, err
	}

	// Handle incoming messages
	for {
		var message Message

		// Bind the incoming message
		err := ctx.Bind(&message)
		if err != nil {
			// If there's an error binding, the connection might be closed
			ctx.Logger.Errorf("Error binding message: %v", err)
			return nil, err
		}

		// Set the username and timestamp
		message.Username = username
		message.Time = time.Now()

		ctx.Logger.Infof("Received message from %s: %s", message.Username, message.Content)

		// Echo the message back to the client
		response := fmt.Sprintf("[%s] %s: %s",
			message.Time.Format("15:04:05"),
			message.Username,
			message.Content)

		err = ctx.WriteMessageToSocket(response)
		if err != nil {
			return nil, err
		}
	}
}
