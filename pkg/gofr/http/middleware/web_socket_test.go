package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"gofr.dev/pkg/gofr/container"
	gofrWebSocket "gofr.dev/pkg/gofr/websocket"
)

func TestWSConnectionCreate_Error(t *testing.T) {
	mockContainer, _ := container.NewMockContainer(t)

	handler := WSConnectionCreate(mockContainer)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if connection is in context
		conn, ok := r.Context().Value(gofrWebSocket.WSKey).(*websocket.Conn)
		if ok {
			t.Errorf("Didn't Expected WebSocket connection in context, but got one")
		}

		if assert.Nil(t, conn) {
			t.Errorf("Expected nil connection in context, but got some different connection")
		}
	}))

	// Create a test request with incomplete upgrade header
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
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
