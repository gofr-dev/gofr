package observability

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics contains the Prometheus metrics for a cache instance.
type Metrics struct {
	hits    *prometheus.CounterVec
	misses  *prometheus.CounterVec
	sets    *prometheus.CounterVec
	deletes *prometheus.CounterVec
	evicts  *prometheus.CounterVec // To be called from within the cache logic
	items   *prometheus.GaugeVec
	latency *prometheus.HistogramVec
}

// metricsRegistry holds the one Metrics per (namespace, subsystem)
var (
    mtx       sync.Mutex
    singletons = make(map[string]*Metrics)
)

// NewMetrics returns a singleton *Metrics for this namespace+subsystem.
// The first call registers the counters/gauges; later calls just reuse them.
func NewMetrics(namespace, subsystem string) *Metrics {
    key := namespace + "/" + subsystem
    mtx.Lock()
    defer mtx.Unlock()
    if m, ok := singletons[key]; ok {
        return m
    }

    // Create and register exactly once:
    factory := promauto.With(prometheus.DefaultRegisterer)
    m := &Metrics{
        hits: factory.NewCounterVec(
            prometheus.CounterOpts{Namespace: namespace, Subsystem: subsystem, Name: "hits_total", Help: "Total number of cache hits."},
            []string{"cache_name"},
        ),
        misses: factory.NewCounterVec(
            prometheus.CounterOpts{Namespace: namespace, Subsystem: subsystem, Name: "misses_total", Help: "Total number of cache misses."},
            []string{"cache_name"},
        ),
        sets: factory.NewCounterVec(
            prometheus.CounterOpts{Namespace: namespace, Subsystem: subsystem, Name: "sets_total", Help: "Total number of set operations."},
            []string{"cache_name"},
        ),
        deletes: factory.NewCounterVec(
            prometheus.CounterOpts{Namespace: namespace, Subsystem: subsystem, Name: "deletes_total", Help: "Total number of delete operations."},
            []string{"cache_name"},
        ),
        evicts: factory.NewCounterVec(
            prometheus.CounterOpts{Namespace: namespace, Subsystem: subsystem, Name: "evictions_total", Help: "Total number of items evicted from the cache."},
            []string{"cache_name"},
        ),
        items: factory.NewGaugeVec(
            prometheus.GaugeOpts{Namespace: namespace, Subsystem: subsystem, Name: "items_current", Help: "Current number of items in the cache."},
            []string{"cache_name"},
        ),
        latency: factory.NewHistogramVec(
            prometheus.HistogramOpts{Namespace: namespace, Subsystem: subsystem, Name: "operation_latency_seconds", Help: "Latency of cache operations in seconds.", Buckets: prometheus.DefBuckets},
            []string{"cache_name", "operation"},
        ),
    }

    singletons[key] = m
    return m
}

func (m *Metrics) Hits() *prometheus.CounterVec      { return m.hits }
func (m *Metrics) Misses() *prometheus.CounterVec    { return m.misses }
func (m *Metrics) Sets() *prometheus.CounterVec      { return m.sets }
func (m *Metrics) Deletes() *prometheus.CounterVec   { return m.deletes }
func (m *Metrics) Evicts() *prometheus.CounterVec    { return m.evicts }
func (m *Metrics) Items() *prometheus.GaugeVec       { return m.items }
func (m *Metrics) Latency() *prometheus.HistogramVec { return m.latency }
