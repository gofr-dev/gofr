package websocket

import (
	"net/http"
	"time"
)

type WebSocketOptions func(u *wsUpgrader)

func WithHandshakeTimeout(t time.Duration) WebSocketOptions {
	return func(u *wsUpgrader) {
		u.HandshakeTimeout = t
	}
}

func WithReadBufferSize(size int) WebSocketOptions {
	return func(u *wsUpgrader) {
		u.ReadBufferSize = size
	}
}

func WithWriteBufferSize(size int) WebSocketOptions {
	return func(u *wsUpgrader) {
		u.WriteBufferSize = size
	}
}

func WithSubprotocols(subprotocols ...string) WebSocketOptions {
	return func(u *wsUpgrader) {
		u.Subprotocols = subprotocols
	}
}

func WithError(fn func(w http.ResponseWriter, r *http.Request, status int, reason error)) WebSocketOptions {
	return func(u *wsUpgrader) {
		u.Error = fn
	}
}

func WithCheckOrigin(fn func(r *http.Request) bool) WebSocketOptions {
	return func(u *wsUpgrader) {
		u.CheckOrigin = fn
	}
}

func WithCompression() WebSocketOptions {
	return func(u *wsUpgrader) {
		u.EnableCompression = true
	}
}
