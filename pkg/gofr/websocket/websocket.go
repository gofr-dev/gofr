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
	Upgrader websocket.Upgrader
}

func NewWsUpgrader(opts ...Options) Upgrader {
	u := &WSUpgrader{
		Upgrader: websocket.Upgrader{},
	}

	for _, opt := range opts {
		opt(u)
	}

	return u
}

func (u *WSUpgrader) Upgrade(w http.ResponseWriter, r *http.Request, responseHeader http.Header) (*websocket.Conn, error) {
	return u.Upgrader.Upgrade(w, r, responseHeader)
}
