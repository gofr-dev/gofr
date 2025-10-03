package websocket

import (
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

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

func TestConnection_Bind_Success(t *testing.T) {
	upgrader := websocket.Upgrader{}

	tests := []struct {
		name         string
		inputMessage []byte
		expectedData any
	}{
		{
			name:         "Bind to string",
			inputMessage: []byte("Hello, WebSocket"),
			expectedData: "Hello, WebSocket",
		},
		{
			name:         "Bind to JSON struct",
			inputMessage: []byte(`{"key":"value"}`),
			expectedData: map[string]any{"key": "value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := upgrader.Upgrade(w, r, nil)
				assert.NoError(t, err)

				defer conn.Close()

				wsConn := &Connection{Conn: conn}

				var data any

				switch tt.expectedData.(type) {
				case string:
					data = new(string)
				default:
					data = &map[string]any{}
				}

				err = wsConn.Bind(data)
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedData, dereference(data))
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
		})

		// waiting for previous connection to close and test for new testcase.
		time.Sleep(500 * time.Millisecond)
	}
}

func TestNewWSUpgrader_WithOptions(t *testing.T) {
	errorHandler := func(_ http.ResponseWriter, _ *http.Request, _ int, _ error) {}

	checkOrigin := func(_ *http.Request) bool {
		return true
	}

	options := []Options{
		WithReadBufferSize(1024),
		WithWriteBufferSize(1024),
		WithHandshakeTimeout(500 * time.Millisecond),
		WithSubprotocols("protocol1", "protocol2"),
		WithCompression(),
		WithError(errorHandler),
		WithCheckOrigin(checkOrigin),
	}

	upgrader := NewWSUpgrader(options...)
	actualUpgrader := upgrader.Upgrader.(*websocket.Upgrader)

	assert.Equal(t, 1024, actualUpgrader.ReadBufferSize)
	assert.Equal(t, 1024, actualUpgrader.WriteBufferSize)
	assert.Equal(t, 500*time.Millisecond, actualUpgrader.HandshakeTimeout)
	assert.Equal(t, []string{"protocol1", "protocol2"}, actualUpgrader.Subprotocols)
	assert.True(t, actualUpgrader.EnableCompression)
	assert.NotNil(t, actualUpgrader.Error)
	assert.NotNil(t, actualUpgrader.CheckOrigin)
}

func Test_Upgrade(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUpgrader := NewMockUpgrader(ctrl)

	expectedConn := &websocket.Conn{}
	mockUpgrader.EXPECT().Upgrade(gomock.Any(), gomock.Any(), gomock.Any()).Return(expectedConn, nil)

	wsUpgrader := WSUpgrader{Upgrader: mockUpgrader}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	require.NoError(t, err)

	w := httptest.NewRecorder()

	conn, err := wsUpgrader.Upgrade(w, req, nil)
	require.NoError(t, err)

	assert.Equal(t, expectedConn, conn)
}

func Test_UnimplementedMethods(t *testing.T) {
	conn := &Connection{}

	assert.Empty(t, conn.Param("test"))
	assert.Empty(t, conn.PathParam("test"))
	assert.Empty(t, conn.HostName())
	assert.NotNil(t, conn.Context())
	assert.Nil(t, conn.Params("test"))
}

func dereference(v any) any {
	switch v := v.(type) {
	case *string:
		return *v
	case *map[string]any:
		return *v
	default:
		return v
	}
}

func TestConcurrentWriteMessageCalls(t *testing.T) {
	upgrader := websocket.Upgrader{}

	const message = "this is a test message"

	loop := 10
	workers := 10

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		assert.NoError(t, err)

		defer conn.Close()

		wc := &Connection{Conn: conn}

		wg := sync.WaitGroup{}

		for range loop {
			for range workers {
				wg.Add(1)

				go func() {
					defer wg.Done()

					if err := wc.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
						t.Errorf("concurrently wc.WriteMessage() returned %v", err)
					}
				}()
			}
		}

		wg.Wait()
	}))

	server.Close()
}

func TestManager_ListConnections(t *testing.T) {
	manager := New()

	// Add mock connections
	manager.AddWebsocketConnection("conn1", &Connection{Conn: &websocket.Conn{}})
	manager.AddWebsocketConnection("conn2", &Connection{Conn: &websocket.Conn{}})
	manager.AddWebsocketConnection("conn3", &Connection{Conn: &websocket.Conn{}})

	// Get the list of connections
	connections := manager.ListConnections()

	assert.ElementsMatch(t, []string{"conn1", "conn2", "conn3"}, connections)
}

func TestManager_GetConnectionByServiceName(t *testing.T) {
	manager := New()

	mockConn := &Connection{Conn: &websocket.Conn{}}
	manager.AddWebsocketConnection("testService", mockConn)

	retrievedConn := manager.GetConnectionByServiceName("testService")

	assert.Equal(t, mockConn, retrievedConn)
}

func TestManager_CloseConnection(t *testing.T) {
	manager := New()

	mockConn := &Connection{
		Conn: &websocket.Conn{},
	}
	mockConn.Conn = nil

	manager.AddWebsocketConnection("testConn", mockConn)

	assert.NotNil(t, manager.GetWebsocketConnection("testConn"))

	manager.CloseConnection("testConn")

	assert.Nil(t, manager.GetWebsocketConnection("testConn"))
}
