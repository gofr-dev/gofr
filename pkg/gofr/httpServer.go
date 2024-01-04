package gofr

import (
	"fmt"
	"net/http"
	"time"

	http2 "gofr.dev/pkg/gofr/http"
)

type httpServer struct {
	router *http2.Router
	port   int
}

func (s *httpServer) Run(container *Container) {
	var srv *http.Server

	container.Logf("Starting server on port: %d\n", s.port)

	srv = &http.Server{
		Addr:              fmt.Sprintf(":%d", s.port),
		Handler:           s.router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	container.Error(srv.ListenAndServe())
}
