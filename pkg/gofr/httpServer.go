package gofr

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
)

type httpServer struct {
	router         *gofrHTTP.Router
	port           int
	requestTimeout time.Duration
}

const defaultRequestTimeout = 5

func newHTTPServer(c *container.Container, port int, requestTimeout string) *httpServer {
	var timeout int

	timeout, err := strconv.Atoi(requestTimeout)
	if err != nil || timeout < 0 {
		c.Error("invalid value of config REQUEST_TIMEOUT. setting default value to 5 seconds.")

		timeout = defaultRequestTimeout
	}

	return &httpServer{
		router:         gofrHTTP.NewRouter(c),
		port:           port,
		requestTimeout: time.Duration(timeout) * time.Second,
	}
}

func (s *httpServer) Run(c *container.Container) {
	var srv *http.Server

	c.Logf("Starting server on port: %d", s.port)

	srv = &http.Server{
		Addr:              fmt.Sprintf(":%d", s.port),
		Handler:           http.TimeoutHandler(s.router, s.requestTimeout, "Request timed out"),
		ReadHeaderTimeout: 5 * time.Second,
	}

	c.Error(srv.ListenAndServe())
}
