package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	gorillaWebsocket "github.com/gorilla/websocket"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/internal/routemark"
	"gofr.dev/pkg/gofr/websocket"
)

// WSHandlerUpgrade middleware upgrades the incoming http request to a websocket connection using websocket upgrader.
//
// Only a request that matched a route registered through App.WebSocket is upgraded. An Upgrade
// header on any other route is ignored and the request is served as plain HTTP: upgrading every
// route hijacked connections no handler ever closed, so repeated upgrade requests to an ordinary
// route exhausted file descriptors (gofr#4635).
//
// The hub key is a server-generated ID, never the client-supplied Sec-WebSocket-Key: a repeated
// or missing key used to overwrite another client's connection (or the same empty key for all of
// them) — also gofr#4635.
func WSHandlerUpgrade(c *container.Container, wsManager *websocket.Manager) func(inner http.Handler) http.Handler {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if gorillaWebsocket.IsWebSocketUpgrade(r) && routemark.IsWebSocket(routemark.Matched(r)) {
				conn, err := wsManager.WebSocketUpgrader.Upgrade(w, r, nil)
				if err != nil {
					c.Errorf("Failed to upgrade to WebSocket: %v", err)
					http.Error(w, "Could not open WebSocket connection", http.StatusBadRequest)

					return
				}

				connID := uuid.New().String()

				// Add the connection to the hub
				wsManager.AddWebsocketConnection(connID, &websocket.Connection{Conn: conn})

				// Store the websocket connection ID in the context
				ctx := context.WithValue(r.Context(), websocket.WSConnectionKey, connID)
				r = r.WithContext(ctx)
			}

			inner.ServeHTTP(w, r)
		})
	}
}
