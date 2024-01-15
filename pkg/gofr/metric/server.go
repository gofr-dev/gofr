package metric

import (
	"net/http"
	"strconv"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	server *http.Server
	port   int
}

func NewServer(c *container.Container, port int) *Server {
	return &Server{
		server: MetricsServer(c.Logger, port),
		port:   port,
	}
}

func MetricsServer(logger logging.Logger, port int) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	//nolint:gosec // not setting ReadHeaderTimeout as of now
	var srv = &http.Server{
		Addr:    ":" + strconv.Itoa(port),
		Handler: mux,
	}

	logger.Infof("Starting metrics server on port: %v", port)

	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			logger.Infof("error in metrics server: %v", err)
		}
	}()

	PushSystemStats()

	return srv
}
