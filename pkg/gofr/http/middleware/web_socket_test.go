package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
	gofrWebSocket "gofr.dev/pkg/gofr/websocket"
)

var errConnection = errors.New("can't create connection")

func initializeWebSocketMocks(t *testing.T) (gofrWebSocket.MockUpgrader, *gofrWebSocket.Manager) {
	t.Helper()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockUpgrader := gofrWebSocket.NewMockUpgrader(mockCtrl)

	wsManager := gofrWebSocket.New()
	wsManager.WebSocketUpgrader = &gofrWebSocket.WSUpgrader{Upgrader: mockUpgrader}

	return *mockUpgrader, wsManager
}

func TestWSConnectionCreate_Error(t *testing.T) {
	mockUpgrader, wsManager := initializeWebSocketMocks(t)
	mockContainer, _ := container.NewMockContainer(t)

	mockUpgrader.EXPECT().Upgrade(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil,
		errConnection).Times(1)

	handler := WSHandlerUpgrade(mockContainer, wsManager)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
	}))

	// Create a test request with incomplete upgrade header
	req := httptest.NewRequest(http.MethodGet, "/ws", http.NoBody)
	req.Header.Set("Connection", "upgrade")
	req.Header.Set("Upgrade", "websocket")

	// Serve the request through the middleware
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	// No response expected, status code should be 400 (Bad Request)
	if status := recorder.Code; status != http.StatusBadRequest {
		t.Errorf("Unexpected status code: %d", status)
	}
}

func Test_WSConnectionCreate_Success(t *testing.T) {
	mockUpgrader, wsManager := initializeWebSocketMocks(t)
	mockContainer, _ := container.NewMockContainer(t)

	mockConn := &gofrWebSocket.Connection{
		Conn: &websocket.Conn{},
	}

	mockUpgrader.EXPECT().Upgrade(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockConn.Conn, nil).Times(1)

	middleware := WSHandlerUpgrade(mockContainer, wsManager)

	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/ws", http.NoBody)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")

	rec := httptest.NewRecorder()

	handler := middleware(innerHandler)
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// ---------------------------------------------------------------------------
// Characterization suite for WSHandlerUpgrade.
//
// Pins the observable behavior of the middleware for both branches: a
// non-upgrade request must pass through completely untouched, and an upgrade
// request must be handed to the upgrader with the resulting connection
// registered and its key placed in the request context.
// ---------------------------------------------------------------------------

// TestWSHandlerUpgrade_Char_NonUpgradePassesThrough pins that a request without
// websocket upgrade headers reaches the inner handler unmodified: no upgrade is
// attempted, the context carries no connection key, no connection is
// registered, and the inner handler's status/headers/body are what the client
// sees.
func TestWSHandlerUpgrade_Char_NonUpgradePassesThrough(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
	}{
		{"no-headers", nil},
		{"connection-only", map[string]string{"Connection": "upgrade"}},
		{"upgrade-only", map[string]string{"Upgrade": "websocket"}},
		{"wrong-upgrade-protocol", map[string]string{"Connection": "upgrade", "Upgrade": "h2c"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// No EXPECT() is registered on the mock upgrader: gomock fails the
			// test if Upgrade is called at all on this path.
			_, wsManager := initializeWebSocketMocks(t)
			mockContainer, _ := container.NewMockContainer(t)

			var (
				called    bool
				gotCtxVal any
				gotMethod string
				gotPath   string
			)

			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				gotCtxVal = r.Context().Value(gofrWebSocket.WSConnectionKey)
				gotMethod = r.Method
				gotPath = r.URL.Path

				w.Header().Set("X-Inner", "yes")
				w.WriteHeader(http.StatusTeapot)
				_, _ = w.Write([]byte("inner body"))
			})

			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/plain?a=b", http.NoBody)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}

			rec := httptest.NewRecorder()

			WSHandlerUpgrade(mockContainer, wsManager)(inner).ServeHTTP(rec, req)

			assert.True(t, called, "inner handler must be invoked")
			assert.Nil(t, gotCtxVal, "no websocket key must be added to the context")
			assert.Equal(t, http.MethodPost, gotMethod)
			assert.Equal(t, "/plain", gotPath)

			// The inner handler's response is passed through byte for byte.
			assert.Equal(t, http.StatusTeapot, rec.Code)
			assert.Equal(t, "yes", rec.Header().Get("X-Inner"))
			assert.Equal(t, "inner body", rec.Body.String())

			assert.Empty(t, wsManager.ListConnections(), "no connection must be registered")
		})
	}
}

// TestWSHandlerUpgrade_Char_UpgradeFailure pins the exact response written when
// the upgrader fails: 400 with net/http's plain-text error envelope, and the
// inner handler is NOT invoked.
func TestWSHandlerUpgrade_Char_UpgradeFailure(t *testing.T) {
	mockUpgrader, wsManager := initializeWebSocketMocks(t)
	mockContainer, _ := container.NewMockContainer(t)

	mockUpgrader.EXPECT().Upgrade(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errConnection).Times(1)

	var called bool

	inner := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/ws", http.NoBody)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")

	rec := httptest.NewRecorder()

	WSHandlerUpgrade(mockContainer, wsManager)(inner).ServeHTTP(rec, req)

	assert.False(t, called, "inner handler must not run after a failed upgrade")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	// http.Error's exact wire shape: plain text, sniffing disabled, trailing \n.
	assert.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "Could not open WebSocket connection\n", rec.Body.String())

	assert.Empty(t, wsManager.ListConnections())
}

// TestWSHandlerUpgrade_Char_UpgradeSuccess pins the success branch: the
// connection is registered under the Sec-WebSocket-Key, that key is placed in
// the request context, and the inner handler still runs and owns the response.
func TestWSHandlerUpgrade_Char_UpgradeSuccess(t *testing.T) {
	mockUpgrader, wsManager := initializeWebSocketMocks(t)
	mockContainer, _ := container.NewMockContainer(t)

	conn := &websocket.Conn{}
	mockUpgrader.EXPECT().Upgrade(gomock.Any(), gomock.Any(), gomock.Any()).Return(conn, nil).Times(1)

	const key = "dGhlIHNhbXBsZSBub25jZQ=="

	var gotCtxVal any

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCtxVal = r.Context().Value(gofrWebSocket.WSConnectionKey)

		w.WriteHeader(http.StatusSwitchingProtocols)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/ws", http.NoBody)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Key", key)

	rec := httptest.NewRecorder()

	WSHandlerUpgrade(mockContainer, wsManager)(inner).ServeHTTP(rec, req)

	assert.Equal(t, key, gotCtxVal, "the Sec-WebSocket-Key must be in the inner request's context")
	assert.Equal(t, []string{key}, wsManager.ListConnections())

	registered := wsManager.GetWebsocketConnection(key)
	if assert.NotNil(t, registered) {
		assert.Equal(t, conn, registered.Conn)
	}

	assert.Equal(t, http.StatusSwitchingProtocols, rec.Code)
}

// TestWSHandlerUpgrade_Char_MissingSecWebSocketKey pins that a successful
// upgrade without a Sec-WebSocket-Key registers the connection under the EMPTY
// string — so two such connections would overwrite one another. Reported, not
// fixed.
func TestWSHandlerUpgrade_Char_MissingSecWebSocketKey(t *testing.T) {
	mockUpgrader, wsManager := initializeWebSocketMocks(t)
	mockContainer, _ := container.NewMockContainer(t)

	mockUpgrader.EXPECT().Upgrade(gomock.Any(), gomock.Any(), gomock.Any()).Return(&websocket.Conn{}, nil).Times(1)

	var gotCtxVal any

	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotCtxVal = r.Context().Value(gofrWebSocket.WSConnectionKey)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/ws", http.NoBody)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")

	WSHandlerUpgrade(mockContainer, wsManager)(inner).ServeHTTP(httptest.NewRecorder(), req)

	assert.NotNil(t, gotCtxVal, "the key is present in the context, it is just empty")
	assert.Empty(t, gotCtxVal)
	assert.Equal(t, []string{""}, wsManager.ListConnections())
}
