package gofr

import (
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

func Test_WebSocket_Success(t *testing.T) {
	t.Setenv("HTTP_PORT", "8002")

	app := New()

	server := httptest.NewServer(app.httpServer.router)
	defer server.Close()

	app.WebSocket("/ws", func(ctx *Context) (interface{}, error) {
		var message string

		err := ctx.Bind(&message)
		if err != nil {
			return nil, err
		}

		response := fmt.Sprintf("Received: %s", message)

		return response, nil
	})

	go app.Run()
	time.Sleep(1 * time.Second)

	// Create a WebSocket client
	wsURL := "ws" + server.URL[len("http"):] + "/ws"

	ws, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert.NoError(t, err)

	defer ws.Close()
	defer resp.Body.Close()

	// Send a test message
	testMessage := "Hello, WebSocket!"
	err = ws.WriteMessage(websocket.TextMessage, []byte(testMessage))
	assert.NoError(t, err)

	// Read the response
	_, message, err := ws.ReadMessage()
	assert.NoError(t, err)

	expectedResponse := fmt.Sprintf("Received: %s", testMessage)
	assert.Equal(t, expectedResponse, string(message))

	// Close the client connection
	err = ws.Close()
	assert.NoError(t, err)
}
