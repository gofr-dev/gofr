package metrics

import (
	"net/http"
	"runtime"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func GetHandler() http.Handler {
	var router = mux.NewRouter()

	// Prometheus
	router.NewRoute().Methods(http.MethodGet).Path("/metrics").Handler(systemMetricsHandler(promhttp.Handler()))

	return router
}

func systemMetricsHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var stats runtime.MemStats

		runtime.ReadMemStats(&stats)

		m := GetMetricsManager()
		m.SetGauge("app_go_routines", float64(runtime.NumGoroutine()))
		m.SetGauge("app_sys_memory_alloc", float64(stats.Alloc))
		m.SetGauge("app_sys_total_alloc", float64(stats.TotalAlloc))
		m.SetGauge("app_go_numGC", float64(stats.NumGC))
		m.SetGauge("app_go_sys", float64(stats.Sys))

		next.ServeHTTP(w, r)
	})
}
