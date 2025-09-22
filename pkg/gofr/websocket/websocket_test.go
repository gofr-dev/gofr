package websocket

import (
	"context"
	"encoding/json"
	"errors"
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
	gomock "go.uber.org/mock/gomock"
)

// Define static errors for better error handling.
var (
	ErrUpgradeFailed = errors.New("upgrade failed")
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

// CORE FUNCTIONALITY TESTS
// TestConnection_Bind tests the Bind method with various data types.
func TestConnection_Bind(t *testing.T) {
	tests := []struct {
		name         string
		inputMessage []byte
		targetType   any
		expected     any
		expectError  bool
	}{
		{
			name:         "Bind to string - success",
			inputMessage: []byte("Hello, WebSocket"),
			targetType:   new(string),
			expected:     "Hello, WebSocket",
			expectError:  false,
		},
		{
			name:         "Bind to JSON struct - success",
			inputMessage: []byte(`{"name":"test","value":123}`),
			targetType:   &map[string]any{},
			expected:     map[string]any{"name": "test", "value": float64(123)},
			expectError:  false,
		},
		{
			name:         "Bind to invalid JSON - error",
			inputMessage: []byte(`{"name":"test",invalid}`),
			targetType:   &map[string]any{},
			expected:     nil,
			expectError:  true,
		},
		{
			name:         "Bind to custom struct - success",
			inputMessage: []byte(`{"id":1,"name":"test"}`),
			targetType:   &testStruct{},
			expected:     testStruct{ID: 1, Name: "test"},
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				upgrader := websocket.Upgrader{}
				conn, err := upgrader.Upgrade(w, r, nil)

				if err != nil {
					t.Errorf("Failed to upgrade connection: %v", err)
					return
				}

				defer conn.Close()

				wsConn := &Connection{Conn: conn}
				err = wsConn.Bind(tt.targetType)

				if tt.expectError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tt.expected, dereferenceValue(tt.targetType))
				}
			}))
			defer server.Close()

			url := "ws" + server.URL[len("http"):] + "/ws"
			dialer := websocket.DefaultDialer
			conn, resp, err := dialer.Dial(url, nil)
			require.NoError(t, err)

			defer conn.Close()
			defer resp.Body.Close()

			err = conn.WriteMessage(websocket.TextMessage, tt.inputMessage)
			require.NoError(t, err)

			time.Sleep(100 * time.Millisecond)
		})
	}
}

// TestConnection_WriteMessage tests thread-safe writing.
func TestConnection_WriteMessage(t *testing.T) {
	upgrader := websocket.Upgrader{}
	message := "test message"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("Failed to upgrade connection: %v", err)
			return
		}
		defer conn.Close()

		wsConn := &Connection{Conn: conn}

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
	}))
	defer server.Close()

	url := "ws" + server.URL[len("http"):] + "/ws"
	dialer := websocket.DefaultDialer
	conn, resp, err := dialer.Dial(url, nil)
	require.NoError(t, err)

	defer conn.Close()
	defer resp.Body.Close()

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
	upgrader := websocket.Upgrader{}
	testMessage := []byte("test read message")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("Failed to upgrade connection: %v", err)
			return
		}
		defer conn.Close()

		wsConn := &Connection{Conn: conn}

		// Write a test message
		err = wsConn.WriteMessage(websocket.TextMessage, testMessage)
		if err != nil {
			t.Errorf("Failed to write message: %v", err)
			return
		}
	}))
	defer server.Close()

	url := "ws" + server.URL[len("http"):] + "/ws"
	dialer := websocket.DefaultDialer
	conn, resp, err := dialer.Dial(url, nil)
	require.NoError(t, err)

	defer conn.Close()
	defer resp.Body.Close()

	wsConn := &Connection{Conn: conn}
	messageType, message, err := wsConn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, websocket.TextMessage, messageType)
	assert.Equal(t, testMessage, message)
}

// TestConnection_Deadlines tests deadline functionality.
func TestConnection_Deadlines(t *testing.T) {
	upgrader := websocket.Upgrader{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("Failed to upgrade connection: %v", err)
			return
		}
		defer conn.Close()

		wsConn := &Connection{Conn: conn}

		// Test read deadline
		err = wsConn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		assert.NoError(t, err)

		// Test write deadline
		err = wsConn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
		assert.NoError(t, err)
	}))
	defer server.Close()

	url := "ws" + server.URL[len("http"):] + "/ws"
	dialer := websocket.DefaultDialer
	conn, resp, err := dialer.Dial(url, nil)
	require.NoError(t, err)

	defer conn.Close()
	defer resp.Body.Close()

	time.Sleep(200 * time.Millisecond)
}

// WS UPGRADER TESTS
// TestWSUpgrader_NewWSUpgrader tests upgrader creation with options.
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

// TestWSUpgrader_Upgrade tests the upgrade functionality.
func TestWSUpgrader_Upgrade(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUpgrader := NewMockUpgrader(ctrl)
	expectedConn := &websocket.Conn{}

	tests := []struct {
		name           string
		setupMock      func()
		expectError    bool
		expectedResult *websocket.Conn
	}{
		{
			name: "Successful upgrade",
			setupMock: func() {
				mockUpgrader.EXPECT().Upgrade(gomock.Any(), gomock.Any(), gomock.Any()).Return(expectedConn, nil)
			},
			expectError:    false,
			expectedResult: expectedConn,
		},
		{
			name: "Upgrade error",
			setupMock: func() {
				mockUpgrader.EXPECT().Upgrade(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, ErrUpgradeFailed)
			},
			expectError:    true,
			expectedResult: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()

			wsUpgrader := &WSUpgrader{Upgrader: mockUpgrader}
			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
			require.NoError(t, err)

			w := httptest.NewRecorder()
			conn, err := wsUpgrader.Upgrade(w, req, nil)

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, conn)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedResult, conn)
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
// TestConnection_Bind_EdgeCases tests edge cases for the Bind method.
func TestConnection_Bind_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		inputMessage []byte
		targetType   any
		expectError  bool
		description  string
	}{
		{
			name:         "Bind to nil pointer",
			inputMessage: []byte("test"),
			targetType:   (*string)(nil),
			expectError:  true,
			description:  "Should handle nil pointer gracefully",
		},
		{
			name:         "Bind to non-pointer",
			inputMessage: []byte("test"),
			targetType:   "not a pointer",
			expectError:  true,
			description:  "Should handle non-pointer types",
		},
		{
			name:         "Bind to empty string",
			inputMessage: []byte(""),
			targetType:   new(string),
			expectError:  false,
			description:  "Should handle empty string",
		},
		{
			name:         "Bind to large JSON",
			inputMessage: createLargeJSON(),
			targetType:   &map[string]any{},
			expectError:  false,
			description:  "Should handle large JSON payloads",
		},
		{
			name:         "Bind to invalid UTF-8",
			inputMessage: []byte{0xff, 0xfe, 0xfd},
			targetType:   new(string),
			expectError:  false,
			description:  "Should handle invalid UTF-8 sequences",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				upgrader := websocket.Upgrader{}
				conn, err := upgrader.Upgrade(w, r, nil)

				if err != nil {
					t.Errorf("Failed to upgrade connection: %v", err)
					return
				}

				defer conn.Close()

				wsConn := &Connection{Conn: conn}
				err = wsConn.Bind(tt.targetType)

				if tt.expectError {
					assert.Error(t, err, tt.description)
				} else {
					assert.NoError(t, err, tt.description)
				}
			}))
			defer server.Close()

			url := "ws" + server.URL[len("http"):] + "/ws"
			dialer := websocket.DefaultDialer
			conn, resp, err := dialer.Dial(url, nil)
			require.NoError(t, err)

			defer conn.Close()
			defer resp.Body.Close()

			err = conn.WriteMessage(websocket.TextMessage, tt.inputMessage)
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
func TestConnection_UnimplementedMethods(t *testing.T) {
	conn := &Connection{}

	// Test Param method
	assert.Empty(t, conn.Param("test"))

	// Test PathParam method
	assert.Empty(t, conn.PathParam("test"))

	// Test HostName method
	assert.Empty(t, conn.HostName())

	// Test Context method
	ctx := conn.Context()
	assert.NotNil(t, ctx)

	// Test Params method
	params := conn.Params("test")
	assert.Nil(t, params)
}

// TestConnection_ErrorHandling tests error handling.
func TestConnection_ErrorHandling(t *testing.T) {
	// Test with nil connection
	conn := &Connection{}

	// These should not panic
	assert.Empty(t, conn.Param("test"))
	assert.Empty(t, conn.PathParam("test"))
	assert.Empty(t, conn.HostName())
	assert.NotNil(t, conn.Context())
	assert.Nil(t, conn.Params("test"))
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

// CONSTANTS TESTS
// TestConstants tests package constants.
func TestConstants(t *testing.T) {
	assert.Equal(t, WSConnectionKey, WSKey("ws-connection-key"))
	assert.Equal(t, 1, TextMessage)
	assert.Equal(t, "couldn't establish connection to web socket", ErrorConnection.Error())
}

// TestConnection_Bind_ErrorPaths tests error paths in Bind method for 100% coverage.
func TestConnection_Bind_ErrorPaths(t *testing.T) {
	upgrader := websocket.Upgrader{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("Failed to upgrade connection: %v", err)
			return
		}
		defer conn.Close()

		wsConn := &Connection{Conn: conn}

		// Test error path in Bind when ReadMessage fails
		// We'll close the connection to force a read error
		conn.Close()

		var data string
		err = wsConn.Bind(&data)

		if err == nil {
			t.Error("Expected error for closed connection")
		}
	}))
	defer server.Close()

	url := "ws" + server.URL[len("http"):] + "/ws"
	dialer := websocket.DefaultDialer
	conn, resp, err := dialer.Dial(url, nil)
	require.NoError(t, err)

	defer conn.Close()
	defer resp.Body.Close()

	// Write a message to trigger the server
	err = conn.WriteMessage(websocket.TextMessage, []byte("test"))
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)
}

// TestConnection_Bind_JSONError tests JSON unmarshaling error path.
func TestConnection_Bind_JSONError(t *testing.T) {
	upgrader := websocket.Upgrader{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("Failed to upgrade connection: %v", err)
			return
		}
		defer conn.Close()

		wsConn := &Connection{Conn: conn}

		// Test with invalid JSON that will cause unmarshaling error
		var data map[string]any
		err = wsConn.Bind(&data)
		// This should fail with invalid JSON
		if err == nil {
			t.Error("Expected error for invalid JSON")
		}
	}))
	defer server.Close()

	url := "ws" + server.URL[len("http"):] + "/ws"
	dialer := websocket.DefaultDialer
	conn, resp, err := dialer.Dial(url, nil)
	require.NoError(t, err)

	defer conn.Close()
	defer resp.Body.Close()

	// Send invalid JSON to trigger error path
	err = conn.WriteMessage(websocket.TextMessage, []byte(`{"invalid": json}`))
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

	// Create a real websocket connection using httptest
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("Failed to upgrade connection: %v", err)
			return
		}
		defer conn.Close()

		wsConn := &Connection{Conn: conn}
		manager.AddWebsocketConnection("test-conn", wsConn)

		// Close connection - should call Close() on the websocket
		manager.CloseConnection("test-conn")

		// Verify connection is removed
		retrieved := manager.GetWebsocketConnection("test-conn")
		assert.Nil(t, retrieved)
	}))

	defer server.Close()

	// Connect to the server to trigger the handler
	url := "ws" + server.URL[len("http"):] + "/ws"
	dialer := websocket.DefaultDialer
	conn, resp, err := dialer.Dial(url, nil)
	require.NoError(t, err)

	defer conn.Close()
	defer resp.Body.Close()

	// Send a message to trigger the server handler
	err = conn.WriteMessage(websocket.TextMessage, []byte("test"))
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)
}

// TestWSUpgrader_Upgrade_Error tests upgrade error path.
func TestWSUpgrader_Upgrade_Error(t *testing.T) {
	// Create a mock upgrader that returns an error
	mockUpgrader := &mockUpgraderWithError{}
	wsUpgrader := &WSUpgrader{Upgrader: mockUpgrader}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	require.NoError(t, err)

	w := httptest.NewRecorder()

	conn, err := wsUpgrader.Upgrade(w, req, nil)
	require.Error(t, err)
	require.Nil(t, conn)
}

// PERFORMANCE TESTS
// TestConnection_Performance tests performance characteristics.
func TestConnection_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	upgrader := websocket.Upgrader{}
	messageCount := 10000

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("Failed to upgrade connection: %v", err)
			return
		}
		defer conn.Close()

		wsConn := &Connection{Conn: conn}

		start := time.Now()

		// Write many messages
		for i := 0; i < messageCount; i++ {
			message := []byte("performance test message")
			err := wsConn.WriteMessage(websocket.TextMessage, message)

			if err != nil {
				t.Errorf("Failed to write message: %v", err)
				return
			}
		}

		duration := time.Since(start)
		t.Logf("Wrote %d messages in %v (%.2f msg/sec)",
			messageCount, duration, float64(messageCount)/duration.Seconds())
	}))
	defer server.Close()

	url := "ws" + server.URL[len("http"):] + "/ws"
	dialer := websocket.DefaultDialer
	conn, resp, err := dialer.Dial(url, nil)
	require.NoError(t, err)

	defer conn.Close()
	defer resp.Body.Close()

	// Read messages
	start := time.Now()

	for i := 0; i < messageCount; i++ {
		_, _, err := conn.ReadMessage()
		require.NoError(t, err)
	}

	duration := time.Since(start)
	t.Logf("Read %d messages in %v (%.2f msg/sec)",
		messageCount, duration, float64(messageCount)/duration.Seconds())
}

// TestManager_Performance tests manager performance.
func TestManager_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	manager := New()
	connectionCount := 10000

	// Test adding many connections
	start := time.Now()

	for i := 0; i < connectionCount; i++ {
		conn := &Connection{Conn: nil} // Use nil to avoid close issues
		manager.AddWebsocketConnection("conn"+string(rune(i)), conn)
	}

	addDuration := time.Since(start)
	t.Logf("Added %d connections in %v (%.2f conn/sec)",
		connectionCount, addDuration, float64(connectionCount)/addDuration.Seconds())

	// Test listing connections
	start = time.Now()
	connections := manager.ListConnections()
	listDuration := time.Since(start)

	assert.Len(t, connections, connectionCount)
	t.Logf("Listed %d connections in %v (%.2f conn/sec)",
		len(connections), listDuration, float64(len(connections))/listDuration.Seconds())

	// Test getting connections
	start = time.Now()

	for i := 0; i < connectionCount; i++ {
		conn := manager.GetWebsocketConnection("conn" + string(rune(i)))
		assert.NotNil(t, conn)
	}

	getDuration := time.Since(start)
	t.Logf("Retrieved %d connections in %v (%.2f conn/sec)",
		connectionCount, getDuration, float64(connectionCount)/getDuration.Seconds())

	// Test closing connections
	start = time.Now()

	for i := 0; i < connectionCount; i++ {
		manager.CloseConnection("conn" + string(rune(i)))
	}

	closeDuration := time.Since(start)
	t.Logf("Closed %d connections in %v (%.2f conn/sec)",
		connectionCount, closeDuration, float64(connectionCount)/closeDuration.Seconds())

	// Verify all connections are closed
	connections = manager.ListConnections()
	assert.Empty(t, connections)
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

// mockUpgraderWithError is a mock upgrader that always returns an error.
type mockUpgraderWithError struct{}

func (*mockUpgraderWithError) Upgrade(_ http.ResponseWriter, _ *http.Request, _ http.Header) (*websocket.Conn, error) {
	return nil, ErrUpgradeFailed
}
