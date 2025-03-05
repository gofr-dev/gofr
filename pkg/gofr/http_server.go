package gofr

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"time"

	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/middleware"
	"gofr.dev/pkg/gofr/websocket"
)

type httpServer struct {
	router      *gofrHTTP.Router
	port        int
	ws          *websocket.Manager
	srv         *http.Server
	certFile    string
	keyFile     string
	staticFiles map[string]string
}

var (
	errInvalidCertificateFile = errors.New("invalid certificate file")
	errInvalidKeyFile         = errors.New("invalid key file")
)

func newHTTPServer(port int) *httpServer {
	r := gofrHTTP.NewRouter()
	wsManager := websocket.New()

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

func (s *httpServer) run(c *container.Container, middlewareConfigs map[string]string) {
	// Developer Note:
	//	WebSocket connections do not inherently support authentication mechanisms.
	//	It is recommended to authenticate users before upgrading to a WebSocket connection.
	//	Hence, we are registering middlewares here, to ensure that authentication or other
	//	middleware logic is executed during the initial HTTP handshake request, prior to upgrading
	//	the connection to WebSocket, if any.
	s.router.Use(
		middleware.WSHandlerUpgrade(c, s.ws),
		middleware.Tracer,
		middleware.CORS(middlewareConfigs, s.router.RegisteredRoutes),
		middleware.Logging(c.Logger),
		middleware.Metrics(c.Metrics()),
	)

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

	// If both certFile and keyFile are provided, validate and run HTTPS server
	if s.certFile != "" && s.keyFile != "" {
		if err := validateCertificateAndKeyFiles(s.certFile, s.keyFile); err != nil {
			c.Error(err)
			return
		}

		// Start HTTPS server with TLS
		if err := s.srv.ListenAndServeTLS(s.certFile, s.keyFile); err != nil {
			c.Errorf("error while listening to https server, err: %v", err)
		}

		return
	}

	// If no certFile/keyFile is provided, run the HTTP server
	if err := s.srv.ListenAndServe(); err != nil {
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

func validateCertificateAndKeyFiles(certificateFile, keyFile string) error {
	if _, err := os.Stat(certificateFile); os.IsNotExist(err) {
		return fmt.Errorf("%w : %v", errInvalidCertificateFile, certificateFile)
	}

	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		return fmt.Errorf("%w : %v", errInvalidKeyFile, keyFile)
	}

	return nil
}
