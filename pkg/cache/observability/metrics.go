package observability

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics encapsulates a set of Prometheus metrics for monitoring a cache.
// It includes counters for hits, misses, sets, deletes, and evictions,
// a gauge for the current number of items, and a histogram for operation latency.
// All metrics are labeled with 'cache_name'.
type Metrics struct {
	hits    *prometheus.CounterVec
	misses  *prometheus.CounterVec
	sets    *prometheus.CounterVec
	deletes *prometheus.CounterVec
	evicts  *prometheus.CounterVec
	items   *prometheus.GaugeVec
	latency *prometheus.HistogramVec
}

type metricsRegistry struct {
	mtx        sync.Mutex
	singletons map[string]*Metrics
}

// getRegistry returns the singleton metricsRegistry instance.
// It uses a function-scoped sync.Once and closure to avoid global variables.
func getRegistry() *metricsRegistry {
	var (
		once sync.Once
		reg  *metricsRegistry
	)

	get := func() *metricsRegistry {
		once.Do(func() {
			reg = &metricsRegistry{
				singletons: make(map[string]*Metrics),
			}
		})

		return reg
	}

	return get()
}

// NewMetrics creates or retrieves a singleton Metrics instance for a given namespace and subsystem.
// This ensures that metrics are registered with Prometheus only once per application lifecycle.
func NewMetrics(namespace, subsystem string) *Metrics {
	key := namespace + "/" + subsystem
	reg := getRegistry()

	reg.mtx.Lock()
	defer reg.mtx.Unlock()

	if m, ok := reg.singletons[key]; ok {
		return m
	}

	// Create and register exactly once.
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
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "evictions_total",
				Help:      "Total number of items evicted from the cache.",
			},
			[]string{"cache_name"},
		),
		items: factory.NewGaugeVec(
			prometheus.GaugeOpts{Namespace: namespace, Subsystem: subsystem, Name: "items_current", Help: "Current number of items in the cache."},
			[]string{"cache_name"},
		),
		latency: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "operation_latency_seconds",
				Help:      "Latency of cache operations in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"cache_name", "operation"},
		),
	}

	reg.singletons[key] = m

	return m
}

// Hits returns the counter for cache hits.
func (m *Metrics) Hits() *prometheus.CounterVec { return m.hits }

// Misses returns the counter for cache misses.
func (m *Metrics) Misses() *prometheus.CounterVec { return m.misses }

// Sets returns the counter for set operations.
func (m *Metrics) Sets() *prometheus.CounterVec { return m.sets }

// Deletes returns the counter for delete operations.
func (m *Metrics) Deletes() *prometheus.CounterVec { return m.deletes }

// Evicts returns the counter for cache evictions.
func (m *Metrics) Evicts() *prometheus.CounterVec { return m.evicts }

// Items returns the gauge for the current number of items in the cache.
func (m *Metrics) Items() *prometheus.GaugeVec { return m.items }

// Latency returns the histogram for cache operation latencies.
func (m *Metrics) Latency() *prometheus.HistogramVec { return m.latency }
