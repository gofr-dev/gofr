package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/internal/routemark"
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

// withWebSocketRoute returns req carrying a route marked as a WebSocket route, as the router
// would after matching a request to a route registered through App.WebSocket. A request without
// such a mark — a plain route, or no matched route at all — must never be upgraded (gofr#4635).
func withWebSocketRoute(req *http.Request) *http.Request {
	route := mux.NewRouter().NewRoute().Path("/ws")

	return routemark.With(req, routemark.MarkWebSocket(route))
}

// withPlainRoute is withWebSocketRoute for a route that is not a WebSocket route.
func withPlainRoute(req *http.Request) *http.Request {
	route := mux.NewRouter().NewRoute().Path("/plain")

	return routemark.With(req, route)
}

func TestWSConnectionCreate_Error(t *testing.T) {
	mockUpgrader, wsManager := initializeWebSocketMocks(t)
	mockContainer, _ := container.NewMockContainer(t)

	mockUpgrader.EXPECT().Upgrade(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil,
		errConnection).Times(1)

	handler := WSHandlerUpgrade(mockContainer, wsManager)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
	}))

	// Create a test request with incomplete upgrade header
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/ws", http.NoBody)
	req.Header.Set("Connection", "upgrade")
	req.Header.Set("Upgrade", "websocket")

	// The request matched a WebSocket route, so the middleware must attempt the upgrade.
	req = withWebSocketRoute(req)

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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/ws", http.NoBody)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")

	rec := httptest.NewRecorder()

	handler := middleware(innerHandler)
	handler.ServeHTTP(rec, withWebSocketRoute(req))

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

	WSHandlerUpgrade(mockContainer, wsManager)(inner).ServeHTTP(rec, withWebSocketRoute(req))

	assert.False(t, called, "inner handler must not run after a failed upgrade")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	// http.Error's exact wire shape: plain text, sniffing disabled, trailing \n.
	assert.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "Could not open WebSocket connection\n", rec.Body.String())

	assert.Empty(t, wsManager.ListConnections())
}

// TestWSHandlerUpgrade_Char_UpgradeSuccess pins the success branch: the connection is
// registered under a server-generated ID, that ID is placed in the request context, and the
// inner handler still runs and owns the response. The ID must NOT be the client-supplied
// Sec-WebSocket-Key (gofr#4635): a repeated or missing key used to overwrite another client's
// connection.
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

	WSHandlerUpgrade(mockContainer, wsManager)(inner).ServeHTTP(rec, withWebSocketRoute(req))

	gotID, ok := gotCtxVal.(string)
	require.True(t, ok, "the context must carry the connection ID as a string")
	assert.NotEmpty(t, gotID, "the connection ID must not be empty")
	assert.NotEqual(t, key, gotID, "the hub key must be server-generated, not the client-supplied Sec-WebSocket-Key")

	_, err := uuid.Parse(gotID)
	require.NoError(t, err, "the connection ID must be a UUID")

	assert.Equal(t, []string{gotID}, wsManager.ListConnections())

	registered := wsManager.GetWebsocketConnection(gotID)
	if assert.NotNil(t, registered) {
		assert.Same(t, conn, registered.Conn)
	}

	assert.Equal(t, http.StatusSwitchingProtocols, rec.Code)
}

// TestWSHandlerUpgrade_Char_MissingSecWebSocketKey pins the fix for the empty-key overwrite:
// a successful upgrade without a Sec-WebSocket-Key registers the connection under a
// server-generated ID, so two such connections no longer overwrite one another. Before the fix
// both registered under the empty string (gofr#4635).
func TestWSHandlerUpgrade_Char_MissingSecWebSocketKey(t *testing.T) {
	mockUpgrader, wsManager := initializeWebSocketMocks(t)
	mockContainer, _ := container.NewMockContainer(t)

	mockUpgrader.EXPECT().Upgrade(gomock.Any(), gomock.Any(), gomock.Any()).Return(&websocket.Conn{}, nil).Times(2)

	var gotIDs []string

	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotIDs = append(gotIDs, r.Context().Value(gofrWebSocket.WSConnectionKey).(string))
	})

	for i := 0; i < 2; i++ {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/ws", http.NoBody)
		req.Header.Set("Connection", "Upgrade")
		req.Header.Set("Upgrade", "websocket")

		WSHandlerUpgrade(mockContainer, wsManager)(inner).ServeHTTP(httptest.NewRecorder(), withWebSocketRoute(req))
	}

	require.Len(t, gotIDs, 2)
	assert.NotEmpty(t, gotIDs[0], "the first connection must get a server-generated ID")
	assert.NotEmpty(t, gotIDs[1], "the second connection must get a server-generated ID")
	assert.NotEqual(t, gotIDs[0], gotIDs[1], "two upgrades must get distinct server-generated IDs")

	for _, id := range gotIDs {
		_, err := uuid.Parse(id)
		require.NoError(t, err, "the connection ID must be a UUID")
	}

	assert.Len(t, wsManager.ListConnections(), 2, "both connections must be registered, neither overwriting the other")
}

// TestWSHandlerUpgrade_Char_UpgradeOnPlainRouteSkipped pins the fix for gofr#4635: an upgrade
// request to a route that is not a WebSocket route — or to no matched route at all — must not
// be upgraded. The request reaches the inner handler as plain HTTP, no connection is
// registered, and nothing is left to leak.
func TestWSHandlerUpgrade_Char_UpgradeOnPlainRouteSkipped(t *testing.T) {
	tests := []struct {
		name string
		mark func(*http.Request) *http.Request
	}{
		{"matched plain route", withPlainRoute},
		{"no matched route", func(req *http.Request) *http.Request { return req }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// No EXPECT() on the mock upgrader: any upgrade attempt fails the test.
			_, wsManager := initializeWebSocketMocks(t)
			mockContainer, _ := container.NewMockContainer(t)

			var called bool

			inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("plain response"))
			})

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/plain", http.NoBody)
			req.Header.Set("Connection", "Upgrade")
			req.Header.Set("Upgrade", "websocket")
			req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

			rec := httptest.NewRecorder()

			WSHandlerUpgrade(mockContainer, wsManager)(inner).ServeHTTP(rec, tc.mark(req))

			assert.True(t, called, "the inner handler must run for a plain route")
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "plain response", rec.Body.String())
			assert.Empty(t, wsManager.ListConnections(), "no connection must be registered")
		})
	}
}

// TestWSHandlerUpgrade_Char_DuplicateSecWebSocketKey pins the fix for the hub-key half of
// gofr#4635: two upgrades carrying the same Sec-WebSocket-Key must register two distinct
// connections — the hub key is server-generated, so a repeated key can no longer overwrite (or
// close) another client's connection.
func TestWSHandlerUpgrade_Char_DuplicateSecWebSocketKey(t *testing.T) {
	mockUpgrader, wsManager := initializeWebSocketMocks(t)
	mockContainer, _ := container.NewMockContainer(t)

	const key = "dGhlIHNhbXBsZSBub25jZQ=="

	conn1, conn2 := &websocket.Conn{}, &websocket.Conn{}
	mockUpgrader.EXPECT().Upgrade(gomock.Any(), gomock.Any(), gomock.Any()).Return(conn1, nil).Times(1)
	mockUpgrader.EXPECT().Upgrade(gomock.Any(), gomock.Any(), gomock.Any()).Return(conn2, nil).Times(1)

	var gotIDs []string

	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotIDs = append(gotIDs, r.Context().Value(gofrWebSocket.WSConnectionKey).(string))
	})

	for i := 0; i < 2; i++ {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/ws", http.NoBody)
		req.Header.Set("Connection", "Upgrade")
		req.Header.Set("Upgrade", "websocket")
		req.Header.Set("Sec-WebSocket-Key", key)

		WSHandlerUpgrade(mockContainer, wsManager)(inner).ServeHTTP(httptest.NewRecorder(), withWebSocketRoute(req))
	}

	require.Len(t, gotIDs, 2)
	assert.NotEqual(t, gotIDs[0], gotIDs[1], "the two upgrades must get distinct IDs")
	assert.NotEqual(t, key, gotIDs[0], "the hub key must be server-generated, not the client key")
	assert.NotEqual(t, key, gotIDs[1], "the hub key must be server-generated, not the client key")
	assert.Len(t, wsManager.ListConnections(), 2, "both connections must be registered")

	assert.Same(t, conn1, wsManager.GetWebsocketConnection(gotIDs[0]).Conn)
	assert.Same(t, conn2, wsManager.GetWebsocketConnection(gotIDs[1]).Conn)
}
