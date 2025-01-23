package gofr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	gWebsocket "github.com/gorilla/websocket"

	"gofr.dev/pkg/gofr/websocket"
)

var ErrMarshalingResponse = errors.New("error marshaling response")

func (a *App) OverrideWebsocketUpgrader(wsUpgrader websocket.Upgrader) {
	a.httpServer.ws.WebSocketUpgrader.Upgrader = wsUpgrader
}

// WebSocket registers a handler function for a WebSocket route. This method allows you to define a route handler for
// WebSocket connections. It internally handles the WebSocket handshake and provides a `websocket.Connection` object
// within the handler context. User can access the underlying WebSocket connection using `ctx.GetWebsocketConnection()`.
func (a *App) WebSocket(route string, handler Handler) {
	a.GET(route, func(ctx *Context) (any, error) {
		connID := ctx.Request.Context().Value(websocket.WSConnectionKey).(string)

		conn := a.httpServer.ws.GetWebsocketConnection(connID)
		if conn.Conn == nil {
			return nil, websocket.ErrorConnection
		}

		ctx.Request = conn

		ctx.Context = context.WithValue(ctx, websocket.WSConnectionKey, conn)

		defer a.httpServer.ws.CloseConnection(connID)

		handleWebSocketConnection(ctx, conn, handler)

		return nil, nil
	})
}

func handleWebSocketConnection(ctx *Context, conn *websocket.Connection, handler Handler) {
	for {
		response, err := handler(ctx)
		if err != nil {
			if gWebsocket.IsCloseError(err, gWebsocket.CloseNormalClosure, gWebsocket.CloseGoingAway, gWebsocket.CloseAbnormalClosure) {
				break
			}

			ctx.Errorf("Error handling message: %v", err)
		}

		message, err := serializeMessage(response)
		if err != nil {
			ctx.Errorf("%v", err)
			continue
		}

		err = conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			ctx.Errorf("Error writing message: %v", err)
		}
	}
}

func serializeMessage(response any) ([]byte, error) {
	var (
		message []byte
		err     error
	)

	switch v := response.(type) {
	case string:
		message = []byte(v)
	case []byte:
		message = v
	default:
		message, err = json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrMarshalingResponse, err)
		}
	}

	return message, nil
}
