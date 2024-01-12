package metric

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
)

//nolint:gochecknoglobals // The declared global variable can be accessed across multiple functions
var (
	httpResponse = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "gofr_http_response",
		Help:    "Histogram of HTTP response times in seconds",
		Buckets: []float64{.001, .003, .005, .01, .02, .03, .05, .1, .2, .3, .5, .75, 1, 2, 3, 5, 10, 30},
	}, []string{"path", "method", "status"})

	goRoutines = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "gofr_go_routines",
		Help: "Gauge of Go routines running",
	}, nil)

	alloc = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "gofr_sys_memory_alloc",
		Help: "Gauge of Heap allocations",
	}, nil)

	totalAlloc = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "gofr_sys_total_alloc",
		Help: "Gauge of cumulative bytes allocated for heap objects",
	}, nil)

	numGC = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "gofr_go_numGC",
		Help: "Gauge of completed GC cycles",
	}, nil)

	sys = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "gofr_go_sys",
		Help: "Gauge of total bytes of memory",
	}, nil)

	_ = prometheus.Register(httpResponse)
	_ = prometheus.Register(goRoutines)
	_ = prometheus.Register(alloc)
	_ = prometheus.Register(totalAlloc)
	_ = prometheus.Register(numGC)
	_ = prometheus.Register(sys)
)

// StatusResponseWriter Defines own Response Writer to be used for logging of status - as http.ResponseWriter does not let us read status.
type StatusResponseWriter struct {
	http.ResponseWriter
	status int
}

// Prometheus implements mux.MiddlewareFunc.
func Prometheus(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ExemptPath(r) {
			next.ServeHTTP(w, r)

			return
		}

		start := time.Now()

		route := mux.CurrentRoute(r)
		path, _ := route.GetPathTemplate()
		path = strings.TrimSuffix(path, "/") // remove the trailing slash

		srw := &StatusResponseWriter{ResponseWriter: w}

		// this has to be called in the end so that status code is populated
		defer func(res *StatusResponseWriter, req *http.Request) {
			duration := time.Since(start)
			httpResponse.WithLabelValues(path, req.Method, fmt.Sprintf("%d", res.status)).Observe(duration.Seconds())
		}(srw, r)

		// set system stats
		PushSystemStats()

		next.ServeHTTP(srw, r)
	})
}

// ExemptPath exempts the default routes from the metrics.
func ExemptPath(r *http.Request) bool {
	return strings.HasSuffix(r.URL.Path, "/metrics") || strings.HasSuffix(r.URL.Path, "/.well-known/health") ||
		strings.HasSuffix(r.URL.Path, "/favicon.ico")
}

// systemStats is used to store the system stats and populate it on prometheus.
type systemStats struct {
	numGoRoutines float64
	alloc         float64
	totalAlloc    float64
	sys           float64
	numGC         float64
}

func getSystemStats() systemStats {
	var (
		stats systemStats
		m     runtime.MemStats
	)

	runtime.ReadMemStats(&m)

	stats.numGoRoutines = float64(runtime.NumGoroutine())
	stats.alloc = float64(m.Alloc)
	stats.totalAlloc = float64(m.TotalAlloc)
	stats.numGC = float64(m.NumGC)
	stats.sys = float64(m.Sys)

	return stats
}

// PushSystemStats push metrics for system stats.
func PushSystemStats() {
	stats := getSystemStats()

	goRoutines.WithLabelValues().Set(stats.numGoRoutines)
	alloc.WithLabelValues().Set(stats.alloc)
	totalAlloc.WithLabelValues().Set(stats.totalAlloc)
	sys.WithLabelValues().Set(stats.sys)
	numGC.WithLabelValues().Set(stats.numGC)
}
