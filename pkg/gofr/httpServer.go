package gofr

import (
	"fmt"
	"net/http"
	"net/http/pprof"
	"time"

	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/middleware"
	"gofr.dev/pkg/gofr/websocket"
)

type httpServer struct {
	router *gofrHTTP.Router
	port   int
	ws     *websocket.Manager
}

func newHTTPServer(c *container.Container, port int, middlewareConfigs map[string]string) *httpServer {
	r := gofrHTTP.NewRouter()
	wsManager := websocket.New()

	r.Use(
		middleware.WSHandlerUpgrade(c, wsManager),
		middleware.Tracer,
		middleware.Logging(c.Logger),
		middleware.CORS(middlewareConfigs, r.RegisteredRoutes),
		middleware.Metrics(c.Metrics()),
	)

	return &httpServer{
		router: r,
		port:   port,
		ws:     wsManager,
	}
}

func (s *httpServer) RegisterProfilingRoutes() {
	s.router.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	s.router.HandleFunc("/debug/pprof/profile", pprof.Profile)
	s.router.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	s.router.HandleFunc("/debug/pprof/trace", pprof.Trace)

	s.router.NewRoute().Methods(http.MethodGet).PathPrefix("/debug/pprof/").HandlerFunc(pprof.Index)
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
