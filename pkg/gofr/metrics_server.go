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
	srv  *http.Server
	mu   sync.Mutex
}

func newMetricServer(port int) *metricServer {
	return &metricServer{port: port}
}

func (m *metricServer) Run(c *container.Container) {
	if m != nil {
		c.Logf("Starting metrics server on port: %d", m.port)

		srv := &http.Server{
			Addr:              fmt.Sprintf(":%d", m.port),
			Handler:           metrics.GetHandler(c.Metrics()),
			ReadHeaderTimeout: 5 * time.Second,
		}

		m.mu.Lock()
		m.srv = srv
		m.mu.Unlock()

		err := srv.ListenAndServe()

		if !errors.Is(err, http.ErrServerClosed) {
			c.Errorf("error while listening to metrics server, err: %v", err)
		}
	}
}

func (m *metricServer) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	srv := m.srv
	m.mu.Unlock()

	if srv == nil {
		return nil
	}

	return ShutdownWithContext(ctx, func(ctx context.Context) error {
		return srv.Shutdown(ctx)
	}, nil)
}
