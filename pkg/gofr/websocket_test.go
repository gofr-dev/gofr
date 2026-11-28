package gofr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

	"gofr.dev/pkg/gofr/testutil"
	gofrWebsocket "gofr.dev/pkg/gofr/websocket"
)

var errWebSocketNotReady = errors.New("websocket server not ready")

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

// Test_WebSocket_UpgradeOnPlainRouteIsIgnored pins the fix for gofr#4635: an Upgrade header sent
// to a route registered via app.GET must not be upgraded. Before the fix the middleware upgraded
// any route, the hijacked connection was never closed, and repeated upgrade requests leaked file
// descriptors. After the fix the request is served as plain HTTP and nothing is registered in
// the hub — while a route registered via app.WebSocket still upgrades normally. Both route
// matchers are covered: the middleware reads the matched route differently under each (mux
// populates it, the trie records it in the request context).
func Test_WebSocket_UpgradeOnPlainRouteIsIgnored(t *testing.T) {
	tests := []struct {
		name      string
		routerEnv string
	}{
		{"mux matcher", ""},
		{"trie matcher", "trie"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GOFR_ROUTER", tc.routerEnv)
			testUpgradeOnPlainRouteIsIgnored(t)
		})
	}
}

func testUpgradeOnPlainRouteIsIgnored(t *testing.T) {
	t.Helper()

	testutil.NewServerConfigs(t)

	app := New()

	server := httptest.NewServer(app.httpServer.router)
	defer server.Close()

	app.GET("/hello", func(*Context) (any, error) {
		return "hi", nil
	})

	app.WebSocket("/ws", func(ctx *Context) (any, error) {
		var message string

		if err := ctx.Bind(&message); err != nil {
			return nil, err
		}

		return fmt.Sprintf("Received: %s", message), nil
	})

	go app.Run()

	time.Sleep(100 * time.Millisecond)

	// Upgrade headers on the plain route must be ignored: a normal 200 JSON
	// response, not a 101, and nothing registered in the hub — even after many
	// repeated upgrade requests, which is what leaked file descriptors.
	for i := 0; i < 20; i++ {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/hello", http.NoBody)
		require.NoError(t, err)

		req.Header.Set("Connection", "Upgrade")
		req.Header.Set("Upgrade", "websocket")
		req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		_ = resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "iteration %d: plain route must answer 200, not upgrade", i)
		assert.Contains(t, string(body), "hi", "iteration %d: plain route must answer with its handler", i)
	}

	assert.Empty(t, app.httpServer.ws.ListConnections(),
		"no upgrade request to the plain route may register a connection in the hub")

	// The WebSocket route itself must still upgrade and echo.
	wsURL := "ws" + server.URL[len("http"):] + "/ws"

	ws, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)

	defer ws.Close()
	defer resp.Body.Close()

	require.NoError(t, ws.WriteMessage(websocket.TextMessage, []byte("Hello, WebSocket!")))

	_, message, err := ws.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, "Received: Hello, WebSocket!", string(message))

	require.NoError(t, ws.Close())
}
