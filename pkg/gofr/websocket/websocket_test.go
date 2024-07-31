package websocket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

func TestConnection_Bind_Success(t *testing.T) {
	upgrader := websocket.Upgrader{}

	tests := []struct {
		name         string
		inputMessage []byte
		expectedData interface{}
	}{
		{
			name:         "Bind to string",
			inputMessage: []byte("Hello, WebSocket"),
			expectedData: "Hello, WebSocket",
		},
		{
			name:         "Bind to JSON struct",
			inputMessage: []byte(`{"key":"value"}`),
			expectedData: map[string]interface{}{"key": "value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := upgrader.Upgrade(w, r, nil)
				assert.NoError(t, err)
				defer conn.Close()

				wsConn := &Connection{Conn: conn}

				var data interface{}
				switch tt.expectedData.(type) {
				case string:
					data = new(string)
				default:
					data = &map[string]interface{}{}
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
		WithHandshakeTimeout(5 * time.Second),
		WithSubprotocols("protocol1", "protocol2"),
		WithCompression(),
		WithError(errorHandler),
		WithCheckOrigin(checkOrigin),
	}

	upgrader := NewWSUpgrader(options...)
	actualUpgrader := upgrader.Upgrader.(*websocket.Upgrader)

	assert.Equal(t, 1024, actualUpgrader.ReadBufferSize)
	assert.Equal(t, 1024, actualUpgrader.WriteBufferSize)
	assert.Equal(t, 5*time.Second, actualUpgrader.HandshakeTimeout)
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

	req, err := http.NewRequestWithContext(context.TODO(), http.MethodGet, "/", http.NoBody)
	require.NoError(t, err)

	w := httptest.NewRecorder()

	conn, err := wsUpgrader.Upgrade(w, req, nil)
	require.NoError(t, err)

	assert.Equal(t, expectedConn, conn)
}

func Test_UnimplementedMethods(t *testing.T) {
	conn := &Connection{}

	assert.Equal(t, "", conn.Param("test"))
	assert.Equal(t, "", conn.PathParam("test"))
	assert.Equal(t, "", conn.HostName())
	assert.NotNil(t, "", conn.Context())
}

func dereference(v interface{}) interface{} {
	switch v := v.(type) {
	case *string:
		return *v
	case *map[string]interface{}:
		return *v
	default:
		return v
	}
}
