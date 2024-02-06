package metrics

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func GetHandler() http.Handler {
	var router = mux.NewRouter()

	// Prometheus
	router.NewRoute().Methods(http.MethodGet).Path("/metrics").Handler(promhttp.Handler())

	return router
}
