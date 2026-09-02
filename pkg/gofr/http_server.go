package gofr

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/middleware"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/websocket"
)

type httpServer struct {
	router *gofrHTTP.Router
	port   int
	ws     *websocket.Manager
	// srvMu guards srv and stopped. run() writes them on the serve goroutine and
	// Shutdown() on the caller goroutine, and the two decide between them — under
	// this lock — whether the server ever comes up. See the note on stopped.
	srvMu sync.Mutex
	srv   *http.Server
	// stopped records that Shutdown has been called. It is sticky: once set, run()
	// will not bring a listener up. srv == nil means "not started", which is not
	// the same as "will not start" — without this flag a Shutdown that lands before
	// run() publishes srv reports success and stops nothing, and the listener that
	// follows serves with no way left to stop it.
	stopped     bool
	certFile    string
	keyFile     string
	staticFiles map[string]string
}

var (
	errInvalidCertificateFile = errors.New("invalid certificate file")
	errInvalidKeyFile         = errors.New("invalid key file")
)

// logRouterChoice reports the route matcher the router resolved to.
//
// It stays quiet for the default, which every service gets and nobody needs told
// about. It speaks up for the two cases that are worth a line: the opt-in matcher
// being active, and a GOFR_ROUTER value that was not understood — the latter
// falls back to mux, which looks exactly like never having set the variable, so a
// typo would otherwise cost the opt-in with nothing said.
func logRouterChoice(logger logging.Logger, r *gofrHTTP.Router) {
	requested := os.Getenv(gofrHTTP.RouterEnvVar)
	if requested == "" {
		return
	}

	if !strings.EqualFold(requested, r.Matcher()) {
		logger.Warnf("unrecognized %s value %q, using the %q router; valid values are %q and %q",
			gofrHTTP.RouterEnvVar, requested, r.Matcher(), gofrHTTP.MatcherMux, gofrHTTP.MatcherTrie)

		return
	}

	logger.Infof("HTTP route matcher: %s", r.Matcher())
}

func newHTTPServer(c *container.Container, port int, middlewareConfigs middleware.Config) *httpServer {
	r := gofrHTTP.NewRouter()

	logRouterChoice(c.Logger, r)

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

	if s.stopped {
		s.srvMu.Unlock()
		c.Logf("Server was shut down before it started on port: %d", s.port)

		return
	}

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
	s.stopped = true
	srv := s.srv
	s.srvMu.Unlock()

	// Nothing has been published yet, so there is nothing to stop — and because
	// stopped was set under the same lock, run() will not start one either.
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
