package middleware

import (
	"context"
	"github.com/google/uuid"
	"gofr.dev/pkg/gofr/websocket"
	"net/http"

	gorillaWebsocket "github.com/gorilla/websocket"

	"gofr.dev/pkg/gofr/container"
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

				connID := generateConnectionID(r)
				c.WebSocketConnections[connID] = &websocket.Connection{Conn: conn}

				ctx := context.WithValue(r.Context(), "connID", connID)
				r = r.WithContext(ctx)
			}

			inner.ServeHTTP(w, r)
		})
	}
}

func generateConnectionID(r *http.Request) string {
	// Implement your logic to generate a unique connection ID (e.g., using a UUID or a hash)
	return uuid.New().String()

}
