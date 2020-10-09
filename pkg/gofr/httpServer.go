package gofr

import (
	"fmt"
	"net/http"

	http2 "github.com/vikash/gofr/pkg/gofr/http"
)

type httpServer struct {
	router *http2.Router
	port   int
}

func (s *httpServer) Run(container *Container) {
	var srv *http.Server

	container.Logf("Starting server on port: %d\n", s.port)

	srv = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: s.router,
	}

	container.Error(srv.ListenAndServe())
}
