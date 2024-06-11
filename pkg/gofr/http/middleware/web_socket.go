package middleware

import (
	"context"
	"net/http"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/websocket"

	gorillaWebsocket "github.com/gorilla/websocket"
)

func WSHandlerUpgrade(c *container.Container) func(inner http.Handler) http.Handler {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if gorillaWebsocket.IsWebSocketUpgrade(r) {
				conn, err := c.WebSocketUpgrader.Upgrade(w, r, nil)
				if err != nil {
					c.Errorf("Failed to upgrade to WebSocket: %v", err)
					http.Error(w, "Could not open WebSocket connection", http.StatusBadRequest)

					return
				}

				c.WebSocketConnections[r.Header.Get("Sec-WebSocket-Key")] = &websocket.Connection{Conn: conn}

				ctx := context.WithValue(r.Context(), websocket.WSConnectionKey, r.Header.Get("Sec-WebSocket-Key"))
				r = r.WithContext(ctx)
			}

			inner.ServeHTTP(w, r)
		})
	}
}
