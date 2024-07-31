package gofr

import (
	"context"
	"errors"
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
	srv    *http.Server
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

// RegisterProfilingRoutes registers pprof endpoints on the HTTP server.
//
// This method adds the following routes to the server's router:
//
//   - /debug/pprof/cmdline
//   - /debug/pprof/profile
//   - /debug/pprof/symbol
//   - /debug/pprof/trace
//   - /debug/pprof/ (index)
//
// These endpoints provide various profiling information for the application,
// such as command-line arguments, memory profiles, symbol information, and
// execution traces.
func (s *httpServer) RegisterProfilingRoutes() {
	s.router.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	s.router.HandleFunc("/debug/pprof/profile", pprof.Profile)
	s.router.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	s.router.HandleFunc("/debug/pprof/trace", pprof.Trace)

	s.router.NewRoute().Methods(http.MethodGet).PathPrefix("/debug/pprof/").HandlerFunc(pprof.Index)
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

	err := s.srv.ListenAndServe()

	if !errors.Is(err, http.ErrServerClosed) {
		c.Errorf("error while listening to http server, err: %v", err)
	}
}

func (s *httpServer) Shutdown(ctx context.Context) error {
	if s.srv == nil {
		return nil
	}

	return ShutdownWithContext(ctx, func(ctx context.Context) error {
		return s.srv.Shutdown(ctx)
	}, func() error {
		if err := s.srv.Close(); err != nil {
			return err
		}

		return nil
	})
}
