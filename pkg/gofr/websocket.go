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
		connID, ok := ctx.Request.Context().Value(websocket.WSConnectionKey).(string)
		if !ok {
			// The request never went through the WebSocket upgrade middleware
			// (e.g. a plain HTTP GET with no Upgrade headers), so no
			// WSConnectionKey was ever set on the context.
			return nil, websocket.ErrorNotWebSocketUpgrade{}
		}

		conn := a.httpServer.ws.GetWebsocketConnection(connID)
		if conn == nil || conn.Conn == nil {
			return nil, websocket.ErrorConnection
		}

		ctx.Request = conn

		// The parent must be ctx.Context (the embedded context.Context), not ctx
		// itself: ctx is a *Context, so passing ctx here would make the value
		// context's parent be ctx, whose .Context field is the very value context
		// being constructed. Any walk of the parent chain other than the lucky
		// first lookup (e.g. .Done(), or a .Value() for any other key) would then
		// recurse into itself forever -- an unrecoverable stack overflow, since
		// recover() cannot catch a fatal error.
		ctx.Context = context.WithValue(ctx.Context, websocket.WSConnectionKey, conn)

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

// maxFailuresWithoutRead is how many consecutive failed passes a connection tolerates when the handler read no
// message in between. A handler that fails before it reads (an auth guard, say), or returns a value that cannot
// be serialized, would otherwise repeat that pass forever. The second pass exists because a message read directly
// from the embedded connection is invisible here: retrying lets such a handler block on its next read instead of
// losing the connection.
const maxFailuresWithoutRead = 2

func handleWebSocketConnection(ctx *Context, conn *websocket.Connection, handler Handler) {
	failuresWithoutRead := 0

	for {
		reads := conn.ReadCount()

		stop, failed := handleWebSocketPass(ctx, conn, handler)
		if stop {
			break
		}

		if !failed || conn.ReadCount() != reads {
			failuresWithoutRead = 0

			continue
		}

		failuresWithoutRead++

		if failuresWithoutRead >= maxFailuresWithoutRead {
			break
		}
	}
}

// handleWebSocketPass runs the handler once and writes its response. stop reports that the loop must end;
// failed reports that this pass produced no response to send.
func handleWebSocketPass(ctx *Context, conn *websocket.Connection, handler Handler) (stop, failed bool) {
	response, err := handler(ctx)
	if err != nil {
		return handleWebSocketError(ctx, "error handling message", err) || conn.ReadFailed(), true
	}

	message, err := serializeMessage(response)
	if err != nil {
		ctx.Errorf("failed to serialize message: %v", err)

		return conn.ReadFailed(), true
	}

	if err = conn.WriteMessage(websocket.TextMessage, message); err != nil {
		ctx.Errorf("failed to write response to websocket: %v", err)

		return true, true
	}

	return conn.ReadFailed(), false
}

func handleWebSocketError(ctx *Context, msg string, err error) bool {
	if err == nil {
		return false
	}

	ctx.Errorf("%s: %v", msg, err)

	// Check if the error is a WebSocket close error (of any close code) or if the underlying TCP connection is closed.
	// This prevents unnecessary retries and avoids an infinite loop of read/write operations on the WebSocket.
	var closeErr *gWebsocket.CloseError

	return errors.As(err, &closeErr) || errors.Is(err, net.ErrClosed) ||
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
