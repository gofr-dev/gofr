package websocket

import (
	"github.com/gorilla/websocket"
)

type wsUpgrader struct {
	websocket.Upgrader
}

func NewWsUpgrader(opts ...WebSocketOptions) Upgrader {
	u := &wsUpgrader{
		Upgrader: websocket.Upgrader{},
	}

	for _, opt := range opts {
		opt(u)
	}

	return u
}
