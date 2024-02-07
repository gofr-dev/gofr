package gofr

import (
	"fmt"
	"net/http"
	"time"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/metrics"
)

type metricServer struct {
	port int
}

func newMetricServer(port int) *metricServer {
	return &metricServer{port: port}
}

func (m *metricServer) Run(c *container.Container) {
	var srv *http.Server

	c.Logf("Starting metrics server on port: %d", m.port)

	srv = &http.Server{
		Addr:              fmt.Sprintf(":%d", m.port),
		Handler:           metrics.GetHandler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	c.Error(srv.ListenAndServe())
}
