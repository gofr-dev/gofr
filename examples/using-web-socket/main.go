package main

import (
	"fmt"
	gorillaSocket "github.com/gorilla/websocket"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/websocket"
)

func main() {
	app := gofr.New()

	app.GET("/ws", WSHandler)

	app.Run()
}

func WSHandler(c *gofr.Context) (interface{}, error) {
	conn := c.GetWebSocketConnection()
	if conn == nil {
		return nil, fmt.Errorf("websocket connection not found in context")
	}

	handleWebSocketMessages(&websocket.Connection{Conn: conn}, c.Logger)

	return nil, nil
}

func handleWebSocketMessages(conn *websocket.Connection, logger logging.Logger) {
	defer conn.Close()
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			if gorillaSocket.IsUnexpectedCloseError(err, gorillaSocket.CloseGoingAway, gorillaSocket.CloseAbnormalClosure) {
				logger.Errorf("Unexpected close error: %v", err)
			}
			break
		}

		logger.Infof("Received message: %s", msg)

		// Echo the message back
		err = conn.WriteMessage(gorillaSocket.TextMessage, append(msg, []byte("from gofr")...))
		if err != nil {
			logger.Errorf("Error writing message: %v", err)
			break
		}
	}
}
