package gofr

import (
	"context"
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
	if m.srv != nil {
		c.Logf("Server already running on port: %d", m.port)
		return
	}

	if m != nil {
		c.Logf("Starting metrics server on port: %d", m.port)

		m.srv = &http.Server{
			Addr:              fmt.Sprintf(":%d", m.port),
			Handler:           metrics.GetHandler(c.Metrics()),
			ReadHeaderTimeout: 5 * time.Second,
		}

		c.Error(m.srv.ListenAndServe())
	}
}

func (m *metricServer) Shutdown(ctx context.Context) error {
	if m.srv == nil {
		return nil
	}

	err := m.srv.Shutdown(ctx)
	m.srv = nil

	return err
}
