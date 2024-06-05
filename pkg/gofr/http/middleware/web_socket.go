package middleware

import (
	"net/http"

	gorillaWebsocket "github.com/gorilla/websocket"

	"gofr.dev/pkg/gofr/container"
)

func WSConnectionCreate(c *container.Container) func(inner http.Handler) http.Handler {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if gorillaWebsocket.IsWebSocketUpgrade(r) {
				conn, err := c.WebSocketUpgrader.Upgrade(w, r, nil)
				if err != nil {
					c.Errorf("Failed to upgrade to WebSocket: %v", err)
					http.Error(w, "Could not open WebSocket connection", http.StatusBadRequest)

					return
				}

				c.WebsocketConnection.Conn = conn

				inner.ServeHTTP(w, r)
			}
		})
	}
}
