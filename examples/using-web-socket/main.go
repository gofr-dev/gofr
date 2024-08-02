package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	app.WebSocket("/ws", WSHandler)

	app.Run()
}

func WSHandler(ctx *gofr.Context) (interface{}, error) {
	var message string

	ctx.WriteMessageToSocket(websocket.TextMessage, []byte(fmt.Sprint("anc")))

	err := ctx.Bind(&message)
	if err != nil {
		ctx.Logger.Errorf("Error binding message: %v", err)
		return nil, err
	}

	ctx.Logger.Infof("Received message: %s", message)

	return message, nil
}
