package metrics

import (
	"net/http"
	"net/http/pprof"
	"runtime"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// GetHandler creates a new HTTP handler that serves metrics collected by the provided metrics manager to the '/metrics' route.
func GetHandler(m Manager) http.Handler {
	var (
		router   = mux.NewRouter()
		gatherer = prometheus.DefaultGatherer
	)

	if mm, ok := m.(*metricsManager); ok && mm.gatherer != nil {
		gatherer = mm.gatherer
	}

	// Prometheus
	h := systemMetricsHandler(m, promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{}))
	router.NewRoute().Methods(http.MethodGet).Path("/metrics").Handler(h)

	//   - /debug/pprof/cmdline
	//   - /debug/pprof/profile
	//   - /debug/pprof/symbol
	//   - /debug/pprof/trace
	//   - /debug/pprof/ (index)
	//
	// These endpoints provide various profiling information for the application,
	// such as command-line arguments, memory profiles, symbol information, and
	// execution traces.
	router.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	router.HandleFunc("/debug/pprof/profile", pprof.Profile)
	router.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	router.HandleFunc("/debug/pprof/trace", pprof.Trace)

	router.NewRoute().Methods(http.MethodGet).PathPrefix("/debug/pprof/").HandlerFunc(pprof.Index)

	return router
}

func systemMetricsHandler(m Manager, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var stats runtime.MemStats

		runtime.ReadMemStats(&stats)

		m.SetGauge("app_go_routines", float64(runtime.NumGoroutine()))
		m.SetGauge("app_sys_memory_alloc", float64(stats.Alloc))
		m.SetGauge("app_sys_total_alloc", float64(stats.TotalAlloc))
		m.SetGauge("app_go_numGC", float64(stats.NumGC))
		m.SetGauge("app_go_sys", float64(stats.Sys))

		next.ServeHTTP(w, r)
	})
}
