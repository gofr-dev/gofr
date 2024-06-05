package websocket

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gorilla/websocket"
)

type Connection struct {
	*websocket.Conn
}

// The message types are defined in RFC 6455, section 11.8.
const (
	// TextMessage denotes a text data message. The text message payload is
	// interpreted as UTF-8 encoded text data.
	TextMessage = 1

	// BinaryMessage denotes a binary data message.
	BinaryMessage = 2

	// CloseMessage denotes a close control message. The optional message
	// payload contains a numeric code and text. Use the FormatCloseMessage
	// function to format a close message payload.
	CloseMessage = 8

	// PingMessage denotes a ping control message. The optional message payload
	// is UTF-8 encoded text.
	PingMessage = 9

	// PongMessage denotes a pong control message. The optional message payload
	// is UTF-8 encoded text.
	PongMessage = 10
)

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

func (w *Connection) Context() context.Context {
	return context.TODO() // Implement proper context handling if needed
}

func (w *Connection) Param(_ string) string {
	return "" // Not applicable for WebSocket, can be implemented if needed
}

func (w *Connection) PathParam(_ string) string {
	return "" // Not applicable for WebSocket, can be implemented if needed
}

func (w *Connection) Bind(v interface{}) error {
	_, message, err := w.Conn.ReadMessage()
	if err != nil {
		return err
	}

	switch v := v.(type) {
	case *string:
		*v = string(message)
	default:
		return json.Unmarshal(message, v)
	}

	return nil
}

func (w *Connection) HostName() string {
	return "" // Not applicable for WebSocket, can be implemented if needed
}

func (u *WSUpgrader) Upgrade(w http.ResponseWriter, r *http.Request, responseHeader http.Header) (*websocket.Conn, error) {
	return u.Upgrader.Upgrade(w, r, responseHeader)
}
