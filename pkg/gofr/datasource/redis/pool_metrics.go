package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// poolMetricsInterval is how often the connection-pool statistics are sampled.
//
//nolint:gochecknoglobals // a variable only so tests can shorten the interval.
var poolMetricsInterval = 10 * time.Second

// Names of the pool metrics; the container registers them under the same names.
const (
	poolHitsMetric        = "app_redis_pool_hits_total"
	poolMissesMetric      = "app_redis_pool_misses_total"
	poolTimeoutsMetric    = "app_redis_pool_timeouts_total"
	poolStaleConnsMetric  = "app_redis_pool_stale_conns_total"
	poolOpenConnsMetric   = "app_redis_open_connections"
	poolIdleConnsMetric   = "app_redis_idle_connections"
	poolHostnameLabelName = "hostname"
)

// poolMetrics is the metrics recorder needed to export pool statistics. It is kept separate from
// Metrics so that existing Metrics implementations keep compiling; the sampler only runs for
// recorders that also support gauges.
type poolMetrics interface {
	Metrics
	SetGauge(name string, value float64, labels ...string)
}

// poolStatser is the part of the go-redis client that reports connection-pool statistics.
type poolStatser interface {
	PoolStats() *redis.PoolStats
}

// poolTotals holds the cumulative pool counters seen at the previous sample.
type poolTotals struct {
	hits, misses, timeouts, stale uint32
}

// pushPoolMetrics samples the connection-pool statistics every interval until stopSignal is closed.
// Hits, misses, timeouts and stale connections are cumulative in go-redis, so they are exported as
// counters by incrementing for each unit of growth since the previous sample; open and idle
// connections are point-in-time values and are exported as gauges.
func pushPoolMetrics(client poolStatser, metrics poolMetrics, hostname string, interval time.Duration, stopSignal <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Counters of this process start at zero, so the baseline is zero too: whatever the pool
	// has counted before the first sample is reported by it.
	var last poolTotals

	for {
		select {
		case <-stopSignal:
			return
		case <-ticker.C:
			last = recordPoolStats(client.PoolStats(), last, metrics, hostname)
		}
	}
}

// recordPoolStats exports stats and returns the new baseline for the next sample.
// A nil snapshot is ignored and keeps the previous baseline.
func recordPoolStats(stats *redis.PoolStats, last poolTotals, metrics poolMetrics, hostname string) poolTotals {
	if stats == nil {
		return last
	}

	labels := []string{poolHostnameLabelName, hostname}

	// uint32 subtraction keeps the delta correct when a go-redis counter wraps around.
	addToCounter(metrics, poolHitsMetric, stats.Hits-last.hits, labels)
	addToCounter(metrics, poolMissesMetric, stats.Misses-last.misses, labels)
	addToCounter(metrics, poolTimeoutsMetric, stats.Timeouts-last.timeouts, labels)
	addToCounter(metrics, poolStaleConnsMetric, stats.StaleConns-last.stale, labels)

	metrics.SetGauge(poolOpenConnsMetric, float64(stats.TotalConns), labels...)
	metrics.SetGauge(poolIdleConnsMetric, float64(stats.IdleConns), labels...)

	return poolTotals{hits: stats.Hits, misses: stats.Misses, timeouts: stats.Timeouts, stale: stats.StaleConns}
}

// addToCounter increments the counter once per unit of delta, as the metrics API cannot add an arbitrary value.
func addToCounter(metrics Metrics, name string, delta uint32, labels []string) {
	for range delta {
		metrics.IncrementCounter(context.Background(), name, labels...)
	}
}
