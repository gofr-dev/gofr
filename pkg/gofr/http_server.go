package gofr

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"
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
	srvMu       sync.Mutex // guards srv, which run() writes on the serve goroutine and Shutdown() reads on the caller goroutine
	srv         *http.Server
	certFile    string
	keyFile     string
	staticFiles map[string]string
}

var (
	errInvalidCertificateFile = errors.New("invalid certificate file")
	errInvalidKeyFile         = errors.New("invalid key file")
)

func newHTTPServer(c *container.Container, port int, middlewareConfigs middleware.Config) *httpServer {
	r := gofrHTTP.NewRouter()
	wsManager := websocket.New()

	r.Use(
		middleware.Tracer,
		middleware.Logging(middlewareConfigs.LogProbes, c.Logger),
		middleware.CORS(middlewareConfigs.CorsHeaders, r.RegisteredRoutes),
		middleware.Metrics(c.Metrics()),
	)

	return &httpServer{
		router: r,
		port:   port,
		ws:     wsManager,
	}
}

func (s *httpServer) run(c *container.Container) {
	// Developer Note:
	//	WebSocket connections do not inherently support authentication mechanisms.
	//	It is recommended to authenticate users before upgrading to a WebSocket connection.
	//	Hence, we are registering websocket middleware here, to ensure that authentication or other
	//	middleware logic is executed during the initial HTTP handshake request, prior to upgrading
	//	the connection to WebSocket, if any.
	s.router.Use(
		middleware.WSHandlerUpgrade(c, s.ws),
	)

	s.srvMu.Lock()

	if s.srv != nil {
		s.srvMu.Unlock()
		c.Logf("Server already running on port: %d", s.port)

		return
	}

	c.Logf("Starting server on port: %d", s.port)

	// Assign under the lock, then operate on the local copy so the blocking
	// ListenAndServe call below never holds it. Shutdown reads srv under the
	// same lock, so the two goroutines no longer race on the field.
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", s.port),
		Handler:           s.router,
		ReadHeaderTimeout: 5 * time.Second,
	}
	s.srv = srv
	s.srvMu.Unlock()

	// If both certFile and keyFile are provided, validate and run HTTPS server
	if s.certFile != "" && s.keyFile != "" {
		if err := validateCertificateAndKeyFiles(s.certFile, s.keyFile); err != nil {
			c.Error(err)
			return
		}

		// Start HTTPS server with TLS
		if err := srv.ListenAndServeTLS(s.certFile, s.keyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
			c.Errorf("error while listening to https server, err: %v", err)
		}

		return
	}

	// If no certFile/keyFile is provided, run the HTTP server
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		c.Errorf("error while listening to http server, err: %v", err)
	}
}

func (s *httpServer) Shutdown(ctx context.Context) error {
	s.srvMu.Lock()
	srv := s.srv
	s.srvMu.Unlock()

	if srv == nil {
		return nil
	}

	return ShutdownWithContext(ctx, func(ctx context.Context) error {
		return srv.Shutdown(ctx)
	}, func() error {
		return srv.Close()
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
