package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/testutil"
)

func Test_WebSocket_Success(t *testing.T) {
	configs := testutil.NewServerConfigs(t)
	wsURL := fmt.Sprintf("ws://localhost:%d/ws", configs.HTTPPort)

	go main()
	time.Sleep(100 * time.Millisecond)

	testMessage := "Hello! GoFr"
	dialer := &websocket.Dialer{}

	conn, _, err := dialer.Dial(wsURL, nil)
	assert.Nil(t, err, "Error dialing websocket : %v", err)

	defer conn.Close()

	// writing test message to websocket connection
	err = conn.WriteMessage(websocket.TextMessage, []byte(testMessage))
	assert.Nil(t, err, "Unexpected error while writing message : %v", err)

	// Read response from server
	_, message, err := conn.ReadMessage()
	assert.Nil(t, err, "Unexpected error while reading message : %v", err)

	assert.Equal(t, string(message), testMessage, "Test_WebSocket_Success Failed!")
}
