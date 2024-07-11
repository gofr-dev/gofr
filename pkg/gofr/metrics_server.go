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
	srvr *http.Server
}

func newMetricServer(port int) *metricServer {
	return &metricServer{port: port}
}

func (m *metricServer) Run(c *container.Container) {
	if m.srvr != nil {
		c.Logf("Server already running on port: %d", m.port)

		return
	}

	if m != nil {
		c.Logf("Starting metrics server on port: %d", m.port)

		m.srvr = &http.Server{
			Addr:              fmt.Sprintf(":%d", m.port),
			Handler:           metrics.GetHandler(c.Metrics()),
			ReadHeaderTimeout: 5 * time.Second,
		}

		c.Error(m.srvr.ListenAndServe())
	}
}

func (m *metricServer) Shutdown(ctx context.Context) error {
	if m.srvr == nil {
		return nil
	}

	err := m.srvr.Shutdown(ctx)
	m.srvr = nil

	return err
}
