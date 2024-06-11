package gofr

import (
	"fmt"

	gWebsocket "github.com/gorilla/websocket"

	"gofr.dev/pkg/gofr/websocket"
)

func (a *App) OverrideWebsocketUpgrader(wsUpgrader websocket.Upgrader) {
	a.httpServer.ws.WebSocketUpgrader.Upgrader = wsUpgrader
}

// WebSocket registers a handler function for a WebSocket route. This method allows you to define a route handler for
// WebSocket connections. It internally handles the WebSocket handshake and provides a `websocket.Connection` object
// within the handler context. User can access the underlying WebSocket connection using `ctx.GetWebsocketConnection()`.
func (a *App) WebSocket(route string, handler Handler) {
	a.GET(route, func(ctx *Context) (interface{}, error) {
		connID := ctx.Request.Context().Value(websocket.WSConnectionKey).(string)

		conn := a.httpServer.ws.GetWebsocketConnection(connID)
		if conn.Conn == nil {
			return nil, websocket.ErrorConnection
		}

		ctx.Request = conn

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

		err = conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprint(response)))
		if err != nil {
			ctx.Errorf("Error writing message: %v", err)
		}
	}
}
