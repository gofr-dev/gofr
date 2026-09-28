package gofr

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/metrics"
)

type metricServer struct {
	port int
	// srvMu guards srv and stopped; see the note on httpServer.stopped for why a
	// nil srv alone cannot tell Shutdown whether a start is still coming.
	srvMu   sync.Mutex
	srv     *http.Server
	stopped bool
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
			Handler:           metrics.GetHandler(c.Metrics()),
			ReadHeaderTimeout: 5 * time.Second,
		}

		m.srvMu.Lock()

		if m.stopped {
			m.srvMu.Unlock()
			c.Logf("Metrics server was shut down before it started on port: %d", m.port)

			return
		}

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
	m.stopped = true
	srv := m.srv
	m.srvMu.Unlock()

	if srv == nil {
		return nil
	}

	return ShutdownWithContext(ctx, func(ctx context.Context) error {
		return srv.Shutdown(ctx)
	}, nil)
}
