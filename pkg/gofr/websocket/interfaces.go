package websocket

import (
	"net/http"

	"github.com/gorilla/websocket"
)

// Upgrader interface for upgrading HTTP connections to WebSocket connections.
type Upgrader interface {
	Upgrade(w http.ResponseWriter, r *http.Request, responseHeader http.Header) (*websocket.Conn, error)
}
