package middleware

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"gofr.dev/pkg"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/types"
)

//nolint:gochecknoglobals // metrics need to be initialized only once
var (
	httpResponse = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "gofr_http_response",
		Help:    "Histogram of HTTP response times in seconds",
		Buckets: []float64{.001, .003, .005, .01, .025, .05, .1, .2, .3, .4, .5, .75, 1, 2, 3, 5, 10, 30},
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

	ErrorTypesStats = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "gofr_server_error",
		Help: "Counter of HTTP Server Error",
	}, []string{"type", "path", "method"})

	deprecatedFeatureCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "gofr_deprecated_feature_counter",
		Help: "Counter for deprecated features",
	}, []string{"appName", "appVersion", "featureName"})

	_ = prometheus.Register(httpResponse)
	_ = prometheus.Register(goRoutines)
	_ = prometheus.Register(alloc)
	_ = prometheus.Register(totalAlloc)
	_ = prometheus.Register(sys)
	_ = prometheus.Register(numGC)
	_ = prometheus.Register(ErrorTypesStats)
	_ = prometheus.Register(deprecatedFeatureCount)
)

// PrometheusMiddleware implements mux.MiddlewareFunc.
func PrometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ExemptPath(r) {
			next.ServeHTTP(w, r)

			return
		}

		start := time.Now()

		route := mux.CurrentRoute(r)
		path, _ := route.GetPathTemplate()
		// remove the trailing slash
		path = strings.TrimSuffix(path, "/")
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

// systemStats is used to store the system stats and populate it on prometheus
type systemStats struct {
	numGoRoutines float64
	alloc         float64
	totalAlloc    float64
	sys           float64
	numGC         float64
}

func getSystemStats() systemStats {
	var (
		s systemStats
		m runtime.MemStats
	)

	runtime.ReadMemStats(&m)

	s.numGoRoutines = float64(runtime.NumGoroutine())
	s.alloc = float64(m.Alloc)
	s.totalAlloc = float64(m.TotalAlloc)
	s.numGC = float64(m.NumGC)
	s.sys = float64(m.Sys)

	return s
}

// PushSystemStats push metrics for system stats
func PushSystemStats() {
	s := getSystemStats()

	goRoutines.WithLabelValues().Set(s.numGoRoutines)
	alloc.WithLabelValues().Set(s.alloc)
	totalAlloc.WithLabelValues().Set(s.totalAlloc)
	sys.WithLabelValues().Set(s.sys)
	numGC.WithLabelValues().Set(s.numGC)
}

// PushDeprecatedFeature increments a Prometheus metric with labels
func PushDeprecatedFeature(featureName string) {
	var app types.AppDetails

	c := config.GoDotEnvProvider{}
	app.Name = c.GetOrDefault("APP_NAME", pkg.DefaultAppName)
	app.Version = c.GetOrDefault("APP_VERSION", pkg.DefaultAppVersion)

	deprecatedFeatureCount.With(prometheus.Labels{"appName": app.Name, "appVersion": app.Version, "featureName": featureName}).Inc()
}
