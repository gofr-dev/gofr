package websocket

import (
	"net/http"

	"github.com/gorilla/websocket"
)

type Connection struct {
	*websocket.Conn
}

type WSUpgrader struct {
	Upgrader Upgrader
}

func NewWSUpgrader(opts ...Options) *WSUpgrader {
	defaultUpgrader := &websocket.Upgrader{}
	for _, opt := range opts {
		opt(defaultUpgrader)
	}

	return &WSUpgrader{
		Upgrader: defaultUpgrader,
	}
}

func (u *WSUpgrader) Upgrade(w http.ResponseWriter, r *http.Request, responseHeader http.Header) (*websocket.Conn, error) {
	return u.Upgrader.Upgrade(w, r, responseHeader)
}
