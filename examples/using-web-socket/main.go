package main

import (
	"errors"
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	app.AddWSService("example-ws", "ws://localhost:8001/ws")

	app.GET("/ws", WSHandler)

	app.Run()
}

func WSHandler(c *gofr.Context) (any, error) {
	wsConn, err := c.GetWSService("example-ws")
	if wsConn == nil || err != nil {
		return nil, errors.New("WebSocket connection not found")
	}

	// Example of receiving a message
	_, message, err := wsConn.ReadMessage()
	if err != nil {
		return nil, err
	}

	return string(message), nil
}
