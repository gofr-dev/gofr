package gofr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/http/middleware"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
	gofrWebsocket "gofr.dev/pkg/gofr/websocket"
)

var (
	errWebSocketNotReady = errors.New("websocket server not ready")
	errInvalidPayload    = errors.New("invalid payload")
	errBrokenPipe        = errors.New("write: broken pipe")
	errConnectionReset   = errors.New("read: connection reset by peer")
)

func Test_WebSocket_Success(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	server := httptest.NewServer(app.httpServer.router)
	defer server.Close()

	app.WebSocket("/ws", func(ctx *Context) (any, error) {
		var message string

		err := ctx.Bind(&message)
		if err != nil {
			return nil, err
		}

		response := fmt.Sprintf("Received: %s", message)

		return response, nil
	})

	go app.Run()

	time.Sleep(100 * time.Millisecond)

	// Create a WebSocket client
	wsURL := "ws" + server.URL[len("http"):] + "/ws"

	ws, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)

	defer ws.Close()
	defer resp.Body.Close()

	// Send a test message
	testMessage := "Hello, WebSocket!"
	err = ws.WriteMessage(websocket.TextMessage, []byte(testMessage))
	require.NoError(t, err)

	// Read the response
	_, message, err := ws.ReadMessage()
	require.NoError(t, err)

	expectedResponse := fmt.Sprintf("Received: %s", testMessage)
	assert.Equal(t, expectedResponse, string(message))

	// Close the client connection
	err = ws.Close()
	require.NoError(t, err)
}

// Test_WebSocket_ContextIsNotSelfReferential pins the fix for the
// self-referential ctx.Context found in review of #4111 (see the issue
// linked from that PR): App.WebSocket built the handler's context with
// context.WithValue(ctx, ...) instead of context.WithValue(ctx.Context, ...).
// ctx is a *Context, which embeds context.Context, so the parent of the
// resulting value-context was ctx itself -- and ctx.Context was the very
// value-context being constructed. Any walk of the parent chain other than
// the lucky first lookup recursed forever.
//
// Calling .Done() is what recurses -- not receiving from the channel it
// returns -- so this handler need only make that call, not read the result,
// to exercise the bug. Pre-fix, that call alone crashes the whole test
// binary with "fatal error: stack overflow" (unrecoverable: recover() cannot
// catch a fatal error), rather than this test failing cleanly.
func Test_WebSocket_ContextIsNotSelfReferential(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	server := httptest.NewServer(app.httpServer.router)
	defer server.Close()

	app.WebSocket("/ws-selfref", func(ctx *Context) (any, error) {
		_ = ctx.Context.Done()

		return "ok", nil
	})

	go app.Run()

	time.Sleep(100 * time.Millisecond)

	wsURL := "ws" + server.URL[len("http"):] + "/ws-selfref"

	ws, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)

	defer ws.Close()
	defer resp.Body.Close()

	err = ws.WriteMessage(websocket.TextMessage, []byte("hi"))
	require.NoError(t, err)

	_, message, err := ws.ReadMessage()
	require.NoError(t, err)

	assert.Equal(t, "ok", string(message))
}

// Test_WebSocket_PlainHTTPRequestDoesNotPanic pins the fix for #3862: a plain
// HTTP request to a route registered via app.WebSocket (no Upgrade headers,
// so no WSConnectionKey is ever set on the request context) must get a clean
// 400 error response instead of panicking. Before the fix, the unsafe type
// assertion on WSConnectionKey panicked; the panic-recovery middleware then
// converted that into a generic 500 "Internal Server Error" response, which
// is indistinguishable by status code alone from a naive fix that also
// returned 500. The response is checked for both the 400 status (per RFC
// 6455 4.2.1 - a request that never attempted an upgrade is a client error,
// same as a request whose upgrade attempt failed, handled a few lines away
// in middleware/web_socket.go) and the specific message, since only the
// panic-recovery path produces the generic one.
func Test_WebSocket_PlainHTTPRequestDoesNotPanic(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	server := httptest.NewServer(app.httpServer.router)
	defer server.Close()

	app.WebSocket("/ws", func(*Context) (any, error) {
		return "unreachable: handler must not run without a WebSocket connection", nil
	})

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/ws", http.NoBody)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Contains(t, string(body), "request did not include a WebSocket upgrade",
		"expected the websocket.ErrorNotWebSocketUpgrade message, not a panic-recovery response")
}

func Test_AddWSService(t *testing.T) {
	port := testutil.GetFreePort(t)
	t.Setenv("HTTP_PORT", fmt.Sprint(port))

	app := New()

	app.WebSocket("/ws", func(ctx *Context) (any, error) {
		var message string

		err := ctx.Bind(&message)
		if err != nil {
			return nil, err
		}

		return "Service Response", nil
	})

	go app.Run()

	wsURL := fmt.Sprintf("ws://localhost:%d/ws", port)

	// Use readiness check instead of sleep
	err := waitForWebSocketReady(wsURL, 3*time.Second)
	require.NoError(t, err, "WebSocket server did not become ready in time")

	serviceName := "test-service"
	err = app.AddWSService(serviceName, wsURL, http.Header{}, true, 100*time.Millisecond)
	require.NoError(t, err, "Failed to add WebSocket service")

	// Verify the connection is registered
	// WebSocket service communication should be handled through the Context
	conn := app.container.WSManager.GetConnectionByServiceName(serviceName)
	require.NotNil(t, conn, "Connection should be registered")
}

func waitForWebSocketReady(wsURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		dialer := websocket.Dialer{}

		conn, resp, err := dialer.Dial(wsURL, nil)
		if resp != nil {
			resp.Body.Close()
		}

		if err == nil {
			conn.Close()

			return nil
		}

		time.Sleep(50 * time.Millisecond)
	}

	return fmt.Errorf("%w after %s", errWebSocketNotReady, timeout)
}

func TestSerializeMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected []byte
	}{
		{
			name:     "String input",
			input:    "hello",
			expected: []byte("hello"),
		},
		{
			name:     "Byte slice input",
			input:    []byte("hello"),
			expected: []byte("hello"),
		},
		{
			name: "Struct input",
			input: struct {
				Data string `json:"data"`
			}{
				Data: "hello",
			},
			expected: []byte(`{"data":"hello"}`),
		},
		{
			name:     "Integer input",
			input:    42,
			expected: []byte(`42`),
		},
		{
			name:     "Map input",
			input:    map[string]any{"key": "value"},
			expected: []byte(`{"key":"value"}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := serializeMessage(tt.input)
			require.NoError(t, err, "TestSerializeMessage Failed!")

			var expectedFormatted, actualFormatted any

			_ = json.Unmarshal(tt.expected, &expectedFormatted)

			_ = json.Unmarshal(actual, &actualFormatted)

			if !reflect.DeepEqual(expectedFormatted, actualFormatted) {
				t.Errorf("serializeMessage() = %s, want %s", string(actual), string(tt.expected))
			}
		})
	}
}

func TestApp_OverrideWebsocketUpgrader(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	custom := &websocket.Upgrader{ReadBufferSize: 42}

	app.OverrideWebsocketUpgrader(custom)

	assert.Same(t, custom, app.httpServer.ws.WebSocketUpgrader.Upgrader)
}

func Test_WebSocket_ConnectionMissingFromManager(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	app.WebSocket("/ws", func(*Context) (any, error) {
		return "unreachable: handler must not run without a registered connection", nil
	})

	ctx := context.WithValue(t.Context(), gofrWebsocket.WSConnectionKey, "unknown-conn-id")
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/ws", http.NoBody)
	rec := httptest.NewRecorder()

	app.httpServer.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), gofrWebsocket.ErrorConnection.Error())
}

// flakyUpgradeServer rejects the first `rejects` handshakes with 400 and upgrades the rest.
func flakyUpgradeServer(t *testing.T, rejects int32) *httptest.Server {
	t.Helper()

	var attempts atomic.Int32

	upgrader := websocket.Upgrader{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) <= rejects {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		defer conn.Close()

		_, _, _ = conn.ReadMessage()
	}))

	t.Cleanup(srv.Close)

	return srv
}

func Test_AddWSService_Failures(t *testing.T) {
	tests := []struct {
		desc               string
		rejects            int32
		enableReconnection bool
		expErr             error
		assertConn         func(t *testing.T, app *App)
	}{
		{
			desc:               "dial failure without reconnection returns error",
			rejects:            1000,
			enableReconnection: false,
			expErr:             websocket.ErrBadHandshake,
			assertConn: func(t *testing.T, app *App) {
				t.Helper()

				// Without reconnection AddWSService returns synchronously and never stores a connection.
				assert.Nil(t, app.container.GetWSConnectionByServiceName("svc"))
			},
		},
		{
			desc:               "dial failure with reconnection connects in background",
			rejects:            2,
			enableReconnection: true,
			expErr:             nil,
			assertConn: func(t *testing.T, app *App) {
				t.Helper()

				var conn *gofrWebsocket.Connection

				require.Eventually(t, func() bool {
					conn = app.container.GetWSConnectionByServiceName("svc")

					return conn != nil
				}, 3*time.Second, 10*time.Millisecond)

				// Registered after the server's cleanup, so it runs first and unblocks the server handler.
				t.Cleanup(func() { _ = conn.Close() })
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			testutil.NewServerConfigs(t)

			app := New()
			srv := flakyUpgradeServer(t, tc.rejects)
			wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

			err := app.AddWSService("svc", wsURL, http.Header{}, tc.enableReconnection, 5*time.Millisecond)

			require.ErrorIs(t, err, tc.expErr)
			tc.assertConn(t, app)
		})
	}
}

func TestSerializeMessage_Error(t *testing.T) {
	tests := []struct {
		desc   string
		input  any
		expErr error
	}{
		{desc: "channel cannot be marshaled", input: make(chan int), expErr: ErrMarshalingResponse},
		{desc: "function cannot be marshaled", input: func() {}, expErr: ErrMarshalingResponse},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			msg, err := serializeMessage(tc.input)

			require.ErrorIs(t, err, tc.expErr)
			assert.Nil(t, msg)
		})
	}
}

func newWebSocketTestApp(t *testing.T, handler Handler) (app *App, wsURL string) {
	t.Helper()

	testutil.NewServerConfigs(t)

	app = New()
	app.httpServer.router.Use(middleware.WSHandlerUpgrade(app.container, app.httpServer.ws))
	app.WebSocket("/ws", handler)

	server := httptest.NewServer(app.httpServer.router)
	t.Cleanup(server.Close)

	return app, "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
}

func dialWebSocket(t *testing.T, app *App, wsURL string) *websocket.Conn {
	t.Helper()

	ws, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)

	t.Cleanup(func() {
		ws.Close()
		resp.Body.Close()
	})

	require.Eventually(t, func() bool {
		return len(app.httpServer.ws.ListConnections()) == 1
	}, 3*time.Second, time.Millisecond, "connection was not registered")

	return ws
}

func requireHandlerLoopEnded(t *testing.T, app *App) {
	t.Helper()

	require.Eventually(t, func() bool {
		return len(app.httpServer.ws.ListConnections()) == 0
	}, 3*time.Second, 10*time.Millisecond, "handler loop did not stop after the connection failed")
}

func Test_WebSocket_ClientCloseCodesStopHandlerLoop(t *testing.T) {
	tests := []struct {
		desc string
		code int
	}{
		{desc: "normal closure", code: websocket.CloseNormalClosure},
		{desc: "going away", code: websocket.CloseGoingAway},
		{desc: "no status received", code: websocket.CloseNoStatusReceived},
		{desc: "policy violation", code: websocket.ClosePolicyViolation},
		{desc: "internal server error", code: websocket.CloseInternalServerErr},
		{desc: "application defined", code: 4000},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			var calls atomic.Int32

			app, wsURL := newWebSocketTestApp(t, func(ctx *Context) (any, error) {
				calls.Add(1)

				var message string

				if err := ctx.Bind(&message); err != nil {
					return nil, err
				}

				return message, nil
			})

			ws := dialWebSocket(t, app, wsURL)

			// FormatCloseMessage sends an empty close frame for CloseNoStatusReceived, which is what a
			// browser's plain ws.close() produces.
			err := ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(tc.code, ""),
				time.Now().Add(time.Second))
			require.NoError(t, err)

			requireHandlerLoopEnded(t, app)

			assert.Equal(t, int32(1), calls.Load())
		})
	}
}

func Test_WebSocket_ReadFailureStopsHandlerLoop(t *testing.T) {
	tests := []struct {
		desc    string
		prepare func(conn *gofrWebsocket.Connection)
		trigger func(t *testing.T, ws *websocket.Conn)
	}{
		{
			desc: "connection dropped without a close frame",
			trigger: func(t *testing.T, ws *websocket.Conn) {
				t.Helper()

				require.NoError(t, ws.UnderlyingConn().Close())
			},
		},
		{
			desc: "read deadline exceeded",
			prepare: func(conn *gofrWebsocket.Connection) {
				_ = conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
			},
			trigger: func(*testing.T, *websocket.Conn) {},
		},
		{
			desc: "read limit exceeded",
			prepare: func(conn *gofrWebsocket.Connection) {
				conn.SetReadLimit(4)
			},
			trigger: func(t *testing.T, ws *websocket.Conn) {
				t.Helper()

				require.NoError(t, ws.WriteMessage(websocket.TextMessage, []byte("longer than the limit")))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			var calls atomic.Int32

			app, wsURL := newWebSocketTestApp(t, func(ctx *Context) (any, error) {
				calls.Add(1)

				if tc.prepare != nil {
					tc.prepare(ctx.Request.(*gofrWebsocket.Connection))
				}

				var message string

				if err := ctx.Bind(&message); err != nil {
					return nil, err
				}

				return message, nil
			})

			tc.trigger(t, dialWebSocket(t, app, wsURL))

			requireHandlerLoopEnded(t, app)

			assert.Equal(t, int32(1), calls.Load())
		})
	}
}

func Test_WebSocket_HandlerIgnoringReadFailureStopsLoop(t *testing.T) {
	var calls atomic.Int32

	app, wsURL := newWebSocketTestApp(t, func(ctx *Context) (any, error) {
		calls.Add(1)

		_ = ctx.Request.(*gofrWebsocket.Connection).SetReadDeadline(time.Now().Add(50 * time.Millisecond))

		var message string

		_ = ctx.Bind(&message)

		return "done", nil
	})

	ws := dialWebSocket(t, app, wsURL)

	_, message, err := ws.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, "done", string(message))

	requireHandlerLoopEnded(t, app)

	assert.Equal(t, int32(1), calls.Load())
}

func Test_WebSocket_WriteFailureStopsHandlerLoop(t *testing.T) {
	var calls atomic.Int32

	app, wsURL := newWebSocketTestApp(t, func(*Context) (any, error) {
		calls.Add(1)

		return "ping", nil
	})

	ws := dialWebSocket(t, app, wsURL)

	require.NoError(t, ws.UnderlyingConn().Close())

	requireHandlerLoopEnded(t, app)

	assert.Positive(t, calls.Load())
}

func Test_WebSocket_HandlerErrorKeepsConnectionOpen(t *testing.T) {
	var calls atomic.Int32

	app, wsURL := newWebSocketTestApp(t, func(ctx *Context) (any, error) {
		calls.Add(1)

		var message string

		if err := ctx.Bind(&message); err != nil {
			return nil, err
		}

		if message == "bad" {
			return nil, errInvalidPayload
		}

		return message, nil
	})

	ws := dialWebSocket(t, app, wsURL)

	for _, payload := range []string{"bad", "good"} {
		require.NoError(t, ws.WriteMessage(websocket.TextMessage, []byte(payload)))
	}

	require.NoError(t, ws.SetReadDeadline(time.Now().Add(3*time.Second)))

	_, message, err := ws.ReadMessage()
	require.NoError(t, err)

	assert.Equal(t, "good", string(message))
	assert.GreaterOrEqual(t, calls.Load(), int32(2))
}

func Test_WebSocket_HandlerErrorBeforeReadingStopsLoop(t *testing.T) {
	var calls atomic.Int32

	app, wsURL := newWebSocketTestApp(t, func(*Context) (any, error) {
		calls.Add(1)

		return nil, errInvalidPayload
	})

	// The handler fails as soon as the connection is registered, so it may already be gone by the time
	// dialWebSocket would look for it.
	ws, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)

	defer ws.Close()
	defer resp.Body.Close()

	require.Eventually(t, func() bool { return calls.Load() == 1 }, 3*time.Second, time.Millisecond)

	requireHandlerLoopEnded(t, app)

	assert.Equal(t, int32(1), calls.Load())

	require.NoError(t, ws.SetReadDeadline(time.Now().Add(3*time.Second)))

	_, _, err = ws.ReadMessage()
	require.Error(t, err, "the client must see the connection close, not a frame")
}

func Test_WebSocket_SerializeFailureDoesNotSendFrame(t *testing.T) {
	var calls atomic.Int32

	app, wsURL := newWebSocketTestApp(t, func(ctx *Context) (any, error) {
		if calls.Add(1) == 1 {
			return make(chan int), nil
		}

		var message string

		if err := ctx.Bind(&message); err != nil {
			return nil, err
		}

		return message, nil
	})

	ws := dialWebSocket(t, app, wsURL)

	require.NoError(t, ws.WriteMessage(websocket.TextMessage, []byte("hello")))
	require.NoError(t, ws.SetReadDeadline(time.Now().Add(3*time.Second)))

	_, message, err := ws.ReadMessage()
	require.NoError(t, err)

	assert.Equal(t, "hello", string(message))
}

func Test_handleWebSocketError(t *testing.T) {
	tests := []struct {
		desc      string
		err       error
		wantAbort bool
	}{
		{desc: "nil error", err: nil, wantAbort: false},
		{
			desc:      "close error with no status",
			err:       &websocket.CloseError{Code: websocket.CloseNoStatusReceived},
			wantAbort: true,
		},
		{desc: "close error with application code", err: &websocket.CloseError{Code: 4000}, wantAbort: true},
		{
			desc:      "wrapped close error",
			err:       fmt.Errorf("bind: %w", &websocket.CloseError{Code: websocket.CloseInternalServerErr}),
			wantAbort: true,
		},
		{desc: "closed network connection", err: net.ErrClosed, wantAbort: true},
		{desc: "broken pipe", err: errBrokenPipe, wantAbort: true},
		{desc: "connection reset by peer", err: errConnectionReset, wantAbort: true},
		{desc: "handler error", err: errInvalidPayload, wantAbort: false},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ctx := &Context{
				Context:       t.Context(),
				ContextLogger: *logging.NewContextLogger(t.Context(), logging.NewMockLogger(logging.FATAL)),
			}

			assert.Equal(t, tc.wantAbort, handleWebSocketError(ctx, "test", tc.err))
		})
	}
}
