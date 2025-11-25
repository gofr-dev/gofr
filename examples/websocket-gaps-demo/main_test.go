package main

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/testutil"
)

func TestWebSocketGaps(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := gofr.New()
	chatRoom := NewChatRoom()
	go chatRoom.run()

	// Register all handlers
	app.WebSocket("/ws/basic", BasicWSHandler)
	app.WebSocket("/ws/chat", func(ctx *gofr.Context) (any, error) {
		return ChatHandler(ctx, chatRoom)
	})
	app.WebSocket("/ws/binary", BinaryWSHandler)
	app.WebSocket("/ws/heartbeat", HeartbeatWSHandler)
	app.WebSocket("/ws/auth", AuthenticatedWSHandler)
	app.GET("/ws/metrics", MetricsHandler)

	server := httptest.NewServer(app.httpServer.router)
	defer server.Close()

	go app.Run()
	time.Sleep(100 * time.Millisecond)

	t.Run("Basic WebSocket Functionality Works", func(t *testing.T) {
		wsURL := "ws" + server.URL[len("http"):] + "/ws/basic"
		ws, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err)
		defer ws.Close()
		defer resp.Body.Close()

		// Send test message
		testMessage := "Hello WebSocket"
		err = ws.WriteMessage(websocket.TextMessage, []byte(testMessage))
		require.NoError(t, err)

		// Read response
		_, message, err := ws.ReadMessage()
		require.NoError(t, err)

		var response Message
		err = json.Unmarshal(message, &response)
		require.NoError(t, err)

		assert.Equal(t, "response", response.Type)
		assert.Contains(t, response.Content, "Echo: Hello WebSocket")
	})

	t.Run("Broadcasting Gap - Multiple Connections Don't Communicate", func(t *testing.T) {
		wsURL := "ws" + server.URL[len("http"):] + "/ws/chat"

		// Connect first client
		ws1, resp1, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err)
		defer ws1.Close()
		defer resp1.Body.Close()

		// Connect second client
		ws2, resp2, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err)
		defer ws2.Close()
		defer resp2.Body.Close()

		// Send message from client 1
		chatMessage := Message{
			Type:    "chat",
			Content: "Hello from client 1",
			UserID:  "user1",
		}
		messageBytes, _ := json.Marshal(chatMessage)
		err = ws1.WriteMessage(websocket.TextMessage, messageBytes)
		require.NoError(t, err)

		// Read response from client 1
		_, response1, err := ws1.ReadMessage()
		require.NoError(t, err)

		var resp1Data map[string]interface{}
		err = json.Unmarshal(response1, &resp1Data)
		require.NoError(t, err)

		// Verify the gap is documented
		assert.Equal(t, "message_received", resp1Data["status"])
		assert.Contains(t, resp1Data["gap"], "Cannot broadcast to other clients")

		// Client 2 should NOT receive the message (demonstrating the gap)
		ws2.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		_, _, err = ws2.ReadMessage()
		assert.Error(t, err) // Should timeout because no broadcast mechanism exists
	})

	t.Run("Binary Message Gap", func(t *testing.T) {
		wsURL := "ws" + server.URL[len("http"):] + "/ws/binary"
		ws, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err)
		defer ws.Close()
		defer resp.Body.Close()

		// Try to send binary data (will be treated as text)
		binaryData := []byte{0x00, 0x01, 0x02, 0x03}
		err = ws.WriteMessage(websocket.TextMessage, binaryData)
		require.NoError(t, err)

		_, message, err := ws.ReadMessage()
		require.NoError(t, err)

		var response map[string]interface{}
		err = json.Unmarshal(message, &response)
		require.NoError(t, err)

		assert.Equal(t, "binary_not_supported", response["status"])
		assert.Contains(t, response["gap"], "only supports text messages")
	})

	t.Run("Heartbeat Gap", func(t *testing.T) {
		wsURL := "ws" + server.URL[len("http"):] + "/ws/heartbeat"
		ws, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err)
		defer ws.Close()
		defer resp.Body.Close()

		// Send ping message
		err = ws.WriteMessage(websocket.TextMessage, []byte("ping"))
		require.NoError(t, err)

		_, message, err := ws.ReadMessage()
		require.NoError(t, err)

		var response map[string]interface{}
		err = json.Unmarshal(message, &response)
		require.NoError(t, err)

		assert.Equal(t, "pong", response["type"])
		assert.Contains(t, response["gap"], "No automatic ping/pong handling")
	})

	t.Run("Authentication Gap", func(t *testing.T) {
		wsURL := "ws" + server.URL[len("http"):] + "/ws/auth"
		ws, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err)
		defer ws.Close()
		defer resp.Body.Close()

		// Send message without token
		message := map[string]interface{}{
			"message": "hello",
		}
		messageBytes, _ := json.Marshal(message)
		err = ws.WriteMessage(websocket.TextMessage, messageBytes)
		require.NoError(t, err)

		_, response, err := ws.ReadMessage()
		require.NoError(t, err)

		var respData map[string]interface{}
		err = json.Unmarshal(response, &respData)
		require.NoError(t, err)

		assert.Equal(t, "authentication_required", respData["error"])
		assert.Contains(t, respData["gap"], "No WebSocket authentication middleware")

		// Send message with invalid token
		messageWithToken := map[string]interface{}{
			"message": "hello",
			"token":   "invalid_token",
		}
		messageBytes, _ = json.Marshal(messageWithToken)
		err = ws.WriteMessage(websocket.TextMessage, messageBytes)
		require.NoError(t, err)

		_, response, err = ws.ReadMessage()
		require.NoError(t, err)

		err = json.Unmarshal(response, &respData)
		require.NoError(t, err)

		assert.Equal(t, "invalid_token", respData["error"])
	})

	t.Run("Metrics Gap", func(t *testing.T) {
		// Test the metrics endpoint
		resp, err := http.Get(server.URL + "/ws/metrics")
		require.NoError(t, err)
		defer resp.Body.Close()

		var metricsResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&metricsResp)
		require.NoError(t, err)

		assert.Contains(t, metricsResp["gap"], "No WebSocket metrics available")
		
		missingMetrics, ok := metricsResp["missing_metrics"].([]interface{})
		require.True(t, ok)
		
		expectedMetrics := []string{
			"active_connections_count",
			"total_messages_sent",
			"total_messages_received",
			"connection_duration",
			"error_count",
			"bytes_transferred",
		}
		
		for _, expected := range expectedMetrics {
			found := false
			for _, metric := range missingMetrics {
				if metric.(string) == expected {
					found = true
					break
				}
			}
			assert.True(t, found, fmt.Sprintf("Missing metric %s not documented", expected))
		}
	})
}

func TestChatRoomGaps(t *testing.T) {
	chatRoom := NewChatRoom()
	
	// Test that the chat room demonstrates the gaps
	t.Run("Chat Room Cannot Manage Connections", func(t *testing.T) {
		// The chat room exists but cannot actually manage connections
		// because there's no way to get connection references from GoFr
		assert.NotNil(t, chatRoom.clients)
		assert.NotNil(t, chatRoom.broadcast)
		assert.NotNil(t, chatRoom.register)
		assert.NotNil(t, chatRoom.unregister)
		
		// These channels exist but cannot be used effectively
		// due to gaps in the WebSocket implementation
	})
}