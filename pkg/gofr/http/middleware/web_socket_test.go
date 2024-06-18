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

	req := httptest.NewRequest("GET", "/ws", http.NoBody)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")

	rec := httptest.NewRecorder()

	handler := middleware(innerHandler)
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}
