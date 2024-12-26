package main

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"gofr.dev/pkg/gofr/testutil"
)

func Test_WebSocket_Success(t *testing.T) {
	port := testutil.GetFreePort(t)
	t.Setenv("HTTP_PORT", strconv.Itoa(port))
	wsURL := fmt.Sprintf("ws://localhost:%d/ws", port)

	metricsPort := testutil.GetFreePort(t)
	t.Setenv("METRICS_PORT", strconv.Itoa(metricsPort))

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
