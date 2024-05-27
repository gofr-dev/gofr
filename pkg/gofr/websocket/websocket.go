package websocket

import (
	"net/http"

	"github.com/gorilla/websocket"
)

type Connection struct {
	*websocket.Conn
}

type key int

// WSKey used for retrieval of websocket connection from context
// custom type to avoid collisions.
const WSKey key = iota

type WSUpgrader struct {
	Upgrader Upgrader
}

func NewWSUpgrader() *WSUpgrader {
	return &WSUpgrader{
		Upgrader: &websocket.Upgrader{},
	}
}

func (u *WSUpgrader) Upgrade(w http.ResponseWriter, r *http.Request, responseHeader http.Header) (*websocket.Conn, error) {
	return u.Upgrader.Upgrade(w, r, responseHeader)
}
