package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// WSKey defines the key type for WSConnectionKey.
type WSKey string

// WSConnectionKey is a key constant that stores the connection id in the request context.
const WSConnectionKey WSKey = "ws-connection-key"

// Connection is a wrapper for gorilla websocket connection.
type Connection struct {
	*websocket.Conn

	// Mutex to prevent race conditions on write operations
	writeMutex sync.Mutex
}

// ErrorConnection is the connection error that occurs when websocket connection cannot be established.
var ErrorConnection = errors.New("couldn't establish connection to web socket")

// The message types are defined in RFC 6455, section 11.8.
const (
	// TextMessage denotes a text data message. The text message payload is
	// interpreted as UTF-8 encoded text data.
	TextMessage = 1
)

type WSUpgrader struct {
	Upgrader Upgrader
}

// NewWSUpgrader initialize a new websocket upgrader that upgrades an incoming http request
// to a websocket connection. It takes in Options that can be used to customize the upgraded connections.
func NewWSUpgrader(opts ...Options) *WSUpgrader {
	defaultUpgrader := &websocket.Upgrader{}
	for _, opt := range opts {
		opt(defaultUpgrader)
	}

	return &WSUpgrader{
		Upgrader: defaultUpgrader,
	}
}

func (*Connection) Context() context.Context {
	return context.TODO() // Implement proper context handling if needed
}

func (*Connection) Param(_ string) string {
	return "" // Not applicable for WebSocket, can be implemented if needed
}

func (*Connection) PathParam(_ string) string {
	return "" // Not applicable for WebSocket, can be implemented if needed
}

func (w *Connection) Bind(v any) error {
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

// WriteMessage writes the data on the underlying ws connection.
//
// This method is thread-safe and be called concurrently with WriteJSON.
func (w *Connection) WriteMessage(messageType int, data []byte) error {
	w.writeMutex.Lock()
	defer w.writeMutex.Unlock()

	return w.Conn.WriteMessage(messageType, data)
}

func (*Connection) HostName() string {
	return "" // Not applicable for WebSocket, can be implemented if needed
}

// Manager is a websocket manager that handles the upgrader and manages all
// active connections through ConnectionHub.
type Manager struct {
	ConnectionHub
	WebSocketUpgrader *WSUpgrader
}

// ConnectionHub stores and provide functionality to work with
// all active connections with websocket clients.
type ConnectionHub struct {
	mu                   sync.RWMutex
	WebSocketConnections map[string]*Connection
}

// New initializes a new websocket manager with default websocket upgrader.
func New() *Manager {
	return &Manager{
		WebSocketUpgrader: NewWSUpgrader(),
		ConnectionHub: ConnectionHub{
			mu:                   sync.RWMutex{},
			WebSocketConnections: make(map[string]*Connection),
		},
	}
}

// Upgrade calls the upgrader to upgrade an http connection to a websocket connection.
func (u *WSUpgrader) Upgrade(w http.ResponseWriter, r *http.Request,
	responseHeader http.Header) (*websocket.Conn, error) {
	return u.Upgrader.Upgrade(w, r, responseHeader)
}

// GetWebsocketConnection returns a websocket connection which has been initialized in the middleware.
func (ws *Manager) GetWebsocketConnection(connID string) *Connection {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	return ws.WebSocketConnections[connID]
}

// ListConnections returns a list of all active WebSocket connection IDs.
func (ws *Manager) ListConnections() []string {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	connections := make([]string, 0, len(ws.WebSocketConnections))
	for connID := range ws.WebSocketConnections {
		connections = append(connections, connID)
	}

	return connections
}

// GetConnectionByServiceName retrieves a WebSocket connection by its service name.
func (ws *Manager) GetConnectionByServiceName(serviceName string) *Connection {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	return ws.WebSocketConnections[serviceName]
}

// AddWebsocketConnection add a new connection with the connection id key.
func (ws *Manager) AddWebsocketConnection(connID string, conn *Connection) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	ws.WebSocketConnections[connID] = conn
}

// CloseConnection closes a websocket connection and then removes it from the connection hub.
func (ws *Manager) CloseConnection(connID string) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if conn, ok := ws.WebSocketConnections[connID]; ok {
		if conn.Conn != nil {
			conn.Close()
		}

		delete(ws.WebSocketConnections, connID)
	}
}

func (*Connection) Params(string) []string {
	return nil
}
