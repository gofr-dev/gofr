package gofr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	gWebsocket "github.com/gorilla/websocket"

	"gofr.dev/pkg/gofr/websocket"
)

var (
	ErrMarshalingResponse = errors.New("error marshaling response")
	ErrConnectionNotFound = errors.New("connection not found for service")
)

func (a *App) OverrideWebsocketUpgrader(wsUpgrader websocket.Upgrader) {
	a.httpServer.ws.WebSocketUpgrader.Upgrader = wsUpgrader
}

// WebSocket registers a handler function for a WebSocket route. This method allows you to define a route handler for
// WebSocket connections. It internally handles the WebSocket handshake and provides a `websocket.Connection` object
// within the handler context. User can access the underlying WebSocket connection using `ctx.GetWebsocketConnection()`.
func (a *App) WebSocket(route string, handler Handler) {
	a.GET(route, func(ctx *Context) (any, error) {
		connID := ctx.Request.Context().Value(websocket.WSConnectionKey).(string)

		conn := a.httpServer.ws.GetWebsocketConnection(connID)
		if conn.Conn == nil {
			return nil, websocket.ErrorConnection
		}

		ctx.Request = conn

		ctx.Context = context.WithValue(ctx, websocket.WSConnectionKey, conn)

		defer a.httpServer.ws.CloseConnection(connID)

		handleWebSocketConnection(ctx, conn, handler)

		return nil, nil
	})
}

// AddWSService registers a WebSocket service, establishes a persistent connection, and optionally handles reconnection.
func (a *App) AddWSService(serviceName, url string, headers http.Header, enableReconnection bool, retryInterval time.Duration) error {
	conn, resp, err := gWebsocket.DefaultDialer.Dial(url, headers)
	if resp != nil {
		resp.Body.Close()
	}

	if err != nil {
		a.Logger().Errorf("Failed to establish WebSocket connection to %s: %v", url, err)

		if enableReconnection {
			a.handleReconnection(serviceName, url, headers, retryInterval)

			return nil
		}

		return err
	}

	a.container.AddConnection(serviceName, &websocket.Connection{Conn: conn})

	a.Logger().Infof("Successfully connected to WebSocket service: %s", serviceName)

	return nil
}

// WriteMessageToService writes a message to a WebSocket service connection.
// This is used for inter-service WebSocket communication.
func (a *App) WriteMessageToService(serviceName string, data any) error {
	conn := a.container.GetWSConnectionByServiceName(serviceName)
	if conn == nil {
		return fmt.Errorf("%w: %s", ErrConnectionNotFound, serviceName)
	}

	message, err := serializeMessage(data)
	if err != nil {
		return err
	}

	return conn.WriteMessage(websocket.TextMessage, message)
}

func (a *App) handleReconnection(serviceName, url string, headers http.Header, retryInterval time.Duration) {
	go func() {
		for {
			conn, resp, err := gWebsocket.DefaultDialer.Dial(url, headers)
			if resp != nil {
				resp.Body.Close()
			}

			if err == nil {
				a.Logger().Infof("Successfully connected to WebSocket service: %s", serviceName)

				a.container.AddConnection(serviceName, &websocket.Connection{Conn: conn})

				return
			}

			time.Sleep(retryInterval)

			a.Logger().Debugf("Reconnecting to WebSocket service: %s. Retry interval: %v", url, retryInterval)
		}
	}()
}

func handleWebSocketConnection(ctx *Context, conn *websocket.Connection, handler Handler) {
	for {
		response, err := handler(ctx)
		if handleWebSocketError(ctx, "error handling message", err) {
			break
		}

		message, err := serializeMessage(response)
		if handleWebSocketError(ctx, "failed to serialize message", err) {
			continue
		}

		err = conn.WriteMessage(websocket.TextMessage, message)
		if handleWebSocketError(ctx, "failed to write response to websocket", err) {
			break
		}
	}
}

func handleWebSocketError(ctx *Context, msg string, err error) bool {
	if err == nil {
		return false
	}

	ctx.Errorf("%s: %v", msg, err)

	// Check if the error is a WebSocket close error or if the underlying TCP connection is closed.
	// This prevents unnecessary retries and avoids an infinite loop of read/write operations on the WebSocket.
	return gWebsocket.IsCloseError(err, gWebsocket.CloseNormalClosure, gWebsocket.CloseGoingAway,
		gWebsocket.CloseAbnormalClosure) || errors.Is(err, net.ErrClosed) ||
		strings.Contains(err.Error(), "broken pipe") ||
		strings.Contains(err.Error(), "connection reset by peer")
}

func serializeMessage(response any) ([]byte, error) {
	var (
		message []byte
		err     error
	)

	switch v := response.(type) {
	case string:
		message = []byte(v)
	case []byte:
		message = v
	default:
		message, err = json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrMarshalingResponse, err)
		}
	}

	return message, nil
}
