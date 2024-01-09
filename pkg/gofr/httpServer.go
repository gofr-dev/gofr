package gofr

import (
	"fmt"
	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"net/http"
	"time"
)

type httpServer struct {
	router *gofrHTTP.Router
	port   int
}

func newHTTPServer(container *container.Container, port int) *httpServer {
	return &httpServer{
		router: gofrHTTP.NewRouter(container),
		port:   port,
	}
}

func (s *httpServer) Run(container *container.Container) {
	var srv *http.Server

	container.Logf("Starting server on port: %d", s.port)

	srv = &http.Server{
		Addr:              fmt.Sprintf(":%d", s.port),
		Handler:           s.router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	container.Error(srv.ListenAndServe())
}
