package gofr

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
)

type httpServer struct {
	router *gofrHTTP.Router
	port   int
	srv    *http.Server
}

func newHTTPServer(c *container.Container, port int) *httpServer {
	return &httpServer{
		router: gofrHTTP.NewRouter(c),
		port:   port,
	}
}

func (s *httpServer) Run(c *container.Container) {
	if s.srv != nil {
		c.Logf("Server already running on port: %d", s.port)
		return
	}

	c.Logf("Starting server on port: %d", s.port)

	s.srv = &http.Server{
		Addr:              fmt.Sprintf(":%d", s.port),
		Handler:           s.router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	c.Error(s.srv.ListenAndServe())
}

func (s *httpServer) Shutdown(ctx context.Context) error {
	if s.srv == nil {
		return nil
	}

	err := s.srv.Shutdown(ctx)
	s.srv = nil
	return err
}
