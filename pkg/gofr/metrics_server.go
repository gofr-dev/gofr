package gofr

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/metrics"
)

type metricServer struct {
	port int
	srv  *http.Server
}

func newMetricServer(port int) *metricServer {
	return &metricServer{port: port}
}

func (m *metricServer) Run(c *container.Container) {
	if m != nil {
		c.Logf("Starting metrics server on port: %d", m.port)

		m.srv = &http.Server{
			Addr:              fmt.Sprintf(":%d", m.port),
			Handler:           metrics.GetHandler(c.Metrics()),
			ReadHeaderTimeout: 5 * time.Second,
		}

		err := m.srv.ListenAndServe()

		if !errors.Is(err, http.ErrServerClosed) {
			c.Errorf("error while listening to metrics server, err: %v", err)
		}
	}
}

func (m *metricServer) Shutdown(ctx context.Context) error {
	if m.srv == nil {
		return nil
	}

	return ShutdownWithContext(ctx, func(ctx context.Context) error {
		return m.srv.Shutdown(ctx)
	}, nil)
}
