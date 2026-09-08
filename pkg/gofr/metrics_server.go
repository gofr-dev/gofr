package gofr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/metrics"
)

type metricServer struct {
	port  int
	srvMu sync.Mutex // guards srv, written by Run on the serve goroutine and read by Shutdown on the caller goroutine
	srv   *http.Server
}

func newMetricServer(port int) *metricServer {
	return &metricServer{port: port}
}

func (m *metricServer) Run(c *container.Container) {
	if m != nil {
		c.Logf("Starting metrics server on port: %d", m.port)

		// Assign under the lock, then serve on the local copy so the blocking
		// ListenAndServe call never holds it while Shutdown reads srv.
		srv := &http.Server{
			Addr:              fmt.Sprintf(":%d", m.port),
			Handler:           newMetricsMux(c),
			ReadHeaderTimeout: 5 * time.Second,
		}

		m.srvMu.Lock()
		m.srv = srv
		m.srvMu.Unlock()

		err := srv.ListenAndServe()

		if !errors.Is(err, http.ErrServerClosed) {
			c.Errorf("error while listening to metrics server, err: %v", err)
		}
	}
}

func (m *metricServer) Shutdown(ctx context.Context) error {
	m.srvMu.Lock()
	srv := m.srv
	m.srvMu.Unlock()

	if srv == nil {
		return nil
	}

	return ShutdownWithContext(ctx, func(ctx context.Context) error {
		return srv.Shutdown(ctx)
	}, nil)
}

// newMetricsMux builds the metrics server's router: /metrics and /debug/pprof come from
// metrics.GetHandler, and the detailed health map is served at /health for ops tooling —
// the counterpart to the redacted public /.well-known/health endpoint. The /health route
// is registered here rather than inside metrics.GetHandler so that function's exported
// signature, and the metrics package's independence from container, stay untouched.
func newMetricsMux(c *container.Container) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", detailedHealthHandler(c))
	mux.Handle("/", metrics.GetHandler(c.Metrics()))

	return mux
}

// detailedHealthHandler serves the full map computed by Container.Health — per-dependency
// details included. It lives on the metrics port behind the same network boundary as
// /metrics and /debug/pprof, never on the public HTTP port, which is what makes serving
// the unredacted map acceptable. METRICS_PORT=0 disables the whole metrics server, this
// endpoint with it.
func detailedHealthHandler(c *container.Container) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// The route carries no method matcher — deliberately, so a non-GET does not
		// fall through to the "/" catch-all and answer 404 for a path that exists.
		// GET-only matches /metrics next to it, which registers Methods(GET).
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed on health endpoint", http.StatusMethodNotAllowed)

			return
		}

		body, err := json.Marshal(c.Health(r.Context()))
		if err != nil {
			c.Errorf("failed to encode detailed health response: %v", err)
			http.Error(w, "failed to encode health response", http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		_, _ = w.Write(body)
	}
}
