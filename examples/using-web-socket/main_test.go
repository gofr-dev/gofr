package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

func Test_WebSocket_Success(t *testing.T) {
	wsURL := fmt.Sprintf("ws://%s/ws", "localhost:8001")

	go main()
	time.Sleep(time.Second * 2)

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
