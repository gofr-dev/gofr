package gofr

import (
	"fmt"
	"net/http"
	"time"

	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/logging"
)

type httpServer struct {
	router *gofrHTTP.Router
	port   int
	logger logging.Logger
}

func newHTTPServer(c *container.Container, port int) *httpServer {
	return &httpServer{
		router: gofrHTTP.NewRouter(c),
		port:   port,
		logger: c.Logger,
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
