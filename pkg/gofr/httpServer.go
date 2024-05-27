package gofr

import (
	"fmt"
	"net/http"
	"time"

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
