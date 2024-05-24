package gofr

import (
	"context"
	"fmt"
	"net/http"
	"time"

	gorillaWebsocket "github.com/gorilla/websocket"
	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/websocket"
)

type httpServer struct {
	router     *gofrHTTP.Router
	port       int
	wsUpgrader websocket.Upgrader
	logger     logging.Logger
}

func newHTTPServer(c *container.Container, port int) *httpServer {
	return &httpServer{
		router:     gofrHTTP.NewRouter(c),
		port:       port,
		wsUpgrader: websocket.NewWsUpgrader(),
		logger:     c.Logger,
	}
}

func (s *httpServer) Run(c *container.Container) {
	var srv *http.Server

	c.Logf("Starting server on port: %d", s.port)

	srv = &http.Server{
		Addr:              fmt.Sprintf(":%d", s.port),
		Handler:           s.router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	c.Error(srv.ListenAndServe())
}

func (s *httpServer) WSConnectionCreate() func(inner http.Handler) http.Handler {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if gorillaWebsocket.IsWebSocketUpgrade(r) {
				conn, err := s.wsUpgrader.Upgrade(w, r, nil)
				if err != nil {
					s.logger.Errorf("Failed to upgrade to WebSocket: %v", err)
					http.Error(w, "Could not open WebSocket connection", http.StatusBadRequest)

					return
				}

				ctx := context.WithValue(r.Context(), websocket.WebsocketKey, conn)
				r = r.WithContext(ctx)

				inner.ServeHTTP(w, r)
			}
		})
	}
}

func (s *httpServer) wrapHandler(h Handler, c *container.Container) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := &Context{
			Context:   r.Context(),
			Request:   gofrHTTP.NewRequest(r),
			Container: c,
			responder: gofrHTTP.NewResponder(w, r.Method),
		}

		// Check if it's a WebSocket connection
		if conn, ok := ctx.Context.Value(websocket.WebsocketKey).(gorillaWebsocket.Conn); ok {
			// Handle WebSocket connection
			result, err := h(ctx)
			if err != nil {
				s.logger.Errorf("WebSocket handler error: %v", err)
				conn.Close()
				return
			}

			if result != nil {
				err := conn.WriteJSON(result)
				if err != nil {
					s.logger.Errorf("Error writing WebSocket response: %v", err)
					conn.Close()
					return
				}
			}
		}
	})
}
