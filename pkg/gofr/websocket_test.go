package gofr

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/testutil"
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
