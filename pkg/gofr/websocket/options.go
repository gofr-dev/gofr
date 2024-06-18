package websocket

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// Options is a function type that applies a configuration to the concrete Upgrader.
type Options func(u *websocket.Upgrader)

// WithHandshakeTimeout sets the HandshakeTimeout option.
func WithHandshakeTimeout(t time.Duration) Options {
	return func(u *websocket.Upgrader) {
		u.HandshakeTimeout = t
	}
}

// WithReadBufferSize sets the ReadBufferSize option.
func WithReadBufferSize(size int) Options {
	return func(u *websocket.Upgrader) {
		u.ReadBufferSize = size
	}
}

// WithWriteBufferSize sets the WriteBufferSize option.
func WithWriteBufferSize(size int) Options {
	return func(u *websocket.Upgrader) {
		u.WriteBufferSize = size
	}
}

// WithSubprotocols sets the Subprotocols option.
func WithSubprotocols(subprotocols ...string) Options {
	return func(u *websocket.Upgrader) {
		u.Subprotocols = subprotocols
	}
}

// WithError sets the Error handler option.
func WithError(fn func(w http.ResponseWriter, r *http.Request, status int, reason error)) Options {
	return func(u *websocket.Upgrader) {
		u.Error = fn
	}
}

// WithCheckOrigin sets the CheckOrigin handler option.
func WithCheckOrigin(fn func(r *http.Request) bool) Options {
	return func(u *websocket.Upgrader) {
		u.CheckOrigin = fn
	}
}

// WithCompression enables compression.
func WithCompression() Options {
	return func(u *websocket.Upgrader) {
		u.EnableCompression = true
	}
}
