package gofr

import (
	"fmt"
	"net/http"
	"time"

	"gofr.dev/pkg/gofr/http/middleware"

	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
)

type httpServer struct {
	router *gofrHTTP.Router
	port   int
}

func newHTTPServer(c *container.Container, port int, middlewareConfigs map[string]string) *httpServer {
	r := gofrHTTP.NewRouter()

	r.Use(
		middleware.WSHandlerUpgrade(c),
		middleware.Tracer,
		middleware.Logging(c.Logger),
		middleware.CORS(middlewareConfigs, r.RegisteredRoutes),
		middleware.Metrics(c.Metrics()),
	)

	return &httpServer{
		router: r,
		port:   port,
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
