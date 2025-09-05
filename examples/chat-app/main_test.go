package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/testutil"
)

func TestMain(m *testing.M) {
	// Set up test environment
	m.Run()
}

func TestChatApp_WebSocketConnection(t *testing.T) {
	configs := testutil.NewServerConfigs(t)
	wsURL := fmt.Sprintf("ws://localhost:%d/ws", configs.HTTPPort)
	
	// Start the chat room
	go chatRoom.Run()
	
	// Start the app
	go main()
	time.Sleep(200 * time.Millisecond)
	
	// Test WebSocket connection
	ws, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer ws.Close()
	defer resp.Body.Close()
	
	// Wait for connection to be established
	time.Sleep(100 * time.Millisecond)
	
	// Verify client count increased
	assert.Greater(t, chatRoom.GetClientCount(), 0)
	
	// Read the welcome message first
	_, welcomeBytes, err := ws.ReadMessage()
	require.NoError(t, err)
	
	var welcomeMsg Message
	err = json.Unmarshal(welcomeBytes, &welcomeMsg)
	require.NoError(t, err)
	
	// Verify it's a welcome message
	assert.Equal(t, "System", welcomeMsg.Username)
	assert.Equal(t, "Welcome to GoFr Chat!", welcomeMsg.Content)
	assert.Equal(t, "join", welcomeMsg.Type)
	
	// Send a test message
	testMessage := Message{
		Username: "testuser",
		Content:  "Hello, World!",
	}
	
	messageBytes, err := json.Marshal(testMessage)
	require.NoError(t, err)
	
	err = ws.WriteMessage(websocket.TextMessage, messageBytes)
	require.NoError(t, err)
	
	// Read the echo response
	_, responseBytes, err := ws.ReadMessage()
	require.NoError(t, err)
	
	var response Message
	err = json.Unmarshal(responseBytes, &response)
	require.NoError(t, err)
	
	assert.Equal(t, testMessage.Username, response.Username)
	assert.Equal(t, testMessage.Content, response.Content)
	assert.NotEmpty(t, response.ID)
	assert.NotZero(t, response.Timestamp)
}

func TestChatApp_HTTPEndpoints(t *testing.T) {
	configs := testutil.NewServerConfigs(t)
	
	// Start the app
	go main()
	time.Sleep(200 * time.Millisecond)
	
	// Test status endpoint
	statusURL := fmt.Sprintf("http://localhost:%d/api/status", configs.HTTPPort)
	resp, err := http.Get(statusURL)
	require.NoError(t, err)
	defer resp.Body.Close()
	
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	
	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)
	
	// GoFr wraps responses in a "data" field
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "Response should contain 'data' field")
	
	assert.Equal(t, "online", data["status"])
	assert.Contains(t, data, "clients")
	assert.Contains(t, data, "uptime")
	
	// Test clients endpoint
	clientsURL := fmt.Sprintf("http://localhost:%d/api/clients", configs.HTTPPort)
	resp, err = http.Get(clientsURL)
	require.NoError(t, err)
	defer resp.Body.Close()
	
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	
	var clientsResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&clientsResponse)
	require.NoError(t, err)
	
	// GoFr wraps responses in a "data" field
	clientsData, ok := clientsResponse["data"].(map[string]interface{})
	require.True(t, ok, "Response should contain 'data' field")
	
	assert.Contains(t, clientsData, "total_clients")
	assert.Contains(t, clientsData, "timestamp")
}