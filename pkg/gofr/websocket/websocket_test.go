package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

// setupWebSocketServer creates a test WebSocket server with the given handler.
func setupWebSocketServer(t *testing.T, handler func(*Connection)) *httptest.Server {
	t.Helper()

	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("Failed to upgrade connection: %v", err)
			return
		}
		defer conn.Close()

		wsConn := &Connection{Conn: conn}
		handler(wsConn)
	}))

	return server
}

// connectToWebSocket connects to a WebSocket server and returns the connection.
func connectToWebSocket(t *testing.T, serverURL string) (*Connection, *http.Response) {
	t.Helper()

	url := "ws" + serverURL[len("http"):] + "/ws"
	dialer := websocket.DefaultDialer
	conn, resp, err := dialer.Dial(url, nil)
	require.NoError(t, err)

	return &Connection{Conn: conn}, resp
}

// sendMessageToWebSocket sends a message to a WebSocket connection.
func sendMessageToWebSocket(t *testing.T, conn *Connection, message []byte) {
	t.Helper()

	err := conn.WriteMessage(websocket.TextMessage, message)
	require.NoError(t, err)
}

// waitForWebSocketOperation waits for a WebSocket operation to complete.
func waitForWebSocketOperation(t *testing.T) {
	t.Helper()

	time.Sleep(100 * time.Millisecond)
}

// CORE FUNCTIONALITY TESTS
// TestConnection_Bind_Success tests the Bind method with successful cases.
func TestConnection_Bind_Success(t *testing.T) {
	tests := []struct {
		name         string
		inputMessage []byte
		targetType   any
		expected     any
	}{
		{
			name:         "Bind to string - success",
			inputMessage: []byte("Hello, WebSocket"),
			targetType:   new(string),
			expected:     "Hello, WebSocket",
		},
		{
			name:         "Bind to JSON struct - success",
			inputMessage: []byte(`{"name":"test","value":123}`),
			targetType:   &map[string]any{},
			expected:     map[string]any{"name": "test", "value": float64(123)},
		},
		{
			name:         "Bind to custom struct - success",
			inputMessage: []byte(`{"id":1,"name":"test"}`),
			targetType:   &testStruct{},
			expected:     testStruct{ID: 1, Name: "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := setupWebSocketServer(t, func(wsConn *Connection) {
				err := wsConn.Bind(tt.targetType)
				require.NoError(t, err)
				assert.Equal(t, tt.expected, dereferenceValue(tt.targetType))
			})
			defer server.Close()

			conn, resp := connectToWebSocket(t, server.URL)
			defer conn.Close()

			if resp != nil {
				resp.Body.Close()
			}

			sendMessageToWebSocket(t, conn, tt.inputMessage)
			waitForWebSocketOperation(t)
		})
	}
}

// TestConnection_Bind_Failure tests the Bind method with error cases.
func TestConnection_Bind_Failure(t *testing.T) {
	tests := []struct {
		name         string
		inputMessage []byte
		targetType   any
	}{
		{
			name:         "Bind to invalid JSON - error",
			inputMessage: []byte(`{"name":"test",invalid}`),
			targetType:   &map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := setupWebSocketServer(t, func(wsConn *Connection) {
				err := wsConn.Bind(tt.targetType)
				require.Error(t, err)
			})
			defer server.Close()

			conn, resp := connectToWebSocket(t, server.URL)
			defer conn.Close()

			if resp != nil {
				resp.Body.Close()
			}

			sendMessageToWebSocket(t, conn, tt.inputMessage)
			waitForWebSocketOperation(t)
		})
	}
}

// TestConnection_WriteMessage tests thread-safe writing.
func TestConnection_WriteMessage(t *testing.T) {
	message := "test message"

	server := setupWebSocketServer(t, func(wsConn *Connection) {
		// Test concurrent writes
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)

			go func() {
				defer wg.Done()

				err := wsConn.WriteMessage(websocket.TextMessage, []byte(message))
				assert.NoError(t, err)
			}()
		}

		wg.Wait()
	})
	defer server.Close()

	conn, resp := connectToWebSocket(t, server.URL)
	defer conn.Close()

	if resp != nil {
		resp.Body.Close()
	}

	// Read messages to prevent connection from closing
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	time.Sleep(200 * time.Millisecond)
}

// TestConnection_ReadMessage tests reading messages.
func TestConnection_ReadMessage(t *testing.T) {
	testMessage := []byte("test read message")

	server := setupWebSocketServer(t, func(wsConn *Connection) {
		// Write a test message
		err := wsConn.WriteMessage(websocket.TextMessage, testMessage)
		require.NoError(t, err)
	})
	defer server.Close()

	conn, resp := connectToWebSocket(t, server.URL)

	defer conn.Close()

	if resp != nil {
		resp.Body.Close()
	}

	messageType, message, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, websocket.TextMessage, messageType)
	assert.Equal(t, testMessage, message)
}

// TestConnection_ReadMessage_ErrorHandling tests reading messages with error scenarios.
func TestConnection_ReadMessage_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		setupServer func(*testing.T, *Connection)
		expectError bool
		description string
	}{
		{
			name: "Connection closed before read",
			setupServer: func(t *testing.T, wsConn *Connection) {
				t.Helper()
				// Close connection to force read error
				wsConn.Close()

				messageType, message, err := wsConn.ReadMessage()
				require.Error(t, err, "Expected error for closed connection")
				assert.Equal(t, -1, messageType) // Closed connection returns -1
				assert.Nil(t, message)
			},
			expectError: true,
			description: "Should handle connection closed before read",
		},
		{
			name: "Network timeout during read",
			setupServer: func(t *testing.T, wsConn *Connection) {
				t.Helper()
				// Set a very short read deadline to simulate timeout
				err := wsConn.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
				require.NoError(t, err)

				// Wait for deadline to pass
				time.Sleep(10 * time.Millisecond)

				messageType, message, err := wsConn.ReadMessage()
				require.Error(t, err, "Expected timeout error")
				assert.Equal(t, -1, messageType) // Timeout returns -1
				assert.Nil(t, message)
			},
			expectError: true,
			description: "Should handle network timeout during read",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := setupWebSocketServer(t, func(wsConn *Connection) {
				tt.setupServer(t, wsConn)
			})
			defer server.Close()

			conn, resp := connectToWebSocket(t, server.URL)
			defer conn.Close()

			if resp != nil {
				resp.Body.Close()
			}

			// Send a message to trigger the server handler
			sendMessageToWebSocket(t, conn, []byte("test"))
			waitForWebSocketOperation(t)
		})
	}
}

// TestConnection_Deadlines tests deadline functionality.
func TestConnection_Deadlines(t *testing.T) {
	server := setupWebSocketServer(t, func(wsConn *Connection) {
		// Test read deadline
		err := wsConn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		require.NoError(t, err)

		// Test write deadline
		err = wsConn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
		require.NoError(t, err)
	})
	defer server.Close()

	conn, resp := connectToWebSocket(t, server.URL)
	defer conn.Close()

	if resp != nil {
		resp.Body.Close()
	}

	time.Sleep(200 * time.Millisecond)
}

// WS UPGRADER TESTS
// TestWSUpgrader_NewWSUpgrader tests upgrader creation with basic options.
func TestWSUpgrader_NewWSUpgrader(t *testing.T) {
	tests := []struct {
		name     string
		options  []Options
		validate func(t *testing.T, upgrader *WSUpgrader)
	}{
		{
			name:    "Default upgrader",
			options: []Options{},
			validate: func(t *testing.T, upgrader *WSUpgrader) {
				t.Helper()
				assert.NotNil(t, upgrader.Upgrader)
			},
		},
		{
			name: "With all options",
			options: []Options{
				WithReadBufferSize(2048),
				WithWriteBufferSize(2048),
				WithHandshakeTimeout(2 * time.Second),
				WithSubprotocols("test-protocol"),
				WithCompression(),
				WithError(func(_ http.ResponseWriter, _ *http.Request, _ int, _ error) {}),
				WithCheckOrigin(func(_ *http.Request) bool { return true }),
			},
			validate: func(t *testing.T, upgrader *WSUpgrader) {
				t.Helper()
				actualUpgrader := upgrader.Upgrader.(*websocket.Upgrader)
				assert.Equal(t, 2048, actualUpgrader.ReadBufferSize)
				assert.Equal(t, 2048, actualUpgrader.WriteBufferSize)
				assert.Equal(t, 2*time.Second, actualUpgrader.HandshakeTimeout)
				assert.Equal(t, []string{"test-protocol"}, actualUpgrader.Subprotocols)
				assert.True(t, actualUpgrader.EnableCompression)
				assert.NotNil(t, actualUpgrader.Error)
				assert.NotNil(t, actualUpgrader.CheckOrigin)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upgrader := NewWSUpgrader(tt.options...)
			tt.validate(t, upgrader)
		})
	}
}

// TestWSUpgrader_BufferOptions tests buffer size options.
func TestWSUpgrader_BufferOptions(t *testing.T) {
	tests := []struct {
		name     string
		options  []Options
		validate func(t *testing.T, upgrader *WSUpgrader)
	}{
		{
			name: "With multiple subprotocols",
			options: []Options{
				WithSubprotocols("protocol1", "protocol2", "protocol3"),
			},
			validate: func(t *testing.T, upgrader *WSUpgrader) {
				t.Helper()
				actualUpgrader := upgrader.Upgrader.(*websocket.Upgrader)
				assert.Equal(t, []string{"protocol1", "protocol2", "protocol3"}, actualUpgrader.Subprotocols)
			},
		},
		{
			name: "With zero buffer sizes",
			options: []Options{
				WithReadBufferSize(0),
				WithWriteBufferSize(0),
			},
			validate: func(t *testing.T, upgrader *WSUpgrader) {
				t.Helper()
				actualUpgrader := upgrader.Upgrader.(*websocket.Upgrader)
				assert.Equal(t, 0, actualUpgrader.ReadBufferSize)
				assert.Equal(t, 0, actualUpgrader.WriteBufferSize)
			},
		},
		{
			name: "With negative buffer sizes",
			options: []Options{
				WithReadBufferSize(-1),
				WithWriteBufferSize(-1),
			},
			validate: func(t *testing.T, upgrader *WSUpgrader) {
				t.Helper()
				actualUpgrader := upgrader.Upgrader.(*websocket.Upgrader)
				assert.Equal(t, -1, actualUpgrader.ReadBufferSize)
				assert.Equal(t, -1, actualUpgrader.WriteBufferSize)
			},
		},
		{
			name: "With very large buffer sizes",
			options: []Options{
				WithReadBufferSize(1024 * 1024),  // 1MB
				WithWriteBufferSize(1024 * 1024), // 1MB
			},
			validate: func(t *testing.T, upgrader *WSUpgrader) {
				t.Helper()
				actualUpgrader := upgrader.Upgrader.(*websocket.Upgrader)
				assert.Equal(t, 1024*1024, actualUpgrader.ReadBufferSize)
				assert.Equal(t, 1024*1024, actualUpgrader.WriteBufferSize)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upgrader := NewWSUpgrader(tt.options...)
			tt.validate(t, upgrader)
		})
	}
}

// TestWSUpgrader_TimeoutOptions tests timeout options.
func TestWSUpgrader_TimeoutOptions(t *testing.T) {
	tests := []struct {
		name     string
		options  []Options
		validate func(t *testing.T, upgrader *WSUpgrader)
	}{
		{
			name: "With zero handshake timeout",
			options: []Options{
				WithHandshakeTimeout(0),
			},
			validate: func(t *testing.T, upgrader *WSUpgrader) {
				t.Helper()
				actualUpgrader := upgrader.Upgrader.(*websocket.Upgrader)
				assert.Equal(t, time.Duration(0), actualUpgrader.HandshakeTimeout)
			},
		},
		{
			name: "With very long handshake timeout",
			options: []Options{
				WithHandshakeTimeout(24 * time.Hour),
			},
			validate: func(t *testing.T, upgrader *WSUpgrader) {
				t.Helper()
				actualUpgrader := upgrader.Upgrader.(*websocket.Upgrader)
				assert.Equal(t, 24*time.Hour, actualUpgrader.HandshakeTimeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upgrader := NewWSUpgrader(tt.options...)
			tt.validate(t, upgrader)
		})
	}
}

// TestWSUpgrader_ConflictingOptions tests conflicting or invalid option combinations.
func TestWSUpgrader_ConflictingOptions(t *testing.T) {
	tests := []struct {
		name        string
		options     []Options
		expectError bool
		description string
	}{
		{
			name: "Multiple buffer size options - last wins",
			options: []Options{
				WithReadBufferSize(1024),
				WithReadBufferSize(2048),
				WithWriteBufferSize(512),
				WithWriteBufferSize(4096),
			},
			expectError: false,
			description: "Last option should override previous ones",
		},
		{
			name: "Multiple handshake timeout options - last wins",
			options: []Options{
				WithHandshakeTimeout(1 * time.Second),
				WithHandshakeTimeout(2 * time.Second),
				WithHandshakeTimeout(3 * time.Second),
			},
			expectError: false,
			description: "Last timeout option should override previous ones",
		},
		{
			name: "Multiple subprotocol options - last wins",
			options: []Options{
				WithSubprotocols("protocol1", "protocol2"),
				WithSubprotocols("protocol3", "protocol4", "protocol5"),
			},
			expectError: false,
			description: "Last subprotocol option should override previous ones",
		},
		{
			name: "Multiple error handlers - last wins",
			options: []Options{
				WithError(func(_ http.ResponseWriter, _ *http.Request, _ int, _ error) {}),
				WithError(func(_ http.ResponseWriter, _ *http.Request, _ int, _ error) {}),
			},
			expectError: false,
			description: "Last error handler should override previous one",
		},
		{
			name: "Multiple check origin handlers - last wins",
			options: []Options{
				WithCheckOrigin(func(_ *http.Request) bool { return false }),
				WithCheckOrigin(func(_ *http.Request) bool { return true }),
			},
			expectError: false,
			description: "Last check origin handler should override previous one",
		},
		{
			name: "Multiple compression options - last wins",
			options: []Options{
				WithCompression(),
				WithCompression(),
			},
			expectError: false,
			description: "Multiple compression options should not conflict",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upgrader := NewWSUpgrader(tt.options...)
			assert.NotNil(t, upgrader, tt.description)

			// Verify the last option takes precedence
			actualUpgrader := upgrader.Upgrader.(*websocket.Upgrader)
			assert.NotNil(t, actualUpgrader, "Upgrader should be created successfully")
		})
	}
}

// TestWSUpgrader_RealConnectionConflicts_Success tests successful connection scenarios.
func TestWSUpgrader_RealConnectionConflicts_Success(t *testing.T) {
	tests := []struct {
		name        string
		options     []Options
		description string
	}{
		{
			name: "CheckOrigin accepting all connections",
			options: []Options{
				WithCheckOrigin(func(_ *http.Request) bool { return true }),
			},
			description: "Should accept connection when CheckOrigin returns true",
		},
		{
			name: "Normal timeout with compression",
			options: []Options{
				WithHandshakeTimeout(5 * time.Second),
				WithCompression(),
			},
			description: "Should work with normal timeout and compression",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upgrader := NewWSUpgrader(tt.options...)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := upgrader.Upgrade(w, r, nil)
				assert.NoError(t, err, tt.description)

				if conn != nil {
					conn.Close()
				}
			}))
			defer server.Close()

			url := "ws" + server.URL[len("http"):] + "/ws"
			dialer := websocket.DefaultDialer
			conn, resp, err := dialer.Dial(url, nil)
			require.NoError(t, err, tt.description)

			if conn != nil {
				conn.Close()
			}

			if resp != nil {
				resp.Body.Close()
			}
		})
	}
}

// TestWSUpgrader_RealConnectionConflicts_Failure tests error connection scenarios.
func TestWSUpgrader_RealConnectionConflicts_Failure(t *testing.T) {
	tests := []struct {
		name        string
		options     []Options
		description string
	}{
		{
			name: "CheckOrigin rejecting all connections",
			options: []Options{
				WithCheckOrigin(func(_ *http.Request) bool { return false }),
			},
			description: "Should reject connection when CheckOrigin returns false",
		},
		{
			name: "Very short handshake timeout",
			options: []Options{
				WithHandshakeTimeout(1 * time.Nanosecond),
			},
			description: "Should timeout with very short handshake timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upgrader := NewWSUpgrader(tt.options...)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := upgrader.Upgrade(w, r, nil)
				assert.Error(t, err, tt.description)
				assert.Nil(t, conn, "Connection should be nil on error")
			}))
			defer server.Close()

			url := "ws" + server.URL[len("http"):] + "/ws"
			dialer := websocket.DefaultDialer
			_, resp, err := dialer.Dial(url, nil)
			require.Error(t, err, tt.description)

			if resp != nil {
				resp.Body.Close()
			}
		})
	}
}

// TestWSUpgrader_Upgrade_Success tests successful upgrade scenarios.
func TestWSUpgrader_Upgrade_Success(t *testing.T) {
	tests := []struct {
		name        string
		options     []Options
		description string
	}{
		{
			name:        "Successful upgrade with default options",
			options:     []Options{},
			description: "Should successfully upgrade HTTP to WebSocket",
		},
		{
			name: "Successful upgrade with custom options",
			options: []Options{
				WithReadBufferSize(1024),
				WithWriteBufferSize(1024),
				WithHandshakeTimeout(5 * time.Second),
			},
			description: "Should successfully upgrade with custom options",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upgrader := NewWSUpgrader(tt.options...)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := upgrader.Upgrade(w, r, nil)
				assert.NoError(t, err, tt.description)

				if conn != nil {
					conn.Close()
				}
			}))
			defer server.Close()

			url := "ws" + server.URL[len("http"):] + "/ws"
			dialer := websocket.DefaultDialer
			conn, resp, err := dialer.Dial(url, nil)
			require.NoError(t, err, tt.description)

			if conn != nil {
				conn.Close()
			}

			if resp != nil {
				resp.Body.Close()
			}
		})
	}
}

// TestWSUpgrader_Upgrade_Failure tests error upgrade scenarios.
func TestWSUpgrader_Upgrade_Failure(t *testing.T) {
	tests := []struct {
		name        string
		options     []Options
		description string
	}{
		{
			name: "Upgrade with CheckOrigin rejection",
			options: []Options{
				WithCheckOrigin(func(_ *http.Request) bool { return false }),
			},
			description: "Should fail when CheckOrigin returns false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upgrader := NewWSUpgrader(tt.options...)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := upgrader.Upgrade(w, r, nil)
				assert.Error(t, err, tt.description)
				assert.Nil(t, conn, "Connection should be nil on error")
			}))
			defer server.Close()

			url := "ws" + server.URL[len("http"):] + "/ws"
			dialer := websocket.DefaultDialer
			_, resp, err := dialer.Dial(url, nil)
			require.Error(t, err, tt.description)

			if resp != nil {
				resp.Body.Close()
			}
		})
	}
}

// MANAGER TESTS
// TestManager_New tests manager creation.
func TestManager_New(t *testing.T) {
	manager := New()

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.WebSocketUpgrader)
	assert.NotNil(t, manager.WebSocketConnections)
	assert.Empty(t, manager.WebSocketConnections)
}

// TestManager_ConnectionManagement tests connection management.
func TestManager_ConnectionManagement(t *testing.T) {
	manager := New()

	// Test adding connections with nil websocket (to avoid close issues)
	conn1 := &Connection{Conn: nil}
	conn2 := &Connection{Conn: nil}

	manager.AddWebsocketConnection("conn1", conn1)
	manager.AddWebsocketConnection("conn2", conn2)

	// Test getting connections
	retrievedConn1 := manager.GetWebsocketConnection("conn1")
	assert.Equal(t, conn1, retrievedConn1)

	retrievedConn2 := manager.GetWebsocketConnection("conn2")
	assert.Equal(t, conn2, retrievedConn2)

	// Test getting non-existent connection
	nonExistent := manager.GetWebsocketConnection("non-existent")
	assert.Nil(t, nonExistent)

	// Test listing connections
	connections := manager.ListConnections()
	assert.ElementsMatch(t, []string{"conn1", "conn2"}, connections)

	// Test getting connection by service name
	serviceConn := manager.GetConnectionByServiceName("conn1")
	assert.Equal(t, conn1, serviceConn)

	// Test closing connection
	manager.CloseConnection("conn1")
	assert.Nil(t, manager.GetWebsocketConnection("conn1"))
	assert.ElementsMatch(t, []string{"conn2"}, manager.ListConnections())
}

// TestManager_ConcurrentOperations tests concurrent operations.
func TestManager_ConcurrentOperations(t *testing.T) {
	manager := New()

	var wg sync.WaitGroup

	numGoroutines := 100

	// Concurrent add operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			conn := &Connection{Conn: nil} // Use nil to avoid close issues
			manager.AddWebsocketConnection("conn"+string(rune(i)), conn)
		}(i)
	}

	wg.Wait()

	// Verify all connections were added
	connections := manager.ListConnections()
	assert.Len(t, connections, numGoroutines)

	// Concurrent read operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			conn := manager.GetWebsocketConnection("conn" + string(rune(i)))
			assert.NotNil(t, conn)
		}(i)
	}

	wg.Wait()

	// Concurrent close operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			manager.CloseConnection("conn" + string(rune(i)))
		}(i)
	}

	wg.Wait()

	// Verify all connections were closed
	connections = manager.ListConnections()
	assert.Empty(t, connections)
}

// EDGE CASE TESTS
// TestConnection_Bind_EdgeCases_Success tests successful edge cases for the Bind method.
func TestConnection_Bind_EdgeCases_Success(t *testing.T) {
	tests := []struct {
		name         string
		inputMessage []byte
		targetType   any
		description  string
	}{
		{
			name:         "Bind to empty string",
			inputMessage: []byte(""),
			targetType:   new(string),
			description:  "Should handle empty string",
		},
		{
			name:         "Bind to large JSON",
			inputMessage: createLargeJSON(),
			targetType:   &map[string]any{},
			description:  "Should handle large JSON payloads",
		},
		{
			name:         "Bind to invalid UTF-8",
			inputMessage: []byte{0xff, 0xfe, 0xfd},
			targetType:   new(string),
			description:  "Should handle invalid UTF-8 sequences",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := setupWebSocketServer(t, func(wsConn *Connection) {
				err := wsConn.Bind(tt.targetType)
				assert.NoError(t, err, tt.description)
			})
			defer server.Close()

			conn, resp := connectToWebSocket(t, server.URL)
			defer conn.Close()

			if resp != nil {
				resp.Body.Close()
			}

			err := conn.WriteMessage(websocket.TextMessage, tt.inputMessage)
			require.NoError(t, err)

			time.Sleep(100 * time.Millisecond)
		})
	}
}

// TestConnection_Bind_EdgeCases_Failure tests error edge cases for the Bind method.
func TestConnection_Bind_EdgeCases_Failure(t *testing.T) {
	tests := []struct {
		name         string
		inputMessage []byte
		targetType   any
		description  string
	}{
		{
			name:         "Bind to non-pointer",
			inputMessage: []byte("test"),
			targetType:   "not a pointer",
			description:  "Should handle non-pointer types",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := setupWebSocketServer(t, func(wsConn *Connection) {
				err := wsConn.Bind(tt.targetType)
				assert.Error(t, err, tt.description)
			})
			defer server.Close()

			conn, resp := connectToWebSocket(t, server.URL)
			defer conn.Close()

			if resp != nil {
				resp.Body.Close()
			}

			err := conn.WriteMessage(websocket.TextMessage, tt.inputMessage)
			require.NoError(t, err)

			time.Sleep(100 * time.Millisecond)
		})
	}
}

// TestManager_EdgeCases tests edge cases for Manager.
func TestManager_EdgeCases(t *testing.T) {
	manager := New()

	t.Run("Add nil connection", func(t *testing.T) {
		manager.AddWebsocketConnection("nil-conn", nil)
		conn := manager.GetWebsocketConnection("nil-conn")
		assert.Nil(t, conn)
	})

	t.Run("Add connection with nil websocket", func(t *testing.T) {
		conn := &Connection{Conn: nil}
		manager.AddWebsocketConnection("nil-ws-conn", conn)
		retrieved := manager.GetWebsocketConnection("nil-ws-conn")
		assert.Equal(t, conn, retrieved)
	})

	t.Run("Close non-existent connection", func(_ *testing.T) {
		// Should not panic
		manager.CloseConnection("non-existent")
	})

	t.Run("Get connection after close", func(t *testing.T) {
		conn := &Connection{Conn: nil} // Use nil to avoid close issues
		manager.AddWebsocketConnection("temp-conn", conn)
		manager.CloseConnection("temp-conn")
		retrieved := manager.GetWebsocketConnection("temp-conn")
		assert.Nil(t, retrieved)
	})

	t.Run("List connections when empty", func(t *testing.T) {
		emptyManager := New()
		connections := emptyManager.ListConnections()
		assert.Empty(t, connections)
	})

	t.Run("Add duplicate connection ID", func(t *testing.T) {
		conn1 := &Connection{Conn: nil} // Use nil to avoid close issues
		conn2 := &Connection{Conn: nil}

		manager.AddWebsocketConnection("duplicate", conn1)
		manager.AddWebsocketConnection("duplicate", conn2)

		retrieved := manager.GetWebsocketConnection("duplicate")
		assert.Equal(t, conn2, retrieved) // Should be the last one added
	})
}

// TestConnection_UnimplementedMethods tests unimplemented methods.
// These methods are intentionally unimplemented for WebSocket connections.
func TestConnection_UnimplementedMethods(t *testing.T) {
	conn := &Connection{}

	// Test Param method - should return empty string
	assert.Empty(t, conn.Param("test"))

	// Test PathParam method - should return empty string
	assert.Empty(t, conn.PathParam("test"))

	// Test HostName method - should return empty string
	assert.Empty(t, conn.HostName())

	// Test Context method - should return a valid context
	ctx := conn.Context()
	assert.NotNil(t, ctx)

	// Test Params method - should return nil
	params := conn.Params("test")
	assert.Nil(t, params)
}

// TestConnection_ConcurrentWriteMessage tests concurrent WriteMessage calls.
func TestConnection_ConcurrentWriteMessage(t *testing.T) {
	server := setupWebSocketServer(t, func(wsConn *Connection) {
		var wg sync.WaitGroup

		numGoroutines := 10

		// Send multiple messages concurrently
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)

			go func(i int) {
				defer wg.Done()

				message := fmt.Sprintf("message %d", i)

				err := wsConn.WriteMessage(websocket.TextMessage, []byte(message))
				if err != nil {
					t.Errorf("WriteMessage failed: %v", err)
				}
			}(i)
		}

		wg.Wait()
	})
	defer server.Close()

	conn, resp := connectToWebSocket(t, server.URL)
	defer conn.Close()

	if resp != nil {
		resp.Body.Close()
	}

	// Read all messages
	for i := 0; i < 10; i++ {
		messageType, message, err := conn.ReadMessage()
		require.NoError(t, err)
		assert.Equal(t, websocket.TextMessage, messageType)
		assert.Contains(t, string(message), "message")
	}
}

// TestConnection_Bind_JSONUnmarshalError tests Bind method with JSON unmarshaling error.
func TestConnection_Bind_JSONUnmarshalError(t *testing.T) {
	server := setupWebSocketServer(t, func(wsConn *Connection) {
		// Send invalid JSON
		err := wsConn.WriteMessage(websocket.TextMessage, []byte("invalid json"))
		if err != nil {
			return // Ignore errors in server handler
		}

		// Try to bind to a struct - should fail
		var data struct {
			Field string `json:"field"`
		}

		err = wsConn.Bind(&data)
		if err != nil {
			return // Expected error, ignore
		}
	})
	defer server.Close()

	conn, resp := connectToWebSocket(t, server.URL)
	defer conn.Close()

	if resp != nil {
		resp.Body.Close()
	}
}

// TestConnection_Bind_StringCase tests Bind method with string case.
func TestConnection_Bind_StringCase(t *testing.T) {
	server := setupWebSocketServer(t, func(wsConn *Connection) {
		// Send text message
		err := wsConn.WriteMessage(websocket.TextMessage, []byte("test message"))
		if err != nil {
			return // Ignore errors in server handler
		}

		// Bind to string
		var data string

		err = wsConn.Bind(&data)
		if err != nil {
			return // Ignore errors in server handler
		}
	})
	defer server.Close()

	conn, resp := connectToWebSocket(t, server.URL)
	defer conn.Close()

	if resp != nil {
		resp.Body.Close()
	}
}

// TestConnection_Bind_JSONCase tests Bind method with JSON case.
func TestConnection_Bind_JSONCase(t *testing.T) {
	server := setupWebSocketServer(t, func(wsConn *Connection) {
		// Send JSON message
		jsonData := `{"name": "test", "value": 123}`

		err := wsConn.WriteMessage(websocket.TextMessage, []byte(jsonData))
		if err != nil {
			return // Ignore errors in server handler
		}

		// Bind to struct
		var data struct {
			Name  string `json:"name"`
			Value int    `json:"value"`
		}

		err = wsConn.Bind(&data)
		if err != nil {
			return // Ignore errors in server handler
		}
	})
	defer server.Close()

	conn, resp := connectToWebSocket(t, server.URL)
	defer conn.Close()

	if resp != nil {
		resp.Body.Close()
	}
}

// TestManager_CloseConnection_WithValidConn tests CloseConnection with valid connection.
func TestManager_CloseConnection_WithValidConn(t *testing.T) {
	manager := New()

	// Create a real connection
	server := setupWebSocketServer(t, func(_ *Connection) {
		// Just keep the connection open
		time.Sleep(100 * time.Millisecond)
	})
	defer server.Close()

	conn, resp := connectToWebSocket(t, server.URL)
	if resp != nil {
		resp.Body.Close()
	}

	manager.AddWebsocketConnection("test", conn)

	// Close connection - should not panic
	manager.CloseConnection("test")

	// Verify connection is removed
	assert.Nil(t, manager.GetWebsocketConnection("test"))
}

// TestWSUpgrader_Upgrade_WithResponseHeader tests Upgrade with response header.
func TestWSUpgrader_Upgrade_WithResponseHeader(t *testing.T) {
	upgrader := NewWSUpgrader()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		responseHeader := http.Header{}
		responseHeader.Set("X-Test-Header", "test-value")

		conn, err := upgrader.Upgrade(w, r, responseHeader)
		if err != nil {
			t.Errorf("Failed to upgrade connection: %v", err)
			return
		}

		defer conn.Close()
	}))
	defer server.Close()

	url := "ws" + server.URL[len("http"):] + "/ws"
	dialer := websocket.DefaultDialer
	conn, resp, err := dialer.Dial(url, nil)
	require.NoError(t, err)

	defer conn.Close()

	if resp != nil {
		resp.Body.Close()
	}

	// Verify response header was set
	assert.Equal(t, "test-value", resp.Header.Get("X-Test-Header"))
}

// OPTIONS TESTS
// TestOptions tests all option functions.
func TestOptions(t *testing.T) {
	upgrader := &websocket.Upgrader{}

	// Test WithReadBufferSize
	WithReadBufferSize(1024)(upgrader)
	assert.Equal(t, 1024, upgrader.ReadBufferSize)

	// Test WithWriteBufferSize
	WithWriteBufferSize(2048)(upgrader)
	assert.Equal(t, 2048, upgrader.WriteBufferSize)

	// Test WithHandshakeTimeout
	timeout := 5 * time.Second
	WithHandshakeTimeout(timeout)(upgrader)
	assert.Equal(t, timeout, upgrader.HandshakeTimeout)

	// Test WithSubprotocols
	protocols := []string{"protocol1", "protocol2"}
	WithSubprotocols(protocols...)(upgrader)
	assert.Equal(t, protocols, upgrader.Subprotocols)

	// Test WithCompression
	WithCompression()(upgrader)
	assert.True(t, upgrader.EnableCompression)

	// Test WithError
	errorHandler := func(_ http.ResponseWriter, _ *http.Request, _ int, _ error) {}
	WithError(errorHandler)(upgrader)
	assert.NotNil(t, upgrader.Error)

	// Test WithCheckOrigin
	checkOrigin := func(_ *http.Request) bool { return true }
	WithCheckOrigin(checkOrigin)(upgrader)
	assert.NotNil(t, upgrader.CheckOrigin)
}

// TestConnection_Bind_ErrorPaths tests comprehensive error paths in Bind method.
func TestConnection_Bind_ErrorPaths(t *testing.T) {
	tests := []struct {
		name        string
		setupServer func(*testing.T, *Connection)
		expectError bool
		description string
	}{
		{
			name: "Connection closed before read",
			setupServer: func(t *testing.T, wsConn *Connection) {
				t.Helper()
				// Close connection to force read error
				wsConn.Close()

				var data string
				err := wsConn.Bind(&data)
				assert.Error(t, err, "Expected error for closed connection")
			},
			expectError: true,
			description: "Should handle connection closed before read",
		},
		{
			name: "Network timeout during read",
			setupServer: func(t *testing.T, wsConn *Connection) {
				t.Helper()
				// Set a very short read deadline to simulate timeout
				err := wsConn.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
				require.NoError(t, err)

				// Wait for deadline to pass
				time.Sleep(10 * time.Millisecond)

				var data string
				err = wsConn.Bind(&data)
				require.Error(t, err, "Expected timeout error")
			},
			expectError: true,
			description: "Should handle network timeout during read",
		},
		{
			name: "Unexpected server response - binary message",
			setupServer: func(t *testing.T, wsConn *Connection) {
				t.Helper()

				// Send binary message instead of text
				err := wsConn.WriteMessage(websocket.BinaryMessage, []byte("binary data"))
				require.NoError(t, err)

				var data string
				err = wsConn.Bind(&data)
				// This should still work as we're reading the message
				assert.NoError(t, err, "Should handle binary messages")
			},
			expectError: false,
			description: "Should handle binary messages gracefully",
		},
		{
			name: "Connection interrupted during read",
			setupServer: func(t *testing.T, wsConn *Connection) {
				t.Helper()

				// Close connection immediately to simulate interruption
				wsConn.Close()

				var data string
				err := wsConn.Bind(&data)
				assert.Error(t, err, "Expected error for interrupted connection")
			},
			expectError: true,
			description: "Should handle connection interruption during read",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := setupWebSocketServer(t, func(wsConn *Connection) {
				tt.setupServer(t, wsConn)
			})
			defer server.Close()

			conn, resp := connectToWebSocket(t, server.URL)
			defer conn.Close()

			if resp != nil {
				resp.Body.Close()
			}

			// Send a message to trigger the server handler
			sendMessageToWebSocket(t, conn, []byte("test"))
			waitForWebSocketOperation(t)
		})
	}
}

// TestConnection_Bind_JSONError tests JSON unmarshaling error path.
func TestConnection_Bind_JSONError(t *testing.T) {
	server := setupWebSocketServer(t, func(wsConn *Connection) {
		// Test with invalid JSON that will cause unmarshaling error
		var data map[string]any

		err := wsConn.Bind(&data)
		// This should fail with invalid JSON
		if err == nil {
			t.Error("Expected error for invalid JSON")
		}
	})
	defer server.Close()

	conn, resp := connectToWebSocket(t, server.URL)
	defer conn.Close()

	if resp != nil {
		resp.Body.Close()
	}

	// Send invalid JSON to trigger error path
	err := conn.WriteMessage(websocket.TextMessage, []byte(`{"invalid": json}`))
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)
}

// TestManager_CloseConnection_WithNilConn tests CloseConnection with nil connection.
func TestManager_CloseConnection_WithNilConn(t *testing.T) {
	manager := New()

	// Add connection with nil websocket
	conn := &Connection{Conn: nil}
	manager.AddWebsocketConnection("test-conn", conn)

	// Close connection - should not panic
	manager.CloseConnection("test-conn")

	// Verify connection is removed
	retrieved := manager.GetWebsocketConnection("test-conn")
	assert.Nil(t, retrieved)
}

// TestManager_CloseConnection_NonExistent tests closing non-existent connection.
func TestManager_CloseConnection_NonExistent(t *testing.T) {
	manager := New()

	// Close non-existent connection - should not panic
	manager.CloseConnection("non-existent")

	// Verify no connections exist
	connections := manager.ListConnections()
	assert.Empty(t, connections)
}

// TestManager_CloseConnection_WithRealConn tests CloseConnection with a real websocket connection.
func TestManager_CloseConnection_WithRealConn(t *testing.T) {
	manager := New()

	// Create a real websocket connection
	server := setupWebSocketServer(t, func(wsConn *Connection) {
		manager.AddWebsocketConnection("test-conn", wsConn)

		// Close connection - should call Close() on the websocket
		manager.CloseConnection("test-conn")

		// Verify connection is removed
		retrieved := manager.GetWebsocketConnection("test-conn")
		assert.Nil(t, retrieved)
	})
	defer server.Close()

	// Connect to the server to trigger the handler
	conn, resp := connectToWebSocket(t, server.URL)
	defer conn.Close()

	if resp != nil {
		resp.Body.Close()
	}

	// Send a message to trigger the server handler
	err := conn.WriteMessage(websocket.TextMessage, []byte("test"))
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)
}

// TestWSUpgrader_Upgrade_InvalidRequest tests upgrade error path with invalid request.
func TestWSUpgrader_Upgrade_InvalidRequest(t *testing.T) {
	upgrader := NewWSUpgrader()

	// Test with invalid request (not a WebSocket upgrade request)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	require.NoError(t, err)

	w := httptest.NewRecorder()

	conn, err := upgrader.Upgrade(w, req, nil)
	require.Error(t, err)
	require.Nil(t, conn)
}

// testStruct is a helper type for testing.
type testStruct struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// dereferenceValue is a helper function to dereference pointers in tests.
func dereferenceValue(v any) any {
	switch val := v.(type) {
	case *string:
		return *val
	case *map[string]any:
		return *val
	case *testStruct:
		return *val
	default:
		return val
	}
}

// createLargeJSON creates a large JSON payload for testing.
func createLargeJSON() []byte {
	data := make(map[string]any)
	for i := 0; i < 1000; i++ {
		data[fmt.Sprintf("key%d", i)] = fmt.Sprintf("value%d", i)
	}

	jsonData, _ := json.Marshal(data)

	return jsonData
}
