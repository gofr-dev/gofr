package websocket

import (
	"github.com/gorilla/websocket"
)

type Connection websocket.Conn

type key int

// WebsocketKey used for retrieval of websocket connection from context
// custom type to avoid collisions
const WebsocketKey key = iota

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
